//go:build postgres

package auth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

var base = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestTakeBlocksOnFifth(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 5; i++ {
		if err := th.Take(ctx, pool, key, base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatalf("attempt %d: %v", i+1, err)
		}
	}
	var te *ErrThrottled
	if err := th.Take(ctx, pool, key, base.Add(4*time.Second)); !errors.As(err, &te) || te.RetryAfter != 300 {
		t.Fatalf("at now+4s: %v", err)
	}
	if err := th.Take(ctx, pool, key, base.Add(5*time.Second)); !errors.As(err, &te) || te.RetryAfter != 299 {
		t.Fatalf("at now+5s: %v", err)
	}
	row, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 5 || !row.UpdatedAt.Equal(base.Add(4*time.Second)) {
		t.Fatalf("a refused attempt changed the row: %+v", row)
	}
}

func TestTakeStartsNewWindowAfter300s(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 4; i++ {
		if err := th.Take(ctx, pool, key, base); err != nil {
			t.Fatal(err)
		}
	}
	if err := th.Take(ctx, pool, key, base.Add(ThrottleWindow)); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 1 || row.BlockedUntil != nil || !row.WindowStartedAt.Equal(base.Add(ThrottleWindow)) {
		t.Fatalf("row %+v", row)
	}
}

func TestTakeDuringBlockKeepsTheBlock(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 5; i++ {
		if err := th.Take(ctx, pool, key, base); err != nil {
			t.Fatal(err)
		}
	}
	// The window has run out, the block has not: the block holds.
	var te *ErrThrottled
	if err := th.Take(ctx, pool, key, base.Add(ThrottleWindow-time.Second)); !errors.As(err, &te) || te.RetryAfter != 1 {
		t.Fatalf("got %v", err)
	}
	row, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 5 || row.BlockedUntil == nil || !row.BlockedUntil.Equal(base.Add(ThrottleBlock)) {
		t.Fatalf("row %+v", row)
	}
}

func TestRecordSuccessResets(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 3; i++ {
		if err := th.Take(ctx, pool, key, base); err != nil {
			t.Fatal(err)
		}
	}
	if err := th.RecordSuccess(ctx, pool, key, base); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 0 || row.BlockedUntil != nil {
		t.Fatalf("row %+v", row)
	}
	unknown := th.Key("nobody", "10.0.0.2")
	if err := th.RecordSuccess(ctx, pool, unknown, base); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetThrottle(ctx, pool, unknown); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("row inserted: %v", err)
	}
}

func TestBlockExpires(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 5; i++ {
		if err := th.Take(ctx, pool, key, base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := th.Take(ctx, pool, key, base.Add(304*time.Second)); err != nil {
		t.Fatalf("block still holds: %v", err)
	}
	row, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 1 || row.BlockedUntil != nil {
		t.Fatalf("row %+v", row)
	}
}

// takeConcurrently runs n attempts on key at base at once and returns how
// many were let through.
func takeConcurrently(t *testing.T, pool store.Querier, th Throttle, key []byte, n int) int {
	t.Helper()
	var wg sync.WaitGroup
	var mu sync.Mutex
	passed := 0
	start := make(chan struct{})
	for range n {
		wg.Go(func() {
			<-start
			err := th.Take(context.Background(), pool, key, base)
			var te *ErrThrottled
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				passed++
			case !errors.As(err, &te):
				t.Errorf("take: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()
	return passed
}

func TestTakeUnderConcurrency(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	// A row whose window ran out, so every attempt meets an existing row.
	if err := th.Take(ctx, pool, key, base.Add(-ThrottleWindow)); err != nil {
		t.Fatal(err)
	}
	if n := takeConcurrently(t, pool, th, key, 30); n != 5 {
		t.Fatalf("%d of 30 concurrent attempts let through, want 5", n)
	}
	var te *ErrThrottled
	if err := th.Take(ctx, pool, key, base.Add(time.Second)); !errors.As(err, &te) || te.RetryAfter != 299 {
		t.Fatalf("after the burst: %v", err)
	}
}

func TestTakeOnFreshKeyUnderConcurrency(t *testing.T) {
	pool := storetest.New(t)
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	if n := takeConcurrently(t, pool, th, key, 40); n != 5 {
		t.Fatalf("%d of 40 concurrent attempts let through, want 5", n)
	}
	row, err := store.GetThrottle(context.Background(), pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 5 || row.BlockedUntil == nil || !row.BlockedUntil.Equal(base.Add(ThrottleBlock)) {
		t.Fatalf("row %+v", row)
	}
}
