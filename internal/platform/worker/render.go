package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/render"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// The codes a render attempt stores.
const (
	codePDFCompileFailed = "PDF_COMPILE_FAILED"
	codePDFTooLarge      = "PDF_TOO_LARGE"
)

var (
	errCompile     = errors.New("compile failed")
	errPDFTooLarge = errors.New("pdf is too large")
	errDigest      = errors.New("snapshot digest mismatch")
)

// renderStaleError is what a stale render returns to River: its text is the
// one River may record, and it matches ErrStaleAttempt.
type renderStaleError struct{}

func (renderStaleError) Error() string { return "render: STALE_ATTEMPT" }

func (renderStaleError) Is(target error) bool { return target == ErrStaleAttempt }

// RenderWorker runs the render job of one language of a report.
type RenderWorker struct {
	river.WorkerDefaults[reports.RenderArgs]
	Deps  Deps
	hooks renderHooks
}

// renderHooks let a test hold an attempt at a point of the run, fail the
// transaction that stores a PDF, or shorten the outcome timeout; the zero
// value of a field does nothing.
type renderHooks struct {
	afterClaim     func(ctx context.Context, language string) // after the row is claimed, before the snapshot is read
	beforeStore    func(ctx context.Context, language string) // after the compile, before the outcome's transaction
	insideStore    func(language string)                      // inside the transaction that stores a PDF
	successError   error                                      // returned from inside that transaction, after insideStore
	outcomeTimeout time.Duration                              // the outcome timeout; zero means outcomeTimeout
}

// outcome is the time the transaction of an outcome may run.
func (w *RenderWorker) outcome() time.Duration {
	if w.hooks.outcomeTimeout > 0 {
		return w.hooks.outcomeTimeout
	}
	return outcomeTimeout
}

// Timeout is the longest a render may run.
func (w *RenderWorker) Timeout(*river.Job[reports.RenderArgs]) time.Duration {
	return w.Deps.Settings.RenderTimeout
}

// Work compiles one language of the stored snapshot and stores the PDF or a
// code, completes the report when no PDF is pending, and completes the job in
// the same transaction.
func (w *RenderWorker) Work(ctx context.Context, job *river.Job[reports.RenderArgs]) error {
	body, done, err := w.load(ctx, job)
	if err == nil && done {
		return nil
	}
	if err == nil {
		var pdf []byte
		pdf, err = w.compile(ctx, job, body)
		if err == nil {
			if h := w.hooks.beforeStore; h != nil {
				h(ctx, job.Args.Language)
			}
			wctx, cancel := successContext(ctx, w.outcome())
			err = successFailure(w.store(wctx, job, pdf, ""))
			cancel()
			if err == nil {
				return nil
			}
		}
	}
	return w.handle(ctx, job, err)
}

// The lines a skipped render writes, one for each reason.
const (
	skipNotCollected = "render skipped: report not collected"
	skipExpired      = "render skipped: report expired"
	skipDeleted      = "render skipped: report deleted"
	skipNoPDFRow     = "render skipped: no PDF row"
	skipPDFSettled   = "render skipped: PDF row is not pending"
)

// reportSkip names why a report no longer wants a render, or is empty when it
// still does.
func reportSkip(report store.Report, now time.Time) string {
	switch {
	case report.DeletedAt != nil:
		return skipDeleted
	case report.Status != "collected":
		return skipNotCollected
	case report.ExpiredAt != nil || !report.ExpiresAt.After(now):
		return skipExpired
	}
	return ""
}

// needsRender names why the report no longer wants this language rendered, or
// is empty when it does.
func needsRender(ctx context.Context, tx pgx.Tx, report store.Report, language string, now time.Time) (string, error) {
	if skip := reportSkip(report, now); skip != "" {
		return skip, nil
	}
	row, err := store.GetPDF(ctx, tx, report.ID, language)
	if errors.Is(err, store.ErrNotFound) {
		return skipNoPDFRow, nil
	}
	if err != nil {
		return "", err
	}
	if row.Status != "pending" {
		return skipPDFSettled, nil
	}
	return "", nil
}

// load claims the PDF row under the lock of the report, in a transaction of
// its own, and then reads the snapshot, so a failure of the read is written
// under the claim. It reports done when the report needs nothing: the job is
// then completed.
func (w *RenderWorker) load(ctx context.Context, job *river.Job[reports.RenderArgs]) (body []byte, done bool, err error) {
	id, lang := job.Args.ReportID, job.Args.Language
	err = store.Transact(ctx, w.Deps.Pool, func(tx pgx.Tx) error {
		report, err := store.GetReportForUpdate(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			w.note(id, job.Attempt, "render job has no report")
			done = true
			_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
			return err
		}
		if err != nil {
			return err
		}
		now := w.Deps.Now().UTC()
		skip, err := needsRender(ctx, tx, report, lang, now)
		if err != nil {
			return err
		}
		if skip != "" {
			w.note(id, job.Attempt, skip)
			done = true
			_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
			return err
		}
		return store.ClaimPDF(ctx, tx, id, lang, job.ID, job.Attempt, now)
	})
	if err != nil || done {
		return nil, done, err
	}
	if h := w.hooks.afterClaim; h != nil {
		h(ctx, lang)
	}
	_, snap, sum, err := store.GetSnapshot(ctx, w.Deps.Pool, id)
	if err != nil {
		return nil, false, err
	}
	if digestHex(snap) != sum {
		return nil, false, errDigest
	}
	return snap, false, nil
}

// compile makes the PDF of one language from the stored snapshot bytes.
func (w *RenderWorker) compile(ctx context.Context, job *river.Job[reports.RenderArgs], body []byte) ([]byte, error) {
	lang, err := catalog.ParseLanguage(job.Args.Language)
	if err != nil {
		return nil, err
	}
	snap, err := snapshot.Decode(body)
	if err != nil {
		return nil, err
	}
	data, err := render.Document(snap, lang, document.Options{})
	if err != nil {
		return nil, err
	}
	pdf, err := render.Compile(ctx, w.Deps.Toolchain, w.Deps.TempDir, data)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errCompile, err)
	}
	if len(pdf) > render.MaxPDFBytes {
		return nil, errPDFTooLarge
	}
	return pdf, nil
}

// store writes the outcome of the row: the PDF when code is empty, else the
// code. It then completes the report when no PDF is pending and completes the
// job, all under the lock of the report.
func (w *RenderWorker) store(ctx context.Context, job *river.Job[reports.RenderArgs], pdf []byte, code string) error {
	d := w.Deps
	id, lang := job.Args.ReportID, job.Args.Language
	return store.Transact(ctx, d.Pool, func(tx pgx.Tx) error {
		report, err := store.GetReportForUpdate(ctx, tx, id)
		if errors.Is(err, store.ErrNotFound) {
			w.note(id, job.Attempt, "render job has no report")
			_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
			return err
		}
		if err != nil {
			return err
		}
		now := d.Now().UTC()
		if skip := reportSkip(report, now); skip != "" {
			w.note(id, job.Attempt, skip)
			_, err := river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
			return err
		}
		event := audit.Event{
			Action: reports.ActionReportRender, TargetType: "report", TargetID: id.String(),
			Metadata: audit.Metadata{"language": lang},
		}
		if code == "" {
			if h := w.hooks.insideStore; h != nil {
				h(lang)
			}
			if w.hooks.successError != nil {
				return w.hooks.successError
			}
			err = store.FinishPDF(ctx, tx, id, lang, job.ID, job.Attempt, pdf, now)
		} else {
			err = store.FailPDF(ctx, tx, id, lang, job.ID, job.Attempt, code, now)
			event.Outcome = audit.OutcomeFailed
			event.Metadata["code"] = code
		}
		if err != nil {
			return err
		}
		pending, err := store.CountPendingPDFs(ctx, tx, id)
		if err != nil {
			return err
		}
		if pending == 0 {
			if err := store.SetReportPhase(ctx, tx, id, "complete", &now, now); err != nil {
				return err
			}
		}
		if err := audit.Append(ctx, tx, now, event); err != nil {
			return err
		}
		_, err = river.JobCompleteTx[*riverpgxv5.Driver](ctx, tx, job)
		return err
	})
}

// renderClassify maps the failure of an attempt to a code. A stop of the
// worker is no failure: stop is then the error to return and nothing is
// written. A deadline is stored at once, since an overrun repeats.
func renderClassify(ctx context.Context, err error) (code string, stop error) {
	switch ctxErr := ctx.Err(); {
	case errors.Is(ctxErr, context.DeadlineExceeded):
		return codePDFCompileFailed, nil
	case ctxErr != nil:
		return "", ctxErr
	}
	switch {
	case errors.Is(err, errPDFTooLarge):
		return codePDFTooLarge, nil
	case errors.Is(err, errCompile):
		return codePDFCompileFailed, nil
	}
	return codeInternalFailure, nil
}

func (w *RenderWorker) note(id uuid.UUID, attempt int, msg string) {
	w.Deps.Logger.LogAttrs(context.Background(), slog.LevelInfo, msg,
		slog.String("report_id", id.String()), slog.Int("attempt", attempt))
}

func (w *RenderWorker) log(job *river.Job[reports.RenderArgs], code string, cause error, msg string) {
	w.Deps.Logger.LogAttrs(context.Background(), slog.LevelWarn, msg,
		slog.String("report_id", job.Args.ReportID.String()), slog.String("language", job.Args.Language),
		slog.Int("attempt", job.Attempt), slog.String("code", code), slog.Any("cause", cause))
}

// handle settles a failed attempt: it returns a stale attempt as it is,
// returns the stop of the worker, asks River to retry, or stores the code and
// completes the job.
func (w *RenderWorker) handle(ctx context.Context, job *river.Job[reports.RenderArgs], cause error) error {
	if errors.Is(cause, ErrStaleAttempt) {
		w.log(job, "STALE_ATTEMPT", cause, "render attempt is stale")
		return renderStaleError{}
	}
	code, stop := codeInternalFailure, error(nil)
	if !errors.Is(cause, errSuccessWrite) {
		code, stop = renderClassify(ctx, cause)
	}
	if stop != nil {
		return stop
	}
	failure := fmt.Errorf("render: %s", code)
	if retried(code) && job.Attempt < job.MaxAttempts {
		w.log(job, code, cause, "render attempt failed")
		return failure
	}
	wctx, cancel := outcomeContext(ctx, w.outcome())
	defer cancel()
	err := w.store(wctx, job, nil, code)
	switch {
	case errors.Is(err, ErrStaleAttempt):
		w.log(job, code, cause, "render attempt is stale")
		return renderStaleError{}
	case err != nil:
		w.log(job, code, errors.Join(cause, err), "render failure was not stored")
		return fmt.Errorf("render: %s", codeInternalFailure)
	}
	w.log(job, code, cause, "render attempt failed")
	return nil
}

// digestHex is the SHA-256 of body in lower-case hex, as the store keeps it.
func digestHex(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
