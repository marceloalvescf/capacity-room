//go:build postgres

package auth

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/audit"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

const newPassword = "a brand new password"

type userFixture struct {
	pool  *pgxpool.Pool
	users *Users
	root  store.User
	ops   store.User
	eve   store.User
}

func newUserFixture(t *testing.T) *userFixture {
	t.Helper()
	pool := storetest.New(t)
	f := &userFixture{pool: pool}
	f.users = &Users{Pool: pool, Now: func() time.Time { return base.Add(time.Minute) }}
	f.root = newUser(t, pool, "root", adminPassword, RoleAdministrator, "en")
	f.ops = newUser(t, pool, "ops", readerPassword, RoleOperator, "en")
	f.eve = newUser(t, pool, "eve", readerPassword, RoleReader, "pt-BR")
	return f
}

func (f *userFixture) actor(u store.User) Principal { return principalOf(u) }

func (f *userFixture) addSession(t *testing.T, u store.User) uuid.UUID {
	t.Helper()
	id := uuid.New()
	err := store.InsertSession(context.Background(), f.pool, store.Session{
		ID: id, TokenHash: Digest(id.String()), CSRFHash: Digest("csrf" + id.String()),
		UserID: u.ID, CreatedAt: base, ExpiresAt: base.Add(time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *userFixture) session(t *testing.T, id uuid.UUID) store.Session {
	t.Helper()
	var s store.Session
	err := f.pool.QueryRow(context.Background(),
		`SELECT revoked_at, revoked_reason FROM sessions WHERE id = $1`, id).Scan(&s.RevokedAt, &s.RevokedReason)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func (f *userFixture) user(t *testing.T, id uuid.UUID) store.User {
	t.Helper()
	u, err := store.GetUser(context.Background(), f.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func (f *userFixture) audits(t *testing.T, action string) []store.AuditEvent {
	t.Helper()
	all, err := store.ListAuditEvents(context.Background(), f.pool)
	if err != nil {
		t.Fatal(err)
	}
	var out []store.AuditEvent
	for _, e := range all {
		if action == "" || e.Action == action {
			out = append(out, e)
		}
	}
	return out
}

func (f *userFixture) requireNoAudit(t *testing.T) {
	t.Helper()
	if got := f.audits(t, ""); len(got) != 0 {
		t.Fatalf("audit rows written: %+v", got)
	}
}

func requireMetadata(t *testing.T, e store.AuditEvent, want string) {
	t.Helper()
	var got, exp any
	if err := json.Unmarshal(e.Metadata, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &exp); err != nil {
		t.Fatal(err)
	}
	g, _ := json.Marshal(got)
	x, _ := json.Marshal(exp)
	if string(g) != string(x) {
		t.Fatalf("metadata %s, want %s", e.Metadata, want)
	}
}

func str(s string) *string { return &s }
func flag(b bool) *bool    { return &b }

func TestCreateUser(t *testing.T) {
	f := newUserFixture(t)
	got, err := f.users.Create(context.Background(), f.actor(f.root),
		NewUser{Username: "Kim", Password: "twelve letters", Role: RoleOperator, Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	row := f.user(t, got.ID)
	if row.Username != "kim" || row.Role != RoleOperator || row.Language != "en" || !row.Active {
		t.Fatalf("row %+v", row)
	}
	if !strings.HasPrefix(row.PasswordHash, "$argon2id$") {
		t.Fatalf("hash %q", row.PasswordHash)
	}
	if match, _, _ := VerifyPassword(context.Background(), "twelve letters", row.PasswordHash); !match {
		t.Fatal("hash does not match the password")
	}
	if !row.CreatedAt.Equal(base.Add(time.Minute)) || !row.UpdatedAt.Equal(base.Add(time.Minute)) {
		t.Fatalf("times %v %v", row.CreatedAt, row.UpdatedAt)
	}
	events := f.audits(t, audit.ActionUserCreate)
	if len(events) != 1 {
		t.Fatalf("events %+v", events)
	}
	e := events[0]
	if e.ActorID == nil || *e.ActorID != f.root.ID || e.TargetType == nil || *e.TargetType != "user" ||
		e.TargetID == nil || *e.TargetID != got.ID.String() {
		t.Fatalf("event %+v", e)
	}
	requireMetadata(t, e, `{"role":"operator"}`)
}

func TestCreateRefusesNonAdministrator(t *testing.T) {
	f := newUserFixture(t)
	_, err := f.users.Create(context.Background(), f.actor(f.ops),
		NewUser{Username: "kim", Password: "twelve letters", Role: RoleReader, Language: "en"})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err %v", err)
	}
	all, err := store.ListUsers(context.Background(), f.pool)
	if err != nil || len(all) != 3 {
		t.Fatalf("users %v %v", all, err)
	}
}

func TestCreateValidatesInOrder(t *testing.T) {
	f := newUserFixture(t)
	cases := []struct {
		name string
		n    NewUser
		want error
	}{
		{"bad name and short password", NewUser{Username: "Bad Name!", Password: "short", Role: RoleReader, Language: "en"}, ErrInvalidUsername},
		{"short password", NewUser{Username: "kim", Password: "short", Role: RoleReader, Language: "en"}, ErrPasswordTooShort},
		{"role", NewUser{Username: "kim", Password: "twelve letters", Role: "boss", Language: "en"}, ErrInvalidRole},
		{"language", NewUser{Username: "kim", Password: "twelve letters", Role: RoleReader, Language: "fr"}, ErrInvalidLanguage},
	}
	for _, c := range cases {
		if _, err := f.users.Create(context.Background(), f.actor(f.root), c.n); !errors.Is(err, c.want) {
			t.Fatalf("%s: err %v, want %v", c.name, err, c.want)
		}
	}
	all, err := store.ListUsers(context.Background(), f.pool)
	if err != nil || len(all) != 3 {
		t.Fatalf("users %v %v", all, err)
	}
	f.requireNoAudit(t)
}

func TestCreateRefusesTakenName(t *testing.T) {
	f := newUserFixture(t)
	n := NewUser{Username: "kim", Password: "twelve letters", Role: RoleReader, Language: "en"}
	if _, err := f.users.Create(context.Background(), f.actor(f.root), n); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kim", "KIM"} {
		n.Username = name
		if _, err := f.users.Create(context.Background(), f.actor(f.root), n); !errors.Is(err, ErrUsernameTaken) {
			t.Fatalf("%s: err %v", name, err)
		}
	}
	if got := f.audits(t, audit.ActionUserCreate); len(got) != 1 {
		t.Fatalf("audit rows %d", len(got))
	}
}

func TestUpdateRoleRevokesSessions(t *testing.T) {
	f := newUserFixture(t)
	s1, s2 := f.addSession(t, f.ops), f.addSession(t, f.ops)
	got, err := f.users.Update(context.Background(), f.actor(f.root), f.ops.ID, UserChange{Role: str(RoleReader)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Role != RoleReader || f.user(t, f.ops.ID).Role != RoleReader {
		t.Fatalf("row %+v", got)
	}
	requireRevoked(t, f.session(t, s1), "privilege_changed")
	requireRevoked(t, f.session(t, s2), "privilege_changed")
	events := f.audits(t, audit.ActionUserUpdate)
	if len(events) != 1 || *events[0].TargetID != f.ops.ID.String() || *events[0].ActorID != f.root.ID {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"changed":{"role":"reader"},"sessions_revoked":2}`)
}

func TestUpdateActiveFalseRevokes(t *testing.T) {
	f := newUserFixture(t)
	s := f.addSession(t, f.ops)
	got, err := f.users.Update(context.Background(), f.actor(f.root), f.ops.ID, UserChange{Active: flag(false)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Active || f.user(t, f.ops.ID).Active {
		t.Fatalf("row %+v", got)
	}
	requireRevoked(t, f.session(t, s), "privilege_changed")
	events := f.audits(t, audit.ActionUserUpdate)
	if len(events) != 1 {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"changed":{"active":false},"sessions_revoked":1}`)
}

func TestUpdatePasswordRevokes(t *testing.T) {
	f := newUserFixture(t)
	s := f.addSession(t, f.eve)
	got, err := f.users.Update(context.Background(), f.actor(f.root), f.eve.ID, UserChange{Password: str(newPassword)})
	if err != nil {
		t.Fatal(err)
	}
	row := f.user(t, f.eve.ID)
	if row.PasswordHash == f.eve.PasswordHash || got.PasswordHash != row.PasswordHash {
		t.Fatal("hash not replaced")
	}
	if match, _, _ := VerifyPassword(context.Background(), newPassword, row.PasswordHash); !match {
		t.Fatal("new hash does not match")
	}
	requireRevoked(t, f.session(t, s), "password_changed")
	events := f.audits(t, audit.ActionUserPasswordChanged)
	if len(events) != 1 || *events[0].TargetID != f.eve.ID.String() {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"sessions_revoked":1}`)
	if update := f.audits(t, audit.ActionUserUpdate); len(update) != 0 {
		t.Fatalf("update events %+v", update)
	}
	for _, e := range f.audits(t, "") {
		if strings.Contains(string(e.Metadata), newPassword) {
			t.Fatalf("password in audit: %s", e.Metadata)
		}
	}
}

func TestUpdateLanguageOnly(t *testing.T) {
	f := newUserFixture(t)
	s := f.addSession(t, f.eve)
	if _, err := f.users.Update(context.Background(), f.actor(f.root), f.eve.ID, UserChange{Language: str("en")}); err != nil {
		t.Fatal(err)
	}
	if f.user(t, f.eve.ID).Language != "en" {
		t.Fatal("language not set")
	}
	if got := f.session(t, s); got.RevokedAt != nil {
		t.Fatalf("session revoked: %+v", got)
	}
	events := f.audits(t, audit.ActionUserUpdate)
	if len(events) != 1 {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"changed":{"language":"en"},"sessions_revoked":0}`)
}

func TestUpdateNoOpWritesNothing(t *testing.T) {
	f := newUserFixture(t)
	got, err := f.users.Update(context.Background(), f.actor(f.root), f.ops.ID,
		UserChange{Role: str(RoleOperator), Active: flag(true), Language: str("en")})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != f.ops.ID || !got.UpdatedAt.Equal(base) || !f.user(t, f.ops.ID).UpdatedAt.Equal(base) {
		t.Fatalf("row %+v", got)
	}
	f.requireNoAudit(t)
}

func TestUpdateSelfChecks(t *testing.T) {
	f := newUserFixture(t)
	ctx, actor := context.Background(), f.actor(f.root)
	if _, err := f.users.Update(ctx, actor, f.root.ID, UserChange{Active: flag(false)}); !errors.Is(err, ErrSelfDeactivate) {
		t.Fatalf("err %v", err)
	}
	if _, err := f.users.Update(ctx, actor, f.root.ID, UserChange{Role: str(RoleOperator)}); !errors.Is(err, ErrSelfDemote) {
		t.Fatalf("err %v", err)
	}
	f.requireNoAudit(t)
}

func TestUpdateRefusesLastAdministratorTransitions(t *testing.T) {
	f := newUserFixture(t)
	ctx := context.Background()
	two := newUser(t, f.pool, "two", adminPassword, RoleAdministrator, "en")
	if err := store.UpdateUserFields(ctx, f.pool, two.ID, two.Role, two.Language, two.LanguageFollowsBrowser, false, base); err != nil {
		t.Fatal(err)
	}
	// An inactive administrator acts here, so root is the last active one.
	inactive := f.actor(two)
	if _, err := f.users.Update(ctx, inactive, f.root.ID, UserChange{Active: flag(false)}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("deactivate: err %v", err)
	}
	if _, err := f.users.Update(ctx, inactive, f.root.ID, UserChange{Role: str(RoleReader)}); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("demote: err %v", err)
	}
	if !f.user(t, f.root.ID).Active {
		t.Fatal("root changed")
	}
	if err := store.UpdateUserFields(ctx, f.pool, two.ID, two.Role, two.Language, two.LanguageFollowsBrowser, true, base); err != nil {
		t.Fatal(err)
	}
	got, err := f.users.Update(ctx, f.actor(two), f.root.ID, UserChange{Role: str(RoleReader)})
	if err != nil || got.Role != RoleReader {
		t.Fatalf("demote with two administrators: %+v %v", got, err)
	}
}

func TestUpdateUnknownUser(t *testing.T) {
	f := newUserFixture(t)
	_, err := f.users.Update(context.Background(), f.actor(f.root), uuid.New(), UserChange{Language: str("en")})
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("err %v", err)
	}
}

func TestUpdateShortPasswordCostsNoHash(t *testing.T) {
	f := newUserFixture(t)
	_, err := f.users.Update(context.Background(), f.actor(f.root), f.eve.ID, UserChange{Password: str("abc")})
	if !errors.Is(err, ErrPasswordTooShort) {
		t.Fatalf("err %v", err)
	}
	if f.user(t, f.eve.ID).PasswordHash != f.eve.PasswordHash {
		t.Fatal("hash changed")
	}
	f.requireNoAudit(t)
}

func TestUpdateRefusesNonAdministrator(t *testing.T) {
	f := newUserFixture(t)
	_, err := f.users.Update(context.Background(), f.actor(f.ops), f.eve.ID, UserChange{Language: str("en")})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("err %v", err)
	}
}

func TestDeleteDeactivates(t *testing.T) {
	f := newUserFixture(t)
	s := f.addSession(t, f.eve)
	if err := f.users.Delete(context.Background(), f.actor(f.root), f.eve.ID); err != nil {
		t.Fatal(err)
	}
	if f.user(t, f.eve.ID).Active {
		t.Fatal("still active")
	}
	requireRevoked(t, f.session(t, s), "user_deleted")
	events := f.audits(t, audit.ActionUserDelete)
	if len(events) != 1 || *events[0].TargetID != f.eve.ID.String() {
		t.Fatalf("events %+v", events)
	}
	requireMetadata(t, events[0], `{"sessions_revoked":1}`)
	if err := f.users.Delete(context.Background(), f.actor(f.root), f.eve.ID); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDeleteSelf(t *testing.T) {
	f := newUserFixture(t)
	if err := f.users.Delete(context.Background(), f.actor(f.root), f.root.ID); !errors.Is(err, ErrSelfDelete) {
		t.Fatalf("err %v", err)
	}
}

func TestDeleteLastAdministrator(t *testing.T) {
	f := newUserFixture(t)
	ctx := context.Background()
	two := newUser(t, f.pool, "two", adminPassword, RoleAdministrator, "en")
	deactivate := func(active bool) {
		t.Helper()
		if err := store.UpdateUserFields(ctx, f.pool, two.ID, two.Role, two.Language, two.LanguageFollowsBrowser, active, base); err != nil {
			t.Fatal(err)
		}
	}
	deactivate(false)
	if err := f.users.Delete(ctx, f.actor(two), f.root.ID); !errors.Is(err, ErrLastAdmin) {
		t.Fatalf("err %v", err)
	}
	deactivate(true)
	if err := f.users.Delete(ctx, f.actor(two), f.root.ID); err != nil {
		t.Fatal(err)
	}
	if f.user(t, f.root.ID).Active {
		t.Fatal("root still active")
	}
}

func TestListAndGet(t *testing.T) {
	f := newUserFixture(t)
	ctx := context.Background()
	if _, err := f.users.List(ctx, f.actor(f.ops)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err %v", err)
	}
	if _, err := f.users.Get(ctx, f.actor(f.ops), f.eve.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("err %v", err)
	}
	all, err := f.users.List(ctx, f.actor(f.root))
	if err != nil || len(all) != 3 || all[0].Username != "eve" || all[1].Username != "ops" || all[2].Username != "root" {
		t.Fatalf("list %+v %v", all, err)
	}
	got, err := f.users.Get(ctx, f.actor(f.root), f.eve.ID)
	if err != nil || got.ID != f.eve.ID || got.Username != "eve" {
		t.Fatalf("get %+v %v", got, err)
	}
	if _, err := f.users.Get(ctx, f.actor(f.root), uuid.New()); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("err %v", err)
	}
}

func TestUpdateMutualDemotionKeepsOneAdministrator(t *testing.T) {
	f := newUserFixture(t)
	ctx := context.Background()
	two := newUser(t, f.pool, "two", adminPassword, RoleAdministrator, "en")
	actors := []store.User{f.root, two}
	targets := []store.User{two, f.root}
	errs := make([]error, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range actors {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, errs[i] = f.users.Update(ctx, f.actor(actors[i]), targets[i].ID, UserChange{Role: str(RoleReader)})
		}()
	}
	close(start)
	wg.Wait()
	succeeded, refused := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrLastAdmin):
			refused++
		default:
			t.Fatalf("err %v", err)
		}
	}
	if succeeded != 1 || refused != 1 {
		t.Fatalf("results %v", errs)
	}
	n, err := store.CountActiveAdministrators(ctx, f.pool)
	if err != nil || n != 1 {
		t.Fatalf("administrators %d %v", n, err)
	}
}

func TestUpdateKeepsAConcurrentPasswordChange(t *testing.T) {
	f := newAccountFixture(t)
	changed := make(chan error, 1)
	f.users.beforeWrite = func() {
		go func() {
			_, err := f.account.Change(context.Background(), f.actor(f.eve), testAddress, AccountChange{
				CurrentPassword: str(readerPassword),
				Password:        str(newPassword),
			})
			changed <- err
		}()
		// Gives the change time to reach its write while the update
		// transaction is still open.
		time.Sleep(300 * time.Millisecond)
	}
	if _, err := f.users.Update(context.Background(), f.actor(f.root), f.eve.ID, UserChange{Role: str(RoleOperator)}); err != nil {
		t.Fatal(err)
	}
	if err := <-changed; err != nil {
		t.Fatal(err)
	}
	row := f.user(t, f.eve.ID)
	if row.Role != RoleOperator {
		t.Fatalf("role %q", row.Role)
	}
	if match, _, _ := VerifyPassword(context.Background(), newPassword, row.PasswordHash); !match {
		t.Fatal("the password change was reverted")
	}
}

func TestCreateUserWithoutLanguageFollowsTheBrowser(t *testing.T) {
	f := newUserFixture(t)
	got, err := f.users.Create(context.Background(), f.actor(f.root),
		NewUser{Username: "Kim", Password: "twelve letters", Role: RoleOperator})
	if err != nil {
		t.Fatal(err)
	}
	if row := f.user(t, got.ID); row.Language != "en" || !row.LanguageFollowsBrowser {
		t.Fatalf("row %+v", row)
	}
	got, err = f.users.Create(context.Background(), f.actor(f.root),
		NewUser{Username: "Lee", Password: "twelve letters", Role: RoleOperator, Language: "pt-BR"})
	if err != nil {
		t.Fatal(err)
	}
	if row := f.user(t, got.ID); row.Language != "pt-BR" || row.LanguageFollowsBrowser {
		t.Fatalf("row %+v", row)
	}
}

func TestUpdateUnpinsTheLanguage(t *testing.T) {
	f := newUserFixture(t)
	ctx := context.Background()
	if _, err := f.users.Update(ctx, f.actor(f.root), f.ops.ID, UserChange{LanguageFollowsBrowser: flag(true)}); err != nil {
		t.Fatal(err)
	}
	row := f.user(t, f.ops.ID)
	if !row.LanguageFollowsBrowser || row.Language != f.ops.Language {
		t.Fatalf("row %+v", row)
	}
	events := f.audits(t, audit.ActionUserUpdate)
	last := events[len(events)-1]
	requireMetadata(t, last, `{"changed":{"language_follows_browser":true},"sessions_revoked":0}`)

	if _, err := f.users.Update(ctx, f.actor(f.root), f.ops.ID, UserChange{Language: str("en")}); err != nil {
		t.Fatal(err)
	}
	if row := f.user(t, f.ops.ID); row.LanguageFollowsBrowser || row.Language != "en" {
		t.Fatalf("pinned row %+v", row)
	}
}
