package auth

import (
	"context"
	"sync"
)

// clientSlots lets one holder at a time per client key. A sign-in holds its
// client's slot around its password work, so one client cycling user names
// queues behind itself and cannot take the global hash slots from others.
// The zero value is ready to use.
type clientSlots struct {
	mu   sync.Mutex
	held map[string]*clientSlot
}

// clientSlot is the turn of one client and the count of holders and
// waiters that keep its entry alive.
type clientSlot struct {
	turn  chan struct{}
	users int
}

// hold waits for the turn of key and returns the function that gives it
// back. A wait that ctx ends returns its error and holds nothing. The entry
// of a key is removed when its last holder or waiter leaves.
func (c *clientSlots) hold(ctx context.Context, key string) (release func(), err error) {
	c.mu.Lock()
	if c.held == nil {
		c.held = make(map[string]*clientSlot)
	}
	slot := c.held[key]
	if slot == nil {
		slot = &clientSlot{turn: make(chan struct{}, 1)}
		c.held[key] = slot
	}
	slot.users++
	c.mu.Unlock()

	leave := func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if slot.users--; slot.users == 0 {
			delete(c.held, key)
		}
	}
	select {
	case slot.turn <- struct{}{}:
	case <-ctx.Done():
		leave()
		return nil, ctx.Err()
	}
	return func() {
		<-slot.turn
		leave()
	}, nil
}

// users counts the holders and waiters of key.
func (c *clientSlots) users(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	if slot := c.held[key]; slot != nil {
		return slot.users
	}
	return 0
}
