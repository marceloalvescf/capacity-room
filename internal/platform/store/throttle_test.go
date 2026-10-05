//go:build postgres

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
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

func TestGetThrottleForUpdateLocks(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	key := []byte("key")
	if err := store.UpsertThrottle(ctx, pool, store.Throttle{KeyHash: key, WindowStartedAt: now, Failures: 1, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()
	if _, err := store.GetThrottleForUpdate(ctx, tx1, key); err != nil {
		t.Fatal(err)
	}
	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx2.Rollback(ctx) }()
	expectBlocksUntilCommit(t, tx1, func() error {
		_, err := store.GetThrottleForUpdate(ctx, tx2, key)
		return err
	})
}

func TestGetThrottleNotFound(t *testing.T) {
	pool := storetest.New(t)
	_, err := store.GetThrottle(context.Background(), pool, []byte("nothing"))
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
	tx, err := pool.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := store.GetThrottleForUpdate(context.Background(), tx, []byte("nothing")); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("for update: got %v, want ErrNotFound", err)
	}
}
