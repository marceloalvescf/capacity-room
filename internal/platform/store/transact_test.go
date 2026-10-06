//go:build postgres

package store_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func TestTransactReadsCommitted(t *testing.T) {
	ctx := context.Background()
	admin := storetest.New(t)
	var name string
	if err := admin.QueryRow(ctx, `SELECT current_database()`).Scan(&name); err != nil {
		t.Fatal(err)
	}
	alter := fmt.Sprintf(`ALTER DATABASE %s SET default_transaction_isolation = 'repeatable read'`, pgx.Identifier{name}.Sanitize())
	if _, err := admin.Exec(ctx, alter); err != nil {
		t.Fatal(err)
	}
	// A fresh pool opens fresh connections, which take the new default.
	pool, err := store.Open(ctx, admin.Config().ConnString(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var plain string
	if err := pool.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&plain); err != nil || plain != "repeatable read" {
		t.Fatalf("default isolation = %q (%v)", plain, err)
	}
	var level string
	err = store.Transact(ctx, pool, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&level)
	})
	if err != nil {
		t.Fatal(err)
	}
	if level != "read committed" {
		t.Fatalf("isolation = %q", level)
	}
}
