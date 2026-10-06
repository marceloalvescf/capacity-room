package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/api"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/netpolicy"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

const shutdownTimeout = 30 * time.Second

// newServeCmd builds the `serve` subcommand: migrate the database and serve
// the HTTP API until the context ends.
func newServeCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	var config, listen string

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Serve the HTTP API of the platform.",
		Args:          noArgument("serve"),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runServe(ctx, env, redactor, cmd, config, listen)
		},
	}
	cmd.Flags().StringVar(&config, "config", platformConfigDefault, "Path of the TOML settings file.")
	cmd.Flags().StringVar(&listen, "listen", ":8080", "Address to listen on.")
	return cmd
}

// runServe opens the database, applies the migrations, and serves until ctx
// ends, then stops the server within shutdownTimeout.
func runServe(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, config, listen string) error {
	loaded, pool, err := openPlatform(ctx, env, redactor, config)
	if err != nil {
		return err
	}
	defer pool.Close()

	secret, err := loaded.ReadServerSecret()
	if err != nil {
		return &exitError{2, err}
	}
	if _, err := store.Migrate(ctx, pool); err != nil {
		return &exitError{1, err}
	}

	logger := newLogger(cmd.ErrOrStderr(), redactor)
	server, err := buildServer(pool, loaded, secret, env.Now, logger, redactor)
	if err != nil {
		return &exitError{1, err}
	}
	handler := server.Handler()

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return &exitError{1, fmt.Errorf("cannot listen on %s: %w", listen, osError(err))}
	}
	logger.Info("serve: listening on " + listener.Addr().String())
	if notice := outboundNotice(loaded); notice != "" {
		logger.Warn(notice)
	}

	httpServer := newHTTPServer(handler)
	served := make(chan error, 1)
	go func() { served <- httpServer.Serve(listener) }()

	select {
	case err := <-served:
		return &exitError{1, err}
	case <-ctx.Done():
	}
	logger.Info("serve: stopping")

	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := httpServer.Shutdown(stopCtx); err != nil {
		return &exitError{1, fmt.Errorf("cannot stop the server: %w", err)}
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return &exitError{1, err}
	}
	return nil
}

// buildServer assembles the API server of serve from the pool, the loaded
// settings, the server secret, the clock, the logger, and the redactor that
// learns every credential the profiles read. It fails when the report queue
// cannot be built.
func buildServer(pool *pgxpool.Pool, loaded settings.Settings, secret []byte, now func() time.Time, logger *slog.Logger, redactor *redact.Redactor) (*api.Server, error) {
	queue, err := reports.NewQueue(pool, logger)
	if err != nil {
		return nil, fmt.Errorf("cannot build the report queue: %w", err)
	}
	sessions := &auth.Sessions{
		Pool:     pool,
		Throttle: auth.Throttle{Secret: secret},
		TTL:      loaded.SessionTTL,
		Idle:     loaded.SessionIdle,
		Now:      now,
	}
	return &api.Server{
		Pool:     pool,
		Settings: loaded,
		Sessions: sessions,
		Users:    &auth.Users{Pool: pool, Now: now},
		Account:  &auth.Account{Pool: pool, Sessions: sessions, Now: now},
		Profiles: &profiles.Service{
			Pool: pool,
			Policy: netpolicy.Policy{
				Hostnames: loaded.AllowedEndpointHostnames,
				Networks:  loaded.AllowedEndpointCIDRs,
			},
			Settings: loaded,
			Now:      now,
			Reveal:   func(secret string) { redactor.Add(secret) },
		},
		Reports: &reports.Service{
			Pool:     pool,
			Queue:    queue,
			Settings: loaded,
			Now:      now,
			NewID:    uuid.New,
		},
		Now:    now,
		Logger: logger,
	}, nil
}

// outboundNotice is the line serve logs at start when both outbound
// allowlists are empty, and the empty string otherwise.
func outboundNotice(loaded settings.Settings) string {
	if len(loaded.AllowedEndpointHostnames) == 0 && len(loaded.AllowedEndpointCIDRs) == 0 {
		return "serve: the outbound allowlists are empty; no metric source is reachable"
	}
	return ""
}

// newHTTPServer is the server of serve. A client has a bounded time to send
// its headers and its body and to keep an idle connection open; no write
// timeout is set, so that a long response is not cut.
func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
}
