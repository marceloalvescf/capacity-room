//go:build postgres

package storetest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

const dsnVariable = "KCR_TEST_DATABASE_DSN"

var databaseName = regexp.MustCompile(`^kcr_test_[0-9a-f]{16}$`)

// New creates a database on the server of KCR_TEST_DATABASE_DSN, named
// kcr_test_<16 hex characters>, applies the migrations to it, and returns a
// pool connected to it. The database is dropped when t ends. The test
// fails when the variable is unset.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool := NewEmpty(t)
	if _, err := store.Migrate(context.Background(), pool); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return pool
}

// NewEmpty is New without the migrations: a fresh, empty database.
func NewEmpty(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	dsn := os.Getenv(dsnVariable)
	if dsn == "" {
		t.Fatalf("%s is not set", dsnVariable)
	}

	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to the test server: %v", err)
	}

	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("name the database: %v", err)
	}
	name := "kcr_test_" + hex.EncodeToString(suffix[:])
	if !databaseName.MatchString(name) {
		_ = admin.Close(ctx)
		t.Fatalf("database name %q is not allowed", name)
	}

	if _, err := admin.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		_ = admin.Close(ctx)
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+name); err != nil {
			t.Errorf("drop database %s: %v", name, err)
		}
		_ = admin.Close(ctx)
	})

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse %s: %v", dsnVariable, err)
	}
	u.Path = "/" + name

	pool, err := store.Open(ctx, u.String(), 0)
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	// Registered after the drop, so it runs first: DROP DATABASE fails while
	// a connection to the database exists.
	t.Cleanup(pool.Close)
	return pool
}
