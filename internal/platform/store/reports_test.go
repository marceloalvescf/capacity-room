//go:build postgres

package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func TestRiverTablesExist(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)

	for _, table := range []string{"river_job", "river_leader", "river_queue", "river_notification"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil || !exists {
			t.Fatalf("table %s exists = %v, %v", table, exists, err)
		}
	}
	for _, table := range []string{"river_client", "river_client_queue", "river_migration"} {
		var exists bool
		if err := pool.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil || exists {
			t.Fatalf("table %s exists = %v, %v; want absent", table, exists, err)
		}
	}
	var persistence string
	if err := pool.QueryRow(ctx, `SELECT relpersistence FROM pg_class WHERE relname = 'river_leader'`).Scan(&persistence); err != nil || persistence != "u" {
		t.Fatalf("river_leader persistence = %q, %v; want u", persistence, err)
	}
	var enum, function bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'river_job_state' AND typtype = 'e')`).Scan(&enum); err != nil || !enum {
		t.Fatalf("enum river_job_state exists = %v, %v", enum, err)
	}
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'river_job_state_in_bitmask')`).Scan(&function); err != nil || !function {
		t.Fatalf("function river_job_state_in_bitmask exists = %v, %v", function, err)
	}
}

const insertReport = `INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, attempt, failure_code, collect_job_id, created_at, expires_at, updated_at)
VALUES ($1, $2, $3, $4, '{}', $5, $6, 0, $7, 1, now(), now(), now())`

// reportFixture holds a user, a profile with one revision, and the ids of
// both.
type reportFixture struct {
	owner    uuid.UUID
	profile  string
	revision uuid.UUID
}

func newReportFixture(t *testing.T, pool *pgxpool.Pool) reportFixture {
	t.Helper()
	ctx := context.Background()
	owner, err := addUser(ctx, pool, "operator", "en")
	if err != nil {
		t.Fatal(err)
	}
	f := reportFixture{owner: owner, profile: "p", revision: uuid.New()}
	if _, err := pool.Exec(ctx, insertProfile, f.profile, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, insertRevision, f.revision, f.profile, 1, "fp"); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f reportFixture) insertReport(ctx context.Context, pool *pgxpool.Pool, status, phase string, failureCode any) (uuid.UUID, error) {
	id := uuid.New()
	_, err := pool.Exec(ctx, insertReport, id, f.owner, f.profile, f.revision, status, phase, failureCode)
	return id, err
}

func wantSQLState(t *testing.T, name string, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s accepted, want a failure", name)
	}
	if code, _ := sqlState(t, err); code != want {
		t.Fatalf("%s: SQLSTATE = %s, want %s", name, code, want)
	}
}

func TestReportChecks(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)

	_, err := f.insertReport(ctx, pool, "done", "complete", nil)
	wantSQLState(t, "status done", err, "23514")
	_, err = f.insertReport(ctx, pool, "queued", "preflight", nil)
	wantSQLState(t, "queued with preflight", err, "23514")
	_, err = f.insertReport(ctx, pool, "failed", "complete", nil)
	wantSQLState(t, "failed without a code", err, "23514")
	_, err = f.insertReport(ctx, pool, "collected", "complete", "INTERNAL_FAILURE")
	wantSQLState(t, "collected with a code", err, "23514")

	reportID, err := f.insertReport(ctx, pool, "collected", "rendering", nil)
	if err != nil {
		t.Fatalf("a valid report: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO report_pdfs (report_id, language, status, job_id, updated_at) VALUES ($1, 'en', 'ready', 1, now())`, reportID)
	wantSQLState(t, "ready PDF without a body", err, "23514")
	_, err = pool.Exec(ctx, `INSERT INTO report_pdfs (report_id, language, status, job_id, updated_at) VALUES ($1, 'es', 'pending', 1, now())`, reportID)
	wantSQLState(t, "PDF language es", err, "23514")
	_, err = pool.Exec(ctx, `INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at) VALUES ($1, '1', $2, $3, 65, now())`,
		reportID, []byte(strings.Repeat("a", 64)), strings.Repeat("0", 64))
	wantSQLState(t, "snapshot declaring 65 bytes for 64", err, "23514")

	if _, err := pool.Exec(ctx, `INSERT INTO report_pdfs (report_id, language, status, job_id, updated_at) VALUES ($1, 'pt-BR', 'pending', 1, now())`, reportID); err != nil {
		t.Fatalf("a valid pending PDF: %v", err)
	}
}

func TestReportSnapshotsAreImmutable(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	reportID, err := f.insertReport(ctx, pool, "collected", "rendering", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at) VALUES ($1, '1', $2, $3, 4, now())`,
		reportID, []byte("abcd"), strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE report_snapshots SET schema_version = '2'`)
	wantSQLState(t, "update of a snapshot", err, "23000")
}

func TestReportForeignKeysRestrict(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	if _, err := f.insertReport(ctx, pool, "queued", "waiting", nil); err != nil {
		t.Fatal(err)
	}
	_, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, f.owner)
	wantSQLState(t, "delete of an owner", err, "23001")
}

// pendingEnPDF returns a pool and a collected report whose en row is pending
// under job 7.
func pendingEnPDF(t *testing.T) (*pgxpool.Pool, store.Report) {
	t.Helper()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "pending"}, map[string]int64{"en": 7})
	return pool, r
}

func claimPDF(pool *pgxpool.Pool, id uuid.UUID, jobID int64, attempt int) error {
	return store.Transact(context.Background(), pool, func(tx pgx.Tx) error {
		return store.ClaimPDF(context.Background(), tx, id, "en", jobID, attempt, now)
	})
}

func wantPDFRow(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, status string, attempt int) store.ReportPDF {
	t.Helper()
	row, err := store.GetPDF(context.Background(), pool, id, "en")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != status || row.Attempt != attempt {
		t.Fatalf("row %+v, want status %s attempt %d", row, status, attempt)
	}
	return row
}

func TestClaimPDFTakesThePendingRow(t *testing.T) {
	pool, r := pendingEnPDF(t)
	for range 2 {
		if err := claimPDF(pool, r.ID, 7, 1); err != nil {
			t.Fatal(err)
		}
	}
	wantPDFRow(t, pool, r.ID, "pending", 1)
}

func TestClaimPDFRefusesAnOlderAttempt(t *testing.T) {
	pool, r := pendingEnPDF(t)
	if err := claimPDF(pool, r.ID, 7, 2); err != nil {
		t.Fatal(err)
	}
	if err := claimPDF(pool, r.ID, 7, 1); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("older attempt: %v", err)
	}
	wantPDFRow(t, pool, r.ID, "pending", 2)
}

func TestClaimPDFRefusesAnotherJob(t *testing.T) {
	pool, r := pendingEnPDF(t)
	if err := claimPDF(pool, r.ID, 8, 1); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("another job: %v", err)
	}
	wantPDFRow(t, pool, r.ID, "pending", 0)
}

func TestFinishPDFNamesItsAttempt(t *testing.T) {
	ctx := context.Background()
	pool, r := pendingEnPDF(t)
	if err := claimPDF(pool, r.ID, 7, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPDF(ctx, pool, r.ID, "en", 7, 1, []byte("%PDF-1.7\n"), now); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("older attempt: %v", err)
	}
	wantPDFRow(t, pool, r.ID, "pending", 2)
	if err := store.FinishPDF(ctx, pool, r.ID, "en", 7, 2, []byte("%PDF-1.7\n"), now); err != nil {
		t.Fatal(err)
	}
	wantPDFRow(t, pool, r.ID, "ready", 2)
}

func TestFailPDFNamesItsJob(t *testing.T) {
	pool, r := pendingEnPDF(t)
	if err := claimPDF(pool, r.ID, 7, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.FailPDF(context.Background(), pool, r.ID, "en", 8, 1, "PDF_COMPILE_FAILED", now); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("another job: %v", err)
	}
	if row := wantPDFRow(t, pool, r.ID, "pending", 1); row.FailureCode != nil {
		t.Fatalf("failure code %q", *row.FailureCode)
	}
}

func TestRetryPDFResetsTheAttempt(t *testing.T) {
	ctx := context.Background()
	pool, r := pendingEnPDF(t)
	if err := claimPDF(pool, r.ID, 7, 3); err != nil {
		t.Fatal(err)
	}
	if err := store.FailPDF(ctx, pool, r.ID, "en", 7, 3, "PDF_COMPILE_FAILED", now); err != nil {
		t.Fatal(err)
	}
	if err := store.RetryPDF(ctx, pool, r.ID, "en", 9, now); err != nil {
		t.Fatal(err)
	}
	if row := wantPDFRow(t, pool, r.ID, "pending", 0); row.JobID != 9 {
		t.Fatalf("job %d", row.JobID)
	}
}

func TestClaimPDFRefusesASettledRow(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour),
		map[string]string{"en": "ready", "pt-BR": "failed"}, map[string]int64{"en": 7, "pt-BR": 7})
	for lang, status := range map[string]string{"en": "ready", "pt-BR": "failed"} {
		err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
			return store.ClaimPDF(ctx, tx, r.ID, lang, 7, 1, now)
		})
		if !errors.Is(err, store.ErrStaleAttempt) {
			t.Fatalf("%s: %v", lang, err)
		}
		row, err := store.GetPDF(ctx, pool, r.ID, lang)
		if err != nil || row.Status != status || row.Attempt != 0 {
			t.Fatalf("%s row %+v, %v", lang, row, err)
		}
	}
}
