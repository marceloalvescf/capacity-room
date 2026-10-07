//go:build postgres && template

package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river/rivertype"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
)

func TestExpiryBeforeRenderWritesNoPDF(t *testing.T) {
	e := maintenanceEnv(t)
	var mu sync.Mutex
	held := 0
	release := make(chan struct{})
	hooks := renderHooks{beforeStore: func(ctx context.Context, _ string) {
		mu.Lock()
		held++
		mu.Unlock()
		select {
		case <-release:
		case <-ctx.Done():
		}
	}}
	e.startRender(both(pdfToolchain(t)), 2, hooks)
	report := e.collected()

	deadline := time.Now().Add(waitLimit)
	for {
		mu.Lock()
		n := held
		mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d renders reached the hold, want 1", n)
		}
		time.Sleep(20 * time.Millisecond)
	}

	jobs := map[string]int64{langPT: e.pdfJob(report.ID, langPT)}
	late := &MaintenanceWorker{Deps: e.deps}
	late.Deps.Now = func() time.Time { return report.ExpiresAt }
	if err := late.Work(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if got := e.report(report.ID); got.ExpiredAt == nil || got.Phase != "complete" || got.CompletedAt == nil || !got.CompletedAt.Equal(report.ExpiresAt) {
		t.Fatalf("report %+v was not expired and completed", got)
	}
	close(release)

	for _, lang := range []string{langPT} {
		job := e.waitJob(jobs[lang], rivertype.JobStateCompleted)
		if job.Attempt != 1 {
			t.Fatalf("%s job attempt %d", lang, job.Attempt)
		}
	}
	if n := e.count(`SELECT count(*) FROM report_pdfs WHERE report_id = $1`, report.ID); n != 0 {
		t.Fatalf("%d pdf rows after the expiry", n)
	}
	if n := len(e.audits(reports.ActionReportRender)); n != 0 {
		t.Fatalf("%d render audit rows", n)
	}
}

func TestDeleteWhileRunningIsSweptAfter(t *testing.T) {
	e, _, _ := scenarioEnv(t, nil)
	var reportID uuid.UUID
	e.startRoutes(collectHooks{beforeCollected: func(ctx context.Context, _ int) {
		if err := e.reports.Delete(ctx, e.op, reportID); err != nil {
			t.Error(err)
		}
	}}, both(pdfToolchain(t)), 2, renderHooks{})
	report := e.request()
	reportID = report.ID
	e.waitJob(report.CollectJobID, rivertype.JobStateCompleted)
	for _, lang := range []string{langPT} {
		e.waitJob(e.pdfJob(report.ID, lang), rivertype.JobStateCompleted)
		if row := e.pdf(report.ID, lang); row.Status != "pending" {
			t.Fatalf("%s row %+v", lang, row)
		}
	}
	got := e.report(report.ID)
	if got.Status != "collected" || got.DeletedAt == nil {
		t.Fatalf("report %+v", got)
	}
	if n := len(e.audits(reports.ActionReportRender)); n != 0 {
		t.Fatalf("%d render audit rows", n)
	}
	if n := e.count(`SELECT count(*) FROM audit_events WHERE action = $1`, reports.ActionReportDelete); n != 1 {
		t.Fatalf("%d delete audit rows", n)
	}

	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"reports", "report_snapshots", "report_pdfs", "report_idempotency_keys"} {
		if n := e.count(`SELECT count(*) FROM ` + table); n != 0 {
			t.Fatalf("%d rows left in %s", n, table)
		}
	}
	purges := e.audits(reports.ActionReportPurge)
	if len(purges) != 1 || purges[0].Meta["cause"] != "deleted" {
		t.Fatalf("purge audit %+v", purges)
	}
}
