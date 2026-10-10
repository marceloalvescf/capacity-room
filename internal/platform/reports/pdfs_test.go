//go:build postgres

package reports

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// seedReport writes a report with the given state and one PDF row per
// language in the given PDF state; an empty pdfState writes no row.
func (f *fixture) seedReport(t *testing.T, status, phase string, expiresAt time.Time, pdfState string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	revID := f.addProfileOnce(t)
	r := store.Report{
		ID: uuid.New(), OwnerID: f.op.ID, ProfileID: "prod", ProfileRevisionID: revID,
		Request: []byte(`{"language":"en"}`), Status: status, Phase: phase, CollectJobID: 1,
		CreatedAt: fixedNow, ExpiresAt: expiresAt, UpdatedAt: fixedNow,
	}
	if status == "collected" && phase == "complete" {
		done := fixedNow
		r.CompletedAt = &done
	}
	if status == "failed" {
		code := "INTERNAL_FAILURE"
		r.FailureCode = &code
	}
	if err := store.InsertReport(ctx, f.pool, r); err != nil {
		t.Fatal(err)
	}
	if pdfState == "" {
		return r.ID
	}
	for _, lang := range []string{"pt-BR", "en"} {
		if err := store.InsertPendingPDF(ctx, f.pool, r.ID, lang, 1, fixedNow); err != nil {
			t.Fatal(err)
		}
		switch pdfState {
		case "ready":
			err := store.FinishPDF(ctx, f.pool, r.ID, lang, 1, 0, []byte("%PDF-1.7 body"), fixedNow)
			if err != nil {
				t.Fatal(err)
			}
		case "failed":
			if err := store.FailPDF(ctx, f.pool, r.ID, lang, 1, 0, "PDF_COMPILE_FAILED", fixedNow); err != nil {
				t.Fatal(err)
			}
		}
	}
	return r.ID
}

func (f *fixture) addProfileOnce(t *testing.T) uuid.UUID {
	t.Helper()
	rev, err := store.GetCurrentRevision(context.Background(), f.pool, "prod")
	if errors.Is(err, store.ErrNotFound) {
		return f.addProfile(t, "prod", baseDocument())
	}
	if err != nil {
		t.Fatal(err)
	}
	return rev.ID
}

// seedOneLanguage is seedReport with the PDF row of pt-BR removed, so the
// report holds the PDF of its own language alone.
func (f *fixture) seedOneLanguage(t *testing.T, status, phase string, expiresAt time.Time, pdfState string) uuid.UUID {
	t.Helper()
	id := f.seedReport(t, status, phase, expiresAt, pdfState)
	f.exec(t, `DELETE FROM report_pdfs WHERE report_id = $1 AND language = 'pt-BR'`, id)
	return id
}

func (f *fixture) onlyAudit(t *testing.T, id uuid.UUID, language string) {
	t.Helper()
	events := f.auditActions(t, ActionReportPDFRequest)
	if len(events) != 1 {
		t.Fatalf("request rows = %d", len(events))
	}
	var meta map[string]any
	if err := json.Unmarshal(events[0].Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["language"] != language || len(meta) != 1 || *events[0].TargetID != id.String() || *events[0].TargetType != "report" ||
		events[0].ActorID == nil || *events[0].ActorID != f.op.ID {
		t.Fatalf("audit = %+v %v", events[0], meta)
	}
}

func (f *fixture) onlyRenderJob(t *testing.T, id uuid.UUID, language string) int64 {
	t.Helper()
	jobs := f.jobs(t, "render")
	if len(jobs) != 1 || jobs[0].Queue != "reports" || jobs[0].Priority != 1 || jobs[0].MaxAttempts != 3 {
		t.Fatalf("render jobs = %+v", jobs)
	}
	var args RenderArgs
	if err := json.Unmarshal([]byte(jobs[0].Args), &args); err != nil || args.ReportID != id || args.Language != language {
		t.Fatalf("job args = %s (%v)", jobs[0].Args, err)
	}
	return jobs[0].ID
}

func TestRequestPDFCreatesAMissingLanguage(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.seedOneLanguage(t, "collected", "complete", fixedNow.Add(time.Hour), "ready")
	if err := f.svc.RequestPDF(ctx, f.op, id, "pt-BR"); err != nil {
		t.Fatal(err)
	}
	jobID := f.onlyRenderJob(t, id, "pt-BR")
	row, err := store.GetPDF(ctx, f.pool, id, "pt-BR")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "pending" || row.FailureCode != nil || row.JobID != jobID {
		t.Fatalf("row = %+v", row)
	}
	other, err := store.GetPDF(ctx, f.pool, id, "en")
	if err != nil || other.Status != "ready" {
		t.Fatalf("the other language changed: %+v (%v)", other, err)
	}
	r, err := store.GetReport(ctx, f.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Phase != "rendering" || r.CompletedAt != nil || r.Status != "collected" {
		t.Fatalf("report = %+v", r)
	}
	f.onlyAudit(t, id, "pt-BR")
}

func TestRequestPDFRetriesAFailedOne(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	if err := f.svc.RequestPDF(ctx, f.op, id, "en"); err != nil {
		t.Fatal(err)
	}
	jobID := f.onlyRenderJob(t, id, "en")
	row, err := store.GetPDF(ctx, f.pool, id, "en")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "pending" || row.FailureCode != nil || row.JobID != jobID {
		t.Fatalf("row = %+v", row)
	}
	other, err := store.GetPDF(ctx, f.pool, id, "pt-BR")
	if err != nil || other.Status != "failed" {
		t.Fatalf("the other language changed: %+v (%v)", other, err)
	}
	r, err := store.GetReport(ctx, f.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Phase != "rendering" || r.CompletedAt != nil || r.Status != "collected" {
		t.Fatalf("report = %+v", r)
	}
	f.onlyAudit(t, id, "en")
}

func TestRequestPDFWhilePending(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "rendering", fixedNow.Add(time.Hour), "pending")
	if err := f.svc.RequestPDF(context.Background(), f.op, id, "en"); err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(f.jobs(t, "render")) != 0 || len(f.auditActions(t, ActionReportPDFRequest)) != 0 {
		t.Fatal("a pending request wrote a row")
	}
}

func TestRequestPDFWhenReady(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "ready")
	if err := f.svc.RequestPDF(context.Background(), f.op, id, "en"); asError(t, err).Code != "PDF_ALREADY_READY" {
		t.Fatalf("ready: %v", err)
	}
	if len(f.jobs(t, "render")) != 0 || len(f.auditActions(t, ActionReportPDFRequest)) != 0 {
		t.Fatal("a refused request wrote a row")
	}
}

func TestRequestPDFNeedsACollectedReport(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	future := fixedNow.Add(time.Hour)
	if err := f.svc.RequestPDF(ctx, f.op, uuid.New(), "en"); asError(t, err).Code != "REPORT_NOT_FOUND" {
		t.Fatalf("absent: %v", err)
	}
	running := f.seedReport(t, "running", "collecting", future, "")
	if err := f.svc.RequestPDF(ctx, f.op, running, "en"); asError(t, err).Code != "SNAPSHOT_NOT_READY" {
		t.Fatalf("running: %v", err)
	}
	queued := f.seedReport(t, "queued", "waiting", future, "")
	if err := f.svc.RequestPDF(ctx, f.op, queued, "en"); asError(t, err).Code != "SNAPSHOT_NOT_READY" {
		t.Fatalf("queued: %v", err)
	}
	failedReport := f.seedReport(t, "failed", "complete", future, "")
	if err := f.svc.RequestPDF(ctx, f.op, failedReport, "en"); asError(t, err).Code != "SNAPSHOT_NOT_READY" {
		t.Fatalf("failed report: %v", err)
	}
	expired := f.seedReport(t, "collected", "complete", fixedNow, "failed")
	if err := f.svc.RequestPDF(ctx, f.op, expired, "en"); asError(t, err).Code != "REPORT_EXPIRED" {
		t.Fatalf("expired: %v", err)
	}
	deleted := f.seedReport(t, "collected", "complete", future, "failed")
	f.markDeleted(t, deleted)
	if err := f.svc.RequestPDF(ctx, f.op, deleted, "en"); asError(t, err).Code != "REPORT_NOT_FOUND" {
		t.Fatalf("deleted: %v", err)
	}
	if len(f.jobs(t, "render")) != 0 {
		t.Fatal("a refused request queued a job")
	}
}

func TestRequestPDFNeedsAnOperator(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	if err := f.svc.RequestPDF(ctx, f.read, id, "en"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("reader: %v", err)
	}
	err := f.svc.RequestPDF(ctx, f.op, id, "fr")
	if e := asError(t, err); e.Code != "REQUEST_INVALID" || e.Fields["language"] != "invalid" {
		t.Fatalf("language: %+v", e)
	}
	if len(f.jobs(t, "render")) != 0 {
		t.Fatal("a refused request queued a job")
	}
}

func TestRequestPDFTwiceAtOnce(t *testing.T) {
	f := newFixture(t)
	id := f.seedOneLanguage(t, "collected", "complete", fixedNow.Add(time.Hour), "ready")
	const n = 10
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f.svc.RequestPDF(context.Background(), f.op, id, "pt-BR")
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
	if len(f.jobs(t, "render")) != 1 || len(f.auditActions(t, ActionReportPDFRequest)) != 1 {
		t.Fatalf("jobs %d audit rows %d", len(f.jobs(t, "render")), len(f.auditActions(t, ActionReportPDFRequest)))
	}
}

func TestRequestPDFRollsBackOnALaterFailure(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.seedOneLanguage(t, "collected", "complete", fixedNow.Add(time.Hour), "ready")
	// The audit row names an actor that no user row holds, so it fails after
	// the job, the row, and the phase were written.
	ghost := auth.Principal{ID: uuid.New(), Username: "ghost", Role: auth.RoleOperator, Language: "en"}
	if err := f.svc.RequestPDF(ctx, ghost, id, "pt-BR"); err == nil {
		t.Fatal("no error")
	}
	if len(f.jobs(t, "render")) != 0 || len(f.auditActions(t, ActionReportPDFRequest)) != 0 {
		t.Fatal("a failed request left a job or an audit row")
	}
	if _, err := store.GetPDF(ctx, f.pool, id, "pt-BR"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("row after the rollback: %v", err)
	}
	r, err := store.GetReport(ctx, f.pool, id)
	if err != nil || r.Phase != "complete" || r.CompletedAt == nil {
		t.Fatalf("report = %+v (%v)", r, err)
	}
}

func TestRequestPDFAcceptsAnAdministrator(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	admin := newPrincipal(t, f.pool, "root", auth.RoleAdministrator)
	if err := f.svc.RequestPDF(context.Background(), admin, id, "en"); err != nil {
		t.Fatal(err)
	}
}
