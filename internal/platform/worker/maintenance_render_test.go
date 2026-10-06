//go:build postgres && template

package worker

import (
	"context"
	"sync"
	"testing"
	"time"

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
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d renders reached the hold, want 2", n)
		}
		time.Sleep(20 * time.Millisecond)
	}

	jobs := map[string]int64{langPT: e.pdfJob(report.ID, langPT), langEN: e.pdfJob(report.ID, langEN)}
	late := &MaintenanceWorker{Deps: e.deps}
	late.Deps.Now = func() time.Time { return report.ExpiresAt }
	if err := late.Work(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	if got := e.report(report.ID); got.ExpiredAt == nil {
		t.Fatalf("report %+v was not expired", got)
	}
	close(release)

	for _, lang := range []string{langPT, langEN} {
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
