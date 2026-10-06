//go:build postgres

package worker

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river/rivertype"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

const (
	testKept       = 30 * 24 * time.Hour
	testSessionTTL = 12 * time.Hour
	absentJob      = int64(900000000)
)

func maintenanceEnv(t *testing.T) *env {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) {
		s.ExpiredReportKept = testKept
		s.SessionTTL = testSessionTTL
		s.JobMaxAttempts = 1
	})
	return e
}

// maintain runs one maintenance job as River would.
func (e *env) maintain() error {
	e.t.Helper()
	w := &MaintenanceWorker{Deps: e.deps}
	return w.Work(context.Background(), nil)
}

// seedReport inserts a report of the profile in the given state.
func (e *env) seedReport(status, phase string, jobID int64, expires time.Time, mod func(*store.Report)) store.Report {
	e.t.Helper()
	var revision uuid.UUID
	if err := e.pool.QueryRow(context.Background(), `SELECT current_revision_id FROM profiles WHERE id = 'prod'`).Scan(&revision); err != nil {
		e.t.Fatal(err)
	}
	r := store.Report{
		ID: uuid.New(), OwnerID: e.op.ID, ProfileID: "prod", ProfileRevisionID: revision, Request: []byte(`{}`),
		Status: status, Phase: phase, CollectJobID: jobID, CreatedAt: fixedNow.Add(-time.Hour),
		ExpiresAt: expires, UpdatedAt: fixedNow.Add(-time.Hour),
	}
	if status == "running" {
		r.Attempt = 1
	}
	if status == "failed" {
		code := "INTERNAL_FAILURE"
		r.FailureCode = &code
	}
	if mod != nil {
		mod(&r)
	}
	if err := store.InsertReport(context.Background(), e.pool, r); err != nil {
		e.t.Fatal(err)
	}
	return r
}

func (e *env) insertJobRow(state string) int64 {
	e.t.Helper()
	var id int64
	err := e.pool.QueryRow(context.Background(), `
INSERT INTO river_job (state, max_attempts, kind, args, finalized_at)
VALUES ($1::river_job_state, 3, 'render', '{}',
        CASE WHEN $1 IN ('discarded', 'cancelled', 'completed') THEN now() END)
RETURNING id`, state).Scan(&id)
	if err != nil {
		e.t.Fatal(err)
	}
	return id
}

func TestPanicAtTheLastAttemptIsReconciled(t *testing.T) {
	e, _, _ := scenarioEnv(t, func(s *settings.Settings) { s.JobMaxAttempts = 1 })
	e.start(collectHooks{afterClaim: func(context.Context, int) { panic("handler bug") }})
	report := e.request()
	e.waitJob(report.CollectJobID, rivertype.JobStateDiscarded)
	if got := e.report(report.ID); got.Status != "running" {
		t.Fatalf("report %+v before the maintenance run", got)
	}
	if global, profile, err := store.CountActiveReports(context.Background(), e.pool, "prod"); err != nil || global != 1 || profile != 1 {
		t.Fatalf("active %d %d, %v", global, profile, err)
	}

	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	got := e.report(report.ID)
	if got.Status != "failed" || got.Phase != "complete" || got.FailureCode == nil || *got.FailureCode != "JOB_ATTEMPTS_EXHAUSTED" ||
		got.CompletedAt == nil || !got.CompletedAt.Equal(fixedNow) {
		t.Fatalf("report %+v", got)
	}
	audits := e.audits(reports.ActionReportCollect)
	if len(audits) != 1 || audits[0].Outcome != "failed" || audits[0].ActorID != nil ||
		len(audits[0].Meta) != 1 || audits[0].Meta["code"] != "JOB_ATTEMPTS_EXHAUSTED" {
		t.Fatalf("audit %+v", audits)
	}
	if global, profile, err := store.CountActiveReports(context.Background(), e.pool, "prod"); err != nil || global != 0 || profile != 0 {
		t.Fatalf("active %d %d, %v", global, profile, err)
	}
}

func (e *env) seedSession(hash string, expires time.Time, revoked *time.Time) uuid.UUID {
	e.t.Helper()
	s := store.Session{
		ID: uuid.New(), TokenHash: []byte(hash), CSRFHash: []byte("c" + hash), UserID: e.op.ID,
		CreatedAt: fixedNow.Add(-48 * time.Hour), ExpiresAt: expires,
	}
	if revoked != nil {
		reason := "logout"
		s.RevokedAt, s.RevokedReason = revoked, &reason
	}
	if err := store.InsertSession(context.Background(), e.pool, s); err != nil {
		e.t.Fatal(err)
	}
	return s.ID
}

func (e *env) exists(query string, args ...any) bool {
	e.t.Helper()
	return e.count(query, args...) == 1
}

// seedDueRows inserts one row of every kind the job handles and returns the
// ids that matter.
type dueRows struct {
	stuck, rendering, toExpire, toPurge uuid.UUID
	oldSession, liveSession             uuid.UUID
}

func (e *env) seedDueRows() dueRows {
	e.t.Helper()
	ctx := context.Background()
	var d dueRows
	future := fixedNow.Add(24 * time.Hour)
	d.stuck = e.seedReport("running", "collecting", absentJob, future, nil).ID

	d.rendering = e.seedReport("collected", "rendering", absentJob, future, nil).ID
	if err := store.InsertSnapshot(ctx, e.pool, d.rendering, "1", []byte("{}"), fixedNow); err != nil {
		e.t.Fatal(err)
	}
	if err := store.InsertPendingPDF(ctx, e.pool, d.rendering, "en", e.insertJobRow("discarded"), fixedNow); err != nil {
		e.t.Fatal(err)
	}

	d.toExpire = e.seedReport("collected", "complete", absentJob, fixedNow, nil).ID
	if err := store.InsertSnapshot(ctx, e.pool, d.toExpire, "1", []byte("{}"), fixedNow); err != nil {
		e.t.Fatal(err)
	}

	expiredAt := fixedNow.Add(-testKept - time.Hour)
	d.toPurge = e.seedReport("failed", "complete", absentJob, fixedNow.Add(-testKept), func(r *store.Report) { r.ExpiredAt = &expiredAt }).ID
	if err := store.InsertIdempotencyKey(ctx, e.pool, e.op.ID, "purge-key", "sum", d.toPurge, fixedNow); err != nil {
		e.t.Fatal(err)
	}

	if err := store.UpsertThrottle(ctx, e.pool, store.Throttle{KeyHash: []byte("old"), WindowStartedAt: fixedNow.Add(-time.Hour), Failures: 1, UpdatedAt: fixedNow}); err != nil {
		e.t.Fatal(err)
	}
	old := fixedNow.Add(-testSessionTTL)
	d.oldSession = e.seedSession("old", old, nil)
	d.liveSession = e.seedSession("live", future, nil)
	return d
}

func TestMaintenanceRunsEveryStep(t *testing.T) {
	e := maintenanceEnv(t)
	d := e.seedDueRows()
	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	if got := e.report(d.stuck); got.Status != "failed" || got.FailureCode == nil || *got.FailureCode != "JOB_ATTEMPTS_EXHAUSTED" {
		t.Fatalf("stuck %+v", got)
	}
	row, err := store.GetPDF(context.Background(), e.pool, d.rendering, "en")
	if err != nil || row.Status != "failed" || row.FailureCode == nil || *row.FailureCode != "PDF_ATTEMPTS_EXHAUSTED" {
		t.Fatalf("pdf %+v, %v", row, err)
	}
	if got := e.report(d.rendering); got.Phase != "complete" || got.CompletedAt == nil {
		t.Fatalf("rendering %+v", got)
	}
	if got := e.report(d.toExpire); got.ExpiredAt == nil || !got.ExpiredAt.Equal(fixedNow) {
		t.Fatalf("expired %+v", got)
	}
	if e.count(`SELECT count(*) FROM report_snapshots WHERE report_id = $1`, d.toExpire) != 0 {
		t.Fatal("the snapshot of the expired report remains")
	}
	if e.count(`SELECT count(*) FROM reports WHERE id = $1`, d.toPurge)+e.count(`SELECT count(*) FROM report_idempotency_keys`) != 0 {
		t.Fatal("the purged report remains")
	}
	if e.count(`SELECT count(*) FROM login_rate_limits`) != 0 {
		t.Fatal("the old throttle row remains")
	}
	if e.exists(`SELECT count(*) FROM sessions WHERE id = $1`, d.oldSession) || !e.exists(`SELECT count(*) FROM sessions WHERE id = $1`, d.liveSession) {
		t.Fatal("wrong sessions purged")
	}

	collect := e.audits(reports.ActionReportCollect)
	if len(collect) != 1 || collect[0].Outcome != "failed" || collect[0].ActorID != nil || collect[0].Meta["code"] != "JOB_ATTEMPTS_EXHAUSTED" {
		t.Fatalf("collect audit %+v", collect)
	}
	render := e.audits(reports.ActionReportRender)
	if len(render) != 1 || render[0].Outcome != "failed" || render[0].ActorID != nil ||
		render[0].Meta["language"] != "en" || render[0].Meta["code"] != "PDF_ATTEMPTS_EXHAUSTED" || len(render[0].Meta) != 2 {
		t.Fatalf("render audit %+v", render)
	}
	for _, action := range []string{reports.ActionReportExpire, reports.ActionReportPurge} {
		got := e.audits(action)
		if len(got) != 1 || got[0].Outcome != "succeeded" || got[0].ActorID != nil || len(got[0].Meta) != 0 {
			t.Fatalf("%s audit %+v", action, got)
		}
	}
	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	if n := len(e.audits(reports.ActionReportExpire)) + len(e.audits(reports.ActionReportPurge)) + len(e.audits(reports.ActionReportCollect)); n != 3 {
		t.Fatalf("a second run wrote more rows: %d", n)
	}
}

func TestMaintenanceContinuesAfterAFailure(t *testing.T) {
	e := maintenanceEnv(t)
	d := e.seedDueRows()
	// With the table gone, the throttle step fails.
	if _, err := e.pool.Exec(context.Background(), `DROP TABLE login_rate_limits`); err != nil {
		t.Fatal(err)
	}
	if err := e.maintain(); err == nil {
		t.Fatal("the failed step returned no error")
	}
	if e.exists(`SELECT count(*) FROM sessions WHERE id = $1`, d.oldSession) {
		t.Fatal("the session step did not run after the failure")
	}
	if got := e.report(d.stuck); got.Status != "failed" {
		t.Fatalf("the steps before the failure did not run: %+v", got)
	}
}

func TestMaintenanceLeavesLiveJobs(t *testing.T) {
	e := maintenanceEnv(t)
	future := fixedNow.Add(24 * time.Hour)
	var kept []uuid.UUID
	for _, state := range []string{"available", "pending", "retryable", "scheduled", "running"} {
		status, phase := "queued", "waiting"
		if state == "running" {
			status, phase = "running", "collecting"
		}
		kept = append(kept, e.seedReport(status, phase, e.insertJobRow(state), future, nil).ID)
	}
	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	for _, id := range kept {
		if got := e.report(id); got.Status == "failed" {
			t.Fatalf("a live job's report was failed: %+v", got)
		}
	}
	if n := len(e.audits(reports.ActionReportCollect)); n != 0 {
		t.Fatalf("%d audit rows", n)
	}
}
