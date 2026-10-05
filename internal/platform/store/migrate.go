package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// LockID is the advisory lock every migration run takes, the number of the
// original's entrypoint.
const LockID int64 = 72427301

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies every pending migration of the embedded folder under the
// session lock LockID and returns the version the schema is at afterwards.
func Migrate(ctx context.Context, pool *pgxpool.Pool) (int64, error) {
	db, provider, err := newProvider(pool)
	if err != nil {
		return 0, err
	}
	defer func() { _ = db.Close() }()
	defer func() { _ = provider.Close() }()

	if _, err := provider.Up(ctx); err != nil {
		return 0, fmt.Errorf("apply migrations: %w", err)
	}
	version, err := provider.GetDBVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

// HasPending reports whether a migration of the embedded folder is not yet
// applied. It takes no lock.
func HasPending(ctx context.Context, pool *pgxpool.Pool) (bool, error) {
	db, provider, err := newProvider(pool)
	if err != nil {
		return false, err
	}
	defer func() { _ = db.Close() }()
	defer func() { _ = provider.Close() }()

	pending, err := provider.HasPending(ctx)
	if err != nil {
		return false, fmt.Errorf("check pending migrations: %w", err)
	}
	return pending, nil
}

func newProvider(pool *pgxpool.Pool) (*sql.DB, *goose.Provider, error) {
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(LockID))
	if err != nil {
		return nil, nil, fmt.Errorf("make migration lock: %w", err)
	}
	db := stdlib.OpenDBFromPool(pool)
	fsys, err := fs.Sub(migrations, "migrations")
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("open migrations: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys,
		goose.WithSessionLocker(locker), goose.WithDisableGlobalRegistry(true))
	if err != nil {
		_ = db.Close()
		return nil, nil, fmt.Errorf("make migration provider: %w", err)
	}
	return db, provider, nil
}
