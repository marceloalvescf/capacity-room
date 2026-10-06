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

// outcomeTimeout bounds the transaction that stores the outcome of an
// attempt, which runs after the attempt's own context may be done.
const outcomeTimeout = 10 * time.Second

// errOutcomeExpired marks the error of a success write whose own context ran
// out: a fault of the platform, whatever the cause says.
var errOutcomeExpired = errors.New("outcome write ran out of time")

// successContext is the context of the transaction that stores a success:
// free of the job's cancel, with its deadline at the later of the job's
// deadline and now plus the outcome timeout, so a write that starts late
// still has the outcome timeout and one that starts early may use what is
// left of the job's budget. It keeps the values of ctx, which the completion
// of the job needs.
func successContext(ctx context.Context, outcome time.Duration) (context.Context, context.CancelFunc) {
	deadline := time.Now().Add(outcome)
	if d, ok := ctx.Deadline(); ok && d.After(deadline) {
		deadline = d
	}
	return context.WithDeadline(context.WithoutCancel(ctx), deadline)
}

// successFailure marks the error of a success write as an internal failure
// when the write's own context is done, and returns any other error as it is.
func successFailure(wctx context.Context, err error) error {
	if err != nil && !errors.Is(err, ErrStaleAttempt) && wctx.Err() != nil {
		return fmt.Errorf("%w: %w", errOutcomeExpired, err)
	}
	return err
}

// outcomeContext is the context of the transaction that stores a failure: it
// does not stop with the attempt's own deadline or cancel, so a failure is
// not lost to a deadline that has passed, and it keeps the values of ctx,
// which the completion of the job needs.
func outcomeContext(ctx context.Context, outcome time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), outcome)
}

// staleAttemptError is what a stale attempt returns to River: its text is the
// one River may record, and it matches ErrStaleAttempt.
type staleAttemptError struct{}

func (staleAttemptError) Error() string { return "collect: STALE_ATTEMPT" }

func (staleAttemptError) Is(target error) bool { return target == ErrStaleAttempt }

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
	insideCollected func(ctx context.Context, attempt int) // inside the transaction that stores the snapshot
	outcomeTimeout  time.Duration                          // the outcome timeout; zero means outcomeTimeout
}

// outcome is the time the transaction of an outcome may run.
func (w *CollectWorker) outcome() time.Duration {
	if w.hooks.outcomeTimeout > 0 {
		return w.hooks.outcomeTimeout
	}
	return outcomeTimeout
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
		if errors.Is(err, store.ErrNotFound) {
			w.note(job.Args.ReportID, job.Attempt, "collect job has no report")
			done = true
			_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
			return err
		}
		if err != nil {
			return err
		}
		if report.Status == "collected" || report.Status == "failed" || report.ExpiredAt != nil {
			w.note(report.ID, job.Attempt, "collect job skipped a report that needs nothing")
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

	wctx, cancel := successContext(ctx, w.outcome())
	defer cancel()
	err = store.Transact(wctx, d.Pool, func(tx pgx.Tx) error {
		now := d.Now().UTC()
		if err := store.CollectReport(wctx, tx, id, attempt, now); err != nil {
			return err
		}
		if h := w.hooks.insideCollected; h != nil {
			h(ctx, attempt)
		}
		if err := store.InsertSnapshot(wctx, tx, id, snapshot.SchemaVersion, body, now); err != nil {
			return err
		}
		queue := river.ClientFromContext[pgx.Tx](wctx)
		for _, lang := range catalog.Languages() {
			inserted, err := queue.InsertTx(wctx, tx, reports.RenderArgs{ReportID: id, Language: string(lang)}, reports.RenderInsertOpts(d.Settings))
			if err != nil {
				return fmt.Errorf("queue render: %w", err)
			}
			if err := store.InsertPendingPDF(wctx, tx, id, string(lang), inserted.Job.ID, now); err != nil {
				return err
			}
		}
		if err := audit.Append(wctx, tx, now, audit.Event{
			Action: reports.ActionReportCollect, TargetType: "report", TargetID: id.String(), Metadata: audit.Metadata{},
		}); err != nil {
			return err
		}
		_, err := river.JobCompleteTx[*riverpgxv5.Driver](wctx, tx, job)
		return err
	})
	return successFailure(wctx, err)
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

func (w *CollectWorker) note(id uuid.UUID, attempt int, msg string) {
	w.Deps.Logger.LogAttrs(context.Background(), slog.LevelInfo, msg,
		slog.String("report_id", id.String()), slog.Int("attempt", attempt))
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
		return staleAttemptError{}
	}
	code, stop := codeInternalFailure, error(nil)
	if !errors.Is(cause, errOutcomeExpired) {
		code, stop = classify(ctx, cause)
	}
	if stop != nil {
		return stop
	}
	wctx, cancel := outcomeContext(ctx, w.outcome())
	defer cancel()
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
		return staleAttemptError{}
	case txErr != nil:
		w.log(id, attempt, code, errors.Join(cause, txErr), "collect failure was not stored")
		return fmt.Errorf("collect: %s", codeInternalFailure)
	}
	w.log(id, attempt, code, cause, "collect attempt failed")
	return result
}
