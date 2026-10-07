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
			err := store.FinishPDF(ctx, f.pool, r.ID, lang, []byte("%PDF-1.7 body"), fixedNow)
			if err != nil {
				t.Fatal(err)
			}
		case "failed":
			if err := store.FailPDF(ctx, f.pool, r.ID, lang, "PDF_COMPILE_FAILED", fixedNow); err != nil {
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

func TestRetryPDFMatrix(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	future := fixedNow.Add(time.Hour)

	if err := f.svc.RetryPDF(ctx, f.op, uuid.New(), "en"); asError(t, err).Code != "REPORT_NOT_FOUND" {
		t.Fatalf("absent: %v", err)
	}
	expired := f.seedReport(t, "collected", "complete", fixedNow, "failed")
	if err := f.svc.RetryPDF(ctx, f.op, expired, "en"); asError(t, err).Code != "REPORT_EXPIRED" {
		t.Fatalf("expired: %v", err)
	}
	queued := f.seedReport(t, "queued", "waiting", future, "")
	if err := f.svc.RetryPDF(ctx, f.op, queued, "en"); asError(t, err).Code != "SNAPSHOT_NOT_READY" {
		t.Fatalf("queued: %v", err)
	}
	failedReport := f.seedReport(t, "failed", "complete", future, "")
	if err := f.svc.RetryPDF(ctx, f.op, failedReport, "en"); asError(t, err).Code != "SNAPSHOT_NOT_READY" {
		t.Fatalf("failed report: %v", err)
	}

	pending := f.seedReport(t, "collected", "rendering", future, "pending")
	if err := f.svc.RetryPDF(ctx, f.op, pending, "en"); err != nil {
		t.Fatalf("pending: %v", err)
	}
	if len(f.jobs(t, "render")) != 0 || len(f.auditActions(t, ActionReportPDFRetry)) != 0 {
		t.Fatal("a pending retry wrote a row")
	}

	ready := f.seedReport(t, "collected", "complete", future, "ready")
	if err := f.svc.RetryPDF(ctx, f.op, ready, "en"); asError(t, err).Code != "PDF_RETRY_NOT_ALLOWED" {
		t.Fatalf("ready: %v", err)
	}
	if len(f.jobs(t, "render")) != 0 {
		t.Fatal("a refused retry queued a job")
	}

	failed := f.seedReport(t, "collected", "complete", future, "failed")
	if err := f.svc.RetryPDF(ctx, f.op, failed, "en"); err != nil {
		t.Fatalf("failed: %v", err)
	}
	jobs := f.jobs(t, "render")
	if len(jobs) != 1 || jobs[0].Queue != "reports" || jobs[0].Priority != 1 || jobs[0].MaxAttempts != 3 {
		t.Fatalf("render jobs = %+v", jobs)
	}
	var args RenderArgs
	if err := json.Unmarshal([]byte(jobs[0].Args), &args); err != nil || args.ReportID != failed || args.Language != "en" {
		t.Fatalf("job args = %s (%v)", jobs[0].Args, err)
	}
	row, err := store.GetPDF(ctx, f.pool, failed, "en")
	if err != nil {
		t.Fatal(err)
	}
	if row.Status != "pending" || row.FailureCode != nil || row.JobID != jobs[0].ID {
		t.Fatalf("row = %+v", row)
	}
	other, err := store.GetPDF(ctx, f.pool, failed, "pt-BR")
	if err != nil || other.Status != "failed" {
		t.Fatalf("the other language changed: %+v (%v)", other, err)
	}
	r, err := store.GetReport(ctx, f.pool, failed)
	if err != nil {
		t.Fatal(err)
	}
	if r.Phase != "rendering" || r.CompletedAt != nil || r.Status != "collected" {
		t.Fatalf("report = %+v", r)
	}
	events := f.auditActions(t, ActionReportPDFRetry)
	if len(events) != 1 {
		t.Fatalf("retry rows = %d", len(events))
	}
	var meta map[string]any
	if err := json.Unmarshal(events[0].Metadata, &meta); err != nil {
		t.Fatal(err)
	}
	if meta["language"] != "en" || len(meta) != 1 || *events[0].TargetID != failed.String() || *events[0].TargetType != "report" ||
		events[0].ActorID == nil || *events[0].ActorID != f.op.ID {
		t.Fatalf("audit = %+v %v", events[0], meta)
	}
}

func TestRetryPDFRefusals(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	if err := f.svc.RetryPDF(ctx, f.read, id, "en"); !errors.Is(err, auth.ErrForbidden) {
		t.Fatalf("reader: %v", err)
	}
	err := f.svc.RetryPDF(ctx, f.op, id, "fr")
	if e := asError(t, err); e.Code != "REQUEST_INVALID" || e.Fields["language"] != "invalid" {
		t.Fatalf("language: %+v", e)
	}
	if len(f.jobs(t, "render")) != 0 {
		t.Fatal("a refused retry queued a job")
	}
}

func TestRetryPDFTwiceAtOnce(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	const n = 10
	errs := make([]error, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs[i] = f.svc.RetryPDF(context.Background(), f.op, id, "en")
		}()
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("retry %d: %v", i, err)
		}
	}
	if len(f.jobs(t, "render")) != 1 || len(f.auditActions(t, ActionReportPDFRetry)) != 1 {
		t.Fatalf("jobs %d audit rows %d", len(f.jobs(t, "render")), len(f.auditActions(t, ActionReportPDFRetry)))
	}
}

func TestRetryPDFRollsBackTheJobOnALaterFailure(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	// The audit row names an actor that no user row holds, so it fails after
	// the job, the row, and the phase were written.
	ghost := auth.Principal{ID: uuid.New(), Username: "ghost", Role: auth.RoleOperator, Language: "en"}
	if err := f.svc.RetryPDF(ctx, ghost, id, "en"); err == nil {
		t.Fatal("no error")
	}
	if len(f.jobs(t, "render")) != 0 || len(f.auditActions(t, ActionReportPDFRetry)) != 0 {
		t.Fatal("a failed retry left a job or an audit row")
	}
	row, err := store.GetPDF(ctx, f.pool, id, "en")
	if err != nil || row.Status != "failed" || row.JobID != 1 {
		t.Fatalf("row = %+v (%v)", row, err)
	}
	r, err := store.GetReport(ctx, f.pool, id)
	if err != nil || r.Phase != "complete" || r.CompletedAt == nil {
		t.Fatalf("report = %+v (%v)", r, err)
	}
}

func TestRetryPDFAcceptsAnAdministrator(t *testing.T) {
	f := newFixture(t)
	id := f.seedReport(t, "collected", "complete", fixedNow.Add(time.Hour), "failed")
	admin := newPrincipal(t, f.pool, "root", auth.RoleAdministrator)
	if err := f.svc.RetryPDF(context.Background(), admin, id, "en"); err != nil {
		t.Fatal(err)
	}
}
