//go:build postgres

package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func markDeleted(t *testing.T, pool *pgxpool.Pool, id uuid.UUID, at time.Time) string {
	t.Helper()
	var status string
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		status, err = store.MarkReportDeleted(context.Background(), tx, id, at)
		return err
	})
	return status
}

func sha(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

func usernameOf(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) string {
	t.Helper()
	var name string
	if err := pool.QueryRow(context.Background(), `SELECT username FROM users WHERE id = $1`, id).Scan(&name); err != nil {
		t.Fatal(err)
	}
	return name
}

func itemIDs(items []store.ReportListItem) []uuid.UUID {
	ids := make([]uuid.UUID, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func TestListReportsNewestFirst(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	var made []store.Report
	for i := range 5 {
		r := queuedReport(f, f.profile, f.revision)
		r.CreatedAt = now.Add(time.Duration(i) * time.Minute)
		mustInsertReport(t, pool, r)
		made = append(made, r)
	}
	collected := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"pt-BR": "ready", "en": "failed"}, nil)
	if _, err := pool.Exec(ctx, `UPDATE reports SET created_at = $2 WHERE id = $1`, collected.ID, now.Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	markDeleted(t, pool, made[2].ID, now)

	items, err := store.ListReports(ctx, pool, nil, 50)
	if err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{collected.ID, made[4].ID, made[3].ID, made[1].ID, made[0].ID}
	if got := itemIDs(items); !slices.Equal(got, want) {
		t.Fatalf("ids = %v, want %v", got, want)
	}
	for _, it := range items {
		if it.OwnerUsername != usernameOf(t, pool, f.owner) {
			t.Fatalf("owner %q", it.OwnerUsername)
		}
	}
	pdfs := items[0].PDFs
	if len(pdfs) != 2 || pdfs[0].Language != "en" || pdfs[0].Status != "failed" || pdfs[1].Language != "pt-BR" || pdfs[1].Status != "ready" {
		t.Fatalf("pdf rows %+v", pdfs)
	}
	if pdfs[1].SHA256 == nil || *pdfs[1].SHA256 != sha("%PDF-") {
		t.Fatalf("pdf digest %v", pdfs[1].SHA256)
	}
	if len(items[1].PDFs) != 0 {
		t.Fatalf("pdf rows of a queued report: %+v", items[1].PDFs)
	}
	if empty, err := store.ListReports(ctx, pool, &store.ReportCursor{CreatedAt: now.Add(-time.Hour), ID: uuid.Nil}, 5); err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("empty page: %#v, %v", empty, err)
	}
	if limited, err := store.ListReports(ctx, pool, nil, 2); err != nil || !slices.Equal(itemIDs(limited), want[:2]) {
		t.Fatalf("limit 2: %v, %v", itemIDs(limited), err)
	}
}

func TestListReportsKeysetOnEqualInstants(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	var want []string
	for range 7 {
		r := queuedReport(f, f.profile, f.revision)
		mustInsertReport(t, pool, r)
		want = append(want, r.ID.String())
	}
	slices.Sort(want)
	slices.Reverse(want)

	var got []string
	var sizes []int
	var after *store.ReportCursor
	for range 5 {
		items, err := store.ListReports(ctx, pool, after, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(items) == 0 {
			break
		}
		sizes = append(sizes, len(items))
		for _, it := range items {
			got = append(got, it.ID.String())
		}
		last := items[len(items)-1]
		after = &store.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	if !slices.Equal(sizes, []int{3, 3, 1}) {
		t.Fatalf("page sizes %v", sizes)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("ids %v, want %v", got, want)
	}
}

func TestListReportsCursorOfADeletedRow(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	var made []store.Report
	for i := range 4 {
		r := queuedReport(f, f.profile, f.revision)
		r.CreatedAt = now.Add(time.Duration(i) * time.Minute)
		mustInsertReport(t, pool, r)
		made = append(made, r)
	}
	page, err := store.ListReports(ctx, pool, nil, 2)
	if err != nil {
		t.Fatal(err)
	}
	last := page[1]
	if last.ID != made[2].ID {
		t.Fatalf("page 1 ends at %v", last.ID)
	}
	markDeleted(t, pool, last.ID, now)
	next, err := store.ListReports(ctx, pool, &store.ReportCursor{CreatedAt: last.CreatedAt, ID: last.ID}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if want := []uuid.UUID{made[1].ID, made[0].ID}; !slices.Equal(itemIDs(next), want) {
		t.Fatalf("page 2 = %v, want %v", itemIDs(next), want)
	}
}

// queryTracer records the statement and the arguments of every query.
type queryTracer struct {
	mu   sync.Mutex
	sqls []string
	args [][]any
}

func (q *queryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.sqls = append(q.sqls, data.SQL)
	q.args = append(q.args, data.Args)
	return ctx
}

func (q *queryTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestListReportsUsesTheIndex(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	if _, err := pool.Exec(ctx, `
INSERT INTO reports (id, owner_id, profile_id, profile_revision_id, request, status, phase, attempt, collect_job_id,
                     created_at, expires_at, updated_at)
SELECT gen_random_uuid(), $1, $2, $3, '{}', 'queued', 'waiting', 0, 1,
       $4::timestamptz + n * interval '1 second', $4::timestamptz + interval '7 days', $4
FROM generate_series(1, 10000) AS n`, f.owner, f.profile, f.revision, now); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE reports`); err != nil {
		t.Fatal(err)
	}

	cfg := pool.Config()
	tracer := &queryTracer{}
	cfg.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(traced.Close)

	cursor := &store.ReportCursor{CreatedAt: now.Add(5000 * time.Second), ID: uuid.New()}
	for name, after := range map[string]*store.ReportCursor{"first page": nil, "next page": cursor} {
		tracer.mu.Lock()
		tracer.sqls, tracer.args = nil, nil
		tracer.mu.Unlock()
		if _, err := store.ListReports(ctx, traced, after, 50); err != nil {
			t.Fatal(err)
		}
		tracer.mu.Lock()
		var sql string
		var args []any
		for i, s := range tracer.sqls {
			if strings.Contains(s, "FROM reports r") {
				sql, args = s, tracer.args[i]
			}
		}
		tracer.mu.Unlock()
		if sql == "" {
			t.Fatalf("%s: the list query was not seen", name)
		}
		rows, err := pool.Query(ctx, "EXPLAIN "+sql, args...)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var plan []string
		for rows.Next() {
			var line string
			if err := rows.Scan(&line); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, line)
		}
		rows.Close()
		text := strings.Join(plan, "\n")
		if !strings.Contains(text, "ix_reports_listing") {
			t.Fatalf("%s: the plan does not use ix_reports_listing:\n%s", name, text)
		}
		if strings.Contains(text, "Sort") {
			t.Fatalf("%s: the plan sorts:\n%s", name, text)
		}
	}
}

func TestGetReportItemHidesDeleted(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready"}, nil)

	item, err := store.GetReportItem(ctx, pool, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.OwnerUsername != usernameOf(t, pool, f.owner) || len(item.PDFs) != 1 || item.PDFs[0].Language != "en" {
		t.Fatalf("item %+v", item)
	}
	markDeleted(t, pool, r.ID, now)
	if _, err := store.GetReportItem(ctx, pool, r.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if _, err := store.GetReportItem(ctx, pool, uuid.New()); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
	if got, err := store.GetReport(ctx, pool, r.ID); err != nil || got.DeletedAt == nil || !got.DeletedAt.Equal(now) {
		t.Fatalf("GetReport of a deleted row: %+v, %v", got, err)
	}
}

func TestReadBodiesAreOneStatement(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	snapSum, pdfSum := sha("{}"), sha("%PDF-")
	var bodies, absent atomic.Int64

	for round := range 200 {
		r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready"}, nil)
		stop := make(chan struct{})
		failures := make(chan string, 1)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			fail := func(msg string) {
				select {
				case failures <- msg:
				default:
				}
			}
			for {
				select {
				case <-stop:
					return
				default:
				}
				snap, err := store.ReadSnapshot(ctx, pool, r.ID, now)
				switch {
				case errors.Is(err, store.ErrNotFound):
					absent.Add(1)
				case err != nil:
					fail(fmt.Sprintf("snapshot read: %v", err))
				case snap.Body == nil && snap.Status == "collected" && snap.ExpiredAt == nil && snap.ExpiresAt.After(now):
					fail("a collected, unexpired report read without a body")
				case snap.Body != nil:
					bodies.Add(1)
					if got := sha(string(snap.Body)); got != snap.SHA256 || got != snapSum {
						fail(fmt.Sprintf("snapshot body %q digest %s, stored %s", snap.Body, got, snap.SHA256))
					}
				case snap.SHA256 != "":
					fail("snapshot digest without a body")
				}
				pdf, err := store.ReadPDF(ctx, pool, r.ID, "en", now)
				switch {
				case errors.Is(err, store.ErrNotFound):
					absent.Add(1)
				case err != nil:
					fail(fmt.Sprintf("pdf read: %v", err))
				case pdf.Body == nil && pdf.Status == "collected" && pdf.ExpiredAt == nil && pdf.ExpiresAt.After(now):
					fail("a collected, unexpired report read without a pdf body")
				case pdf.Body != nil:
					if pdf.PDF == nil || pdf.PDF.SHA256 == nil || sha(string(pdf.Body)) != *pdf.PDF.SHA256 || *pdf.PDF.SHA256 != pdfSum {
						fail(fmt.Sprintf("pdf body %q does not match its row %+v", pdf.Body, pdf.PDF))
					}
				}
			}
		}()
		at := now.Add(2 * time.Hour)
		if ids := expireDue(t, pool, at, 100); len(ids) != 1 || ids[0] != r.ID {
			t.Fatalf("round %d: expired %v", round, ids)
		}
		inTx(t, pool, func(tx pgx.Tx) error {
			_, err := store.PurgeExpiredReports(ctx, tx, at, 0, 100)
			return err
		})
		close(stop)
		wg.Wait()
		select {
		case msg := <-failures:
			t.Fatalf("round %d: %s", round, msg)
		default:
		}
	}
	if bodies.Load() == 0 {
		t.Fatal("no read ever saw a body: the race was not exercised")
	}
	t.Logf("%d whole bodies and %d absent reports read", bodies.Load(), absent.Load())
}

func TestReadSnapshotStates(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)

	queued := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, queued)
	got, err := store.ReadSnapshot(ctx, pool, queued.ID, now)
	if err != nil || got.Status != "queued" || got.Body != nil || got.SHA256 != "" || !got.ExpiresAt.Equal(queued.ExpiresAt) {
		t.Fatalf("queued: %+v, %v", got, err)
	}

	collected := collectedReport(t, pool, f, now.Add(time.Hour), nil, nil)
	got, err = store.ReadSnapshot(ctx, pool, collected.ID, now)
	if err != nil || got.Status != "collected" || string(got.Body) != "{}" || got.SHA256 != sha("{}") || got.ExpiredAt != nil {
		t.Fatalf("collected: %+v, %v", got, err)
	}
	if got.ExpiresAt.Location() != time.UTC {
		t.Fatalf("expiry not in UTC: %v", got.ExpiresAt)
	}

	expired := collectedReport(t, pool, f, now, nil, nil)
	got, err = store.ReadSnapshot(ctx, pool, expired.ID, now)
	if err != nil || got.Status != "collected" || got.Body != nil || got.SHA256 != "" || !got.ExpiresAt.Equal(now) {
		t.Fatalf("expired at now: %+v, %v", got, err)
	}

	maintained := collectedReport(t, pool, f, now.Add(time.Hour), nil, nil)
	if ids := expireDue(t, pool, now.Add(2*time.Hour), 100); !slices.Contains(ids, maintained.ID) {
		t.Fatalf("expired %v", ids)
	}
	flagged := collectedReport(t, pool, f, now.Add(time.Hour), nil, nil)
	if _, err := pool.Exec(ctx, `UPDATE reports SET expired_at = $2 WHERE id = $1`, flagged.ID, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadSnapshot(ctx, pool, flagged.ID, now)
	if err != nil || got.Body != nil || got.SHA256 != "" || got.ExpiredAt == nil {
		t.Fatalf("expired_at set while the rows remain: %+v, %v", got, err)
	}
	got, err = store.ReadSnapshot(ctx, pool, maintained.ID, now)
	if err != nil || got.Status != "collected" || got.Body != nil || got.SHA256 != "" || got.ExpiredAt == nil || !got.ExpiredAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("expired by maintenance after the reader's clock: %+v, %v", got, err)
	}

	markDeleted(t, pool, collected.ID, now)
	if _, err := store.ReadSnapshot(ctx, pool, collected.ID, now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
	if _, err := store.ReadSnapshot(ctx, pool, uuid.New(), now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
}

func TestReadPDFStates(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "pending", "pt-BR": "failed"},
		map[string]int64{"en": 3})

	none := collectedReport(t, pool, f, now.Add(time.Hour), nil, nil)
	got, err := store.ReadPDF(ctx, pool, none.ID, "en", now)
	if err != nil || got.PDF != nil || got.Body != nil || got.Status != "collected" || !got.ExpiresAt.Equal(none.ExpiresAt) {
		t.Fatalf("absent row: %+v, %v", got, err)
	}
	got, err = store.ReadPDF(ctx, pool, r.ID, "en", now)
	if err != nil || got.PDF == nil || got.PDF.Status != "pending" || got.PDF.JobID != 3 || got.Body != nil {
		t.Fatalf("pending: %+v, %v", got, err)
	}
	got, err = store.ReadPDF(ctx, pool, r.ID, "pt-BR", now)
	if err != nil || got.PDF == nil || got.PDF.Status != "failed" || got.PDF.FailureCode == nil ||
		*got.PDF.FailureCode != "PDF_COMPILE_FAILED" || got.Body != nil {
		t.Fatalf("failed: %+v, %v", got, err)
	}

	ready := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready"}, nil)
	got, err = store.ReadPDF(ctx, pool, ready.ID, "en", now)
	if err != nil || got.PDF == nil || got.PDF.Status != "ready" || string(got.Body) != "%PDF-" ||
		got.PDF.SHA256 == nil || *got.PDF.SHA256 != sha("%PDF-") || got.PDF.SizeBytes == nil || *got.PDF.SizeBytes != 5 {
		t.Fatalf("ready: %+v, %v", got, err)
	}
	got, err = store.ReadPDF(ctx, pool, ready.ID, "en", now.Add(time.Hour))
	if err != nil || got.PDF == nil || got.Body != nil || !got.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("ready at its expiry: %+v, %v", got, err)
	}

	maintained := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready"}, nil)
	if ids := expireDue(t, pool, now.Add(2*time.Hour), 100); !slices.Contains(ids, maintained.ID) {
		t.Fatalf("expired %v", ids)
	}
	flagged := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready"}, nil)
	if _, err := pool.Exec(ctx, `UPDATE reports SET expired_at = $2 WHERE id = $1`, flagged.ID, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadPDF(ctx, pool, flagged.ID, "en", now)
	if err != nil || got.Body != nil || got.PDF == nil || got.ExpiredAt == nil {
		t.Fatalf("expired_at set while the rows remain: %+v, %v", got, err)
	}
	got, err = store.ReadPDF(ctx, pool, maintained.ID, "en", now)
	if err != nil || got.Status != "collected" || got.Body != nil || got.ExpiredAt == nil || !got.ExpiredAt.Equal(now.Add(2*time.Hour)) {
		t.Fatalf("expired by maintenance after the reader's clock: %+v, %v", got, err)
	}

	markDeleted(t, pool, ready.ID, now)
	if _, err := store.ReadPDF(ctx, pool, ready.ID, "en", now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("deleted: %v", err)
	}
}

func TestMarkReportDeletedOnce(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := claimedReport(t, pool, f, 1)

	if status := markDeleted(t, pool, r.ID, now.Add(time.Hour)); status != "running" {
		t.Fatalf("status %q", status)
	}
	err := store.Transact(context.Background(), pool, func(tx pgx.Tx) error {
		_, err := store.MarkReportDeleted(context.Background(), tx, r.ID, now.Add(2*time.Hour))
		return err
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("second mark: %v", err)
	}
	if got := getReportT(t, pool, r.ID); got.DeletedAt == nil || !got.DeletedAt.Equal(now.Add(time.Hour)) ||
		!got.UpdatedAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("deleted_at %v, updated_at %v, want both %v", got.DeletedAt, got.UpdatedAt, now.Add(time.Hour))
	}
	err = store.Transact(context.Background(), pool, func(tx pgx.Tx) error {
		_, err := store.MarkReportDeleted(context.Background(), tx, uuid.New(), now)
		return err
	})
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("absent: %v", err)
	}
}

func TestMarkReportDeletedConcurrent(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)

	const workers = 20
	var wg sync.WaitGroup
	start := make(chan struct{})
	var marked, missing, other atomic.Int64
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			err := store.Transact(ctx, pool, func(tx pgx.Tx) error {
				_, err := store.MarkReportDeleted(ctx, tx, r.ID, now)
				return err
			})
			switch {
			case err == nil:
				marked.Add(1)
			case errors.Is(err, store.ErrNotFound):
				missing.Add(1)
			default:
				other.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	if marked.Load() != 1 || missing.Load() != workers-1 || other.Load() != 0 {
		t.Fatalf("marked %d, not found %d, other %d", marked.Load(), missing.Load(), other.Load())
	}
}

func TestClaimRefusesADeletedReport(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, r)
	markDeleted(t, pool, r.ID, now)
	before := getReportT(t, pool, r.ID)

	if err := claim(ctx, pool, r.ID, 1, now.Add(time.Minute)); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("claim: %v", err)
	}
	if after := getReportT(t, pool, r.ID); !reflect.DeepEqual(before, after) {
		t.Fatalf("row changed: %+v -> %+v", before, after)
	}
	if err := claim(ctx, pool, uuid.New(), 1, now); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("claim of an absent report: %v", err)
	}
}

func sweep(t *testing.T, pool *pgxpool.Pool, limit int) []uuid.UUID {
	t.Helper()
	var ids []uuid.UUID
	inTx(t, pool, func(tx pgx.Tx) (err error) {
		ids, err = store.SweepDeletedReports(context.Background(), tx, limit)
		return err
	})
	return ids
}

func TestSweepSkipsRunning(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	queued := activeReport(t, pool, f, "queued", 1, now)
	running := activeReport(t, pool, f, "running", insertJob(t, pool, "running"), now)
	collected := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "ready", "pt-BR": "pending"}, map[string]int64{"pt-BR": 4})
	failed := claimedReport(t, pool, f, 1)
	if err := store.FailReport(ctx, pool, failed.ID, 1, "INTERNAL_FAILURE", now); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertIdempotencyKey(ctx, pool, f.owner, "key-q", "digest", queued.ID, now); err != nil {
		t.Fatal(err)
	}
	for i, id := range []uuid.UUID{queued.ID, running.ID, collected.ID, failed.ID} {
		markDeleted(t, pool, id, now.Add(time.Duration(i)*time.Minute))
	}

	ids := sweep(t, pool, 100)
	want := []uuid.UUID{queued.ID, collected.ID, failed.ID}
	if !slices.Equal(ids, want) {
		t.Fatalf("swept %v, want %v", ids, want)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM reports`); n != 1 {
		t.Fatalf("%d reports left", n)
	}
	if _, err := store.GetReport(ctx, pool, running.ID); err != nil {
		t.Fatalf("running report: %v", err)
	}
	for _, q := range []string{`SELECT count(*) FROM report_pdfs`, `SELECT count(*) FROM report_snapshots`, `SELECT count(*) FROM report_idempotency_keys`} {
		if n := countRows(t, pool, q); n != 0 {
			t.Fatalf("%s = %d", q, n)
		}
	}
	if again := sweep(t, pool, 100); len(again) != 0 {
		t.Fatalf("second sweep %v", again)
	}
}

func TestSweepOrderAndLimit(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	var made []store.Report
	for range 4 {
		r := queuedReport(f, f.profile, f.revision)
		mustInsertReport(t, pool, r)
		made = append(made, r)
	}
	// Marked in the reverse of creation order.
	for i, r := range made {
		markDeleted(t, pool, r.ID, now.Add(time.Duration(len(made)-i)*time.Minute))
	}
	ids := sweep(t, pool, 3)
	want := []uuid.UUID{made[3].ID, made[2].ID, made[1].ID}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	slices.SortFunc(want, func(a, b uuid.UUID) int { return strings.Compare(a.String(), b.String()) })
	if !slices.Equal(ids, want) {
		t.Fatalf("swept %v, want %v", ids, want)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM reports WHERE id = $1`, made[0].ID); n != 1 {
		t.Fatalf("the newest mark was swept")
	}
}

func TestSweepSkipsLocked(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	locked := queuedReport(f, f.profile, f.revision)
	other := queuedReport(f, f.profile, f.revision)
	mustInsertReport(t, pool, locked)
	mustInsertReport(t, pool, other)
	markDeleted(t, pool, locked.ID, now)
	markDeleted(t, pool, other.ID, now)
	holdReportLock(t, pool, locked.ID)

	var ids []uuid.UUID
	boundedTx(t, pool, func(ctx context.Context, tx pgx.Tx) (err error) {
		ids, err = store.SweepDeletedReports(ctx, tx, 100)
		return err
	})
	if len(ids) != 1 || ids[0] != other.ID {
		t.Fatalf("swept %v, want only %v", ids, other.ID)
	}
	if _, err := store.GetReport(context.Background(), pool, locked.ID); err != nil {
		t.Fatalf("locked report: %v", err)
	}
}

func TestReconcileAndExpireLeaveDeletedAlone(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	queued := activeReport(t, pool, f, "queued", deletedJob(t, pool), now)
	pending := collectedReport(t, pool, f, now.Add(2*time.Hour), map[string]string{"en": "pending"}, map[string]int64{"en": deletedJob(t, pool)})
	due := collectedReport(t, pool, f, now.Add(time.Minute), map[string]string{"en": "ready"}, nil)
	running := activeReport(t, pool, f, "running", deletedJob(t, pool), now)
	for _, id := range []uuid.UUID{queued.ID, pending.ID, due.ID, running.ID} {
		markDeleted(t, pool, id, now)
	}

	if ids := reconcileReports(t, pool, 100); len(ids) != 0 {
		t.Fatalf("reconcile failed %v", ids)
	}
	if keys := reconcilePDFs(t, pool, 100); len(keys) != 0 {
		t.Fatalf("reconcile failed pdf rows %v", keys)
	}
	if ids := expireDue(t, pool, now.Add(time.Hour), 100); len(ids) != 0 {
		t.Fatalf("expired %v", ids)
	}
	wantUntouched(t, "queued", getReportT(t, pool, queued.ID), "queued")
	wantUntouched(t, "running", getReportT(t, pool, running.ID), "running")
	if p, err := store.GetPDF(context.Background(), pool, pending.ID, "en"); err != nil || p.Status != "pending" {
		t.Fatalf("pending row %+v, %v", p, err)
	}
	if got := getReportT(t, pool, due.ID); got.ExpiredAt != nil {
		t.Fatalf("expired a deleted report: %+v", got)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM report_snapshots`); n != 2 {
		t.Fatalf("%d snapshots", n)
	}
}

func TestSweepTakesADeletedRunningReportWhoseJobEnded(t *testing.T) {
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	gone := activeReport(t, pool, f, "running", deletedJob(t, pool), now)
	final := activeReport(t, pool, f, "running", insertJob(t, pool, "completed"), now)
	discarded := activeReport(t, pool, f, "running", insertJob(t, pool, "discarded"), now)
	cancelled := activeReport(t, pool, f, "running", insertJob(t, pool, "cancelled"), now)
	live := activeReport(t, pool, f, "running", insertJob(t, pool, "running"), now)
	available := activeReport(t, pool, f, "running", insertJob(t, pool, "available"), now)
	for i, r := range []store.Report{gone, final, discarded, cancelled, live, available} {
		markDeleted(t, pool, r.ID, now.Add(time.Duration(i)*time.Minute))
	}

	ids := sweep(t, pool, 100)
	want := []uuid.UUID{gone.ID, final.ID, discarded.ID, cancelled.ID}
	if !slices.Equal(ids, want) {
		t.Fatalf("swept %v, want %v", ids, want)
	}
	if n := countRows(t, pool, `SELECT count(*) FROM reports WHERE id = ANY($1)`, []uuid.UUID{live.ID, available.ID}); n != 2 {
		t.Fatalf("%d of the two reports with a live job are left", n)
	}
}
