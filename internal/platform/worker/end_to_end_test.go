//go:build postgres && template

package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/pipeline"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/render"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

const (
	endToEndWait     = 60 * time.Second
	endToEndRetained = 30 * 24 * time.Hour
)

// settableClock is a clock the test moves; every read takes the lock.
type settableClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *settableClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *settableClock) set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

// runMaintenance inserts one maintenance job through the client and waits
// until it completes.
func (e *env) runMaintenance() {
	e.t.Helper()
	res, err := e.client.Insert(context.Background(), MaintenanceArgs{},
		&river.InsertOpts{Queue: queueMaintenance, MaxAttempts: e.settings.JobMaxAttempts})
	if err != nil {
		e.t.Fatal(err)
	}
	e.waitJob(res.Job.ID, rivertype.JobStateCompleted)
}

// reportAudits returns the audit rows of target type report, in order.
func (e *env) reportAudits(id uuid.UUID) []store.AuditEvent {
	e.t.Helper()
	events, err := store.ListAuditEvents(context.Background(), e.pool)
	if err != nil {
		e.t.Fatal(err)
	}
	var out []store.AuditEvent
	for _, ev := range events {
		if ev.TargetType != nil && *ev.TargetType == "report" && ev.TargetID != nil && *ev.TargetID == id.String() {
			out = append(out, ev)
		}
	}
	return out
}

func TestReportEndToEnd(t *testing.T) {
	clock := &settableClock{now: fixedNow}
	e, sw, _ := scenarioEnv(t, func(s *settings.Settings) {
		clientSettings(s)
		s.WorkerSlots = 2
		s.ExpiredReportKept = endToEndRetained
	})
	tc := realToolchain(t)
	e.deps.Now = clock.Now
	e.deps.Toolchain = tc
	e.deps.TempDir = t.TempDir()
	e.reports.Now = clock.Now

	acc, err := e.reports.Request(context.Background(), e.op, uuid.NewString(),
		reports.Selection{ProfileID: "prod", Namespaces: []string{"payments"}, Language: ptrTo(langEN)})
	if err != nil {
		t.Fatal(err)
	}
	id := acc.ID
	requested := e.report(id)

	client := e.newClient(collectHooks{})
	ctx := context.Background()
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.StopAndCancel(stopCtx); err != nil {
			t.Errorf("stop client: %v", err)
		}
	})

	deadline := time.Now().Add(endToEndWait)
	var got store.Report
	for {
		got = e.report(id)
		if got.Status == "collected" && got.Phase == "complete" {
			break
		}
		if got.Status == "failed" || time.Now().After(deadline) {
			t.Fatalf("report did not complete: %+v", got)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got.CompletedAt == nil || got.FailureCode != nil {
		t.Fatalf("report %+v", got)
	}

	_, body, sum, err := store.GetSnapshot(ctx, e.pool, id)
	if err != nil {
		t.Fatal(err)
	}
	if sum != digestOf(body) {
		t.Fatalf("snapshot sha256 %s", sum)
	}

	// The same run, made by the pipeline on a fresh server of the scenario.
	sw.set(promtest.NewServer(scenarioFixture(t)))
	rev, err := e.profiles.Revision(ctx, requested.ProfileRevisionID)
	if err != nil {
		t.Fatal(err)
	}
	var r reports.Resolved
	if err := json.Unmarshal(requested.Request, &r); err != nil {
		t.Fatal(err)
	}
	cfg := rev.Document.RunConfig(r.Period, r.Timezone, r.ReportProfile, r.MinSeverity, r.Namespaces)
	opts, err := e.profiles.Source(rev.Document, cfg.Metrics)
	if err != nil {
		t.Fatal(err)
	}
	window, err := collect.NewWindow(cfg.Analysis, fixedNow)
	if err != nil {
		t.Fatal(err)
	}
	want, err := pipeline.Collect(ctx, pipeline.Run{
		Options: opts, Config: cfg, Window: window, EmittedAt: fixedNow,
		Now:     clock.Now,
		NewID:   func() (string, error) { return id.String(), nil },
		Version: testVersion,
	})
	if err != nil {
		t.Fatal(err)
	}
	wantBody, err := snapshot.Encode(want.Snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(body, wantBody) {
		t.Fatalf("the stored snapshot (%d bytes) differs from the pipeline's (%d bytes)", len(body), len(wantBody))
	}

	for lang, cat := range map[string]catalog.Language{langEN: catalog.English} {
		row := e.pdf(id, lang)
		if row.Status != "ready" {
			t.Fatalf("%s row %+v", lang, row)
		}
		data, err := render.Document(want.Snapshot, cat, document.Options{})
		if err != nil {
			t.Fatal(err)
		}
		wantPDF, err := render.Compile(ctx, tc, t.TempDir(), data)
		if err != nil {
			t.Fatal(err)
		}
		if pdf := e.pdfBody(id, lang); !bytes.Equal(pdf, wantPDF) {
			t.Fatalf("%s: the stored PDF has %d bytes, the compile in the test %d", lang, len(pdf), len(wantPDF))
		}
	}

	// Expiry: the clock reaches expires_at, and one maintenance run expires
	// the report.
	clock.set(got.ExpiresAt)
	e.runMaintenance()
	expired := e.report(id)
	if expired.ExpiredAt == nil || !expired.ExpiredAt.Equal(got.ExpiresAt) || expired.Status != "collected" || expired.Phase != "complete" {
		t.Fatalf("expired report %+v", expired)
	}
	if n := e.count(`SELECT count(*) FROM report_snapshots WHERE report_id = $1`, id); n != 0 {
		t.Fatalf("%d snapshot rows after expiry", n)
	}
	if n := e.count(`SELECT count(*) FROM report_pdfs WHERE report_id = $1`, id); n != 0 {
		t.Fatalf("%d PDF rows after expiry", n)
	}

	// Purge: the clock moves expired_report_days past the moment the report
	// was marked expired.
	clock.set(expired.ExpiredAt.Add(endToEndRetained))
	e.runMaintenance()
	if n := e.count(`SELECT count(*) FROM reports WHERE id = $1`, id); n != 0 {
		t.Fatalf("%d report rows after the purge", n)
	}
	if n := e.count(`SELECT count(*) FROM report_idempotency_keys WHERE report_id = $1`, id); n != 0 {
		t.Fatalf("%d idempotency rows after the purge", n)
	}

	var actions, languages []string
	for _, ev := range e.reportAudits(id) {
		actions = append(actions, ev.Action)
		var m map[string]any
		if err := json.Unmarshal(ev.Metadata, &m); err != nil {
			t.Fatal(err)
		}
		if ev.Action == reports.ActionReportPurge && (len(m) != 1 || m["cause"] != "expired") {
			t.Fatalf("purge audit metadata %v", m)
		}
		if ev.Action != reports.ActionReportRender {
			continue
		}
		if ev.Outcome != "succeeded" || ev.ActorID != nil {
			t.Fatalf("render audit %+v", ev)
		}
		languages = append(languages, m["language"].(string))
	}
	wantActions := []string{
		reports.ActionReportSubmit, reports.ActionReportCollect,
		reports.ActionReportRender,
		reports.ActionReportExpire, reports.ActionReportPurge,
	}
	if !slices.Equal(actions, wantActions) {
		t.Fatalf("audit actions %v, want %v", actions, wantActions)
	}
	slices.Sort(languages)
	if !slices.Equal(languages, []string{langEN}) {
		t.Fatalf("render languages %v", languages)
	}
}
