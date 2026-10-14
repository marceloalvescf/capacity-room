package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
	"github.com/spf13/cobra"

	"github.com/marceloalvescf/capacity-room/internal/platform/health"
	"github.com/marceloalvescf/capacity-room/internal/platform/netpolicy"
	"github.com/marceloalvescf/capacity-room/internal/platform/profiles"
	"github.com/marceloalvescf/capacity-room/internal/platform/settings"
	"github.com/marceloalvescf/capacity-room/internal/platform/store"
	"github.com/marceloalvescf/capacity-room/internal/platform/worker"
	"github.com/marceloalvescf/capacity-room/internal/redact"
	"github.com/marceloalvescf/capacity-room/internal/typst"
)

// typstProbeTimeout bounds the check of the Typst version at start.
const typstProbeTimeout = 10 * time.Second

// healthShutdownTimeout bounds the stop of the health server.
const healthShutdownTimeout = 5 * time.Second

// poolHeadroom is what the pool holds beyond the worker slots: the listener
// of River, its leader and maintenance work, and the handlers' own
// transactions.
const poolHeadroom = 8

// workerPoolConns is the floor of MaxConns of the worker's pool.
func workerPoolConns(loaded settings.Settings) int32 {
	return int32(loaded.WorkerSlots + poolHeadroom)
}

// newWorkerCmd builds the `worker` subcommand: run the collect, render, and
// maintenance jobs until the context ends.
func newWorkerCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	var config, listen string

	cmd := &cobra.Command{
		Use:           "worker",
		Short:         "Run the report jobs of the platform.",
		Args:          noArgument("worker"),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWorker(ctx, env, redactor, cmd, config, listen)
		},
	}
	cmd.Flags().StringVar(&config, "config", platformConfigDefault, "Path of the TOML settings file.")
	cmd.Flags().StringVar(&listen, "health-listen", ":8091", "Address of the health routes.")
	return cmd
}

// runWorker loads the settings, probes Typst, opens the pool, applies the
// migrations, listens for health, starts the River client, and holds it
// until ctx ends or the health server fails, then stops it: the running jobs
// get worker_drain_seconds to finish, then are cancelled and get as long
// again. A job that has not returned by then is an error.
func runWorker(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, config, listen string) error {
	loaded, err := loadPlatformSettings(env, redactor, config)
	if err != nil {
		return err
	}
	tc, err := locateTypst(ctx, env)
	if err != nil {
		return &exitError{1, err}
	}
	pool, err := openPool(ctx, loaded, workerPoolConns(loaded))
	if err != nil {
		return err
	}
	// Closing the pool waits for every connection a handler holds, so a
	// pool left to a job that did not stop is not closed: the process exits.
	abandoned := false
	defer func() {
		if !abandoned {
			pool.Close()
		}
	}()
	if _, err := store.Migrate(ctx, pool); err != nil {
		return &exitError{1, err}
	}

	logger := newLogger(cmd.ErrOrStderr(), redactor)
	client, err := buildWorker(pool, loaded, tc, env, logger, redactor)
	if err != nil {
		return &exitError{1, err}
	}

	// The port is taken before the client starts, so that a taken port
	// fails the command before any job is fetched; requests wait in the
	// backlog until the client has started.
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return &exitError{1, fmt.Errorf("cannot listen on %s: %w", listen, osError(err))}
	}
	lifecycle := &worker.Lifecycle{Client: client}
	healthServer := newHTTPServer(health.Handler([]health.Check{
		health.Database(pool), health.Migrations(pool), lifecycle.Check(),
	}), logger)
	defer func() { _ = listener.Close() }()

	// Start gets a context that outlives ctx: the stop sequence is
	// the lifecycle's, not River's.
	if err := client.Start(context.WithoutCancel(ctx)); err != nil {
		return &exitError{1, fmt.Errorf("cannot start the worker: %w", err)}
	}
	served := make(chan error, 1)
	go func() { served <- healthServer.Serve(listener) }()
	logger.Info(fmt.Sprintf("worker: started with %d slots", loaded.WorkerSlots))

	var failure error
	select {
	case err := <-served:
		failure = &exitError{1, fmt.Errorf("health server failed: %w", err)}
		var cancel context.CancelFunc
		ctx, cancel = context.WithCancel(ctx)
		cancel()
	case <-client.Stopped():
	case <-ctx.Done():
	}
	logger.Info("worker: stopping")

	stopErr := lifecycle.Run(ctx, loaded.WorkerDrain)
	abandoned = errors.Is(stopErr, worker.ErrJobsDidNotStop)

	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), healthShutdownTimeout)
	defer cancel()
	_ = healthServer.Shutdown(shutdown)

	switch {
	case failure != nil:
		return failure
	case stopErr != nil:
		return &exitError{1, stopErr}
	}
	logger.Info("worker: stopped")
	return nil
}

// locateTypst finds Typst and checks that it is the pinned version.
func locateTypst(ctx context.Context, env Env) (typst.Toolchain, error) {
	tc, err := typst.Locate(env.Getenv, env.LookPath)
	if err != nil {
		return typst.Toolchain{}, err
	}
	probe, cancel := context.WithTimeout(ctx, typstProbeTimeout)
	defer cancel()
	if err := tc.Probe(probe); err != nil {
		return typst.Toolchain{}, err
	}
	return tc, nil
}

// buildWorker assembles the River client of the worker from the pool, the
// loaded settings, the Typst toolchain, the environment, the logger, and the
// redactor that learns every credential the profiles read.
func buildWorker(pool *pgxpool.Pool, loaded settings.Settings, tc typst.Toolchain, env Env, logger *slog.Logger, redactor *redact.Redactor) (*river.Client[pgx.Tx], error) {
	return worker.NewClient(worker.Config{
		Slots: loaded.WorkerSlots,
		Deps: worker.Deps{
			Pool: pool,
			Profiles: &profiles.Service{
				Pool: pool,
				Policy: netpolicy.Policy{
					Hostnames: loaded.AllowedEndpointHostnames,
					Networks:  loaded.AllowedEndpointCIDRs,
				},
				Settings: loaded,
				Now:      env.Now,
				Reveal:   func(secret string) { redactor.Add(secret) },
			},
			Settings:  loaded,
			Toolchain: tc,
			TempDir:   env.TempDir,
			Version:   env.Version,
			Now:       env.Now,
			Logger:    logger,
		},
	})
}
