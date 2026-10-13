//go:build postgres

package auth

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

// TestLockHoldsOffAPasswordWriter holds the users table lock in one
// transaction and starts a password write outside it, as the sign-in rehash
// does: the write waits until the lock is released, so a transaction that
// read a hash under the lock writes over that same hash.
func TestLockHoldsOffAPasswordWriter(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	user := newUser(t, pool, "lock-user", "lock-user-password", string(RoleOperator), "en")
	locked := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	free := func() { once.Do(func() { close(release) }) }
	t.Cleanup(free)
	done := make(chan error, 1)
	go func() {
		done <- store.Transact(ctx, pool, func(tx pgx.Tx) error {
			if err := store.LockUsers(ctx, tx); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked
	wrote := make(chan bool, 1)
	go func() {
		ok, err := store.UpdatePasswordHash(ctx, pool, user.ID, user.PasswordHash, "other", time.Now())
		if err != nil {
			t.Error(err)
		}
		wrote <- ok
	}()
	select {
	case <-wrote:
		t.Fatal("the password write did not wait for the table lock")
	case <-time.After(300 * time.Millisecond):
	}
	free()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if ok := <-wrote; !ok {
		t.Fatal("the password write missed after the lock was released")
	}
}
