//go:build postgres

package auth

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

const (
	adminPassword  = "correct horse battery"
	readerPassword = "another long password"
	testAddress    = "10.0.0.1"
)

type sessionFixture struct {
	pool     *pgxpool.Pool
	sessions *Sessions
	admin    store.User
	reader   store.User
	now      time.Time
}

func (f *sessionFixture) at(d time.Duration) { f.now = base.Add(d) }

func newUser(t *testing.T, pool *pgxpool.Pool, name, password, role, language string) store.User {
	t.Helper()
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	u := store.User{
		ID: uuid.New(), Username: name, PasswordHash: hash, Role: role,
		Language: language, Active: true, CreatedAt: base, UpdatedAt: base,
	}
	if err := store.InsertUser(context.Background(), pool, u); err != nil {
		t.Fatal(err)
	}
	return u
}

func newSessionFixture(t *testing.T) *sessionFixture {
	t.Helper()
	pool := storetest.New(t)
	f := &sessionFixture{pool: pool, now: base}
	f.sessions = &Sessions{
		Pool:     pool,
		Throttle: Throttle{Secret: []byte("0123456789abcdef0123456789abcdef")},
		TTL:      12 * time.Hour,
		Idle:     time.Hour,
		Now:      func() time.Time { return f.now },
	}
	f.admin = newUser(t, pool, "admin", adminPassword, RoleAdministrator, "en")
	f.reader = newUser(t, pool, "reader", readerPassword, RoleReader, "pt-BR")
	return f
}

func (f *sessionFixture) login(t *testing.T, name, password, presented string) Issued {
	t.Helper()
	issued, err := f.sessions.Login(context.Background(), name, password, testAddress, presented)
	if err != nil {
		t.Fatal(err)
	}
	return issued
}

func (f *sessionFixture) session(t *testing.T, id uuid.UUID) store.Session {
	t.Helper()
	var s store.Session
	err := f.pool.QueryRow(context.Background(), `
SELECT id, token_hash, csrf_hash, user_id, created_at, expires_at, revoked_at, revoked_reason, rotated_from_id
  FROM sessions WHERE id = $1`, id).Scan(&s.ID, &s.TokenHash, &s.CSRFHash, &s.UserID, &s.CreatedAt,
		&s.ExpiresAt, &s.RevokedAt, &s.RevokedReason, &s.RotatedFromID)
	if err != nil {
		t.Fatal(err)
	}
	s.ExpiresAt = s.ExpiresAt.UTC()
	return s
}

func (f *sessionFixture) audits(t *testing.T, action string) []store.AuditEvent {
	t.Helper()
	all, err := store.ListAuditEvents(context.Background(), f.pool)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.AuditEvent
	for _, e := range all {
		if e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

func (f *sessionFixture) failures(t *testing.T, name string) int {
	t.Helper()
	row, err := store.GetThrottle(context.Background(), f.pool, f.sessions.Throttle.Key(name, testAddress))
	if errors.Is(err, store.ErrNotFound) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return row.Failures
}

func requireRevoked(t *testing.T, s store.Session, reason string) {
	t.Helper()
	if s.RevokedAt == nil || s.RevokedReason == nil || *s.RevokedReason != reason {
		t.Fatalf("session not revoked as %q: %+v", reason, s)
	}
}

func TestLoginIssuesSession(t *testing.T) {
	f := newSessionFixture(t)
	issued := f.login(t, "admin", adminPassword, "")
	if len(issued.Token) != 43 || issued.CSRFToken == "" {
		t.Fatalf("tokens: %q %q", issued.Token, issued.CSRFToken)
	}
	if !issued.ExpiresAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("expires at %v", issued.ExpiresAt)
	}
	want := Principal{ID: f.admin.ID, Username: "admin", Role: RoleAdministrator, Language: "en"}
	if issued.Principal != want {
		t.Fatalf("principal %+v", issued.Principal)
	}
	s := f.session(t, issued.SessionID)
	if string(s.TokenHash) != string(Digest(issued.Token)) || string(s.CSRFHash) != string(Digest(issued.CSRFToken)) {
		t.Fatal("digests differ from the tokens")
	}
	if s.UserID != f.admin.ID || s.RevokedAt != nil || !s.ExpiresAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("row %+v", s)
	}
	rows := f.audits(t, audit.ActionLoginSucceeded)
	if len(rows) != 1 {
		t.Fatalf("%d succeeded rows", len(rows))
	}
	r := rows[0]
	if r.ActorID == nil || *r.ActorID != f.admin.ID || r.TargetType == nil || *r.TargetType != "session" ||
		r.TargetID == nil || *r.TargetID != issued.SessionID.String() || r.Outcome != audit.OutcomeSucceeded {
		t.Fatalf("audit row %+v", r)
	}
}

func requireInvalid(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("got %v", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	f := newSessionFixture(t)
	_, err := f.sessions.Login(context.Background(), "admin", "wrong password here", testAddress, "")
	requireInvalid(t, err)
	rows := f.audits(t, audit.ActionLoginFailed)
	if len(rows) != 1 {
		t.Fatalf("%d failed rows", len(rows))
	}
	r := rows[0]
	if r.ActorID != nil || r.Outcome != audit.OutcomeFailed || r.TargetType == nil || *r.TargetType != "user" ||
		r.TargetID == nil || *r.TargetID != f.admin.ID.String() {
		t.Fatalf("audit row %+v", r)
	}
	if !strings.Contains(string(r.Metadata), "INVALID_CREDENTIALS") {
		t.Fatalf("metadata %s", r.Metadata)
	}
	if n := f.failures(t, "admin"); n != 1 {
		t.Fatalf("%d failures", n)
	}
}

func TestLoginUnknownUser(t *testing.T) {
	f := newSessionFixture(t)
	_, err := f.sessions.Login(context.Background(), "nobody", adminPassword, testAddress, "")
	requireInvalid(t, err)
	rows := f.audits(t, audit.ActionLoginFailed)
	if len(rows) != 1 || rows[0].TargetType != nil || rows[0].TargetID != nil || rows[0].ActorID != nil {
		t.Fatalf("audit rows %+v", rows)
	}
	if n := f.failures(t, "nobody"); n != 1 {
		t.Fatalf("%d failures", n)
	}
}

func TestLoginInactiveUserLooksUnknown(t *testing.T) {
	f := newSessionFixture(t)
	f.reader.Active = false
	if err := store.UpdateUserFields(context.Background(), f.pool, f.reader.ID, f.reader.Role, f.reader.Language, f.reader.Active, base); err != nil {
		t.Fatal(err)
	}
	_, err := f.sessions.Login(context.Background(), "reader", readerPassword, testAddress, "")
	requireInvalid(t, err)
	rows := f.audits(t, audit.ActionLoginFailed)
	if len(rows) != 1 || rows[0].TargetType != nil || rows[0].TargetID != nil {
		t.Fatalf("audit rows %+v", rows)
	}
}

func TestLoginNameOutsidePatternSkipsLookup(t *testing.T) {
	f := newSessionFixture(t)
	name := "a\x00b"
	_, err := f.sessions.Login(context.Background(), name, adminPassword, testAddress, "")
	requireInvalid(t, err)
	if n := f.failures(t, name); n != 1 {
		t.Fatalf("%d failures", n)
	}
}

func TestLoginThrottledWritesNoAudit(t *testing.T) {
	f := newSessionFixture(t)
	for i := 0; i < 5; i++ {
		_, err := f.sessions.Login(context.Background(), "admin", "wrong password here", testAddress, "")
		requireInvalid(t, err)
	}
	_, err := f.sessions.Login(context.Background(), "admin", adminPassword, testAddress, "")
	var te *ErrThrottled
	if !errors.As(err, &te) || te.RetryAfter != 300 {
		t.Fatalf("got %v", err)
	}
	if n := len(f.audits(t, audit.ActionLoginFailed)); n != 5 {
		t.Fatalf("%d failed rows", n)
	}
	if n := len(f.audits(t, audit.ActionLoginSucceeded)); n != 0 {
		t.Fatalf("%d succeeded rows", n)
	}
}

func TestLoginRotatesPresentedSession(t *testing.T) {
	f := newSessionFixture(t)
	first := f.login(t, "admin", adminPassword, "")
	second := f.login(t, "admin", adminPassword, first.Token)
	requireRevoked(t, f.session(t, first.SessionID), "rotated")
	s := f.session(t, second.SessionID)
	if s.RotatedFromID == nil || *s.RotatedFromID != first.SessionID || s.RevokedAt != nil {
		t.Fatalf("second row %+v", s)
	}
}

func TestLoginDoesNotChainAcrossAccounts(t *testing.T) {
	f := newSessionFixture(t)
	a := f.login(t, "admin", adminPassword, "")
	b := f.login(t, "reader", readerPassword, a.Token)
	requireRevoked(t, f.session(t, a.SessionID), "rotated")
	if s := f.session(t, b.SessionID); s.RotatedFromID != nil {
		t.Fatalf("chained to %v", s.RotatedFromID)
	}
}

func TestLoginRehashesOutdatedHash(t *testing.T) {
	f := newSessionFixture(t)
	old, err := argon2id.CreateHash(adminPassword, &argon2id.Params{Memory: 65536, Iterations: 1, Parallelism: 4, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, f.admin.ID, old); err != nil {
		t.Fatal(err)
	}
	f.login(t, "admin", adminPassword, "")
	u, err := store.GetUser(context.Background(), f.pool, f.admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(u.PasswordHash, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("hash %q", u.PasswordHash)
	}
	if u.PasswordHash == old {
		t.Fatal("hash unchanged")
	}
}

func TestLoginRehashKeepsTheSession(t *testing.T) {
	f := newSessionFixture(t)
	old, err := argon2id.CreateHash(adminPassword, &argon2id.Params{Memory: 65536, Iterations: 1, Parallelism: 4, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, f.admin.ID, old); err != nil {
		t.Fatal(err)
	}
	issued := f.login(t, "admin", adminPassword, "")
	if s := f.session(t, issued.SessionID); s.RevokedAt != nil {
		t.Fatalf("session revoked: %+v", s)
	}
	if _, err := f.sessions.Authenticate(context.Background(), issued.Token); err != nil {
		t.Fatal(err)
	}
	if n := len(f.audits(t, audit.ActionLoginSucceeded)); n != 1 {
		t.Fatalf("%d succeeded rows", n)
	}
}

func TestLoginRefusesWhenPasswordChangedMidFlight(t *testing.T) {
	f := newSessionFixture(t)
	f.sessions.beforeSession = func() {
		hash, err := HashPassword("a password set meanwhile")
		if err != nil {
			t.Error(err)
			return
		}
		if _, err := f.pool.Exec(context.Background(), `UPDATE users SET password_hash = $2 WHERE id = $1`, f.admin.ID, hash); err != nil {
			t.Error(err)
		}
	}
	_, err := f.sessions.Login(context.Background(), "admin", adminPassword, testAddress, "")
	requireInvalid(t, err)
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d sessions", n)
	}
}

func TestLoginRefusesWhenUserDeactivatedMidFlight(t *testing.T) {
	f := newSessionFixture(t)
	f.sessions.beforeSession = func() {
		if _, err := f.pool.Exec(context.Background(), `UPDATE users SET active = false WHERE id = $1`, f.admin.ID); err != nil {
			t.Error(err)
		}
	}
	_, err := f.sessions.Login(context.Background(), "admin", adminPassword, testAddress, "")
	requireInvalid(t, err)
	var n int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM sessions`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("%d sessions", n)
	}
}

func TestLoginNormalisesUsername(t *testing.T) {
	f := newSessionFixture(t)
	issued := f.login(t, "  ADMIN ", adminPassword, "")
	if issued.Principal.Username != "admin" {
		t.Fatalf("principal %+v", issued.Principal)
	}
}

func TestAuthenticateSlides(t *testing.T) {
	f := newSessionFixture(t)
	issued := f.login(t, "admin", adminPassword, "")
	f.at(30 * time.Minute)
	got, err := f.sessions.Authenticate(context.Background(), issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Equal(base.Add(90*time.Minute)) || got.Principal != issued.Principal || got.SessionID != issued.SessionID {
		t.Fatalf("got %+v", got)
	}
}

func TestAuthenticateExpired(t *testing.T) {
	f := newSessionFixture(t)
	issued := f.login(t, "admin", adminPassword, "")
	f.at(61 * time.Minute)
	if _, err := f.sessions.Authenticate(context.Background(), issued.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

func TestAuthenticateRevokesSessionOfInactiveUser(t *testing.T) {
	f := newSessionFixture(t)
	issued := f.login(t, "admin", adminPassword, "")
	f.admin.Active = false
	if err := store.UpdateUserFields(context.Background(), f.pool, f.admin.ID, f.admin.Role, f.admin.Language, f.admin.Active, base); err != nil {
		t.Fatal(err)
	}
	if _, err := f.sessions.Authenticate(context.Background(), issued.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
	requireRevoked(t, f.session(t, issued.SessionID), "user_inactive")
}

func TestAuthenticateReadsRoleFromDatabase(t *testing.T) {
	f := newSessionFixture(t)
	issued := f.login(t, "admin", adminPassword, "")
	f.admin.Role = RoleReader
	if err := store.UpdateUserFields(context.Background(), f.pool, f.admin.ID, f.admin.Role, f.admin.Language, f.admin.Active, base); err != nil {
		t.Fatal(err)
	}
	got, err := f.sessions.Authenticate(context.Background(), issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if got.Principal.Role != RoleReader {
		t.Fatalf("role %q", got.Principal.Role)
	}
}

func TestVerifyCSRF(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	issued := f.login(t, "admin", adminPassword, "")
	if err := f.sessions.VerifyCSRF(ctx, issued.SessionID, issued.CSRFToken); err != nil {
		t.Fatal(err)
	}
	if err := f.sessions.VerifyCSRF(ctx, issued.SessionID, "wrong"); !errors.Is(err, ErrCSRF) {
		t.Fatalf("wrong: %v", err)
	}
	if err := f.sessions.VerifyCSRF(ctx, issued.SessionID, ""); !errors.Is(err, ErrCSRF) {
		t.Fatalf("empty: %v", err)
	}
	if err := f.sessions.Logout(ctx, issued.Token); err != nil {
		t.Fatal(err)
	}
	if err := f.sessions.VerifyCSRF(ctx, issued.SessionID, issued.CSRFToken); !errors.Is(err, ErrCSRF) {
		t.Fatalf("revoked: %v", err)
	}
}

func TestRotateCSRF(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	issued := f.login(t, "admin", adminPassword, "")
	fresh, err := f.sessions.RotateCSRF(ctx, issued.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh == issued.CSRFToken {
		t.Fatal("token not renewed")
	}
	if err := f.sessions.VerifyCSRF(ctx, issued.SessionID, fresh); err != nil {
		t.Fatalf("new token: %v", err)
	}
	if err := f.sessions.VerifyCSRF(ctx, issued.SessionID, issued.CSRFToken); !errors.Is(err, ErrCSRF) {
		t.Fatalf("old token: %v", err)
	}
}

func TestLogout(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	issued := f.login(t, "admin", adminPassword, "")
	f.at(time.Minute)
	if err := f.sessions.Logout(ctx, issued.Token); err != nil {
		t.Fatal(err)
	}
	requireRevoked(t, f.session(t, issued.SessionID), "logout")
	rows := f.audits(t, audit.ActionLogout)
	if len(rows) != 1 || rows[0].ActorID == nil || *rows[0].ActorID != f.admin.ID ||
		rows[0].TargetType == nil || *rows[0].TargetType != "session" ||
		rows[0].TargetID == nil || *rows[0].TargetID != issued.SessionID.String() {
		t.Fatalf("audit rows %+v", rows)
	}
	if _, err := f.sessions.Authenticate(ctx, issued.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("got %v", err)
	}
}

func TestLogoutUnknownToken(t *testing.T) {
	f := newSessionFixture(t)
	token, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.sessions.Logout(context.Background(), token); err != nil {
		t.Fatal(err)
	}
	if n := len(f.audits(t, audit.ActionLogout)); n != 0 {
		t.Fatalf("%d logout rows", n)
	}
}

func TestLoginHoldsNoConnectionDuringTheHash(t *testing.T) {
	f := newSessionFixture(t)
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(os.Getenv("KCR_TEST_DATABASE_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Database = f.pool.Config().ConnConfig.Database
	cfg.MaxConns = 1
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sessions := *f.sessions
	sessions.Pool = pool

	// Every hash slot is held, so the sign-in waits at its hash.
	for range cap(hashSlots) {
		hashSlots <- struct{}{}
	}
	released := false
	release := func() {
		if !released {
			released = true
			for range cap(hashSlots) {
				<-hashSlots
			}
		}
	}
	defer release()
	done := make(chan error, 1)
	go func() {
		_, err := sessions.Login(ctx, "admin", "wrong password here", testAddress, "")
		done <- err
	}()
	// The throttle and the read of the user come before the hash.
	for pool.Stat().AcquireCount() < 2 {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)

	probe, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var one int
	if err := pool.QueryRow(probe, "SELECT 1").Scan(&one); err != nil {
		t.Fatalf("the pool is held during the hash: %v", err)
	}
	release()
	requireInvalid(t, <-done)
}

func TestLoginSessionWaitsForAPasswordChange(t *testing.T) {
	f := newSessionFixture(t)
	account := &Account{Pool: f.pool, Sessions: f.sessions, Now: f.sessions.Now}
	changed := make(chan error, 1)
	f.sessions.afterUserRead = func() {
		go func() {
			_, err := account.Change(context.Background(), principalOf(f.admin), testAddress, AccountChange{
				CurrentPassword: str(adminPassword),
				Password:        str(newPassword),
			})
			changed <- err
		}()
		// Gives the change time to reach its write while the session
		// transaction is still open.
		time.Sleep(300 * time.Millisecond)
	}
	issued, loginErr := f.sessions.Login(context.Background(), "admin", adminPassword, testAddress, "")
	if err := <-changed; err != nil {
		t.Fatal(err)
	}
	if loginErr != nil {
		requireInvalid(t, loginErr)
	}
	if loginErr == nil {
		_, err := store.FindActiveSessionByTokenHash(context.Background(), f.pool, Digest(issued.Token), f.now)
		if !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("a session of the old password is live: %v", err)
		}
	}
	var live int
	err := f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM sessions WHERE user_id = $1 AND revoked_at IS NULL`, f.admin.ID).Scan(&live)
	if err != nil {
		t.Fatal(err)
	}
	if live != 0 {
		t.Fatalf("%d live sessions after the password change", live)
	}
}
