//go:build postgres

package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

// addReportProfile creates a profile with one revision and returns the
// revision id.
func addReportProfile(t *testing.T, pool *pgxpool.Pool, id string) uuid.UUID {
	t.Helper()
	revision := uuid.New()
	if _, err := pool.Exec(context.Background(), insertProfile, id, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(context.Background(), insertRevision, revision, id, 1, "fp-"+id); err != nil {
		t.Fatal(err)
	}
	return revision
}

func queuedReport(f reportFixture, profile string, revision uuid.UUID) store.Report {
	return store.Report{
		ID:                uuid.New(),
		OwnerID:           f.owner,
		ProfileID:         profile,
		ProfileRevisionID: revision,
		Request:           []byte(`{"a": 1}`),
		Status:            "queued",
		Phase:             "waiting",
		CollectJobID:      7,
		CreatedAt:         now,
		ExpiresAt:         now.Add(7 * 24 * time.Hour),
		UpdatedAt:         now,
	}
}

func mustInsertReport(t *testing.T, pool *pgxpool.Pool, r store.Report) {
	t.Helper()
	if err := store.InsertReport(context.Background(), pool, r); err != nil {
		t.Fatal(err)
	}
}

func claimedReport(t *testing.T, pool *pgxpool.Pool, f reportFixture, attempt int) store.Report {
	t.Helper()
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)
	if err := store.Transact(context.Background(), pool, func(tx pgx.Tx) error {
		return store.ClaimReport(context.Background(), tx, r.ID, attempt, now.Add(time.Minute))
	}); err != nil {
		t.Fatal(err)
	}
	return r
}

func claim(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, attempt int, at time.Time) error {
	return store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return store.ClaimReport(ctx, tx, id, attempt, at)
	})
}

func TestInsertAndGetReport(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)

	got, err := store.GetReport(ctx, pool, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != r.ID || got.OwnerID != f.owner || got.ProfileID != "p" || got.ProfileRevisionID != f.revision ||
		got.Status != "queued" || got.Phase != "waiting" || got.Attempt != 0 || got.FailureCode != nil ||
		got.CollectJobID != 7 || got.StartedAt != nil || got.CollectedAt != nil || got.CompletedAt != nil ||
		got.ExpiredAt != nil {
		t.Fatalf("got %+v", got)
	}
	if !got.CreatedAt.Equal(now) || got.CreatedAt.Location() != time.UTC ||
		!got.ExpiresAt.Equal(now.Add(7*24*time.Hour)) || got.ExpiresAt.Location() != time.UTC ||
		!got.UpdatedAt.Equal(now) || got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("times: %+v", got)
	}
	if string(got.Request) != `{"a": 1}` {
		t.Fatalf("request = %s", got.Request)
	}
	if _, err := store.GetReport(ctx, pool, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	locked, err := store.GetReportForUpdate(ctx, tx, r.ID)
	if err != nil || locked.ID != r.ID {
		t.Fatalf("for update: %+v, %v", locked, err)
	}
	if _, err := store.GetReportForUpdate(ctx, tx, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("for update missing: %v", err)
	}
}

func TestClaimReportSetsTheAttempt(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)

	first := now.Add(time.Minute)
	if err := claim(ctx, pool, r.ID, 1, first); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetReport(ctx, pool, r.ID)
	if got.Status != "running" || got.Phase != "preflight" || got.Attempt != 1 ||
		got.StartedAt == nil || !got.StartedAt.Equal(first) || !got.UpdatedAt.Equal(first) {
		t.Fatalf("after claim 1: %+v", got)
	}

	if err := claim(ctx, pool, r.ID, 2, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetReport(ctx, pool, r.ID)
	if got.Status != "running" || got.Attempt != 2 || got.StartedAt == nil || !got.StartedAt.Equal(first) ||
		!got.UpdatedAt.Equal(now.Add(2*time.Minute)) {
		t.Fatalf("after claim 2: %+v", got)
	}
}

func TestWritesOfAStaleAttempt(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := claimedReport(t, pool, f, 2)
	before, _ := store.GetReport(ctx, pool, r.ID)

	later := now.Add(time.Hour)
	writes := map[string]error{
		"advance": store.AdvanceReport(ctx, pool, r.ID, 1, "collecting", later),
		"fail":    store.FailReport(ctx, pool, r.ID, 1, "INTERNAL_FAILURE", later),
		"collect": store.CollectReport(ctx, pool, r.ID, 1, later),
		"requeue": store.RequeueReport(ctx, pool, r.ID, 1, later),
	}
	for name, err := range writes {
		if !errors.Is(err, store.ErrStaleAttempt) {
			t.Errorf("%s: got %v, want ErrStaleAttempt", name, err)
		}
	}
	after, _ := store.GetReport(ctx, pool, r.ID)
	if after.Status != before.Status || after.Phase != before.Phase || after.Attempt != before.Attempt ||
		!after.UpdatedAt.Equal(before.UpdatedAt) || after.FailureCode != nil || after.CollectedAt != nil {
		t.Fatalf("row changed: %+v -> %+v", before, after)
	}

	if err := store.AdvanceReport(ctx, pool, r.ID, 2, "collecting", later); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetReport(ctx, pool, r.ID); got.Phase != "collecting" || !got.UpdatedAt.Equal(later) {
		t.Fatalf("advance: %+v", got)
	}
	if err := store.RequeueReport(ctx, pool, r.ID, 2, later); err != nil {
		t.Fatal(err)
	}
	if got, _ := store.GetReport(ctx, pool, r.ID); got.Status != "queued" || got.Phase != "waiting" {
		t.Fatalf("requeue: %+v", got)
	}
}

func TestOutcomesOfARunningReport(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	at := now.Add(time.Hour)

	failed := claimedReport(t, pool, f, 1)
	if err := store.FailReport(ctx, pool, failed.ID, 1, "NOTHING_TO_REPORT", at); err != nil {
		t.Fatal(err)
	}
	got, _ := store.GetReport(ctx, pool, failed.ID)
	if got.Status != "failed" || got.Phase != "complete" || got.FailureCode == nil || *got.FailureCode != "NOTHING_TO_REPORT" ||
		got.CompletedAt == nil || !got.CompletedAt.Equal(at) {
		t.Fatalf("failed: %+v", got)
	}

	collected := claimedReport(t, pool, f, 1)
	if err := store.CollectReport(ctx, pool, collected.ID, 1, at); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetReport(ctx, pool, collected.ID)
	if got.Status != "collected" || got.Phase != "rendering" || got.CollectedAt == nil || !got.CollectedAt.Equal(at) ||
		got.CompletedAt != nil {
		t.Fatalf("collected: %+v", got)
	}

	done := now.Add(2 * time.Hour)
	if err := store.SetReportPhase(ctx, pool, collected.ID, "complete", &done, done); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetReport(ctx, pool, collected.ID)
	if got.Phase != "complete" || got.CompletedAt == nil || !got.CompletedAt.Equal(done) {
		t.Fatalf("complete: %+v", got)
	}
	if err := store.SetReportPhase(ctx, pool, collected.ID, "rendering", nil, done); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetReport(ctx, pool, collected.ID)
	if got.Phase != "rendering" || got.CompletedAt != nil {
		t.Fatalf("rendering again: %+v", got)
	}

	queued := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, queued)
	if err := store.SetReportPhase(ctx, pool, queued.ID, "complete", &done, done); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("phase of a queued report: %v", err)
	}
}

func TestClaimRefusesATerminalReport(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)

	collected := claimedReport(t, pool, f, 1)
	if err := store.CollectReport(ctx, pool, collected.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	failed := claimedReport(t, pool, f, 1)
	if err := store.FailReport(ctx, pool, failed.ID, 1, "INTERNAL_FAILURE", now); err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]uuid.UUID{"collected": collected.ID, "failed": failed.ID} {
		if err := claim(ctx, pool, id, 5, now); !errors.Is(err, store.ErrStaleAttempt) {
			t.Errorf("%s: got %v, want ErrStaleAttempt", name, err)
		}
	}
}

func TestClaimRefusesAnOlderAttempt(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)

	if err := claim(ctx, pool, r.ID, 2, now); err != nil {
		t.Fatal(err)
	}
	if err := claim(ctx, pool, r.ID, 1, now.Add(time.Minute)); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("older claim: %v", err)
	}
	got, _ := store.GetReport(ctx, pool, r.ID)
	if got.Attempt != 2 || !got.UpdatedAt.Equal(now) {
		t.Fatalf("older claim wrote: %+v", got)
	}
	if err := claim(ctx, pool, r.ID, 2, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("same attempt again: %v", err)
	}
}

func TestClaimNeverLowersTheAttemptOfAQueuedReport(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := claimedReport(t, pool, f, 2)
	if err := store.RequeueReport(ctx, pool, r.ID, 2, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	before, _ := store.GetReport(ctx, pool, r.ID)
	if before.Status != "queued" || before.Attempt != 2 {
		t.Fatalf("setup: %+v", before)
	}

	if err := claim(ctx, pool, r.ID, 1, now.Add(2*time.Minute)); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("claim with 1: %v", err)
	}
	if after, _ := store.GetReport(ctx, pool, r.ID); !reflect.DeepEqual(before, after) {
		t.Fatalf("row changed: %+v -> %+v", before, after)
	}
	if err := claim(ctx, pool, r.ID, 2, now.Add(3*time.Minute)); err != nil {
		t.Fatalf("claim with 2: %v", err)
	}
	if err := store.RequeueReport(ctx, pool, r.ID, 2, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := claim(ctx, pool, r.ID, 3, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("claim with 3: %v", err)
	}
	if got, _ := store.GetReport(ctx, pool, r.ID); got.Attempt != 3 || got.Status != "running" {
		t.Fatalf("got %+v", got)
	}
}

func TestGuardedWritesRefuseWhatIsNotRunningAtTheirAttempt(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	at := now.Add(time.Hour)

	collected := claimedReport(t, pool, f, 2)
	if err := store.CollectReport(ctx, pool, collected.ID, 2, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	failed := claimedReport(t, pool, f, 2)
	if err := store.FailReport(ctx, pool, failed.ID, 2, "INTERNAL_FAILURE", now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	queued := claimedReport(t, pool, f, 2)
	if err := store.RequeueReport(ctx, pool, queued.ID, 2, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	running := claimedReport(t, pool, f, 2)

	writes := map[string]func(id uuid.UUID, attempt int) error{
		"advance": func(id uuid.UUID, a int) error { return store.AdvanceReport(ctx, pool, id, a, "collecting", at) },
		"requeue": func(id uuid.UUID, a int) error { return store.RequeueReport(ctx, pool, id, a, at) },
		"fail":    func(id uuid.UUID, a int) error { return store.FailReport(ctx, pool, id, a, "INTERNAL_FAILURE", at) },
		"collect": func(id uuid.UUID, a int) error { return store.CollectReport(ctx, pool, id, a, at) },
	}
	cases := []struct {
		name    string
		id      uuid.UUID
		attempt int
	}{
		{"collected, matching attempt", collected.ID, 2},
		{"failed, matching attempt", failed.ID, 2},
		{"queued, matching attempt", queued.ID, 2},
		{"running, higher attempt", running.ID, 3},
	}
	for write, call := range writes {
		for _, c := range cases {
			before, err := store.GetReport(ctx, pool, c.id)
			if err != nil {
				t.Fatal(err)
			}
			if err := call(c.id, c.attempt); !errors.Is(err, store.ErrStaleAttempt) {
				t.Errorf("%s, %s: got %v, want ErrStaleAttempt", write, c.name, err)
			}
			if after, _ := store.GetReport(ctx, pool, c.id); !reflect.DeepEqual(before, after) {
				t.Errorf("%s, %s: row changed: %+v -> %+v", write, c.name, before, after)
			}
		}
		if err := call(uuid.New(), 2); !errors.Is(err, store.ErrStaleAttempt) {
			t.Errorf("%s, absent id: got %v, want ErrStaleAttempt", write, err)
		}
	}
}

func TestFinishPDFAndInsertSnapshotRefuseANilBody(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)
	if err := store.InsertPendingPDF(ctx, pool, r.ID, "en", 1, now); err != nil {
		t.Fatal(err)
	}
	if err := store.FinishPDF(ctx, pool, r.ID, "en", nil, now); err == nil || errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("FinishPDF nil: %v", err)
	}
	if err := store.InsertSnapshot(ctx, pool, r.ID, "1.0", nil, now); err == nil {
		t.Fatal("InsertSnapshot nil: accepted")
	}
	if got, _ := store.GetPDF(ctx, pool, r.ID, "en"); got.Status != "pending" {
		t.Fatalf("row changed: %+v", got)
	}
}

func TestCountActiveReports(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	prod := addReportProfile(t, pool, "prod")
	stage := addReportProfile(t, pool, "stage")

	for range 2 {
		mustInsertReport(t, pool, queuedReport(f, "prod", prod))
	}
	running := queuedReport(f, "prod", prod)
	mustInsertReport(t, pool, running)
	mustInsertReport(t, pool, queuedReport(f, "stage", stage))
	collected := queuedReport(f, "prod", prod)
	mustInsertReport(t, pool, collected)
	failed := queuedReport(f, "prod", prod)
	mustInsertReport(t, pool, failed)
	for _, id := range []uuid.UUID{running.ID, collected.ID, failed.ID} {
		if err := claim(ctx, pool, id, 1, now); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.CollectReport(ctx, pool, collected.ID, 1, now); err != nil {
		t.Fatal(err)
	}
	if err := store.FailReport(ctx, pool, failed.ID, 1, "INTERNAL_FAILURE", now); err != nil {
		t.Fatal(err)
	}

	global, profile, err := store.CountActiveReports(ctx, pool, "prod")
	if err != nil || global != 4 || profile != 3 {
		t.Fatalf("got %d, %d, %v, want 4, 3", global, profile, err)
	}
}

func TestInsertSnapshotDigest(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)

	if err := store.InsertSnapshot(ctx, pool, r.ID, "1.0", []byte("{\"a\":1}\n"), now); err != nil {
		t.Fatal(err)
	}
	version, body, sum, err := store.GetSnapshot(ctx, pool, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.0" || string(body) != "{\"a\":1}\n" ||
		sum != "e346432021b04179518d9614f3560ccd71354a4ee101ddcb893d6959a9d6301c" {
		t.Fatalf("got %q, %q, %s", version, body, sum)
	}
	var size int64
	if err := pool.QueryRow(ctx, `SELECT size_bytes FROM report_snapshots WHERE report_id = $1`, r.ID).Scan(&size); err != nil || size != 8 {
		t.Fatalf("size = %d, %v", size, err)
	}
	if _, _, _, err := store.GetSnapshot(ctx, pool, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestPDFTransitions(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)

	if err := store.InsertPendingPDF(ctx, pool, r.ID, "en", 11, now); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertPendingPDF(ctx, pool, r.ID, "pt-BR", 12, now); err != nil {
		t.Fatal(err)
	}
	if n, err := store.CountPendingPDFs(ctx, pool, r.ID); err != nil || n != 2 {
		t.Fatalf("pending = %d, %v", n, err)
	}

	if err := store.FinishPDF(ctx, pool, r.ID, "en", []byte("%PDF-1.7\n"), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetPDF(ctx, pool, r.ID, "en")
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "ready" || got.JobID != 11 || got.FailureCode != nil || got.SHA256 == nil || got.SizeBytes == nil ||
		*got.SizeBytes != 9 || len(*got.SHA256) != 64 || !got.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("ready: %+v", got)
	}
	if err := store.FinishPDF(ctx, pool, r.ID, "en", []byte("%PDF-1.7\n"), now); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("finish twice: %v", err)
	}
	if err := store.FailPDF(ctx, pool, r.ID, "en", "PDF_COMPILE_FAILED", now); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("fail a ready row: %v", err)
	}
	if n, _ := store.CountPendingPDFs(ctx, pool, r.ID); n != 1 {
		t.Fatalf("pending = %d, want 1", n)
	}

	if err := store.FailPDF(ctx, pool, r.ID, "pt-BR", "PDF_COMPILE_FAILED", now); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetPDF(ctx, pool, r.ID, "pt-BR")
	if got.Status != "failed" || got.FailureCode == nil || *got.FailureCode != "PDF_COMPILE_FAILED" || got.SHA256 != nil {
		t.Fatalf("failed: %+v", got)
	}
	if err := store.RetryPDF(ctx, pool, r.ID, "en", 13, now); !errors.Is(err, store.ErrStaleAttempt) {
		t.Fatalf("retry a ready row: %v", err)
	}
	if err := store.RetryPDF(ctx, pool, r.ID, "pt-BR", 14, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetPDF(ctx, pool, r.ID, "pt-BR")
	if got.Status != "pending" || got.FailureCode != nil || got.JobID != 14 || !got.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("retried: %+v", got)
	}
	if _, err := store.GetPDF(ctx, pool, uuid.New(), "en"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing: %v", err)
	}
}

func TestLockReportRequestsSerialises(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()
	if err := store.LockReportRequests(ctx, tx1); err != nil {
		t.Fatal(err)
	}
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx2.Rollback(ctx) }()
	expectBlocksUntilCommit(t, tx1, func() error {
		return store.LockReportRequests(ctx, tx2)
	})
}

func TestLockReportRequestsLeavesWritesFree(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := claimedReport(t, pool, f, 1)

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()
	if err := store.LockReportRequests(ctx, tx1); err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		done <- store.Transact(ctx, pool, func(tx pgx.Tx) error {
			return store.AdvanceReport(ctx, tx, r.ID, 1, "collecting", now.Add(time.Hour))
		})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a write of a running report waited for the request lock")
	}
}

func TestIdempotencyKeyPerOwner(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	other, err := addUser(ctx, pool, "operator", "en")
	if err != nil {
		t.Fatal(err)
	}
	a := queuedReport(f, f.profile, f.revision)
	b := queuedReport(f, f.profile, f.revision)
	c := queuedReport(f, f.profile, f.revision)
	for _, r := range []store.Report{a, b, c} {
		mustInsertReport(t, pool, r)
	}

	if _, _, err := store.GetIdempotentReport(ctx, pool, f.owner, "k"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
	if err := store.InsertIdempotencyKey(ctx, pool, f.owner, "k", "sha-a", a.ID, now); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertIdempotencyKey(ctx, pool, other, "k", "sha-b", b.ID, now); err != nil {
		t.Fatal(err)
	}
	sum, got, err := store.GetIdempotentReport(ctx, pool, f.owner, "k")
	if err != nil || sum != "sha-a" || got.ID != a.ID {
		t.Fatalf("owner: %q, %v, %v", sum, got.ID, err)
	}
	sum, got, err = store.GetIdempotentReport(ctx, pool, other, "k")
	if err != nil || sum != "sha-b" || got.ID != b.ID {
		t.Fatalf("other: %q, %v, %v", sum, got.ID, err)
	}
	err = store.InsertIdempotencyKey(ctx, pool, f.owner, "k", "sha-c", c.ID, now)
	wantSQLState(t, "second insert for one owner", err, "23505")
}
