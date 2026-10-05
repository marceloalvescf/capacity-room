//go:build postgres

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

var now = time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)

func newUser(username, role string, active bool) store.User {
	return store.User{
		ID:           uuid.New(),
		Username:     username,
		PasswordHash: "hash-" + username,
		Role:         role,
		Language:     "en",
		Active:       active,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func mustInsertUser(t *testing.T, q store.Querier, u store.User) {
	t.Helper()
	if err := store.InsertUser(context.Background(), q, u); err != nil {
		t.Fatalf("insert user: %v", err)
	}
}

func TestInsertAndGetUser(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u := newUser("amy", "operator", true)
	u.Language = "pt-BR"
	mustInsertUser(t, pool, u)

	got, err := store.GetUser(ctx, pool, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != u.ID || got.Username != "amy" || got.PasswordHash != "hash-amy" ||
		got.Role != "operator" || got.Language != "pt-BR" || !got.Active {
		t.Fatalf("got %+v, want %+v", got, u)
	}
	want := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	if !got.CreatedAt.Equal(want) || !got.UpdatedAt.Equal(want) {
		t.Fatalf("times %v %v, want %v", got.CreatedAt, got.UpdatedAt, want)
	}
	if got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
		t.Fatalf("times are not in UTC: %v %v", got.CreatedAt.Location(), got.UpdatedAt.Location())
	}
}

func TestInsertUserRefusesTakenName(t *testing.T) {
	pool := storetest.New(t)
	mustInsertUser(t, pool, newUser("amy", "reader", true))
	err := store.InsertUser(context.Background(), pool, newUser("amy", "reader", true))
	if !errors.Is(err, store.ErrUsernameTaken) {
		t.Fatalf("got %v, want ErrUsernameTaken", err)
	}
}

func TestGetUserNotFound(t *testing.T) {
	pool := storetest.New(t)
	_, err := store.GetUser(context.Background(), pool, uuid.New())
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("got %v, want ErrNotFound", err)
	}
}

func TestFindActiveUserByUsername(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	active := newUser("amy", "reader", true)
	inactive := newUser("kim", "reader", false)
	mustInsertUser(t, pool, active)
	mustInsertUser(t, pool, inactive)

	got, err := store.FindActiveUserByUsername(ctx, pool, "amy")
	if err != nil || got.ID != active.ID {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := store.FindActiveUserByUsername(ctx, pool, "kim"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("inactive: got %v, want ErrNotFound", err)
	}
}

func TestListUsersOrdersByUsername(t *testing.T) {
	pool := storetest.New(t)
	for _, name := range []string{"zed", "amy", "kim"} {
		mustInsertUser(t, pool, newUser(name, "reader", true))
	}
	users, err := store.ListUsers(context.Background(), pool)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, u := range users {
		names = append(names, u.Username)
	}
	if len(names) != 3 || names[0] != "amy" || names[1] != "kim" || names[2] != "zed" {
		t.Fatalf("got %v", names)
	}
}

func TestUpdateUserNeverWritesUsername(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()
	u := newUser("amy", "reader", true)
	mustInsertUser(t, pool, u)

	u.Username = "other"
	u.Role = "administrator"
	u.Language = "pt-BR"
	u.Active = false
	u.PasswordHash = "new-hash"
	u.UpdatedAt = now.Add(time.Hour)
	if err := store.UpdateUser(ctx, pool, u); err != nil {
		t.Fatal(err)
	}
	got, err := store.GetUser(ctx, pool, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Username != "amy" || got.Role != "administrator" || got.Language != "pt-BR" ||
		got.Active || got.PasswordHash != "new-hash" ||
		!got.UpdatedAt.Equal(time.Date(2026, 10, 5, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("got %+v", got)
	}

	missing := newUser("ghost", "reader", true)
	if err := store.UpdateUser(ctx, pool, missing); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("missing row: got %v, want ErrNotFound", err)
	}
}

func TestCountActiveAdministrators(t *testing.T) {
	pool := storetest.New(t)
	mustInsertUser(t, pool, newUser("a1", "administrator", true))
	mustInsertUser(t, pool, newUser("a2", "administrator", true))
	mustInsertUser(t, pool, newUser("a3", "administrator", false))
	mustInsertUser(t, pool, newUser("o1", "operator", true))
	n, err := store.CountActiveAdministrators(context.Background(), pool)
	if err != nil || n != 2 {
		t.Fatalf("got %d, %v, want 2", n, err)
	}
}

// expectBlocksUntilCommit starts call in a goroutine, asserts it does not
// return within 200 ms, commits first, and asserts call returns within 2 s.
func expectBlocksUntilCommit(t *testing.T, first pgx.Tx, call func() error) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- call() }()
	select {
	case err := <-done:
		t.Fatalf("second call returned before the first committed: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if err := first.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second call: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second call did not return after the first committed")
	}
}

func TestLockUsersAndCheckEmpty(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()
	empty, err := store.LockUsersAndCheckEmpty(ctx, tx1)
	if err != nil || !empty {
		t.Fatalf("empty table: got %v, %v, want true", empty, err)
	}

	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx2.Rollback(ctx) }()
	expectBlocksUntilCommit(t, tx1, func() error {
		_, err := store.LockUsersAndCheckEmpty(ctx, tx2)
		return err
	})
	if err := tx2.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	mustInsertUser(t, pool, newUser("amy", "reader", true))
	tx3, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx3.Rollback(ctx) }()
	empty, err = store.LockUsersAndCheckEmpty(ctx, tx3)
	if err != nil || empty {
		t.Fatalf("one user: got %v, %v, want false", empty, err)
	}
}

func TestLockUsersBlocksASecondLocker(t *testing.T) {
	pool := storetest.New(t)
	ctx := context.Background()

	tx1, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx1.Rollback(ctx) }()
	if err := store.LockUsers(ctx, tx1); err != nil {
		t.Fatal(err)
	}

	tx2, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx2.Rollback(ctx) }()
	expectBlocksUntilCommit(t, tx1, func() error {
		return store.LockUsers(ctx, tx2)
	})
}
