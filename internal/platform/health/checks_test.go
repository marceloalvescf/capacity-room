//go:build postgres

package health

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

func TestDatabaseAndMigrations(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	if !Database(pool).Run(ctx) || !Migrations(pool).Run(ctx) {
		t.Fatal("a migrated database must pass both checks")
	}
}

func TestMigrationsPending(t *testing.T) {
	ctx := context.Background()
	pool := storetest.NewEmpty(t)
	if !Database(pool).Run(ctx) {
		t.Fatal("the database answers")
	}
	if Migrations(pool).Run(ctx) {
		t.Fatal("an unmigrated database must fail the migrations check")
	}
}

func TestChecksDatabaseDown(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://x:y@127.0.0.1:1/x?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if Database(pool).Run(context.Background()) || Migrations(pool).Run(context.Background()) {
		t.Fatal("a down database must fail both checks")
	}
}

func TestChecksWithoutPool(t *testing.T) {
	if Database(nil).Run(context.Background()) || Migrations(nil).Run(context.Background()) {
		t.Fatal("no pool must fail both checks")
	}
}
