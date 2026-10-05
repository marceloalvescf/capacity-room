package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

// Throttle is the sign-in rate limit.
type Throttle struct {
	Secret []byte // the server secret
}

// The limits of the sign-in throttle.
const (
	ThrottleMaxFailures = 5
	ThrottleWindow      = 300 * time.Second
	ThrottleBlock       = 300 * time.Second
)

// Key is HMAC-SHA256 of normalised username, a NUL byte, and the client
// address, under the secret.
func (t Throttle) Key(username, clientAddress string) []byte {
	m := hmac.New(sha256.New, t.Secret)
	m.Write([]byte(NormalizeUsername(username)))
	m.Write([]byte{0})
	m.Write([]byte(clientAddress))
	return m.Sum(nil)
}

// ErrThrottled carries the seconds a client must wait.
type ErrThrottled struct{ RetryAfter int }

func (e *ErrThrottled) Error() string { return "too many sign-in attempts" }

// Decide is the pure part of Check: the seconds to wait and whether the
// key is blocked at now.
func Decide(row *store.Throttle, now time.Time) (retryAfter int, blocked bool) {
	if row == nil || row.BlockedUntil == nil {
		return 0, false
	}
	remaining := row.BlockedUntil.Sub(now)
	if remaining <= 0 {
		return 0, false
	}
	return max(1, int(math.Ceil(remaining.Seconds()))), true
}

// Check returns *ErrThrottled when the key is blocked at now.
func (t Throttle) Check(ctx context.Context, q store.Querier, key []byte, now time.Time) error {
	row, err := store.GetThrottle(ctx, q, key)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if retry, blocked := Decide(&row, now); blocked {
		return &ErrThrottled{RetryAfter: retry}
	}
	return nil
}

// RecordFailure counts one failure in a transaction, starting a window
// when none is open or the open one ran out, and blocks on the fifth.
func (t Throttle) RecordFailure(ctx context.Context, pool *pgxpool.Pool, key []byte, now time.Time) error {
	return store.Transact(ctx, pool, func(tx pgx.Tx) error {
		row, err := store.GetThrottleForUpdate(ctx, tx, key)
		next := store.Throttle{KeyHash: key, WindowStartedAt: now, Failures: 1, UpdatedAt: now}
		switch {
		case errors.Is(err, store.ErrNotFound):
		case err != nil:
			return err
		case now.Sub(row.WindowStartedAt) < ThrottleWindow:
			next.WindowStartedAt = row.WindowStartedAt
			next.Failures = row.Failures + 1
		}
		if next.Failures >= ThrottleMaxFailures {
			until := now.Add(ThrottleBlock)
			next.BlockedUntil = &until
		}
		return store.UpsertThrottle(ctx, tx, next)
	})
}

// RecordSuccess resets the row when it exists.
func (t Throttle) RecordSuccess(ctx context.Context, q store.Querier, key []byte, now time.Time) error {
	_, err := store.GetThrottle(ctx, q, key)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return store.UpsertThrottle(ctx, q, store.Throttle{KeyHash: key, WindowStartedAt: now, UpdatedAt: now})
}
