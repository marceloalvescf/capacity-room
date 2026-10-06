package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// PDFKey names one row of report_pdfs.
type PDFKey struct {
	ReportID uuid.UUID
	Language string
}

// jobGone is a condition that holds when the River job whose id is the
// column jobID no longer runs: it is absent, or in a final state.
func jobGone(jobID string) string {
	return `NOT EXISTS (
    SELECT 1 FROM river_job j
    WHERE j.id = ` + jobID + ` AND j.state NOT IN ('discarded', 'cancelled', 'completed'))`
}

func scanIDs(rows pgx.Rows) ([]uuid.UUID, error) {
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ReconcileReports fails up to limit queued or running reports whose collect
// job is absent from river_job or final, with the code JOB_ATTEMPTS_EXHAUSTED.
// Rows another transaction holds are left alone. It returns the failed ids.
func ReconcileReports(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
WITH due AS (
    SELECT r.id FROM reports r
    WHERE r.status IN ('queued', 'running') AND `+jobGone("r.collect_job_id")+`
    ORDER BY r.created_at, r.id
    LIMIT $2
    FOR UPDATE OF r SKIP LOCKED)
UPDATE reports
SET status = 'failed', phase = 'complete', failure_code = 'JOB_ATTEMPTS_EXHAUSTED', completed_at = $1, updated_at = $1
FROM due
WHERE reports.id = due.id AND reports.status IN ('queued', 'running')
RETURNING reports.id`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("reconcile reports: %w", err)
	}
	ids, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("reconcile reports: %w", err)
	}
	return ids, nil
}

// ReconcilePDFs fails up to limit pending PDF rows whose render job is absent
// from river_job or final, with the code PDF_ATTEMPTS_EXHAUSTED, and moves a
// report to phase complete when none of its rows is pending any more. The
// report is locked first; a report another transaction holds is left alone,
// and so is one past its expires_at, which expiry settles.
func ReconcilePDFs(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]PDFKey, error) {
	rows, err := tx.Query(ctx, `
WITH due AS (
    SELECT p.report_id, p.language FROM report_pdfs p
    JOIN reports r ON r.id = p.report_id
    WHERE p.status = 'pending' AND r.status = 'collected' AND r.expires_at > $1 AND `+jobGone("p.job_id")+`
    ORDER BY p.updated_at, p.report_id, p.language
    LIMIT $2
    FOR UPDATE OF r SKIP LOCKED)
UPDATE report_pdfs p
SET status = 'failed', failure_code = 'PDF_ATTEMPTS_EXHAUSTED', updated_at = $1
FROM due
WHERE p.report_id = due.report_id AND p.language = due.language AND p.status = 'pending'
RETURNING p.report_id, p.language`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("reconcile pdfs: %w", err)
	}
	defer rows.Close()
	var keys []PDFKey
	seen := map[uuid.UUID]bool{}
	var reportIDs []uuid.UUID
	for rows.Next() {
		var k PDFKey
		if err := rows.Scan(&k.ReportID, &k.Language); err != nil {
			return nil, fmt.Errorf("reconcile pdfs: %w", err)
		}
		keys = append(keys, k)
		if !seen[k.ReportID] {
			seen[k.ReportID] = true
			reportIDs = append(reportIDs, k.ReportID)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reconcile pdfs: %w", err)
	}
	rows.Close()
	if len(reportIDs) == 0 {
		return nil, nil
	}
	_, err = tx.Exec(ctx, `
UPDATE reports
SET phase = 'complete', completed_at = $2, updated_at = $2
WHERE id = ANY($1) AND status = 'collected' AND phase = 'rendering'
  AND NOT EXISTS (SELECT 1 FROM report_pdfs p WHERE p.report_id = reports.id AND p.status = 'pending')`,
		reportIDs, now)
	if err != nil {
		return nil, fmt.Errorf("complete reconciled reports: %w", err)
	}
	return keys, nil
}

// ExpireDueReports expires up to limit collected or failed reports whose
// expires_at is not after now, oldest first: their snapshot and PDF rows are
// deleted and expired_at is set. A report still rendering becomes complete,
// completed_at now, since its PDF rows are gone. Rows another transaction holds are left
// alone. It returns the expired ids.
func ExpireDueReports(ctx context.Context, tx pgx.Tx, now time.Time, limit int) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
SELECT id FROM reports
WHERE status IN ('collected', 'failed') AND expires_at <= $1 AND expired_at IS NULL
ORDER BY expires_at, id
LIMIT $2
FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, fmt.Errorf("expire reports: %w", err)
	}
	due, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("expire reports: %w", err)
	}
	if len(due) == 0 {
		return nil, nil
	}
	for _, sql := range []string{
		`DELETE FROM report_pdfs WHERE report_id = ANY($1)`,
		`DELETE FROM report_snapshots WHERE report_id = ANY($1)`,
	} {
		if _, err := tx.Exec(ctx, sql, due); err != nil {
			return nil, fmt.Errorf("expire reports: %w", err)
		}
	}
	rows, err = tx.Query(ctx, `
UPDATE reports
SET expired_at = $2, updated_at = $2,
    phase = CASE WHEN phase = 'rendering' THEN 'complete' ELSE phase END,
    completed_at = CASE WHEN phase = 'rendering' THEN $2 ELSE completed_at END
WHERE id = ANY($1) AND expired_at IS NULL
RETURNING id`, due, now)
	if err != nil {
		return nil, fmt.Errorf("expire reports: %w", err)
	}
	ids, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("expire reports: %w", err)
	}
	return ids, nil
}

// PurgeExpiredReports deletes up to limit expired reports whose expires_at
// plus kept is not after now, oldest first: their PDF, snapshot, and
// idempotency rows, then the reports. Rows another transaction holds are left alone. It returns the
// purged ids.
func PurgeExpiredReports(ctx context.Context, tx pgx.Tx, now time.Time, kept time.Duration, limit int) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, `
SELECT id FROM reports
WHERE expired_at IS NOT NULL AND expires_at <= $1
ORDER BY expires_at, id
LIMIT $2
FOR UPDATE SKIP LOCKED`, now.Add(-kept), limit)
	if err != nil {
		return nil, fmt.Errorf("purge reports: %w", err)
	}
	due, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("purge reports: %w", err)
	}
	if len(due) == 0 {
		return nil, nil
	}
	for _, sql := range []string{
		`DELETE FROM report_pdfs WHERE report_id = ANY($1)`,
		`DELETE FROM report_snapshots WHERE report_id = ANY($1)`,
		`DELETE FROM report_idempotency_keys WHERE report_id = ANY($1)`,
	} {
		if _, err := tx.Exec(ctx, sql, due); err != nil {
			return nil, fmt.Errorf("purge reports: %w", err)
		}
	}
	rows, err = tx.Query(ctx, `DELETE FROM reports WHERE id = ANY($1) RETURNING id`, due)
	if err != nil {
		return nil, fmt.Errorf("purge reports: %w", err)
	}
	ids, err := scanIDs(rows)
	if err != nil {
		return nil, fmt.Errorf("purge reports: %w", err)
	}
	return ids, nil
}

// PurgeThrottle deletes up to limit sign-in throttle rows whose window ended
// at least window ago and that are not blocked at now. It returns the count.
func PurgeThrottle(ctx context.Context, q Querier, now time.Time, window time.Duration, limit int) (int64, error) {
	tag, err := q.Exec(ctx, `
DELETE FROM login_rate_limits
WHERE key_hash IN (
    SELECT key_hash FROM login_rate_limits
    WHERE window_started_at <= $2 AND (blocked_until IS NULL OR blocked_until <= $1)
    LIMIT $3
    FOR UPDATE SKIP LOCKED)`, now, now.Add(-window), limit)
	if err != nil {
		return 0, fmt.Errorf("purge throttle: %w", err)
	}
	return tag.RowsAffected(), nil
}

// PurgeSessions deletes up to limit sessions that were revoked or expired at
// least ttl ago. A session that was rotated from one of them keeps its row
// and loses its link to the parent. It returns the count deleted.
func PurgeSessions(ctx context.Context, tx pgx.Tx, now time.Time, ttl time.Duration, limit int) (int64, error) {
	cutoff := now.Add(-ttl)
	rows, err := tx.Query(ctx, `
SELECT id FROM sessions
WHERE revoked_at <= $1 OR expires_at <= $1
ORDER BY expires_at, id
LIMIT $2
FOR UPDATE SKIP LOCKED`, cutoff, limit)
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	due, err := scanIDs(rows)
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	if len(due) == 0 {
		return 0, nil
	}
	if _, err := tx.Exec(ctx, `UPDATE sessions SET rotated_from_id = NULL WHERE rotated_from_id = ANY($1)`, due); err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	tag, err := tx.Exec(ctx, `DELETE FROM sessions WHERE id = ANY($1)`, due)
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	return tag.RowsAffected(), nil
}
