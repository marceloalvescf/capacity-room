//go:build postgres

package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

const insertUser = `INSERT INTO users (id, username, password_hash, role, language, active, created_at, updated_at)
VALUES ($1, $2, 'hash', $3, $4, true, now(), now())`

func sqlState(t *testing.T, err error) (string, string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error %v is not a PostgreSQL error", err)
	}
	return pgErr.Code, pgErr.Message
}

func addUser(ctx context.Context, pool *pgxpool.Pool, role, language string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := pool.Exec(ctx, insertUser, id, "user-"+id.String(), role, language)
	return id, err
}

func countUsers(t *testing.T, pool *pgxpool.Pool) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), "SELECT count(*) FROM users").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestMigrateIsIdempotent(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)

	for range 2 {
		version, err := store.Migrate(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if version != 1 {
			t.Fatalf("version = %d, want 1", version)
		}
		pending, err := store.HasPending(ctx, pool)
		if err != nil {
			t.Fatal(err)
		}
		if pending {
			t.Fatal("HasPending = true, want false")
		}
	}
}

func TestHasPendingOnEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)

	pending, err := store.HasPending(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if !pending {
		t.Fatal("HasPending before Migrate = false, want true")
	}
	if _, err := store.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	pending, err = store.HasPending(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if pending {
		t.Fatal("HasPending after Migrate = true, want false")
	}
}

func TestSchemaRefusesBadRole(t *testing.T) {
	pool := storetest.New(t)
	_, err := addUser(context.Background(), pool, "user", "en")
	if code, _ := sqlState(t, err); code != "23514" {
		t.Fatalf("SQLSTATE = %s, want 23514", code)
	}
}

func TestSchemaRefusesBadLanguage(t *testing.T) {
	pool := storetest.New(t)
	_, err := addUser(context.Background(), pool, "reader", "fr")
	if code, _ := sqlState(t, err); code != "23514" {
		t.Fatalf("SQLSTATE = %s, want 23514", code)
	}
}

func TestAuditEventsAreAppendOnly(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	if _, err := pool.Exec(ctx, `INSERT INTO audit_events (occurred_at, action, outcome, metadata)
VALUES (now(), 'login', 'succeeded', '{}')`); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"UPDATE audit_events SET action = 'other'",
		"DELETE FROM audit_events",
	} {
		_, err := pool.Exec(ctx, statement)
		if err == nil {
			t.Fatalf("%s succeeded, want a failure", statement)
		}
		code, message := sqlState(t, err)
		if code != "23000" || !strings.Contains(message, "append-only") {
			t.Fatalf("%s: SQLSTATE %s, message %q", statement, code, message)
		}
	}
}

func TestAuditEventsRefuseTruncate(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	if _, err := pool.Exec(ctx, `INSERT INTO audit_events (occurred_at, action, outcome, metadata)
VALUES (now(), 'login', 'succeeded', '{}')`); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, "TRUNCATE audit_events")
	if err == nil {
		t.Fatal("TRUNCATE succeeded, want a failure")
	}
	code, message := sqlState(t, err)
	if code != "23000" || !strings.Contains(message, "append-only") {
		t.Fatalf("SQLSTATE %s, message %q", code, message)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_events").Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d rows, %v", n, err)
	}
}

func TestSessionRevocationIsPaired(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	userID, err := addUser(ctx, pool, "reader", "en")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO sessions (id, token_hash, csrf_hash, user_id, created_at, expires_at, revoked_at)
VALUES ($1, '\x01', '\x02', $2, now(), now() + interval '1 hour', now())`, uuid.New(), userID)
	if code, _ := sqlState(t, err); code != "23514" {
		t.Fatalf("SQLSTATE = %s, want 23514", code)
	}
}

func TestTransactCommits(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, insertUser, uuid.New(), "committed", "reader", "en")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n := countUsers(t, pool); n != 1 {
		t.Fatalf("users = %d, want 1", n)
	}
}

func TestTransactRollsBack(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	want := errors.New("stop")
	err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, insertUser, uuid.New(), "discarded", "reader", "en"); err != nil {
			return err
		}
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want %v", err, want)
	}
	if n := countUsers(t, pool); n != 0 {
		t.Fatalf("users = %d, want 0", n)
	}
}

func TestTransactRollsBackOnPanic(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic did not reach the caller")
			}
		}()
		_ = store.Transact(ctx, pool, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, insertUser, uuid.New(), "panicked", "reader", "en"); err != nil {
				return err
			}
			panic("boom")
		})
	}()
	if n := countUsers(t, pool); n != 0 {
		t.Fatalf("users = %d, want 0", n)
	}
}
