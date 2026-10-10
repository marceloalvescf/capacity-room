//go:build postgres

package worker

import (
	"context"
	"encoding/json"
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

// dueRows holds the ids that matter of what seedDueRows inserts.
type dueRows struct {
	stuck, rendering, toExpire, toPurge uuid.UUID
	expiredLate                         uuid.UUID
	oldSession, liveSession             uuid.UUID
}

// seedDueRows inserts one row of every kind the job handles.
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

	// Both reports were expired an hour after their deadline; the keeping
	// period runs from the moment each was marked expired.
	deadline := fixedNow.Add(-testKept - time.Hour)
	expiredAt := fixedNow.Add(-testKept)
	d.toPurge = e.seedReport("failed", "complete", absentJob, deadline, func(r *store.Report) { r.ExpiredAt = &expiredAt }).ID
	expiredLater := expiredAt.Add(time.Second)
	d.expiredLate = e.seedReport("failed", "complete", absentJob, deadline, func(r *store.Report) { r.ExpiredAt = &expiredLater }).ID
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
	if e.count(`SELECT count(*) FROM reports WHERE id = $1`, d.expiredLate) != 1 {
		t.Fatal("a report expired less than the keeping period ago was purged")
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
	expire := e.audits(reports.ActionReportExpire)
	if len(expire) != 1 || expire[0].Outcome != "succeeded" || expire[0].ActorID != nil || len(expire[0].Meta) != 0 {
		t.Fatalf("expire audit %+v", expire)
	}
	purge := e.audits(reports.ActionReportPurge)
	if len(purge) != 1 || purge[0].Outcome != "succeeded" || purge[0].ActorID != nil || len(purge[0].Meta) != 1 || purge[0].Meta["cause"] != "expired" {
		t.Fatalf("purge audit %+v", purge)
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
	if err := e.maintain(); err == nil || err.Error() != "maintenance: INTERNAL_FAILURE" {
		t.Fatalf("error %v, want maintenance: INTERNAL_FAILURE", err)
	}
	if e.exists(`SELECT count(*) FROM sessions WHERE id = $1`, d.oldSession) {
		t.Fatal("the session step did not run after the failure")
	}
	if got := e.report(d.stuck); got.Status != "failed" {
		t.Fatalf("the steps before the failure did not run: %+v", got)
	}
}

func TestMaintenanceDoesNotBlameARenderPastExpiry(t *testing.T) {
	e := maintenanceEnv(t)
	past := fixedNow.Add(-time.Minute)
	id := e.seedReport("collected", "rendering", absentJob, past, nil).ID
	if err := store.InsertSnapshot(context.Background(), e.pool, id, "1", []byte("{}"), fixedNow); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertPendingPDF(context.Background(), e.pool, id, "en", e.insertJobRow("completed"), fixedNow); err != nil {
		t.Fatal(err)
	}
	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	got := e.report(id)
	if got.ExpiredAt == nil || got.Phase != "complete" || got.CompletedAt == nil || !got.CompletedAt.Equal(fixedNow) {
		t.Fatalf("report %+v", got)
	}
	if e.count(`SELECT count(*) FROM report_pdfs WHERE report_id = $1`, id) != 0 || len(e.audits(reports.ActionReportRender)) != 0 {
		t.Fatal("a pdf row or a render audit row remains")
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

func (e *env) deleteMark(r store.Report) {
	e.t.Helper()
	if _, err := e.pool.Exec(context.Background(), `UPDATE reports SET deleted_at = $2 WHERE id = $1`, r.ID, fixedNow.Add(-time.Minute)); err != nil {
		e.t.Fatal(err)
	}
}

func (e *env) purgeCauses(id uuid.UUID) []any {
	e.t.Helper()
	events, err := store.ListAuditEvents(context.Background(), e.pool)
	if err != nil {
		e.t.Fatal(err)
	}
	var causes []any
	for _, ev := range events {
		if ev.Action != reports.ActionReportPurge || ev.TargetID == nil || *ev.TargetID != id.String() {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(ev.Metadata, &m); err != nil {
			e.t.Fatal(err)
		}
		causes = append(causes, m["cause"])
	}
	return causes
}

func TestSweepStepRemovesDeleted(t *testing.T) {
	e := maintenanceEnv(t)
	ctx := context.Background()
	future := fixedNow.Add(24 * time.Hour)
	queued := e.seedReport("queued", "waiting", absentJob, future, nil)
	failed := e.seedReport("failed", "complete", absentJob, future, nil)
	collected := e.seedReport("collected", "complete", absentJob, future, nil)
	live := e.insertJobRow("running")
	running := e.seedReport("running", "collecting", live, future, nil)
	live2 := e.insertJobRow("running")
	untouched := e.seedReport("queued", "waiting", live2, future, nil)
	if err := store.InsertSnapshot(ctx, e.pool, collected.ID, "1", []byte("{}"), fixedNow); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"en", "pt-BR"} {
		if err := store.InsertPendingPDF(ctx, e.pool, collected.ID, lang, absentJob, fixedNow); err != nil {
			t.Fatal(err)
		}
		if err := store.FinishPDF(ctx, e.pool, collected.ID, lang, absentJob, 0, []byte("%PDF-"+lang), fixedNow); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.InsertIdempotencyKey(ctx, e.pool, e.op.ID, "swept-key", "sum", queued.ID, fixedNow); err != nil {
		t.Fatal(err)
	}
	for _, r := range []store.Report{queued, failed, collected, running} {
		e.deleteMark(r)
	}

	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	for _, r := range []store.Report{queued, failed, collected} {
		if e.exists(`SELECT count(*) FROM reports WHERE id = $1`, r.ID) {
			t.Errorf("the deleted %s report remains", r.Status)
		}
		if c := e.purgeCauses(r.ID); len(c) != 1 || c[0] != "deleted" {
			t.Errorf("%s purge causes %v", r.Status, c)
		}
	}
	if e.count(`SELECT count(*) FROM report_snapshots`)+e.count(`SELECT count(*) FROM report_pdfs`)+e.count(`SELECT count(*) FROM report_idempotency_keys`) != 0 {
		t.Fatal("the rows of a swept report remain")
	}
	if !e.exists(`SELECT count(*) FROM reports WHERE id = $1`, running.ID) || !e.exists(`SELECT count(*) FROM reports WHERE id = $1`, untouched.ID) {
		t.Fatal("a report that must stay was removed")
	}
	if n := len(e.audits(reports.ActionReportPurge)); n != 3 {
		t.Fatalf("%d purge audit rows", n)
	}
	for _, a := range e.audits(reports.ActionReportPurge) {
		if a.ActorID != nil || a.Outcome != "succeeded" || len(a.Meta) != 1 {
			t.Fatalf("purge audit %+v", a)
		}
	}

	// The collect job of the running report ends; the next run removes it.
	if _, err := e.pool.Exec(ctx, `UPDATE river_job SET state = 'discarded', finalized_at = now() WHERE id = $1`, live); err != nil {
		t.Fatal(err)
	}
	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	if e.exists(`SELECT count(*) FROM reports WHERE id = $1`, running.ID) {
		t.Fatal("the deleted running report remains after its job ended")
	}
	if c := e.purgeCauses(running.ID); len(c) != 1 || c[0] != "deleted" {
		t.Fatalf("running purge causes %v", c)
	}
	if got := e.report(untouched.ID); got.Status != "queued" || got.DeletedAt != nil {
		t.Fatalf("report %+v", got)
	}
}

func TestPurgeNamesItsCause(t *testing.T) {
	e := maintenanceEnv(t)
	deadline := fixedNow.Add(-testKept - time.Hour)
	expiredAt := fixedNow.Add(-testKept)
	id := e.seedReport("failed", "complete", absentJob, deadline, func(r *store.Report) { r.ExpiredAt = &expiredAt }).ID
	if err := e.maintain(); err != nil {
		t.Fatal(err)
	}
	if c := e.purgeCauses(id); len(c) != 1 || c[0] != "expired" {
		t.Fatalf("purge causes %v", c)
	}
}
