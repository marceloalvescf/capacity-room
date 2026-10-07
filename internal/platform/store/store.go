package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rollbackTimeout bounds the rollback of a transaction that did not commit.
const rollbackTimeout = 5 * time.Second

// Open connects a pool to dsn and pings it once. MaxConns is raised to
// maxConnsFloor when the DSN sets fewer; zero keeps the DSN's value.
func Open(ctx context.Context, dsn string, maxConnsFloor int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	cfg.MaxConns = max(cfg.MaxConns, maxConnsFloor)
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// DownloadConns is the number of connections of the pool that reads the
// body of a view, a snapshot, or a PDF, and the number of downloads the API
// serves at once.
const DownloadConns = 4

// OpenDownloads connects the pool that reads report bodies to dsn and pings
// it once. A connection of this pool is closed when it is released, so no
// idle connection keeps the buffer of a large row.
func OpenDownloads(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("open download pool: %w", err)
	}
	cfg.MaxConns = DownloadConns
	cfg.MinConns = 0
	cfg.AfterRelease = func(*pgx.Conn) bool { return false }
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open download pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Transact runs fn inside one transaction, committing when fn returns nil
// and rolling back otherwise, including on a panic, which it re-raises. The
// transaction reads committed rows whatever the database default is, which
// the exact admission of reports depends on.
func Transact(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) (err error) {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		// The context may be the reason fn failed, so the rollback does not
		// depend on it; it has its own bound, so a database that stopped
		// answering does not hold the caller forever.
		rollback, cancel := context.WithTimeout(context.WithoutCancel(ctx), rollbackTimeout)
		defer cancel()
		_ = tx.Rollback(rollback)
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	committed = true
	return nil
}
