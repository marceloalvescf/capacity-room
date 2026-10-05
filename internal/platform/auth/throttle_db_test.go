//go:build postgres

package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

var base = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func TestRecordFailureBlocksOnFifth(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 5; i++ {
		at := base.Add(time.Duration(i) * time.Second)
		if err := th.RecordFailure(ctx, pool, key, at); err != nil {
			t.Fatal(err)
		}
		err := th.Check(ctx, pool, key, at)
		if i < 4 && err != nil {
			t.Fatalf("after failure %d: %v", i+1, err)
		}
	}
	var te *ErrThrottled
	if err := th.Check(ctx, pool, key, base.Add(4*time.Second)); !errors.As(err, &te) || te.RetryAfter != 300 {
		t.Fatalf("at now+4s: %v", err)
	}
	if err := th.Check(ctx, pool, key, base.Add(5*time.Second)); !errors.As(err, &te) || te.RetryAfter != 299 {
		t.Fatalf("at now+5s: %v", err)
	}
}

func TestRecordFailureStartsNewWindowAfter300s(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 4; i++ {
		if err := th.RecordFailure(ctx, pool, key, base); err != nil {
			t.Fatal(err)
		}
	}
	if err := th.RecordFailure(ctx, pool, key, base.Add(ThrottleWindow)); err != nil {
		t.Fatal(err)
	}
	row, err := store.GetThrottle(ctx, pool, key)
	if err != nil {
		t.Fatal(err)
	}
	if row.Failures != 1 || row.BlockedUntil != nil {
		t.Fatalf("row %+v", row)
	}
}

func TestRecordSuccessResets(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	th := Throttle{Secret: []byte("k")}
	key := th.Key("admin", "10.0.0.1")
	for i := 0; i < 3; i++ {
		if err := th.RecordFailure(ctx, pool, key, base); err != nil {
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
		if err := th.RecordFailure(ctx, pool, key, base.Add(time.Duration(i)*time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	if err := th.Check(ctx, pool, key, base.Add(304*time.Second)); err != nil {
		t.Fatalf("block still holds: %v", err)
	}
}
