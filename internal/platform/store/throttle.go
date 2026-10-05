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

func scanThrottle(row pgx.Row) (Throttle, error) {
	var t Throttle
	if err := row.Scan(&t.KeyHash, &t.WindowStartedAt, &t.Failures, &t.BlockedUntil, &t.UpdatedAt); err != nil {
		return Throttle{}, err
	}
	t.WindowStartedAt = t.WindowStartedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	if t.BlockedUntil != nil {
		b := t.BlockedUntil.UTC()
		t.BlockedUntil = &b
	}
	return t, nil
}

// GetThrottleForUpdate reads the row and locks it for the transaction.
func GetThrottleForUpdate(ctx context.Context, tx pgx.Tx, keyHash []byte) (Throttle, error) {
	t, err := scanThrottle(tx.QueryRow(ctx, `
SELECT `+throttleColumns+` FROM login_rate_limits WHERE key_hash = $1 FOR UPDATE`, keyHash))
	if errors.Is(err, pgx.ErrNoRows) {
		return Throttle{}, ErrNotFound
	}
	if err != nil {
		return Throttle{}, fmt.Errorf("get throttle for update: %w", err)
	}
	return t, nil
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
