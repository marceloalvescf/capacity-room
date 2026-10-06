package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/pipeline"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// maxSnapshotBytes is the largest snapshot a report stores.
const maxSnapshotBytes = 32 << 20

// The codes a collect attempt stores that the profile test does not know.
const (
	codeNothingToReport   = "NOTHING_TO_REPORT"
	codeSnapshotTooLarge  = "SNAPSHOT_TOO_LARGE"
	codeCollectionTimeout = "COLLECTION_TIMEOUT"
	codeInternalFailure   = "INTERNAL_FAILURE"
)

var errSnapshotTooLarge = errors.New("snapshot is too large")

// checkSnapshotSize refuses a snapshot above the limit.
func checkSnapshotSize(n int) error {
	if n > maxSnapshotBytes {
		return errSnapshotTooLarge
	}
	return nil
}

// retried reports whether River tries a failure with this code again while
// attempts remain.
func retried(code string) bool {
	return code == profiles.CodeUnreachable || code == codeInternalFailure
}

// CollectWorker runs the collect job of a report.
type CollectWorker struct {
	river.WorkerDefaults[reports.CollectArgs]
	Deps  Deps
	hooks collectHooks
}

// collectHooks let a test hold an attempt at a point of the run. Each is
// called with the context of the job and its attempt; nil does nothing.
type collectHooks struct {
	beforeClaim     func(ctx context.Context, attempt int) // before the claim's transaction
	afterClaim      func(ctx context.Context, attempt int) // after the claim committed
	beforeCollected func(ctx context.Context, attempt int) // after the snapshot is encoded, before its transaction
}

// Timeout is the longest a collection may run.
func (w *CollectWorker) Timeout(*river.Job[reports.CollectArgs]) time.Duration {
	return w.Deps.Settings.CollectTimeout
}

// Work collects the report of the job. It claims the report under the
// attempt of the job, runs the pipeline against the pinned revision, stores
// the snapshot, queues the two renders, and completes the job in the same
// transaction. A failure is classified: River retries some codes, the rest
// are stored at once. The cause of a failure goes to the log alone.
func (w *CollectWorker) Work(ctx context.Context, job *river.Job[reports.CollectArgs]) error {
	attempt := job.Attempt

	if h := w.hooks.beforeClaim; h != nil {
		h(ctx, attempt)
	}
	done, err := w.claim(ctx, job)
	if err == nil && done {
		return nil
	}
	if err == nil {
		if h := w.hooks.afterClaim; h != nil {
			h(ctx, attempt)
		}
		err = w.collect(ctx, job)
		if err == nil {
			return nil
		}
	}
	return w.handle(ctx, job, err)
}

// claim reads the report under a lock and takes it. It reports done when the
// report needs nothing: the job is then completed.
func (w *CollectWorker) claim(ctx context.Context, job *river.Job[reports.CollectArgs]) (done bool, err error) {
	d := w.Deps
	err = store.Transact(ctx, d.Pool, func(tx pgx.Tx) error {
		report, err := store.GetReportForUpdate(ctx, tx, job.Args.ReportID)
		if err != nil {
			return err
		}
		if report.Status == "collected" || report.Status == "failed" || report.ExpiredAt != nil {
			done = true
			_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
			return err
		}
		return store.ClaimReport(ctx, tx, report.ID, job.Attempt, d.Now().UTC())
	})
	return done, err
}

// collect runs the pipeline and stores its outcome.
func (w *CollectWorker) collect(ctx context.Context, job *river.Job[reports.CollectArgs]) error {
	d := w.Deps
	id := job.Args.ReportID
	attempt := job.Attempt

	report, err := store.GetReport(ctx, d.Pool, id)
	if err != nil {
		return err
	}
	rev, err := d.Profiles.Revision(ctx, report.ProfileRevisionID)
	if err != nil {
		return err
	}
	var r reports.Resolved
	if err := json.Unmarshal(report.Request, &r); err != nil {
		return fmt.Errorf("decode request: %w", err)
	}
	cfg := rev.Document.RunConfig(r.Period, r.Timezone, r.ReportProfile, r.MinSeverity, r.Namespaces)
	opts, err := d.Profiles.Source(rev.Document, cfg.Metrics)
	if err != nil {
		return err
	}

	emittedAt := d.Now().UTC()
	window, err := collect.NewWindow(cfg.Analysis, emittedAt)
	if err != nil {
		return err
	}
	result, err := pipeline.Collect(ctx, pipeline.Run{
		Options:   opts,
		Config:    cfg,
		Window:    window,
		EmittedAt: emittedAt,
		Now:       func() time.Time { return d.Now().UTC() },
		NewID:     func() (string, error) { return id.String(), nil },
		Version:   d.Version,
		Step: func(ctx context.Context, s pipeline.Step) error {
			return store.AdvanceReport(ctx, d.Pool, id, attempt, string(s), d.Now().UTC())
		},
	})
	if err != nil {
		return err
	}
	body, err := snapshot.Encode(result.Snapshot)
	if err != nil {
		return err
	}
	if err := checkSnapshotSize(len(body)); err != nil {
		return err
	}
	if h := w.hooks.beforeCollected; h != nil {
		h(ctx, attempt)
	}

	return store.Transact(ctx, d.Pool, func(tx pgx.Tx) error {
		now := d.Now().UTC()
		if err := store.CollectReport(ctx, tx, id, attempt, now); err != nil {
			return err
		}
		if err := store.InsertSnapshot(ctx, tx, id, snapshot.SchemaVersion, body, now); err != nil {
			return err
		}
		queue := river.ClientFromContext[pgx.Tx](ctx)
		for _, lang := range catalog.Languages() {
			inserted, err := queue.InsertTx(ctx, tx, reports.RenderArgs{ReportID: id, Language: string(lang)}, reports.RenderInsertOpts(d.Settings))
			if err != nil {
				return fmt.Errorf("queue render: %w", err)
			}
			if err := store.InsertPendingPDF(ctx, tx, id, string(lang), inserted.Job.ID, now); err != nil {
				return err
			}
		}
		if err := audit.Append(ctx, tx, now, audit.Event{
			Action: reports.ActionReportCollect, TargetType: "report", TargetID: id.String(), Metadata: audit.Metadata{},
		}); err != nil {
			return err
		}
		_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
		return err
	})
}

// classify maps the failure of an attempt to a code. A stop of the worker is
// no failure: stop is then the error to return and nothing is written.
func classify(ctx context.Context, err error) (code string, stop error) {
	switch ctxErr := ctx.Err(); {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return codeCollectionTimeout, nil
	case ctxErr != nil:
		return "", ctxErr
	}
	if errors.Is(err, errSnapshotTooLarge) {
		return codeSnapshotTooLarge, nil
	}
	if errors.Is(err, collect.ErrNoUsefulSections) {
		return codeNothingToReport, nil
	}
	if code, ok := profiles.Classify(err); ok {
		return code, nil
	}
	return codeInternalFailure, nil
}

func (w *CollectWorker) log(id uuid.UUID, attempt int, code string, cause error, msg string) {
	w.Deps.Logger.LogAttrs(context.Background(), slog.LevelWarn, msg,
		slog.String("report_id", id.String()), slog.Int("attempt", attempt), slog.String("code", code),
		slog.Any("cause", cause))
}

// handle settles a failed attempt: it returns a stale attempt as it is,
// returns the stop of the worker, asks River to retry, or stores the code and
// completes the job.
func (w *CollectWorker) handle(ctx context.Context, job *river.Job[reports.CollectArgs], cause error) error {
	d := w.Deps
	id := job.Args.ReportID
	attempt := job.Attempt

	if errors.Is(cause, ErrStaleAttempt) {
		w.log(id, attempt, "STALE_ATTEMPT", cause, "collect attempt is stale")
		return ErrStaleAttempt
	}
	code, stop := classify(ctx, cause)
	if stop != nil {
		return stop
	}
	// The transaction of the outcome does not stop with the attempt's own
	// deadline; the values of the context stay, which the completion needs.
	wctx := context.WithoutCancel(ctx)
	failure := fmt.Errorf("collect: %s", code)

	if retried(code) && attempt < job.MaxAttempts {
		err := store.Transact(wctx, d.Pool, func(tx pgx.Tx) error {
			return store.RequeueReport(wctx, tx, id, attempt, d.Now().UTC())
		})
		return w.settled(id, attempt, code, cause, err, failure)
	}
	err := store.Transact(wctx, d.Pool, func(tx pgx.Tx) error {
		now := d.Now().UTC()
		if err := store.FailReport(wctx, tx, id, attempt, code, now); err != nil {
			return err
		}
		if err := audit.Append(wctx, tx, now, audit.Event{
			Action: reports.ActionReportCollect, TargetType: "report", TargetID: id.String(),
			Outcome: audit.OutcomeFailed, Metadata: audit.Metadata{"code": code},
		}); err != nil {
			return err
		}
		_, err := river.JobCompleteTx[*riverpgxv5.Driver](wctx, tx, job)
		return err
	})
	return w.settled(id, attempt, code, cause, err, nil)
}

// settled logs the outcome of a failure's own transaction and returns what
// River receives: the retry error, nil for a stored failure, or a bare code
// when the transaction itself failed.
func (w *CollectWorker) settled(id uuid.UUID, attempt int, code string, cause, txErr, result error) error {
	switch {
	case errors.Is(txErr, ErrStaleAttempt):
		w.log(id, attempt, code, cause, "collect attempt is stale")
		return ErrStaleAttempt
	case txErr != nil:
		w.log(id, attempt, code, errors.Join(cause, txErr), "collect failure was not stored")
		return fmt.Errorf("collect: %s", codeInternalFailure)
	}
	w.log(id, attempt, code, cause, "collect attempt failed")
	return result
}
