//go:build postgres

package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

const lockWait = 5 * time.Second

// insertJob adds a River job in state and returns its id; a final state
// carries the finalized_at River's check demands.
func insertJob(t *testing.T, pool *pgxpool.Pool, state string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(context.Background(), `
INSERT INTO river_job (state, max_attempts, kind, args, finalized_at)
VALUES ($1::river_job_state, 3, 'collect', '{}',
        CASE WHEN $1 IN ('discarded', 'cancelled', 'completed') THEN now() END)
RETURNING id`, state).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// deletedJob returns the id of a job that was inserted and then deleted.
func deletedJob(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	id := insertJob(t, pool, "available")
	if _, err := pool.Exec(context.Background(), `DELETE FROM river_job WHERE id = $1`, id); err != nil {
		t.Fatal(err)
	}
	return id
}

// activeReport inserts a report in status (queued or running) whose collect
// job is jobID.
func activeReport(t *testing.T, pool *pgxpool.Pool, f reportFixture, status string, jobID int64, created time.Time) store.Report {
	t.Helper()
	r := queuedReport(f, f.profile, f.revision)
	r.CollectJobID = jobID
	r.CreatedAt = created
	if status == "running" {
		r.Status, r.Phase, r.Attempt = "running", "collecting", 1
	}
	mustInsertReport(t, pool, r)
	return r
}

// collectedReport inserts a collected report, in phase rendering when a row is pending and complete otherwise, with a
// snapshot and one pdf row per language: statuses maps a language to
// "pending", "ready", or "failed", and jobs to the job of a pending row.
func collectedReport(t *testing.T, pool *pgxpool.Pool, f reportFixture, expires time.Time, statuses map[string]string, jobs map[string]int64) store.Report {
	t.Helper()
	ctx := context.Background()
	r := queuedReport(f, f.profile, f.revision)
	r.Status, r.Phase, r.Attempt = "collected", "complete", 1
	collected := now
	r.CollectedAt, r.CompletedAt = &collected, &collected
	for _, status := range statuses {
		if status == "pending" {
			r.Phase, r.CompletedAt = "rendering", nil
		}
	}
	r.ExpiresAt = expires
	mustInsertReport(t, pool, r)
	if err := store.InsertSnapshot(ctx, pool, r.ID, "4.0", []byte("{}"), now); err != nil {
		t.Fatal(err)
	}
	for lang, status := range statuses {
		if err := store.InsertPendingPDF(ctx, pool, r.ID, lang, jobs[lang], now); err != nil {
			t.Fatal(err)
		}
		switch status {
		case "ready":
			if err := store.FinishPDF(ctx, pool, r.ID, lang, jobs[lang], 0, []byte("%PDF-"), now); err != nil {
				t.Fatal(err)
			}
		case "failed":
			if err := store.FailPDF(ctx, pool, r.ID, lang, jobs[lang], 0, "PDF_COMPILE_FAILED", now); err != nil {
				t.Fatal(err)
			}
		}
	}
	return r
}

func inTx(t *testing.T, pool *pgxpool.Pool, fn func(tx pgx.Tx) error) {
	t.Helper()
	if err := store.Transact(context.Background(), pool, fn); err != nil {
		t.Fatal(err)
	}
}

func reconcileReports(t *testing.T, pool *pgxpool.Pool, limit int) []uuid.UUID {
	t.Helper()
	var ids []uuid.UUID
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		ids, err = store.ReconcileReports(context.Background(), tx, now.Add(time.Hour), limit)
		return err
	})
	return ids
}

func reconcilePDFs(t *testing.T, pool *pgxpool.Pool, limit int) []store.PDFKey {
	t.Helper()
	var keys []store.PDFKey
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		keys, err = store.ReconcilePDFs(context.Background(), tx, now.Add(time.Hour), limit)
		return err
	})
	return keys
}

func getReportT(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) store.Report {
	t.Helper()
	r, err := store.GetReport(context.Background(), pool, id)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func wantFailedReport(t *testing.T, name string, r store.Report) {
	t.Helper()
	at := now.Add(time.Hour)
	if r.Status != "failed" || r.Phase != "complete" || r.FailureCode == nil || *r.FailureCode != "JOB_ATTEMPTS_EXHAUSTED" ||
		r.CompletedAt == nil || !r.CompletedAt.Equal(at) || !r.UpdatedAt.Equal(at) {
		t.Fatalf("%s: %+v", name, r)
	}
}

func wantUntouched(t *testing.T, name string, r store.Report, status string) {
	t.Helper()
	if r.Status != status || r.FailureCode != nil || r.CompletedAt != nil || !r.UpdatedAt.Equal(now) {
		t.Fatalf("%s: %+v", name, r)
	}
}

func TestReconcileReports(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	discarded := activeReport(t, pool, f, "running", insertJob(t, pool, "discarded"), now)
	cancelled := activeReport(t, pool, f, "running", insertJob(t, pool, "cancelled"), now)
	completed := activeReport(t, pool, f, "queued", insertJob(t, pool, "completed"), now)
	gone := activeReport(t, pool, f, "running", deletedJob(t, pool), now)
	alive := map[string]store.Report{}
	for _, state := range []string{"available", "pending", "retryable", "scheduled", "running"} {
		status := "queued"
		if state == "running" {
			status = "running"
		}
		alive[state] = activeReport(t, pool, f, status, insertJob(t, pool, state), now)
	}

	ids := reconcileReports(t, pool, 100)
	if len(ids) != 4 {
		t.Fatalf("reconciled %v, want four", ids)
	}
	wantFailedReport(t, "discarded", getReportT(t, pool, discarded.ID))
	wantFailedReport(t, "cancelled", getReportT(t, pool, cancelled.ID))
	wantFailedReport(t, "completed", getReportT(t, pool, completed.ID))
	wantFailedReport(t, "absent", getReportT(t, pool, gone.ID))
	for state, r := range alive {
		wantUntouched(t, state, getReportT(t, pool, r.ID), r.Status)
	}
	if again := reconcileReports(t, pool, 100); len(again) != 0 {
		t.Fatalf("second run reconciled %v", again)
	}
}

func TestReconcileReportsLeavesTerminalReports(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready"}, nil)
	if ids := reconcileReports(t, pool, 100); len(ids) != 0 {
		t.Fatalf("reconciled %v", ids)
	}
	if got := getReportT(t, pool, r.ID); got.Status != "collected" {
		t.Fatalf("%+v", got)
	}
}

func TestReconcileReportsBatch(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	var made []store.Report
	for i := 0; i < 3; i++ {
		made = append(made, activeReport(t, pool, f, "running", deletedJob(t, pool), now.Add(time.Duration(i)*time.Second)))
	}
	ids := reconcileReports(t, pool, 2)
	if len(ids) != 2 {
		t.Fatalf("reconciled %v, want two", ids)
	}
	wantFailedReport(t, "oldest", getReportT(t, pool, made[0].ID))
	wantFailedReport(t, "second", getReportT(t, pool, made[1].ID))
	wantUntouched(t, "newest", getReportT(t, pool, made[2].ID), "running")
}

func TestReconcilePDFs(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	discarded := insertJob(t, pool, "discarded")
	first := collectedReport(t, pool, f, now.Add(2*time.Hour),
		map[string]string{"en": "pending", "pt-BR": "ready"}, map[string]int64{"en": discarded})
	second := collectedReport(t, pool, f, now.Add(2*time.Hour),
		map[string]string{"en": "pending", "pt-BR": "pending"},
		map[string]int64{"en": insertJob(t, pool, "discarded"), "pt-BR": insertJob(t, pool, "available")})

	keys := reconcilePDFs(t, pool, 100)
	if len(keys) != 2 {
		t.Fatalf("reconciled %v, want two rows", keys)
	}
	at := now.Add(time.Hour)
	row := func(r store.Report, lang string) store.ReportPDF {
		p, err := store.GetPDF(context.Background(), pool, r.ID, lang)
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	en := row(first, "en")
	if en.Status != "failed" || en.FailureCode == nil || *en.FailureCode != "PDF_ATTEMPTS_EXHAUSTED" || !en.UpdatedAt.Equal(at) {
		t.Fatalf("first en %+v", en)
	}
	if pt := row(first, "pt-BR"); pt.Status != "ready" {
		t.Fatalf("first pt-BR %+v", pt)
	}
	if r := getReportT(t, pool, first.ID); r.Phase != "complete" || r.CompletedAt == nil || !r.CompletedAt.Equal(at) || r.Status != "collected" {
		t.Fatalf("first report %+v", r)
	}
	if en := row(second, "en"); en.Status != "failed" || en.FailureCode == nil || *en.FailureCode != "PDF_ATTEMPTS_EXHAUSTED" {
		t.Fatalf("second en %+v", en)
	}
	if pt := row(second, "pt-BR"); pt.Status != "pending" || pt.FailureCode != nil {
		t.Fatalf("second pt-BR %+v", pt)
	}
	if r := getReportT(t, pool, second.ID); r.Phase != "rendering" || r.CompletedAt != nil {
		t.Fatalf("second report %+v", r)
	}
}

func TestReconcilePDFsLeavesAReportPastExpiry(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "pending"}, map[string]int64{"en": insertJob(t, pool, "completed")})
	var keys []store.PDFKey
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		keys, err = store.ReconcilePDFs(context.Background(), tx, now.Add(time.Hour), 100)
		return err
	})
	if len(keys) != 0 {
		t.Fatalf("reconciled %v a report that is due for expiry", keys)
	}
	if p, _ := store.GetPDF(context.Background(), pool, r.ID, "en"); p.Status != "pending" {
		t.Fatalf("row %+v", p)
	}
}

// holdReportLock locks the report row in a transaction that stays open until
// the test ends.
func holdReportLock(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	if _, err := store.GetReportForUpdate(ctx, tx, id); err != nil {
		t.Fatal(err)
	}
}

// boundedTx runs fn in a transaction that fails, rather than waits, when it
// is blocked for longer than lockWait.
func boundedTx(t *testing.T, pool *pgxpool.Pool, fn func(ctx context.Context, tx pgx.Tx) error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), lockWait)
	defer cancel()
	if err := store.Transact(ctx, pool, func(tx pgx.Tx) error { return fn(ctx, tx) }); err != nil {
		t.Fatalf("blocked or failed: %v", err)
	}
}

func TestReconcileSkipsALockedReport(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	locked := activeReport(t, pool, f, "running", deletedJob(t, pool), now)
	other := activeReport(t, pool, f, "running", deletedJob(t, pool), now.Add(time.Second))
	holdReportLock(t, pool, locked.ID)

	var ids []uuid.UUID
	boundedTx(t, pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		ids, err = store.ReconcileReports(ctx, tx, now.Add(time.Hour), 100)
		return err
	})
	if len(ids) != 1 || ids[0] != other.ID {
		t.Fatalf("reconciled %v, want only %v", ids, other.ID)
	}
	wantUntouched(t, "locked", getReportT(t, pool, locked.ID), "running")
	wantFailedReport(t, "other", getReportT(t, pool, other.ID))
}

func TestReconcilePDFsSkipsALockedReport(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	gone := map[string]int64{"en": deletedJob(t, pool)}
	locked := collectedReport(t, pool, f, now.Add(2*time.Hour), map[string]string{"en": "pending"}, gone)
	other := collectedReport(t, pool, f, now.Add(2*time.Hour), map[string]string{"en": "pending"}, map[string]int64{"en": deletedJob(t, pool)})
	holdReportLock(t, pool, locked.ID)

	var keys []store.PDFKey
	boundedTx(t, pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		keys, err = store.ReconcilePDFs(ctx, tx, now.Add(time.Hour), 100)
		return err
	})
	if len(keys) != 1 || keys[0].ReportID != other.ID {
		t.Fatalf("reconciled %v, want only %v", keys, other.ID)
	}
	if p, _ := store.GetPDF(context.Background(), pool, locked.ID, "en"); p.Status != "pending" {
		t.Fatalf("locked row %+v", p)
	}
}

func expireDue(t *testing.T, pool *pgxpool.Pool, at time.Time, limit int) []uuid.UUID {
	t.Helper()
	var ids []uuid.UUID
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		ids, err = store.ExpireDueReports(context.Background(), tx, at, limit)
		return err
	})
	return ids
}

func countRows(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestExpireDueReports(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	at := now.Add(time.Hour)
	langs := map[string]string{"en": "ready", "pt-BR": "failed"}
	due := collectedReport(t, pool, f, at, map[string]string{"en": "pending", "pt-BR": "ready"}, nil)
	complete := collectedReport(t, pool, f, at, langs, nil)
	later := collectedReport(t, pool, f, at.Add(time.Second), langs, nil)
	running := queuedReport(f, f.profile, f.revision)
	running.Status, running.Phase, running.Attempt, running.ExpiresAt = "running", "collecting", 1, at.Add(-time.Hour)
	mustInsertReport(t, pool, running)
	already := collectedReport(t, pool, f, at.Add(-time.Hour), langs, nil)
	earlier := now.Add(30 * time.Minute)
	if _, err := pool.Exec(context.Background(), `UPDATE reports SET expired_at = $2 WHERE id = $1`, already.ID, earlier); err != nil {
		t.Fatal(err)
	}

	ids := expireDue(t, pool, at, 100)
	if len(ids) != 2 || !slices.Contains(ids, due.ID) || !slices.Contains(ids, complete.ID) {
		t.Fatalf("expired %v, want %v and %v", ids, due.ID, complete.ID)
	}
	got := getReportT(t, pool, due.ID)
	if got.ExpiredAt == nil || !got.ExpiredAt.Equal(at) || !got.UpdatedAt.Equal(at) || got.Status != "collected" ||
		got.Phase != "complete" || got.CompletedAt == nil || !got.CompletedAt.Equal(at) {
		t.Fatalf("expired report that was rendering %+v", got)
	}
	got = getReportT(t, pool, complete.ID)
	if got.ExpiredAt == nil || got.Phase != "complete" || got.CompletedAt == nil || !got.CompletedAt.Equal(now) {
		t.Fatalf("expired report that was complete %+v", got)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM report_snapshots WHERE report_id = $1`, due.ID) +
		countRows(t, pool, `SELECT count(*) FROM report_pdfs WHERE report_id = $1`, due.ID); n != 0 {
		t.Fatalf("%d snapshot and pdf rows remain", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM report_snapshots WHERE report_id = $1`, later.ID) +
		countRows(t, pool, `SELECT count(*) FROM report_pdfs WHERE report_id = $1`, later.ID); n != 3 {
		t.Fatalf("later report holds %d rows, want 3", n)
	}
	if r := getReportT(t, pool, later.ID); r.ExpiredAt != nil {
		t.Fatalf("later %+v", r)
	}
	if r := getReportT(t, pool, running.ID); r.ExpiredAt != nil {
		t.Fatalf("running %+v", r)
	}
	if r := getReportT(t, pool, already.ID); r.ExpiredAt == nil || !r.ExpiredAt.Equal(earlier) {
		t.Fatalf("already expired %+v", r)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM report_snapshots WHERE report_id = $1`, already.ID); n != 1 {
		t.Fatalf("the already expired report lost its snapshot")
	}
}

func TestExpireFailedReport(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	r.Status, r.Phase, r.FailureCode, r.ExpiresAt = "failed", "complete", ptr("INTERNAL_FAILURE"), now
	mustInsertReport(t, pool, r)
	if ids := expireDue(t, pool, now, 100); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("expired %v", ids)
	}
	if got := getReportT(t, pool, r.ID); got.Phase != "complete" || got.CompletedAt != nil || got.Status != "failed" {
		t.Fatalf("expired failed report %+v", got)
	}
}

func ptr[T any](v T) *T { return &v }

func TestExpireBatchOrder(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	var made []store.Report
	for _, offset := range []time.Duration{2 * time.Second, 0, time.Second} {
		made = append(made, collectedReport(t, pool, f, now.Add(offset), map[string]string{"en": "ready"}, nil))
	}
	ids := expireDue(t, pool, now.Add(time.Minute), 2)
	if len(ids) != 2 {
		t.Fatalf("expired %v", ids)
	}
	got := map[uuid.UUID]bool{ids[0]: true, ids[1]: true}
	if !got[made[1].ID] || !got[made[2].ID] || got[made[0].ID] {
		t.Fatalf("expired %v, want the two oldest %v %v", ids, made[1].ID, made[2].ID)
	}
}

func TestExpireSkipsALockedRow(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	locked := collectedReport(t, pool, f, now, map[string]string{"en": "ready"}, nil)
	other := collectedReport(t, pool, f, now.Add(time.Second), map[string]string{"en": "ready"}, nil)
	holdReportLock(t, pool, locked.ID)

	var ids []uuid.UUID
	boundedTx(t, pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		ids, err = store.ExpireDueReports(ctx, tx, now.Add(time.Minute), 100)
		return err
	})
	if len(ids) != 1 || ids[0] != other.ID {
		t.Fatalf("expired %v, want only %v", ids, other.ID)
	}
	if r := getReportT(t, pool, locked.ID); r.ExpiredAt != nil {
		t.Fatalf("locked %+v", r)
	}
}

func TestPurgeExpiredReports(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	kept := 30 * 24 * time.Hour
	r := queuedReport(f, f.profile, f.revision)
	r.Status, r.Phase, r.FailureCode, r.ExpiresAt = "failed", "complete", ptr("INTERNAL_FAILURE"), now
	mustInsertReport(t, pool, r)
	if err := store.InsertIdempotencyKey(context.Background(), pool, f.owner, "key", "sum", r.ID, now); err != nil {
		t.Fatal(err)
	}
	live := queuedReport(f, f.profile, f.revision)
	live.Status, live.Phase, live.FailureCode, live.ExpiresAt = "failed", "complete", ptr("INTERNAL_FAILURE"), now.Add(-kept)
	mustInsertReport(t, pool, live)
	purge := func(at time.Time) []uuid.UUID {
		var ids []uuid.UUID
		inTx(t, pool, func(tx pgx.Tx) (err error) {
			ids, err = store.PurgeExpiredReports(context.Background(), tx, at, kept, 100)
			return err
		})
		return ids
	}
	if ids := purge(now.Add(kept)); len(ids) != 0 {
		t.Fatalf("purged %v before any report was expired", ids)
	}
	if _, err := pool.Exec(context.Background(), `UPDATE reports SET expired_at = $1 WHERE id = $2`, now, r.ID); err != nil {
		t.Fatal(err)
	}
	if ids := purge(now.Add(kept - time.Second)); len(ids) != 0 {
		t.Fatalf("purged %v a second early", ids)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM reports WHERE id = $1`, r.ID); n != 1 {
		t.Fatal("the report went early")
	}
	ids := purge(now.Add(kept))
	if len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("purged %v, want %v", ids, r.ID)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM reports WHERE id = $1`, r.ID) +
		countRows(t, pool, `SELECT count(*) FROM report_idempotency_keys`); n != 0 {
		t.Fatalf("%d rows remain", n)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM reports WHERE id = $1`, live.ID); n != 1 {
		t.Fatal("a report that was never expired was purged")
	}
}

// A report kept past its deadline, because it was not yet terminal or the
// maintenance did not run, is kept for the whole period from the moment it
// was marked expired.
func TestPurgeMeasuresFromExpiredAt(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	kept := 30 * 24 * time.Hour
	late := now.Add(10 * 24 * time.Hour)
	r := queuedReport(f, f.profile, f.revision)
	r.Status, r.Phase, r.FailureCode, r.ExpiresAt = "failed", "complete", ptr("INTERNAL_FAILURE"), now
	mustInsertReport(t, pool, r)
	if _, err := pool.Exec(context.Background(), `UPDATE reports SET expired_at = $1 WHERE id = $2`, late, r.ID); err != nil {
		t.Fatal(err)
	}
	purge := func(at time.Time) []uuid.UUID {
		var ids []uuid.UUID
		inTx(t, pool, func(tx pgx.Tx) (err error) {
			ids, err = store.PurgeExpiredReports(context.Background(), tx, at, kept, 100)
			return err
		})
		return ids
	}
	for _, at := range []time.Time{now.Add(kept), late.Add(kept - time.Second)} {
		if ids := purge(at); len(ids) != 0 {
			t.Fatalf("purged %v at %v, before expired_at plus the kept period", ids, at)
		}
	}
	if ids := purge(late.Add(kept)); len(ids) != 1 || ids[0] != r.ID {
		t.Fatalf("purged %v at expired_at plus the kept period, want %v", ids, r.ID)
	}
}

func TestPurgeExpiredReportsRemovesStrayRows(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	kept := time.Hour
	r := collectedReport(t, pool, f, now, map[string]string{"en": "ready"}, nil)
	if _, err := pool.Exec(context.Background(), `UPDATE reports SET expired_at = $1 WHERE id = $2`, now, r.ID); err != nil {
		t.Fatal(err)
	}
	var ids []uuid.UUID
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		ids, err = store.PurgeExpiredReports(context.Background(), tx, now.Add(kept), kept, 100)
		return err
	})
	if len(ids) != 1 || countRows(t, pool, `SELECT count(*) FROM reports`)+countRows(t, pool, `SELECT count(*) FROM report_pdfs`)+
		countRows(t, pool, `SELECT count(*) FROM report_snapshots`) != 0 {
		t.Fatalf("purged %v, rows remain", ids)
	}
}

func TestPurgeThrottle(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	window := 300 * time.Second
	later := now.Add(time.Hour)
	for _, row := range []store.Throttle{
		{KeyHash: []byte("over"), WindowStartedAt: now.Add(-window), Failures: 2, UpdatedAt: now},
		{KeyHash: []byte("blocked"), WindowStartedAt: now.Add(-window), Failures: 5, BlockedUntil: &later, UpdatedAt: now},
		{KeyHash: []byte("open"), WindowStartedAt: now.Add(-window + time.Second), Failures: 1, UpdatedAt: now},
	} {
		if err := store.UpsertThrottle(ctx, pool, row); err != nil {
			t.Fatal(err)
		}
	}
	n, err := store.PurgeThrottle(ctx, pool, now, window, 1000)
	if err != nil || n != 1 {
		t.Fatalf("purged %d, %v", n, err)
	}
	if _, err := store.GetThrottle(ctx, pool, []byte("over")); err == nil {
		t.Fatal("the row whose window ended is still there")
	}
	for _, key := range []string{"blocked", "open"} {
		if _, err := store.GetThrottle(ctx, pool, []byte(key)); err != nil {
			t.Fatalf("%s: %v", key, err)
		}
	}
}

func TestPurgeThrottleLimit(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	for _, k := range []string{"a", "b", "c"} {
		if err := store.UpsertThrottle(ctx, pool, store.Throttle{KeyHash: []byte(k), WindowStartedAt: now.Add(-time.Hour), Failures: 1, UpdatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := store.PurgeThrottle(ctx, pool, now, 300*time.Second, 2); err != nil || n != 2 {
		t.Fatalf("purged %d, %v", n, err)
	}
}

func TestPurgeSessions(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	ttl := 12 * time.Hour
	user := newUser("session-owner", "reader", true)
	mustInsertUser(t, pool, user)
	insert := func(hash string, expires time.Time, revoked *time.Time, parent *uuid.UUID) store.Session {
		s := newSession(user.ID, hash, expires)
		s.RevokedAt, s.RotatedFromID = revoked, parent
		if revoked != nil {
			s.RevokedReason = ptr("logout")
		}
		mustInsertSession(t, pool, s)
		return s
	}
	live := now.Add(24 * time.Hour)
	longAgo := now.Add(-ttl)
	shortOf := now.Add(-ttl + time.Second)
	revokedOld := insert("revoked-old", live, &longAgo, nil)
	revokedNew := insert("revoked-new", live, &shortOf, nil)
	expiredOld := insert("expired-old", longAgo, nil, nil)
	alive := insert("alive", live, nil, nil)
	rotated := insert("rotated", live, nil, &revokedOld.ID)

	var n int64
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		n, err = store.PurgeSessions(ctx, tx, now, ttl, 1000)
		return err
	})
	if n != 2 {
		t.Fatalf("purged %d, want 2", n)
	}
	exists := func(id uuid.UUID) bool {
		return countRows(t, pool, `SELECT count(*) FROM sessions WHERE id = $1`, id) == 1
	}
	if exists(revokedOld.ID) || exists(expiredOld.ID) || !exists(revokedNew.ID) || !exists(alive.ID) || !exists(rotated.ID) {
		t.Fatal("wrong rows purged")
	}
	if got, err := store.FindActiveSessionByID(ctx, pool, rotated.ID, now); err != nil || got.RotatedFromID != nil {
		t.Fatalf("rotated session %+v, %v", got, err)
	}
}

func TestPurgeSessionsChain(t *testing.T) {
	pool := storetest.New(t)
	ttl := time.Hour
	user := newUser("chain-owner", "reader", true)
	mustInsertUser(t, pool, user)
	old := now.Add(-2 * ttl)
	first := newSession(user.ID, "first", old)
	first.RevokedAt, first.RevokedReason = &old, ptr("rotated")
	mustInsertSession(t, pool, first)
	second := newSession(user.ID, "second", old)
	second.RevokedAt, second.RevokedReason, second.RotatedFromID = &old, ptr("rotated"), &first.ID
	mustInsertSession(t, pool, second)
	var n int64
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		n, err = store.PurgeSessions(context.Background(), tx, now, ttl, 1000)
		return err
	})
	if n != 2 {
		t.Fatalf("purged %d, want 2", n)
	}
}
