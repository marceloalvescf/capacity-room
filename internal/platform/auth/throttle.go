package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"math"
	"time"

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

// Decide is the pure part of Take: the seconds to wait and whether the
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

// Take records one attempt under key at now and reports whether it may
// proceed. It is one statement, so concurrent attempts serialise on the
// row: a blocked key answers *ErrThrottled without counting; otherwise the
// window opens when none is open or the open one ran out, the attempt is
// counted, and the fifth attempt of a window sets the block. A later
// RecordSuccess resets the row.
func (t Throttle) Take(ctx context.Context, q store.Querier, key []byte, now time.Time) error {
	row, refused, err := store.TakeThrottle(ctx, q, key, now, ThrottleWindow, ThrottleBlock, ThrottleMaxFailures)
	if err != nil {
		return err
	}
	if retry, blocked := Decide(&row, now); refused && blocked {
		return &ErrThrottled{RetryAfter: retry}
	}
	return nil
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
