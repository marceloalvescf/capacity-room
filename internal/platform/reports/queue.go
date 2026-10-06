package reports

import (
	"fmt"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

// NewQueue returns a River client that only inserts jobs: it has no queues
// and no workers, and it is never started.
func NewQueue(pool *pgxpool.Pool, logger *slog.Logger) (*river.Client[pgx.Tx], error) {
	client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{Logger: logger})
	if err != nil {
		return nil, fmt.Errorf("create queue client: %w", err)
	}
	return client, nil
}
