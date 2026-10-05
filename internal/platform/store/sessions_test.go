//go:build postgres

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func newSession(userID uuid.UUID, hash string, expires time.Time) store.Session {
	return store.Session{
		ID:        uuid.New(),
		TokenHash: []byte(hash),
		CSRFHash:  []byte("csrf-" + hash),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: expires,
	}
}

func mustInsertSession(t *testing.T, q store.Querier, s store.Session) {
	t.Helper()
	if err := store.InsertSession(context.Background(), q, s); err != nil {
		t.Fatalf("insert session: %v", err)
	}
}

func userWithSession(t *testing.T, q store.Querier, name, hash string, expires time.Time) (store.User, store.Session) {
	t.Helper()
	u := newUser(name, "reader", true)
	mustInsertUser(t, q, u)
	s := newSession(u.ID, hash, expires)
	mustInsertSession(t, q, s)
	return u, s
}

func TestInsertAndFindSession(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u, s := userWithSession(t, pool, "amy", "tok", now.Add(time.Hour))

	got, err := store.FindActiveSessionByTokenHash(ctx, pool, []byte("tok"), now)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != s.ID || got.UserID != u.ID || string(got.CSRFHash) != "csrf-tok" ||
		!got.ExpiresAt.Equal(time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)) ||
		got.RevokedAt != nil || got.RevokedReason != nil || got.RotatedFromID != nil {
		t.Fatalf("got %+v", got)
	}
	byID, err := store.FindActiveSessionByID(ctx, pool, s.ID, now)
	if err != nil || byID.ID != s.ID {
		t.Fatalf("by id: %+v, %v", byID, err)
	}
	if _, err := store.FindActiveSessionByTokenHash(ctx, pool, []byte("tok"), now.Add(2*time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired: got %v", err)
	}
	if _, err := store.FindActiveSessionByID(ctx, pool, s.ID, now.Add(2*time.Hour)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired by id: got %v", err)
	}
	if _, err := store.FindActiveSessionByTokenHash(ctx, pool, []byte("wrong"), now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("wrong hash: got %v", err)
	}
}

func TestTouchSessionSlides(t *testing.T) {
	pool := storetest.New(t)
	userWithSession(t, pool, "amy", "tok", now.Add(time.Hour))
	got, err := store.TouchSession(context.Background(), pool, []byte("tok"), now.Add(30*time.Minute), 12*time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 5, 13, 30, 0, 0, time.UTC); !got.ExpiresAt.Equal(want) {
		t.Fatalf("expires %v, want %v", got.ExpiresAt, want)
	}
}

func TestTouchSessionCapsAtAbsoluteLifetime(t *testing.T) {
	pool := storetest.New(t)
	userWithSession(t, pool, "amy", "tok", now.Add(12*time.Hour))
	// now+11h30m plus the idle hour would pass the cap of created_at + 12h.
	got, err := store.TouchSession(context.Background(), pool, []byte("tok"), now.Add(11*time.Hour+30*time.Minute), 12*time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 6, 0, 0, 0, 0, time.UTC); !got.ExpiresAt.Equal(want) {
		t.Fatalf("expires %v, want %v", got.ExpiresAt, want)
	}
}

func TestTouchSessionIgnoresExpiredAndRevoked(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u, _ := userWithSession(t, pool, "amy", "expired", now.Add(time.Hour))
	revoked := newSession(u.ID, "revoked", now.Add(time.Hour))
	mustInsertSession(t, pool, revoked)
	if ok, err := store.RevokeSession(ctx, pool, revoked.ID, "logout", now); err != nil || !ok {
		t.Fatalf("revoke: %v %v", ok, err)
	}

	at := now.Add(2 * time.Hour)
	if _, err := store.TouchSession(ctx, pool, []byte("expired"), at, 12*time.Hour, time.Hour); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("expired: got %v", err)
	}
	if _, err := store.TouchSession(ctx, pool, []byte("revoked"), now, 12*time.Hour, time.Hour); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("revoked: got %v", err)
	}
}

func TestReplaceSessionCSRF(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u, active := userWithSession(t, pool, "amy", "active", now.Add(time.Hour))
	revoked := newSession(u.ID, "revoked", now.Add(time.Hour))
	mustInsertSession(t, pool, revoked)
	if _, err := store.RevokeSession(ctx, pool, revoked.ID, "logout", now); err != nil {
		t.Fatal(err)
	}

	if err := store.ReplaceSessionCSRF(ctx, pool, active.ID, []byte("new"), now); err != nil {
		t.Fatal(err)
	}
	got, err := store.FindActiveSessionByID(ctx, pool, active.ID, now)
	if err != nil || string(got.CSRFHash) != "new" {
		t.Fatalf("got %+v, %v", got, err)
	}

	if err := store.ReplaceSessionCSRF(ctx, pool, revoked.ID, []byte("new"), now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("revoked: got %v", err)
	}
	var csrf []byte
	if err := pool.QueryRow(ctx, `SELECT csrf_hash FROM sessions WHERE id = $1`, revoked.ID).Scan(&csrf); err != nil {
		t.Fatal(err)
	}
	if string(csrf) != "csrf-revoked" {
		t.Fatalf("revoked row changed: %q", csrf)
	}
}

func TestRevokeSessionOnce(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	_, s := userWithSession(t, pool, "amy", "tok", now.Add(time.Hour))

	first, err := store.RevokeSession(ctx, pool, s.ID, "logout", now)
	if err != nil || !first {
		t.Fatalf("first: %v %v", first, err)
	}
	second, err := store.RevokeSession(ctx, pool, s.ID, "other", now.Add(time.Minute))
	if err != nil || second {
		t.Fatalf("second: %v %v", second, err)
	}
	var reason string
	if err := pool.QueryRow(ctx, `SELECT revoked_reason FROM sessions WHERE id = $1`, s.ID).Scan(&reason); err != nil {
		t.Fatal(err)
	}
	if reason != "logout" {
		t.Fatalf("reason %q, want logout", reason)
	}
}

func TestRevokeSessionsOfUser(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u := newUser("amy", "reader", true)
	other := newUser("kim", "reader", true)
	mustInsertUser(t, pool, u)
	mustInsertUser(t, pool, other)
	a := newSession(u.ID, "a", now.Add(time.Hour))
	b := newSession(u.ID, "b", now.Add(time.Hour))
	c := newSession(u.ID, "c", now.Add(time.Hour))
	o := newSession(other.ID, "o", now.Add(time.Hour))
	for _, s := range []store.Session{a, b, c, o} {
		mustInsertSession(t, pool, s)
	}
	if _, err := store.RevokeSession(ctx, pool, c.ID, "logout", now); err != nil {
		t.Fatal(err)
	}

	n, err := store.RevokeSessionsOfUser(ctx, pool, u.ID, "password-changed", now)
	if err != nil || n != 2 {
		t.Fatalf("got %d, %v, want 2", n, err)
	}
	if _, err := store.FindActiveSessionByID(ctx, pool, o.ID, now); err != nil {
		t.Fatalf("other user's session: %v", err)
	}
	var reason string
	if err := pool.QueryRow(ctx, `SELECT revoked_reason FROM sessions WHERE id = $1`, c.ID).Scan(&reason); err != nil || reason != "logout" {
		t.Fatalf("already revoked changed: %q %v", reason, err)
	}
}

func TestRotatedFromReferencesASession(t *testing.T) {
	pool := storetest.New(t)
	u := newUser("amy", "reader", true)
	mustInsertUser(t, pool, u)
	s := newSession(u.ID, "tok", now.Add(time.Hour))
	missing := uuid.New()
	s.RotatedFromID = &missing
	err := store.InsertSession(context.Background(), pool, s)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("got %v, want SQLSTATE 23503", err)
	}
}
