package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// The codes the reconcile step stores for a job that River gave up on.
const (
	codeJobAttemptsExhausted = "JOB_ATTEMPTS_EXHAUSTED"
	codePDFAttemptsExhausted = "PDF_ATTEMPTS_EXHAUSTED"
)

// The most rows each step handles in one run.
const (
	reportBatch = 100
	oldRowBatch = 1000
)

// MaintenanceArgs is the periodic job that keeps the tables tidy.
type MaintenanceArgs struct{}

// Kind returns the routing key of the maintenance job.
func (MaintenanceArgs) Kind() string { return "maintenance" }

// MaintenanceWorker runs the maintenance job.
type MaintenanceWorker struct {
	river.WorkerDefaults[MaintenanceArgs]
	Deps Deps
}

// Work reads the clock once and runs five steps, each in its own transaction:
// it fails the reports and PDF rows whose job River gave up on, expires the
// reports past their retention, purges the expired reports past their keeping
// period, and deletes old sign-in throttle rows and old sessions. A failed
// step is logged with its cause and the next one still runs; Work returns
// "maintenance: INTERNAL_FAILURE" when any step failed.
func (w *MaintenanceWorker) Work(ctx context.Context, _ *river.Job[MaintenanceArgs]) error {
	now := w.Deps.Now().UTC()
	steps := []struct {
		name string
		run  func(ctx context.Context, tx pgx.Tx, now time.Time) error
	}{
		{"reconcile", w.reconcile},
		{"expire", w.expire},
		{"purge", w.purge},
		{"throttle", w.purgeThrottle},
		{"sessions", w.purgeSessions},
	}
	var first error
	for _, step := range steps {
		err := store.Transact(ctx, w.Deps.Pool, func(tx pgx.Tx) error { return step.run(ctx, tx, now) })
		if err == nil {
			continue
		}
		w.Deps.Logger.LogAttrs(context.Background(), slog.LevelError, "maintenance step failed",
			slog.String("step", step.name), slog.Any("error", err))
		if first == nil {
			first = err
		}
	}
	if first != nil {
		return fmt.Errorf("maintenance: %s", codeInternalFailure)
	}
	return nil
}

func reportEvent(action string, id string, metadata audit.Metadata, failed bool) audit.Event {
	e := audit.Event{Action: action, TargetType: "report", TargetID: id, Metadata: metadata}
	if failed {
		e.Outcome = audit.OutcomeFailed
	}
	return e
}

func (w *MaintenanceWorker) reconcile(ctx context.Context, tx pgx.Tx, now time.Time) error {
	ids, err := store.ReconcileReports(ctx, tx, now, reportBatch)
	if err != nil {
		return err
	}
	for _, id := range ids {
		meta := audit.Metadata{"code": codeJobAttemptsExhausted}
		if err := audit.Append(ctx, tx, now, reportEvent(reports.ActionReportCollect, id.String(), meta, true)); err != nil {
			return err
		}
	}
	keys, err := store.ReconcilePDFs(ctx, tx, now, reportBatch)
	if err != nil {
		return err
	}
	for _, k := range keys {
		meta := audit.Metadata{"language": k.Language, "code": codePDFAttemptsExhausted}
		if err := audit.Append(ctx, tx, now, reportEvent(reports.ActionReportRender, k.ReportID.String(), meta, true)); err != nil {
			return err
		}
	}
	return nil
}

func (w *MaintenanceWorker) expire(ctx context.Context, tx pgx.Tx, now time.Time) error {
	ids, err := store.ExpireDueReports(ctx, tx, now, reportBatch)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := audit.Append(ctx, tx, now, reportEvent(reports.ActionReportExpire, id.String(), audit.Metadata{}, false)); err != nil {
			return err
		}
	}
	return nil
}

func (w *MaintenanceWorker) purge(ctx context.Context, tx pgx.Tx, now time.Time) error {
	ids, err := store.PurgeExpiredReports(ctx, tx, now, w.Deps.Settings.ExpiredReportKept, reportBatch)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := audit.Append(ctx, tx, now, reportEvent(reports.ActionReportPurge, id.String(), audit.Metadata{}, false)); err != nil {
			return err
		}
	}
	return nil
}

func (w *MaintenanceWorker) purgeThrottle(ctx context.Context, tx pgx.Tx, now time.Time) error {
	_, err := store.PurgeThrottle(ctx, tx, now, auth.ThrottleWindow, oldRowBatch)
	return err
}

func (w *MaintenanceWorker) purgeSessions(ctx context.Context, tx pgx.Tx, now time.Time) error {
	_, err := store.PurgeSessions(ctx, tx, now, w.Deps.Settings.SessionTTL, oldRowBatch)
	return err
}
