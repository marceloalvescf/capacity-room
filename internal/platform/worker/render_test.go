//go:build postgres && template

package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/render"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

const (
	langPT = "pt-BR"
	langEN = "en"
)

// scriptToolchain writes a shell script that stands for the compiler. Its
// variable out holds the path the compiler is asked to write.
func scriptToolchain(t *testing.T, body string) typst.Toolchain {
	t.Helper()
	script := "#!/bin/sh\nout=\"\"\nfor a in \"$@\"; do out=\"$a\"; done\n" + body + "\n"
	binary := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return typst.Toolchain{Binary: binary}
}

func pdfToolchain(t *testing.T) typst.Toolchain {
	return scriptToolchain(t, `printf '%%PDF-1.7 fake' > "$out"`)
}

func realToolchain(t *testing.T) typst.Toolchain {
	t.Helper()
	tc, err := typst.Locate(os.Getenv, exec.LookPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := tc.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	return tc
}

// renderRoute registers one render worker per language and holds the jobs of
// a language until it is released.
type renderRoute struct {
	river.WorkerDefaults[reports.RenderArgs]
	byLang map[string]*RenderWorker
	gates  map[string]chan struct{}
}

func (r *renderRoute) Timeout(job *river.Job[reports.RenderArgs]) time.Duration {
	return r.byLang[job.Args.Language].Timeout(job)
}

func (r *renderRoute) Work(ctx context.Context, job *river.Job[reports.RenderArgs]) error {
	if g := r.gates[job.Args.Language]; g != nil {
		select {
		case <-g:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return r.byLang[job.Args.Language].Work(ctx, job)
}

func (r *renderRoute) release(langs ...string) {
	for _, l := range langs {
		close(r.gates[l])
	}
}

// startRender runs a real River client with the collect worker and the render
// route, and stops it when the test ends. gated languages wait for release.
func (e *env) startRender(toolchains map[string]typst.Toolchain, workers int, hooks renderHooks, gated ...string) *renderRoute {
	e.t.Helper()
	if e.deps.Settings.RenderTimeout == 0 {
		e.deps.Settings.RenderTimeout = time.Minute
	}
	route := &renderRoute{byLang: map[string]*RenderWorker{}, gates: map[string]chan struct{}{}}
	for lang, tc := range toolchains {
		d := e.deps
		d.Toolchain = tc
		d.TempDir = e.t.TempDir()
		route.byLang[lang] = &RenderWorker{Deps: d, hooks: hooks}
	}
	for _, l := range gated {
		route.gates[l] = make(chan struct{})
	}
	wk := river.NewWorkers()
	river.AddWorker(wk, &CollectWorker{Deps: e.deps})
	river.AddWorker(wk, route)
	client, err := river.NewClient(riverpgxv5.New(e.pool), &river.Config{
		TestOnly:     true,
		Logger:       slog.New(slog.NewTextHandler(e.logs, nil)),
		Queues:       map[string]river.QueueConfig{"reports": {MaxWorkers: workers}},
		Workers:      wk,
		ErrorHandler: e.errs,
	})
	if err != nil {
		e.t.Fatal(err)
	}
	if err := client.Start(context.Background()); err != nil {
		e.t.Fatal(err)
	}
	e.client = client
	e.t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.StopAndCancel(ctx); err != nil {
			e.t.Errorf("stop client: %v", err)
		}
	})
	return route
}

func both(tc typst.Toolchain) map[string]typst.Toolchain {
	return map[string]typst.Toolchain{langPT: tc, langEN: tc}
}

// collected queues a report and waits for its collection.
func (e *env) collected() store.Report {
	e.t.Helper()
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	return report
}

func (e *env) pdf(id uuid.UUID, lang string) store.ReportPDF {
	e.t.Helper()
	row, err := store.GetPDF(context.Background(), e.pool, id, lang)
	if err != nil {
		e.t.Fatal(err)
	}
	return row
}

// waitPDF waits for the row to leave pending and returns it.
func (e *env) waitPDF(id uuid.UUID, lang, status string) store.ReportPDF {
	e.t.Helper()
	deadline := time.Now().Add(waitLimit)
	for time.Now().Before(deadline) {
		if row := e.pdf(id, lang); row.Status == status {
			return row
		}
		time.Sleep(20 * time.Millisecond)
	}
	e.t.Fatalf("%s row stayed %s, want %s", lang, e.pdf(id, lang).Status, status)
	return store.ReportPDF{}
}

func (e *env) pdfBody(id uuid.UUID, lang string) []byte {
	e.t.Helper()
	var body []byte
	if err := e.pool.QueryRow(context.Background(), `SELECT body FROM report_pdfs WHERE report_id = $1 AND language = $2`, id, lang).Scan(&body); err != nil {
		e.t.Fatal(err)
	}
	return body
}

func (e *env) pdfJob(id uuid.UUID, lang string) int64 {
	return e.pdf(id, lang).JobID
}

func (e *env) assertFailedPDF(id uuid.UUID, lang, code string) {
	e.t.Helper()
	row := e.waitPDF(id, lang, "failed")
	if row.FailureCode == nil || *row.FailureCode != code {
		e.t.Fatalf("%s row %+v", lang, row)
	}
	if body := e.pdfBody(id, lang); body != nil {
		e.t.Fatalf("%s row holds %d bytes", lang, len(body))
	}
	job := e.waitJob(e.pdfJob(id, lang), rivertype.JobStateCompleted)
	if job.Attempt != 1 {
		e.t.Fatalf("%s job attempt %d", lang, job.Attempt)
	}
	var found bool
	for _, a := range e.audits(reports.ActionReportRender) {
		if a.Outcome == "failed" && a.Meta["language"] == lang && a.Meta["code"] == code && a.ActorID == nil {
			found = true
		}
	}
	if !found {
		e.t.Fatalf("no failed audit row for %s %s: %+v", lang, code, e.audits(reports.ActionReportRender))
	}
}

func TestRenderStoresThePDF(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	tc := realToolchain(t)
	e.startRender(both(tc), 2, renderHooks{}, langPT)
	report := e.collected()
	row := e.waitPDF(report.ID, langEN, "ready")

	_, raw, _, err := store.GetSnapshot(context.Background(), e.pool, report.ID)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := snapshot.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	data, err := render.Document(snap, catalog.English, document.Options{})
	if err != nil {
		t.Fatal(err)
	}
	want, err := render.Compile(context.Background(), tc, t.TempDir(), data)
	if err != nil {
		t.Fatal(err)
	}
	body := e.pdfBody(report.ID, langEN)
	if !bytes.Equal(body, want) {
		t.Fatalf("stored PDF has %d bytes, the compile in the test %d", len(body), len(want))
	}
	if row.SHA256 == nil || *row.SHA256 != digestOf(body) || row.SizeBytes == nil || *row.SizeBytes != int64(len(body)) {
		t.Fatalf("row %+v", row)
	}
	got := e.report(report.ID)
	if got.Phase != "rendering" || got.CompletedAt != nil {
		t.Fatalf("report %+v", got)
	}
	if pt := e.pdf(report.ID, langPT); pt.Status != "pending" {
		t.Fatalf("pt-BR row %+v", pt)
	}
	e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)
	audits := e.audits(reports.ActionReportRender)
	if len(audits) != 1 || audits[0].Outcome != "succeeded" || audits[0].ActorID != nil ||
		len(audits[0].Meta) != 1 || audits[0].Meta["language"] != langEN {
		t.Fatalf("audit %+v", audits)
	}
}

func TestRenderCompletesTheReport(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	e.startRender(both(pdfToolchain(t)), 2, renderHooks{})
	report := e.collected()
	e.waitPDF(report.ID, langPT, "ready")
	e.waitPDF(report.ID, langEN, "ready")
	e.waitJob(e.pdfJob(report.ID, langPT), rivertype.JobStateCompleted)
	e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)
	got := e.report(report.ID)
	if got.Status != "collected" || got.Phase != "complete" || got.CompletedAt == nil || !got.CompletedAt.Equal(fixedNow) {
		t.Fatalf("report %+v", got)
	}
	if n := len(e.audits(reports.ActionReportRender)); n != 2 {
		t.Fatalf("%d render audit rows", n)
	}
}

// A deadline that passes inside the transaction that stores a PDF does not
// turn the finished render into a failure.
func TestRenderSuccessOutlivesTheDeadline(t *testing.T) {
	const timeout = 2 * time.Second
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.RenderTimeout = timeout })
	e.startRender(both(pdfToolchain(t)), 2, renderHooks{insideStore: func(string) {
		time.Sleep(timeout + time.Second)
	}})
	report := e.collected()
	e.waitPDF(report.ID, langPT, "ready")
	e.waitPDF(report.ID, langEN, "ready")
	e.waitJob(e.pdfJob(report.ID, langPT), rivertype.JobStateCompleted)
	e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)
	if got := e.report(report.ID); got.Phase != "complete" || got.CompletedAt == nil {
		t.Fatalf("report %+v", got)
	}
}

// barrier holds the first arrivals until n have come.
type barrier struct {
	n       int32
	arrived atomic.Int32
	open    chan struct{}
	once    sync.Once
}

func newBarrier(n int32) *barrier { return &barrier{n: n, open: make(chan struct{})} }

func (b *barrier) wait(ctx context.Context) {
	if b.arrived.Add(1) >= b.n {
		b.once.Do(func() { close(b.open) })
	}
	select {
	case <-b.open:
	case <-ctx.Done():
	case <-time.After(waitLimit):
	}
}

func TestRenderBothLanguagesCompleteOnce(t *testing.T) {
	e, sw, _ := scenarioEnv(t, nil)
	var current atomic.Pointer[barrier]
	hooks := renderHooks{beforeStore: func(ctx context.Context, _ string) {
		if b := current.Load(); b != nil {
			b.wait(ctx)
		}
	}}
	e.startRender(both(pdfToolchain(t)), 2, hooks)
	for run := 0; run < 20; run++ {
		current.Store(newBarrier(2))
		sw.set(promtest.NewServer(scenarioFixture(t)))
		report := e.collected()
		e.waitPDF(report.ID, langPT, "ready")
		e.waitPDF(report.ID, langEN, "ready")
		e.waitJob(e.pdfJob(report.ID, langPT), rivertype.JobStateCompleted)
		e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)
		got := e.report(report.ID)
		if got.Phase != "complete" || got.CompletedAt == nil {
			t.Fatalf("run %d: report %+v", run, got)
		}
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE action = $1`, reports.ActionReportRender); n != 40 {
		t.Fatalf("%d render audit rows", n)
	}
}

func TestRenderCompileFailure(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	route := e.startRender(map[string]typst.Toolchain{
		langPT: scriptToolchain(t, "exit 1"),
		langEN: pdfToolchain(t),
	}, 2, renderHooks{}, langEN)
	report := e.collected()
	e.assertFailedPDF(report.ID, langPT, "PDF_COMPILE_FAILED")
	if row := e.pdf(report.ID, langEN); row.Status != "pending" || row.FailureCode != nil {
		t.Fatalf("en row %+v", row)
	}
	if got := e.report(report.ID); got.Phase != "rendering" || got.CompletedAt != nil {
		t.Fatalf("report %+v", got)
	}

	route.release(langEN)
	e.waitPDF(report.ID, langEN, "ready")
	e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)
	if got := e.report(report.ID); got.Phase != "complete" || got.CompletedAt == nil {
		t.Fatalf("report %+v", got)
	}
}

func TestRenderTimeout(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.RenderTimeout = time.Second; s.JobMaxAttempts = 3 })
	e.startRender(map[string]typst.Toolchain{
		langPT: scriptToolchain(t, "exec /bin/sleep 60"),
		langEN: pdfToolchain(t),
	}, 2, renderHooks{}, langEN)
	report := e.collected()
	e.assertFailedPDF(report.ID, langPT, "PDF_COMPILE_FAILED")
	job, err := e.client.JobGet(context.Background(), e.pdfJob(report.ID, langPT))
	if err != nil {
		t.Fatal(err)
	}
	if len(job.Errors) != 0 {
		t.Fatalf("river recorded %+v against the job", job.Errors)
	}
}

func TestRenderTooLarge(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	e.startRender(map[string]typst.Toolchain{
		langPT: scriptToolchain(t, `{ printf '%%PDF-'; head -c 67108865 /dev/zero; } > "$out"`),
		langEN: pdfToolchain(t),
	}, 2, renderHooks{}, langEN)
	report := e.collected()
	e.assertFailedPDF(report.ID, langPT, "PDF_TOO_LARGE")
	if row := e.pdf(report.ID, langPT); row.SHA256 != nil || row.SizeBytes != nil {
		t.Fatalf("row %+v", row)
	}
}

func TestRenderDigestMismatch(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 1 })
	route := e.startRender(both(pdfToolchain(t)), 2, renderHooks{}, langPT, langEN)
	report := e.collected()
	ctx := context.Background()
	for _, stmt := range []string{
		`ALTER TABLE report_snapshots DISABLE TRIGGER report_snapshots_are_immutable`,
		`UPDATE report_snapshots SET body = body || '\x20'::bytea, size_bytes = size_bytes + 1`,
	} {
		if _, err := e.pool.Exec(ctx, stmt); err != nil {
			t.Fatal(err)
		}
	}
	route.release(langPT)
	e.assertFailedPDF(report.ID, langPT, "INTERNAL_FAILURE")
	if row := e.pdf(report.ID, langEN); row.Status != "pending" {
		t.Fatalf("en row %+v", row)
	}
}

func TestRenderSkipsAnExpiredReport(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	route := e.startRender(both(pdfToolchain(t)), 2, renderHooks{}, langPT, langEN)
	report := e.collected()
	if _, err := e.pool.Exec(context.Background(), `UPDATE reports SET expired_at = $2 WHERE id = $1`, report.ID, fixedNow); err != nil {
		t.Fatal(err)
	}
	route.release(langPT, langEN)
	for _, lang := range []string{langPT, langEN} {
		job := e.waitJob(e.pdfJob(report.ID, lang), rivertype.JobStateCompleted)
		if job.Attempt != 1 {
			t.Fatalf("%s attempt %d", lang, job.Attempt)
		}
		if row := e.pdf(report.ID, lang); row.Status != "pending" {
			t.Fatalf("%s row %+v", lang, row)
		}
	}
	if n := len(e.audits(reports.ActionReportRender)); n != 0 {
		t.Fatalf("%d render audit rows", n)
	}
	if got := e.report(report.ID); got.Phase != "rendering" {
		t.Fatalf("report %+v", got)
	}
	if !strings.Contains(e.logs.String(), report.ID.String()) {
		t.Fatalf("no log line names the report:\n%s", e.logs.String())
	}
}

func TestRenderSkipsAMissingReport(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	e.startRender(both(pdfToolchain(t)), 2, renderHooks{})
	id := uuid.New()
	inserted, err := e.reports.Queue.Insert(context.Background(), reports.RenderArgs{ReportID: id, Language: langEN}, reports.RenderInsertOpts(e.settings))
	if err != nil {
		t.Fatal(err)
	}
	e.waitJob(inserted.Job.ID, rivertype.JobStateCompleted)
	for _, table := range []string{"reports", "report_snapshots", "report_pdfs"} {
		if n := e.count(`SELECT count(*) FROM ` + table); n != 0 {
			t.Fatalf("%d rows in %s", n, table)
		}
	}
	if !strings.Contains(e.logs.String(), id.String()) {
		t.Fatalf("no log line names the report:\n%s", e.logs.String())
	}
}

func TestRenderAfterARetry(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	flag := filepath.Join(t.TempDir(), "compiler-works")
	e.startRender(map[string]typst.Toolchain{
		langPT: scriptToolchain(t, `if [ -e `+flag+` ]; then printf '%%PDF-1.7 fake' > "$out"; else exit 1; fi`),
		langEN: pdfToolchain(t),
	}, 2, renderHooks{})
	report := e.collected()
	e.assertFailedPDF(report.ID, langPT, "PDF_COMPILE_FAILED")
	e.waitPDF(report.ID, langEN, "ready")
	e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)
	if got := e.report(report.ID); got.Phase != "complete" {
		t.Fatalf("report %+v", got)
	}
	if err := os.WriteFile(flag, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := e.reports.RetryPDF(context.Background(), e.op, report.ID, langPT); err != nil {
		t.Fatal(err)
	}
	if got := e.report(report.ID); got.Phase != "rendering" || got.CompletedAt != nil {
		t.Fatalf("report after the retry request: %+v", got)
	}
	e.waitPDF(report.ID, langPT, "ready")
	e.waitJob(e.pdfJob(report.ID, langPT), rivertype.JobStateCompleted)
	got := e.report(report.ID)
	if got.Phase != "complete" || got.CompletedAt == nil {
		t.Fatalf("report %+v", got)
	}
	if body := e.pdfBody(report.ID, langPT); !bytes.HasPrefix(body, []byte("%PDF-")) {
		t.Fatalf("body %q", body)
	}
}

func TestRenderStaleRowWritesNothing(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	var once sync.Once
	hooks := renderHooks{beforeStore: func(ctx context.Context, lang string) {
		if lang != langEN {
			return
		}
		once.Do(func() {
			// Another run finished the row while this attempt compiled.
			err := store.Transact(ctx, e.pool, func(tx pgx.Tx) error {
				_, err := tx.Exec(ctx, `UPDATE report_pdfs SET status = 'failed', failure_code = 'PDF_COMPILE_FAILED' WHERE language = 'en'`)
				return err
			})
			if err != nil {
				t.Error(err)
			}
		})
	}}
	e.startRender(both(pdfToolchain(t)), 2, hooks, langPT)
	report := e.collected()
	e.waitJob(e.pdfJob(report.ID, langEN), rivertype.JobStateCompleted)

	var stale bool
	for _, err := range e.errs.all() {
		if err.Error() == "render: STALE_ATTEMPT" && errors.Is(err, ErrStaleAttempt) {
			stale = true
		}
	}
	if !stale {
		t.Fatalf("river recorded %v", e.errs.all())
	}
	row := e.pdf(report.ID, langEN)
	if row.Status != "failed" || row.SHA256 != nil || e.pdfBody(report.ID, langEN) != nil {
		t.Fatalf("row %+v", row)
	}
	if n := len(e.audits(reports.ActionReportRender)); n != 0 {
		t.Fatalf("%d render audit rows", n)
	}
	if got := e.report(report.ID); got.Phase != "rendering" {
		t.Fatalf("report %+v", got)
	}
}
