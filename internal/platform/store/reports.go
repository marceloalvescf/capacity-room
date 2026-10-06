package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Report is a row of the reports table.
type Report struct {
	ID                                  uuid.UUID
	OwnerID                             uuid.UUID
	ProfileID                           string
	ProfileRevisionID                   uuid.UUID
	Request                             []byte // jsonb
	Status, Phase                       string
	Attempt                             int
	FailureCode                         *string
	CollectJobID                        int64
	CreatedAt                           time.Time
	StartedAt, CollectedAt, CompletedAt *time.Time
	ExpiresAt                           time.Time
	ExpiredAt                           *time.Time
	UpdatedAt                           time.Time
}

// ReportPDF is a row of the report_pdfs table, without its body.
type ReportPDF struct {
	ReportID    uuid.UUID
	Language    string
	Status      string
	FailureCode *string
	JobID       int64
	SHA256      *string
	SizeBytes   *int64
	UpdatedAt   time.Time
}

// ErrStaleAttempt is returned by a write of a running report whose attempt
// or status no longer matches, and by a write of a PDF row that is not in the
// state the write needs.
var ErrStaleAttempt = errors.New("report attempt is stale")

// ReportRequestLockID is the transaction advisory lock that serialises report
// requests: the migration lock plus one.
const ReportRequestLockID = 72427302

const reportColumns = `id, owner_id, profile_id, profile_revision_id, request, status, phase, attempt, failure_code,
collect_job_id, created_at, started_at, collected_at, completed_at, expires_at, expired_at, updated_at`

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func scanReport(row pgx.Row) (Report, error) {
	var r Report
	err := row.Scan(&r.ID, &r.OwnerID, &r.ProfileID, &r.ProfileRevisionID, &r.Request, &r.Status, &r.Phase, &r.Attempt,
		&r.FailureCode, &r.CollectJobID, &r.CreatedAt, &r.StartedAt, &r.CollectedAt, &r.CompletedAt, &r.ExpiresAt,
		&r.ExpiredAt, &r.UpdatedAt)
	if err != nil {
		return Report{}, err
	}
	r.CreatedAt = r.CreatedAt.UTC()
	r.ExpiresAt = r.ExpiresAt.UTC()
	r.UpdatedAt = r.UpdatedAt.UTC()
	r.StartedAt = utcPtr(r.StartedAt)
	r.CollectedAt = utcPtr(r.CollectedAt)
	r.CompletedAt = utcPtr(r.CompletedAt)
	r.ExpiredAt = utcPtr(r.ExpiredAt)
	return r, nil
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// LockReportRequests takes the transaction advisory lock that serialises
// report requests. It locks no table, so the writes of a running report never
// wait for it.
func LockReportRequests(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(ReportRequestLockID)); err != nil {
		return fmt.Errorf("lock report requests: %w", err)
	}
	return nil
}

// InsertReport inserts r.
func InsertReport(ctx context.Context, q Querier, r Report) error {
	_, err := q.Exec(ctx, `
INSERT INTO reports (`+reportColumns+`)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		r.ID, r.OwnerID, r.ProfileID, r.ProfileRevisionID, r.Request, r.Status, r.Phase, r.Attempt, r.FailureCode,
		r.CollectJobID, r.CreatedAt, r.StartedAt, r.CollectedAt, r.CompletedAt, r.ExpiresAt, r.ExpiredAt, r.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert report: %w", err)
	}
	return nil
}

func getReport(ctx context.Context, q Querier, suffix string, id uuid.UUID) (Report, error) {
	r, err := scanReport(q.QueryRow(ctx, `SELECT `+reportColumns+` FROM reports WHERE id = $1`+suffix, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Report{}, ErrNotFound
	}
	if err != nil {
		return Report{}, fmt.Errorf("get report: %w", err)
	}
	return r, nil
}

// GetReport returns the report with the given id.
func GetReport(ctx context.Context, q Querier, id uuid.UUID) (Report, error) {
	return getReport(ctx, q, ``, id)
}

// GetReportForUpdate reads the report and holds a lock on the row until the
// transaction ends.
func GetReportForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Report, error) {
	return getReport(ctx, tx, ` FOR UPDATE`, id)
}

// CountActiveReports counts the queued and running reports, all of them and
// those of one profile.
func CountActiveReports(ctx context.Context, q Querier, profileID string) (global, profile int, err error) {
	err = q.QueryRow(ctx, `
SELECT count(*), count(*) FILTER (WHERE profile_id = $1)
FROM reports WHERE status IN ('queued', 'running')`, profileID).Scan(&global, &profile)
	if err != nil {
		return 0, 0, fmt.Errorf("count active reports: %w", err)
	}
	return global, profile, nil
}

// execRunning runs a write of a running report that names its attempt, and
// returns ErrStaleAttempt when no row matched.
func execRunning(ctx context.Context, q Querier, sql string, args ...any) error {
	tag, err := q.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrStaleAttempt
	}
	return nil
}

// ClaimReport marks the report running in phase preflight under attempt. It
// takes a queued report, or a running one whose stored attempt is not above
// attempt, and returns ErrStaleAttempt otherwise.
func ClaimReport(ctx context.Context, tx pgx.Tx, id uuid.UUID, attempt int, now time.Time) error {
	return execRunning(ctx, tx, `
UPDATE reports
SET status = 'running', phase = 'preflight', attempt = $2, started_at = COALESCE(started_at, $3), updated_at = $3
WHERE id = $1 AND (status = 'queued' OR (status = 'running' AND attempt <= $2))`, id, attempt, now)
}

// AdvanceReport moves a running report to phase.
func AdvanceReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, phase string, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE reports SET phase = $3, updated_at = $4
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, phase, now)
}

// RequeueReport returns a running report to the queue.
func RequeueReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE reports SET status = 'queued', phase = 'waiting', updated_at = $3
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, now)
}

// FailReport marks a running report failed with code.
func FailReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, code string, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE reports SET status = 'failed', phase = 'complete', failure_code = $3, completed_at = $4, updated_at = $4
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, code, now)
}

// CollectReport marks a running report collected and waiting for its PDFs.
func CollectReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE reports SET status = 'collected', phase = 'rendering', collected_at = $3, updated_at = $3
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, now)
}

// SetReportPhase sets the phase and the completion time of a collected
// report, and returns ErrStaleAttempt for a report in another status.
func SetReportPhase(ctx context.Context, q Querier, id uuid.UUID, phase string, completedAt *time.Time, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE reports SET phase = $2, completed_at = $3, updated_at = $4
WHERE id = $1 AND status = 'collected'`, id, phase, completedAt, now)
}

// InsertSnapshot stores the snapshot bytes with their SHA-256 and size.
func InsertSnapshot(ctx context.Context, q Querier, reportID uuid.UUID, schemaVersion string, body []byte, now time.Time) error {
	_, err := q.Exec(ctx, `
INSERT INTO report_snapshots (report_id, schema_version, body, sha256, size_bytes, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`, reportID, schemaVersion, body, digest(body), int64(len(body)), now)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

// GetSnapshot returns the schema version, the bytes, and the SHA-256 of the
// stored snapshot.
func GetSnapshot(ctx context.Context, q Querier, reportID uuid.UUID) (schemaVersion string, body []byte, sha256 string, err error) {
	err = q.QueryRow(ctx, `SELECT schema_version, body, sha256 FROM report_snapshots WHERE report_id = $1`, reportID).
		Scan(&schemaVersion, &body, &sha256)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, "", ErrNotFound
	}
	if err != nil {
		return "", nil, "", fmt.Errorf("get snapshot: %w", err)
	}
	return schemaVersion, body, sha256, nil
}

// InsertPendingPDF adds a pending row for one language, run by the job.
func InsertPendingPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, now time.Time) error {
	_, err := q.Exec(ctx, `
INSERT INTO report_pdfs (report_id, language, status, job_id, updated_at)
VALUES ($1, $2, 'pending', $3, $4)`, reportID, language, jobID, now)
	if err != nil {
		return fmt.Errorf("insert pending pdf: %w", err)
	}
	return nil
}

// GetPDF returns the row of one language, without its body.
func GetPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string) (ReportPDF, error) {
	var p ReportPDF
	err := q.QueryRow(ctx, `
SELECT report_id, language, status, failure_code, job_id, sha256, size_bytes, updated_at
FROM report_pdfs WHERE report_id = $1 AND language = $2`, reportID, language).
		Scan(&p.ReportID, &p.Language, &p.Status, &p.FailureCode, &p.JobID, &p.SHA256, &p.SizeBytes, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReportPDF{}, ErrNotFound
	}
	if err != nil {
		return ReportPDF{}, fmt.Errorf("get pdf: %w", err)
	}
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, nil
}

// FinishPDF stores the PDF bytes with their SHA-256 and size and marks the row
// ready. It returns ErrStaleAttempt when the row is not pending.
func FinishPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, body []byte, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE report_pdfs SET status = 'ready', body = $3, sha256 = $4, size_bytes = $5, updated_at = $6
WHERE report_id = $1 AND language = $2 AND status = 'pending'`,
		reportID, language, body, digest(body), int64(len(body)), now)
}

// FailPDF marks a pending row failed with code. It returns ErrStaleAttempt
// when the row is not pending.
func FailPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, code string, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE report_pdfs SET status = 'failed', failure_code = $3, updated_at = $4
WHERE report_id = $1 AND language = $2 AND status = 'pending'`, reportID, language, code, now)
}

// RetryPDF returns a failed row to pending under a new job. It returns
// ErrStaleAttempt when the row is not failed.
func RetryPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, now time.Time) error {
	return execRunning(ctx, q, `
UPDATE report_pdfs SET status = 'pending', failure_code = NULL, job_id = $3, updated_at = $4
WHERE report_id = $1 AND language = $2 AND status = 'failed'`, reportID, language, jobID, now)
}

// CountPendingPDFs counts the pending rows of the report.
func CountPendingPDFs(ctx context.Context, q Querier, reportID uuid.UUID) (int, error) {
	var n int
	err := q.QueryRow(ctx, `SELECT count(*) FROM report_pdfs WHERE report_id = $1 AND status = 'pending'`, reportID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count pending pdfs: %w", err)
	}
	return n, nil
}

// GetIdempotencyKey returns the request digest and the report stored under
// the owner's key.
func GetIdempotencyKey(ctx context.Context, q Querier, ownerID uuid.UUID, key string) (requestSHA256 string, reportID uuid.UUID, err error) {
	err = q.QueryRow(ctx, `SELECT request_sha256, report_id FROM report_idempotency_keys WHERE owner_id = $1 AND key = $2`,
		ownerID, key).Scan(&requestSHA256, &reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", uuid.Nil, ErrNotFound
	}
	if err != nil {
		return "", uuid.Nil, fmt.Errorf("get idempotency key: %w", err)
	}
	return requestSHA256, reportID, nil
}

// InsertIdempotencyKey stores the owner's key.
func InsertIdempotencyKey(ctx context.Context, q Querier, ownerID uuid.UUID, key, requestSHA256 string, reportID uuid.UUID, now time.Time) error {
	_, err := q.Exec(ctx, `
INSERT INTO report_idempotency_keys (owner_id, key, request_sha256, report_id, created_at)
VALUES ($1, $2, $3, $4, $5)`, ownerID, key, requestSHA256, reportID, now)
	if err != nil {
		return fmt.Errorf("insert idempotency key: %w", err)
	}
	return nil
}
