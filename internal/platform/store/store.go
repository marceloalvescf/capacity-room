package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

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
		// depend on it.
		_ = tx.Rollback(context.WithoutCancel(ctx))
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
