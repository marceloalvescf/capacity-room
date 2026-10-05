package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Throttle is a row of the login_rate_limits table.
type Throttle struct {
	KeyHash         []byte
	WindowStartedAt time.Time
	Failures        int
	BlockedUntil    *time.Time
	UpdatedAt       time.Time
}

const throttleColumns = `key_hash, window_started_at, failures, blocked_until, updated_at`

func (t Throttle) inUTC() Throttle {
	t.WindowStartedAt = t.WindowStartedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	if t.BlockedUntil != nil {
		b := t.BlockedUntil.UTC()
		t.BlockedUntil = &b
	}
	return t
}

func scanThrottle(row pgx.Row) (Throttle, error) {
	var t Throttle
	if err := row.Scan(&t.KeyHash, &t.WindowStartedAt, &t.Failures, &t.BlockedUntil, &t.UpdatedAt); err != nil {
		return Throttle{}, err
	}
	return t.inUTC(), nil
}

// GetThrottle reads the row of keyHash.
func GetThrottle(ctx context.Context, q Querier, keyHash []byte) (Throttle, error) {
	t, err := scanThrottle(q.QueryRow(ctx, `
SELECT `+throttleColumns+` FROM login_rate_limits WHERE key_hash = $1`, keyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return Throttle{}, ErrNotFound
	}
	if err != nil {
		return Throttle{}, fmt.Errorf("get throttle: %w", err)
	}
	return t, nil
}

// UpsertThrottle inserts the row or replaces every column but the key.
func UpsertThrottle(ctx context.Context, q Querier, t Throttle) error {
	_, err := q.Exec(ctx, `
INSERT INTO login_rate_limits (key_hash, window_started_at, failures, blocked_until, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (key_hash) DO UPDATE
   SET window_started_at = EXCLUDED.window_started_at,
       failures = EXCLUDED.failures,
       blocked_until = EXCLUDED.blocked_until,
       updated_at = EXCLUDED.updated_at`,
		t.KeyHash, t.WindowStartedAt, t.Failures, t.BlockedUntil, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("upsert throttle: %w", err)
	}
	return nil
}

// TakeThrottle counts one attempt under keyHash at now in one statement, so
// that concurrent attempts serialise on the row. When the row is blocked at
// now it keeps every column and refused is true. Otherwise the window
// restarts at now with one attempt when none is open or the open one is at
// least window old, or the attempt is added to it; the row is blocked until
// now plus block when its attempts reach maxFailures. A missing row is
// inserted with one attempt and no block. The row returned is the row after
// the statement.
func TakeThrottle(ctx context.Context, q Querier, keyHash []byte, now time.Time, window, block time.Duration, maxFailures int) (row Throttle, refused bool, err error) {
	var t Throttle
	err = q.QueryRow(ctx, `
INSERT INTO login_rate_limits AS r (key_hash, window_started_at, failures, blocked_until, updated_at)
VALUES ($1, $2, 1, NULL, $2)
ON CONFLICT (key_hash) DO UPDATE
   SET window_started_at = CASE
           WHEN r.blocked_until > $2 THEN r.window_started_at
           WHEN r.window_started_at <= $3 THEN $2
           ELSE r.window_started_at END,
       failures = CASE
           WHEN r.blocked_until > $2 THEN r.failures
           WHEN r.window_started_at <= $3 THEN 1
           ELSE r.failures + 1 END,
       blocked_until = CASE
           WHEN r.blocked_until > $2 THEN r.blocked_until
           WHEN (CASE WHEN r.window_started_at <= $3 THEN 1 ELSE r.failures + 1 END) >= $5 THEN $4
           ELSE NULL END,
       updated_at = CASE
           WHEN r.blocked_until > $2 THEN r.updated_at
           ELSE $2 END
RETURNING `+throttleColumns+`, COALESCE(old.blocked_until > $2, false)`,
		keyHash, now, now.Add(-window), now.Add(block), maxFailures,
	).Scan(&t.KeyHash, &t.WindowStartedAt, &t.Failures, &t.BlockedUntil, &t.UpdatedAt, &refused)
	if err != nil {
		return Throttle{}, false, fmt.Errorf("take throttle: %w", err)
	}
	return t.inUTC(), refused, nil
}
