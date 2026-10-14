//go:build postgres

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

func TestThrottleUpsert(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	key := []byte("key")
	first := store.Throttle{KeyHash: key, WindowStartedAt: now, Failures: 1, UpdatedAt: now}
	if err := store.UpsertThrottle(ctx, pool, first); err != nil {
		t.Fatal(err)
	}
	blocked := now.Add(15 * time.Minute)
	second := store.Throttle{KeyHash: key, WindowStartedAt: now.Add(time.Minute), Failures: 3, BlockedUntil: &blocked, UpdatedAt: now.Add(2 * time.Minute)}
	if err := store.UpsertThrottle(ctx, pool, second); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.KeyHash) != "key" || got.Failures != 3 ||
		!got.WindowStartedAt.Equal(time.Date(2026, 10, 5, 12, 1, 0, 0, time.UTC)) ||
		got.BlockedUntil == nil || !got.BlockedUntil.Equal(time.Date(2026, 10, 5, 12, 15, 0, 0, time.UTC)) ||
		!got.UpdatedAt.Equal(time.Date(2026, 10, 5, 12, 2, 0, 0, time.UTC)) {
		t.Fatalf("got %+v", got)
	}
}

func TestGetThrottleNotFound(t *testing.T) {
	pool := storetest.New(t)
	_, err := store.GetThrottle(context.Background(), pool, []byte("nothing"))
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}
