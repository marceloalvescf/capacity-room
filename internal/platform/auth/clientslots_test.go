package auth

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

// settle is how long a test gives a goroutine that must not get the turn to
// take it wrongly.
const settle = 50 * time.Millisecond

func mustHold(t *testing.T, c *clientSlots, key string) func() {
	t.Helper()
	release, err := c.hold(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	return release
}

// holdAsync holds key from a goroutine and sends its release once held.
func holdAsync(c *clientSlots, key string) <-chan func() {
	entered := make(chan func(), 1)
	go func() {
		release, err := c.hold(context.Background(), key)
		if err == nil {
			entered <- release
		}
	}()
	return entered
}

// waitUsers waits until key has n holders and waiters, for at most a second.
func waitUsers(t *testing.T, c *clientSlots, key string, n int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for c.users(key) != n {
		if time.Now().After(deadline) {
			t.Fatalf("users of %q = %d, want %d", key, c.users(key), n)
		}
		runtime.Gosched()
	}
}

// receive waits for a release from entered, for at most a second.
func receive(t *testing.T, entered <-chan func()) func() {
	t.Helper()
	select {
	case release := <-entered:
		return release
	case <-time.After(time.Second):
		t.Fatal("the waiter never got the turn")
		return nil
	}
}

func TestClientSlotsSameKeySerialises(t *testing.T) {
	var c clientSlots
	release := mustHold(t, &c, "a")
	entered := holdAsync(&c, "a")
	waitUsers(t, &c, "a", 2)
	select {
	case <-entered:
		t.Fatal("second hold entered while the first was held")
	case <-time.After(settle):
	}
	release()
	receive(t, entered)()
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
	errs := make(chan error, 1)
	go func() {
		_, err := c.hold(ctx, "a")
		errs <- err
	}()
	waitUsers(t, &c, "a", 2)
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
		go func() {
			if r, err := c.hold(context.Background(), "a"); err == nil {
				entered <- r
			}
		}()
	}
	waitUsers(t, &c, "a", 3)
	release()
	first := receive(t, entered)
	select {
	case <-entered:
		t.Fatal("two waiters held the slot at once")
	case <-time.After(settle):
	}
	first()
	receive(t, entered)()
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
