package auth

import (
	"context"
	"errors"
	"runtime"
	"testing"
)

func mustHold(t *testing.T, c *clientSlots, key string) func() {
	t.Helper()
	release, err := c.hold(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return release
}

func TestClientSlotsSameKeySerialises(t *testing.T) {
	var c clientSlots
	release := mustHold(t, &c, "a")
	entered := make(chan func())
	go func() { entered <- mustHold(t, &c, "a") }()
	for c.users("a") != 2 {
		runtime.Gosched()
	}
	select {
	case <-entered:
		t.Fatal("second hold entered while the first was held")
	default:
	}
	release()
	(<-entered)()
	if got := len(c.held); got != 0 {
		t.Fatalf("entries left = %d, want 0", got)
	}
}

func TestClientSlotsDifferentKeysDoNotBlock(t *testing.T) {
	var c clientSlots
	a := mustHold(t, &c, "a")
	b := mustHold(t, &c, "b")
	a()
	b()
	if got := len(c.held); got != 0 {
		t.Fatalf("entries left = %d, want 0", got)
	}
}

func TestClientSlotsWaiterContextEnds(t *testing.T) {
	var c clientSlots
	release := mustHold(t, &c, "a")
	ctx, cancel := context.WithCancel(context.Background())
	errs := make(chan error)
	go func() {
		_, err := c.hold(ctx, "a")
		errs <- err
	}()
	for c.users("a") != 2 {
		runtime.Gosched()
	}
	cancel()
	if err := <-errs; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if got := c.users("a"); got != 1 {
		t.Fatalf("users after the waiter left = %d, want 1", got)
	}
	release()
	if got := len(c.held); got != 0 {
		t.Fatalf("entries left = %d, want 0", got)
	}
}

func TestClientSlotsReleaseLetsOneWaiterIn(t *testing.T) {
	var c clientSlots
	release := mustHold(t, &c, "a")
	entered := make(chan func(), 2)
	for range 2 {
		go func() { entered <- mustHold(t, &c, "a") }()
	}
	for c.users("a") != 3 {
		runtime.Gosched()
	}
	release()
	first := <-entered
	select {
	case <-entered:
		t.Fatal("two waiters held the slot at once")
	default:
	}
	first()
	(<-entered)()
	if got := len(c.held); got != 0 {
		t.Fatalf("entries left = %d, want 0", got)
	}
}

func TestClientSlotsEmptyAfterCycle(t *testing.T) {
	var c clientSlots
	mustHold(t, &c, "a")()
	if got := len(c.held); got != 0 {
		t.Fatalf("entries left = %d, want 0", got)
	}
}
