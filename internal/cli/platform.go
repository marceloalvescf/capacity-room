package cli

import (
	"context"
	"errors"
	"fmt"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// platformConfigDefault is the settings file the platform commands read
// when --config is not given.
const platformConfigDefault = "platform.toml"

// openPlatform loads the settings of configPath, adds the DSN to the
// redactor, and opens the pool. It is the first thing serve and the admin
// commands that need the database do. A settings problem is exit code 2 and
// a database problem is exit code 1.
func openPlatform(ctx context.Context, env Env, redactor *redact.Redactor, configPath string) (settings.Settings, *pgxpool.Pool, error) {
	loaded, err := loadPlatformSettings(env, redactor, configPath)
	if err != nil {
		return settings.Settings{}, nil, err
	}
	pool, err := openPool(ctx, loaded, 0)
	if err != nil {
		return settings.Settings{}, nil, err
	}
	return loaded, pool, nil
}

// loadPlatformSettings reads the settings of configPath and adds the DSN to
// the redactor. A problem is exit code 2.
func loadPlatformSettings(env Env, redactor *redact.Redactor, configPath string) (settings.Settings, error) {
	loaded, err := settings.Load(configPath, env.Getenv)
	if err != nil {
		return settings.Settings{}, &exitError{2, err}
	}
	redactor.Add(loaded.DatabaseDSN)
	return loaded, nil
}

// openPool opens the pool of the loaded settings, with at least minConns
// connections allowed. A problem is exit code 1.
func openPool(ctx context.Context, loaded settings.Settings, minConns int32) (*pgxpool.Pool, error) {
	pool, err := store.Open(ctx, loaded.DatabaseDSN, minConns)
	if err != nil {
		return nil, &exitError{1, fmt.Errorf("cannot connect to the database: %w", err)}
	}
	return pool, nil
}

// noArgument is the Args check of a command that takes none. Like report's,
// it never repeats an argument: one might be a secret typed at the wrong
// place.
func noArgument(name string) func(*cobra.Command, []string) error {
	return func(_ *cobra.Command, args []string) error {
		if len(args) > 0 {
			return fmt.Errorf("%s takes no argument; found %d", name, len(args))
		}
		return nil
	}
}

// osError returns the operating system's own words for a failure of the
// network package, without the operation and address it repeats.
func osError(err error) error {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Err
	}
	return err
}
