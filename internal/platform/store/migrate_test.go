//go:build postgres

package store_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

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
		if version != 5 {
			t.Fatalf("version = %d, want 5", version)
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

const insertProfile = `INSERT INTO profiles (id, current_revision_id, created_at, updated_at) VALUES ($1, $2, now(), now())`

const insertRevision = `INSERT INTO profile_revisions (id, profile_id, revision, fingerprint, display_name, active, document, credential_file, ca_bundle_file, created_by, created_at)
VALUES ($1, $2, $3, $4, 'name', true, '{}', '', '', NULL, now())`

func TestMigrateToVersionFive(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	version, err := store.Migrate(ctx, pool)
	if err != nil {
		t.Fatal(err)
	}
	if version != 5 {
		t.Fatalf("version = %d, want 5", version)
	}
	for _, table := range []string{"profiles", "profile_revisions", "river_job", "reports", "report_snapshots", "report_pdfs", "report_idempotency_keys"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("table %s does not exist", table)
		}
	}
}

func TestProfileRevisionsAreImmutable(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	revisionID := uuid.New()
	if _, err := pool.Exec(ctx, insertProfile, "a", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, insertRevision, revisionID, "a", 1, "fp"); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"UPDATE profile_revisions SET display_name = 'other'",
		"DELETE FROM profile_revisions",
		"TRUNCATE profile_revisions CASCADE",
	} {
		_, err := pool.Exec(ctx, statement)
		if err == nil {
			t.Fatalf("%s succeeded, want a failure", statement)
		}
		code, message := sqlState(t, err)
		if code != "23000" || !strings.Contains(message, "immutable") {
			t.Fatalf("%s: SQLSTATE %s, message %q", statement, code, message)
		}
	}
	// A plain TRUNCATE never reaches the trigger: the foreign key of the
	// pointer refuses it first.
	_, err := pool.Exec(ctx, "TRUNCATE profile_revisions")
	if err == nil {
		t.Fatal("plain TRUNCATE succeeded, want a failure")
	}
	if code, _ := sqlState(t, err); code != "0A000" {
		t.Fatalf("plain TRUNCATE: SQLSTATE = %s, want 0A000", code)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM profile_revisions").Scan(&n); err != nil || n != 1 {
		t.Fatalf("%d rows, %v", n, err)
	}
}

func TestProfilePointerMustNameOwnRevision(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	revisionB := uuid.New()
	for _, id := range []string{"a", "b"} {
		if _, err := pool.Exec(ctx, insertProfile, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, insertRevision, revisionB, "b", 1, "fp"); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE profiles SET current_revision_id = $1 WHERE id = 'a'`, revisionB); err != nil {
		t.Fatalf("the update itself must succeed before the commit: %v", err)
	}
	err = tx.Commit(ctx)
	if err == nil {
		t.Fatal("commit succeeded, want a failure")
	}
	if code, _ := sqlState(t, err); code != "23000" {
		t.Fatalf("SQLSTATE = %s, want 23000", code)
	}
}

func TestProfilePointerDeferred(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	revisionID := uuid.New()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, insertProfile, "a", revisionID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, insertRevision, revisionID, "a", 1, "fp"); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	var got uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT current_revision_id FROM profiles WHERE id = 'a'`).Scan(&got); err != nil || got != revisionID {
		t.Fatalf("pointer = %v, %v", got, err)
	}
}

func TestProfileIDCheck(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	for _, id := range []string{"A", "..a", "a..b", strings.Repeat("a", 64)} {
		_, err := pool.Exec(ctx, insertProfile, id, nil)
		if err == nil {
			t.Fatalf("id %q accepted, want a failure", id)
		}
		if code, _ := sqlState(t, err); code != "23514" {
			t.Fatalf("id %q: SQLSTATE = %s, want 23514", id, code)
		}
	}
	for _, id := range []string{"a", "a.b-c_d", strings.Repeat("a", 63)} {
		if _, err := pool.Exec(ctx, insertProfile, id, nil); err != nil {
			t.Fatalf("id %q refused: %v", id, err)
		}
	}
}

// TestMigrateDownAndUp runs the three newest Down sections through goose, over the
// migration files of the folder, and applies the migrations again.
func TestMigrateDownAndUp(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, os.DirFS("migrations"), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = provider.Close() }()

	exists := func(table string) bool {
		var found bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&found); err != nil {
			t.Fatal(err)
		}
		return found
	}
	columnExists := func(table, column string) bool {
		var found bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = $1 AND column_name = $2)`, table, column).Scan(&found); err != nil {
			t.Fatal(err)
		}
		return found
	}
	if !columnExists("reports", "worst_severity") {
		t.Fatal("reports.worst_severity does not exist at version 5")
	}
	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("down to 4: %v", err)
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 4 {
		t.Fatalf("version after down = %d, %v; want 4", version, err)
	}
	if columnExists("reports", "worst_severity") || columnExists("reports", "cluster_name") || !exists("reports") {
		t.Fatal("at version 4 the summary columns must be gone and reports must remain")
	}
	for _, step := range []struct {
		version int64
		absent  string
		present string
	}{
		{3, "reports", "river_job"},
		{2, "river_job", "profiles"},
	} {
		if _, err := provider.Down(ctx); err != nil {
			t.Fatalf("down to %d: %v", step.version, err)
		}
		version, err := provider.GetDBVersion(ctx)
		if err != nil || version != step.version {
			t.Fatalf("version after down = %d, %v; want %d", version, err, step.version)
		}
		if exists(step.absent) || !exists(step.present) {
			t.Fatalf("at version %d: %s exists = %v, %s exists = %v", step.version, step.absent, exists(step.absent), step.present, exists(step.present))
		}
	}

	version, err := store.Migrate(ctx, pool)
	if err != nil || version != 5 {
		t.Fatalf("version after up = %d, %v; want 5", version, err)
	}
	for _, table := range []string{"profiles", "profile_revisions", "river_job", "reports", "report_snapshots", "report_pdfs", "report_idempotency_keys"} {
		if !exists(table) {
			t.Fatalf("table %s does not exist after up", table)
		}
	}
}
