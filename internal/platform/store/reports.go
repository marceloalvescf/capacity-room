package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
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
	DeletedAt                           *time.Time
	UpdatedAt                           time.Time
	WorstSeverity, ClusterName          *string
}

// ReportPDF is a row of the report_pdfs table, without its body.
type ReportPDF struct {
	ReportID    uuid.UUID
	Language    string
	Status      string
	FailureCode *string
	JobID       int64
	Attempt     int
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
collect_job_id, created_at, started_at, collected_at, completed_at, expires_at, expired_at, deleted_at, updated_at, worst_severity, cluster_name`

// prefixColumns qualifies each column of a comma-separated list with alias.
func prefixColumns(alias, list string) string {
	columns := strings.Split(list, ",")
	for i, c := range columns {
		columns[i] = alias + "." + strings.TrimSpace(c)
	}
	return strings.Join(columns, ", ")
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func scanReport(row pgx.Row) (Report, error) { return scanReportAfter(row) }

// scanReportAfter scans a row whose columns are the destinations of lead,
// then those of reportColumns.
func scanReportAfter(row pgx.Row, lead ...any) (Report, error) {
	var r Report
	err := row.Scan(append(lead, &r.ID, &r.OwnerID, &r.ProfileID, &r.ProfileRevisionID, &r.Request, &r.Status, &r.Phase, &r.Attempt,
		&r.FailureCode, &r.CollectJobID, &r.CreatedAt, &r.StartedAt, &r.CollectedAt, &r.CompletedAt, &r.ExpiresAt,
		&r.ExpiredAt, &r.DeletedAt, &r.UpdatedAt, &r.WorstSeverity, &r.ClusterName)...)
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
	r.DeletedAt = utcPtr(r.DeletedAt)
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
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)`,
		r.ID, r.OwnerID, r.ProfileID, r.ProfileRevisionID, r.Request, r.Status, r.Phase, r.Attempt, r.FailureCode,
		r.CollectJobID, r.CreatedAt, r.StartedAt, r.CollectedAt, r.CompletedAt, r.ExpiresAt, r.ExpiredAt, r.DeletedAt, r.UpdatedAt, r.WorstSeverity, r.ClusterName)
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

// execGuarded runs a write whose WHERE clause names the state it needs, and
// returns ErrStaleAttempt when no row matched.
func execGuarded(ctx context.Context, q Querier, sql string, args ...any) error {
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
// takes a queued or running report that is not deleted and whose stored
// attempt is not above attempt, so the attempt never goes down. It returns
// ErrNotFound for a deleted report and ErrStaleAttempt otherwise.
func ClaimReport(ctx context.Context, tx pgx.Tx, id uuid.UUID, attempt int, now time.Time) error {
	err := execGuarded(ctx, tx, `
UPDATE reports
SET status = 'running', phase = 'preflight', attempt = $2, started_at = COALESCE(started_at, $3), updated_at = $3
WHERE id = $1 AND status IN ('queued', 'running') AND attempt <= $2 AND deleted_at IS NULL`, id, attempt, now)
	if !errors.Is(err, ErrStaleAttempt) {
		return err
	}
	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT deleted_at IS NOT NULL FROM reports WHERE id = $1`, id).Scan(&deleted); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("claim report: %w", err)
	}
	if deleted {
		return ErrNotFound
	}
	return ErrStaleAttempt
}

// AdvanceReport moves a running report to phase.
func AdvanceReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, phase string, now time.Time) error {
	return execGuarded(ctx, q, `
UPDATE reports SET phase = $3, updated_at = $4
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, phase, now)
}

// RequeueReport returns a running report to the queue.
func RequeueReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time) error {
	return execGuarded(ctx, q, `
UPDATE reports SET status = 'queued', phase = 'waiting', updated_at = $3
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, now)
}

// FailReport marks a running report failed with code.
func FailReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, code string, now time.Time) error {
	return execGuarded(ctx, q, `
UPDATE reports SET status = 'failed', phase = 'complete', failure_code = $3, completed_at = $4, updated_at = $4
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, code, now)
}

// CollectedSummary is what the list shows of a collection: the worst
// severity the report shows and the name of its cluster.
type CollectedSummary struct {
	WorstSeverity string
	ClusterName   string
}

// CollectReport marks a running report collected and waiting for its PDFs,
// and stores the summary the list shows.
func CollectReport(ctx context.Context, q Querier, id uuid.UUID, attempt int, now time.Time, summary CollectedSummary) error {
	return execGuarded(ctx, q, `
UPDATE reports SET status = 'collected', phase = 'rendering', collected_at = $3, updated_at = $3,
    worst_severity = $4, cluster_name = $5
WHERE id = $1 AND attempt = $2 AND status = 'running'`, id, attempt, now, summary.WorstSeverity, summary.ClusterName)
}

// SetReportPhase sets the phase of a collected report, which is rendering or
// complete: completedAt is set exactly for complete and nil for rendering. It
// returns ErrStaleAttempt for a report in another status.
func SetReportPhase(ctx context.Context, q Querier, id uuid.UUID, phase string, completedAt *time.Time, now time.Time) error {
	return execGuarded(ctx, q, `
UPDATE reports SET phase = $2, completed_at = $3, updated_at = $4
WHERE id = $1 AND status = 'collected'`, id, phase, completedAt, now)
}

// InsertSnapshot stores the snapshot bytes with their SHA-256 and size.
func InsertSnapshot(ctx context.Context, q Querier, reportID uuid.UUID, schemaVersion string, body []byte, now time.Time) error {
	if body == nil {
		return errors.New("insert snapshot: nil body")
	}
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

// InsertPendingPDF adds a pending row for one language; jobID is the id of
// the River job that renders the row.
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
SELECT report_id, language, status, failure_code, job_id, attempt, sha256, size_bytes, updated_at
FROM report_pdfs WHERE report_id = $1 AND language = $2`, reportID, language).
		Scan(&p.ReportID, &p.Language, &p.Status, &p.FailureCode, &p.JobID, &p.Attempt, &p.SHA256, &p.SizeBytes, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReportPDF{}, ErrNotFound
	}
	if err != nil {
		return ReportPDF{}, fmt.Errorf("get pdf: %w", err)
	}
	p.UpdatedAt = p.UpdatedAt.UTC()
	return p, nil
}

// ClaimPDF gives the pending PDF row of a report's language to one attempt of
// its render job. It refuses another job and an older attempt with
// ErrStaleAttempt.
func ClaimPDF(ctx context.Context, tx pgx.Tx, reportID uuid.UUID, language string, jobID int64, attempt int, now time.Time) error {
	return execGuarded(ctx, tx, `
UPDATE report_pdfs SET attempt = $4, updated_at = $5
WHERE report_id = $1 AND language = $2 AND status = 'pending' AND job_id = $3 AND attempt <= $4`,
		reportID, language, jobID, attempt, now)
}

// FinishPDF stores the PDF bytes with their SHA-256 and size and marks the row
// ready. It returns ErrStaleAttempt when the row is not pending under jobID
// and attempt.
func FinishPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, attempt int, body []byte, now time.Time) error {
	if body == nil {
		return errors.New("finish pdf: nil body")
	}
	return execGuarded(ctx, q, `
UPDATE report_pdfs SET status = 'ready', body = $5, sha256 = $6, size_bytes = $7, updated_at = $8
WHERE report_id = $1 AND language = $2 AND status = 'pending' AND job_id = $3 AND attempt = $4`,
		reportID, language, jobID, attempt, body, digest(body), int64(len(body)), now)
}

// FailPDF marks a pending row failed with code. It returns ErrStaleAttempt
// when the row is not pending under jobID and attempt.
func FailPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, attempt int, code string, now time.Time) error {
	return execGuarded(ctx, q, `
UPDATE report_pdfs SET status = 'failed', failure_code = $5, updated_at = $6
WHERE report_id = $1 AND language = $2 AND status = 'pending' AND job_id = $3 AND attempt = $4`,
		reportID, language, jobID, attempt, code, now)
}

// RetryPDF returns a failed row to pending under a new job and attempt zero.
// It returns ErrStaleAttempt when the row is not failed.
func RetryPDF(ctx context.Context, q Querier, reportID uuid.UUID, language string, jobID int64, now time.Time) error {
	return execGuarded(ctx, q, `
UPDATE report_pdfs SET status = 'pending', failure_code = NULL, job_id = $3, attempt = 0, updated_at = $4
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

// GetIdempotentReport returns, in one statement, the request digest stored
// under the owner's key and the report it names, deleted or not; ErrNotFound
// when the key has no row.
func GetIdempotentReport(ctx context.Context, q Querier, ownerID uuid.UUID, key string) (requestSHA256 string, report Report, err error) {
	report, err = scanReportAfter(q.QueryRow(ctx, `SELECT k.request_sha256, `+prefixColumns("r", reportColumns)+`
FROM report_idempotency_keys k JOIN reports r ON r.id = k.report_id
WHERE k.owner_id = $1 AND k.key = $2`, ownerID, key), &requestSHA256)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", Report{}, ErrNotFound
	}
	if err != nil {
		return "", Report{}, fmt.Errorf("get idempotent report: %w", err)
	}
	return requestSHA256, report, nil
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

// ReportListItem is one row of the list: the report, its owner's user name,
// and its PDF rows without bodies, in language order.
type ReportListItem struct {
	Report
	OwnerUsername string
	PDFs          []ReportPDF
}

// ReportCursor is the position after which a page starts.
type ReportCursor struct {
	CreatedAt time.Time
	ID        uuid.UUID
}

const listColumns = `r.id, r.owner_id, r.profile_id, r.profile_revision_id, r.request, r.status, r.phase, r.attempt,
r.failure_code, r.collect_job_id, r.created_at, r.started_at, r.collected_at, r.completed_at, r.expires_at,
r.expired_at, r.deleted_at, r.updated_at, r.worst_severity, r.cluster_name, u.username`

func scanItem(row pgx.Row) (ReportListItem, error) {
	var (
		it ReportListItem
		r  = &it.Report
	)
	err := row.Scan(&r.ID, &r.OwnerID, &r.ProfileID, &r.ProfileRevisionID, &r.Request, &r.Status, &r.Phase, &r.Attempt,
		&r.FailureCode, &r.CollectJobID, &r.CreatedAt, &r.StartedAt, &r.CollectedAt, &r.CompletedAt, &r.ExpiresAt,
		&r.ExpiredAt, &r.DeletedAt, &r.UpdatedAt, &r.WorstSeverity, &r.ClusterName, &it.OwnerUsername)
	if err != nil {
		return ReportListItem{}, err
	}
	r.CreatedAt = r.CreatedAt.UTC()
	r.ExpiresAt = r.ExpiresAt.UTC()
	r.UpdatedAt = r.UpdatedAt.UTC()
	r.StartedAt = utcPtr(r.StartedAt)
	r.CollectedAt = utcPtr(r.CollectedAt)
	r.CompletedAt = utcPtr(r.CompletedAt)
	r.ExpiredAt = utcPtr(r.ExpiredAt)
	r.DeletedAt = utcPtr(r.DeletedAt)
	return it, nil
}

// attachPDFs fills the PDF rows of items, in language order, with one read.
func attachPDFs(ctx context.Context, q Querier, items []ReportListItem) error {
	if len(items) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(items))
	at := make(map[uuid.UUID]int, len(items))
	for i, it := range items {
		ids[i] = it.ID
		at[it.ID] = i
	}
	rows, err := q.Query(ctx, `
SELECT report_id, language, status, failure_code, job_id, sha256, size_bytes, updated_at
FROM report_pdfs WHERE report_id = ANY($1) ORDER BY report_id, language`, ids)
	if err != nil {
		return fmt.Errorf("read report pdfs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p ReportPDF
		if err := rows.Scan(&p.ReportID, &p.Language, &p.Status, &p.FailureCode, &p.JobID, &p.SHA256, &p.SizeBytes, &p.UpdatedAt); err != nil {
			return fmt.Errorf("read report pdfs: %w", err)
		}
		p.UpdatedAt = p.UpdatedAt.UTC()
		items[at[p.ReportID]].PDFs = append(items[at[p.ReportID]].PDFs, p)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read report pdfs: %w", err)
	}
	return nil
}

// ListReports returns at most limit rows not deleted, newest first, after
// the cursor when one is given.
func ListReports(ctx context.Context, q Querier, after *ReportCursor, limit int) ([]ReportListItem, error) {
	var (
		rows pgx.Rows
		err  error
	)
	const head = `SELECT ` + listColumns + `
FROM reports r JOIN users u ON u.id = r.owner_id
WHERE r.deleted_at IS NULL`
	const tail = `
ORDER BY r.created_at DESC, r.id DESC
LIMIT `
	if after == nil {
		rows, err = q.Query(ctx, head+tail+`$1`, limit)
	} else {
		rows, err = q.Query(ctx, head+` AND (r.created_at, r.id) < ($1, $2)`+tail+`$3`, after.CreatedAt, after.ID, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	defer rows.Close()
	items := []ReportListItem{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, fmt.Errorf("list reports: %w", err)
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reports: %w", err)
	}
	rows.Close()
	if err := attachPDFs(ctx, q, items); err != nil {
		return nil, err
	}
	return items, nil
}

// GetReportItem returns one report not deleted, with its owner's user name
// and PDF rows; ErrNotFound when absent or deleted.
func GetReportItem(ctx context.Context, q Querier, id uuid.UUID) (ReportListItem, error) {
	it, err := scanItem(q.QueryRow(ctx, `SELECT `+listColumns+`
FROM reports r JOIN users u ON u.id = r.owner_id
WHERE r.id = $1 AND r.deleted_at IS NULL`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return ReportListItem{}, ErrNotFound
	}
	if err != nil {
		return ReportListItem{}, fmt.Errorf("get report item: %w", err)
	}
	items := []ReportListItem{it}
	if err := attachPDFs(ctx, q, items); err != nil {
		return ReportListItem{}, err
	}
	return items[0], nil
}

// SnapshotRead is what ReadSnapshot returns.
type SnapshotRead struct {
	Status    string
	ExpiresAt time.Time
	ExpiredAt *time.Time // set once maintenance has expired the report
	Body      []byte     // nil unless collected and not expired
	SHA256    string     // empty when Body is nil
}

// ReadSnapshot returns, in one statement, the snapshot of a report that is
// collected, not deleted, and not expired at now, with the report's status
// and expiry: ErrNotFound when the report is absent or deleted.
func ReadSnapshot(ctx context.Context, q Querier, id uuid.UUID, now time.Time) (SnapshotRead, error) {
	var (
		out SnapshotRead
		sum *string
	)
	err := q.QueryRow(ctx, `
SELECT r.status, r.expires_at, r.expired_at,
       CASE WHEN r.status = 'collected' AND r.expires_at > $2 AND r.expired_at IS NULL THEN s.body END,
       CASE WHEN r.status = 'collected' AND r.expires_at > $2 AND r.expired_at IS NULL THEN s.sha256 END
FROM reports r LEFT JOIN report_snapshots s ON s.report_id = r.id
WHERE r.id = $1 AND r.deleted_at IS NULL`, id, now).Scan(&out.Status, &out.ExpiresAt, &out.ExpiredAt, &out.Body, &sum)
	if errors.Is(err, pgx.ErrNoRows) {
		return SnapshotRead{}, ErrNotFound
	}
	if err != nil {
		return SnapshotRead{}, fmt.Errorf("read snapshot: %w", err)
	}
	out.ExpiresAt = out.ExpiresAt.UTC()
	out.ExpiredAt = utcPtr(out.ExpiredAt)
	if sum != nil {
		out.SHA256 = *sum
	}
	return out, nil
}

// PDFRead is what ReadPDF returns.
type PDFRead struct {
	Status    string // of the report
	ExpiresAt time.Time
	ExpiredAt *time.Time // set once maintenance has expired the report
	PDF       *ReportPDF // nil when the row is absent
	Body      []byte     // nil unless ready and not expired
}

// ReadPDF returns, in one statement, the PDF row of one language with its
// body when the row is ready and the report is not expired at now, and the
// report's status and expiry; ErrNotFound when the report is absent or
// deleted.
func ReadPDF(ctx context.Context, q Querier, id uuid.UUID, language string, now time.Time) (PDFRead, error) {
	var (
		out          PDFRead
		lang, status *string
		failure, sum *string
		jobID, size  *int64
		updated      *time.Time
	)
	err := q.QueryRow(ctx, `
SELECT r.status, r.expires_at, r.expired_at, p.language, p.status, p.failure_code, p.job_id, p.sha256, p.size_bytes, p.updated_at,
       CASE WHEN p.status = 'ready' AND r.expires_at > $3 AND r.expired_at IS NULL THEN p.body END
FROM reports r LEFT JOIN report_pdfs p ON p.report_id = r.id AND p.language = $2
WHERE r.id = $1 AND r.deleted_at IS NULL`, id, language, now).
		Scan(&out.Status, &out.ExpiresAt, &out.ExpiredAt, &lang, &status, &failure, &jobID, &sum, &size, &updated, &out.Body)
	if errors.Is(err, pgx.ErrNoRows) {
		return PDFRead{}, ErrNotFound
	}
	if err != nil {
		return PDFRead{}, fmt.Errorf("read pdf: %w", err)
	}
	out.ExpiresAt = out.ExpiresAt.UTC()
	out.ExpiredAt = utcPtr(out.ExpiredAt)
	if lang != nil {
		out.PDF = &ReportPDF{
			ReportID: id, Language: *lang, Status: *status, FailureCode: failure, JobID: *jobID,
			SHA256: sum, SizeBytes: size, UpdatedAt: updated.UTC(),
		}
	}
	return out, nil
}

// MarkReportDeleted sets deleted_at on a report not deleted and returns the
// status it had; ErrNotFound when absent or already deleted.
func MarkReportDeleted(ctx context.Context, tx pgx.Tx, id uuid.UUID, now time.Time) (status string, err error) {
	err = tx.QueryRow(ctx, `
UPDATE reports SET deleted_at = $2, updated_at = $2 WHERE id = $1 AND deleted_at IS NULL RETURNING status`, id, now).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("mark report deleted: %w", err)
	}
	return status, nil
}
