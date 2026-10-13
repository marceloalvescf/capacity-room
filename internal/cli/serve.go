package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
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
	"github.com/marceloalvescf/k8s-capacity-report/web"
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

	secret, err := readServerSecret(loaded, redactor)
	if err != nil {
		return &exitError{2, err}
	}
	if _, err := store.Migrate(ctx, pool); err != nil {
		return &exitError{1, err}
	}

	downloads, err := store.OpenDownloads(ctx, loaded.DatabaseDSN)
	if err != nil {
		return &exitError{1, fmt.Errorf("cannot connect to the database: %w", err)}
	}
	defer downloads.Close()

	logger := newLogger(cmd.ErrOrStderr(), redactor)
	server, err := buildServer(pool, downloads, loaded, secret, env.Now, logger, redactor)
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

	httpServer, conns := newServeServer(handler, logger)
	served := make(chan error, 1)
	go func() { served <- httpServer.Serve(listener) }()

	select {
	case err := <-served:
		return &exitError{1, err}
	case <-ctx.Done():
	}
	logger.Info("serve: stopping")

	if err := stopServer(ctx, httpServer, conns, shutdownTimeout, logger); err != nil {
		return &exitError{1, err}
	}
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return &exitError{1, err}
	}
	return nil
}

// connCount follows the last state of each connection of a server that is
// not yet closed or handed away, through http.Server.ConnState.
type connCount struct {
	mu    sync.Mutex
	state map[net.Conn]http.ConnState
}

// track is the ConnState hook.
func (c *connCount) track(conn net.Conn, state http.ConnState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch state {
	case http.StateClosed, http.StateHijacked:
		delete(c.state, conn)
	default:
		if c.state == nil {
			c.state = map[net.Conn]http.ConnState{}
		}
		c.state[conn] = state
	}
}

// active is the number of connections serving a request.
func (c *connCount) active() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, state := range c.state {
		if state == http.StateActive {
			n++
		}
	}
	return n
}

// stopServer shuts httpServer down within timeout; connections still serving
// a request then are closed and counted in one warning. It fails only when the
// listener cannot be closed.
func stopServer(ctx context.Context, httpServer *http.Server, conns *connCount, timeout time.Duration, logger *slog.Logger) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
	defer cancel()
	err := httpServer.Shutdown(stopCtx)
	if err == nil {
		return nil
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("cannot stop the server: %w", err)
	}
	open := conns.active()
	// Shutdown has closed the listeners already, so Close only ends the
	// connections still serving a request.
	_ = httpServer.Close()
	logger.Warn(fmt.Sprintf("serve: connections closed before they finished: %d", open))
	return nil
}

// buildServer assembles the API server of serve from the pool, the pool that
// reads report bodies, the loaded settings, the server secret, the clock, the
// logger, and the redactor that learns every credential the profiles
// read. It fails when the report queue cannot be built.
func buildServer(pool, downloads *pgxpool.Pool, loaded settings.Settings, secret []byte, now func() time.Time, logger *slog.Logger, redactor *redact.Redactor) (*api.Server, error) {
	queue, err := reports.NewQueue(pool, logger)
	if err != nil {
		return nil, fmt.Errorf("cannot build the report queue: %w", err)
	}
	sessions := auth.NewSessions(pool, auth.Throttle{Secret: secret}, loaded.SessionTTL, loaded.SessionIdle, now)
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
			Bodies:   downloads,
			Queue:    queue,
			Settings: loaded,
			Now:      now,
			NewID:    uuid.New,
		},
		Now:    now,
		Logger: logger,
		Static: web.Handler(),
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

// newServeServer is the server of serve with the count of its open
// connections, which the stop reads to name the ones it closed.
func newServeServer(handler http.Handler, logger *slog.Logger) (*http.Server, *connCount) {
	httpServer := newHTTPServer(handler, logger)
	conns := &connCount{}
	httpServer.ConnState = conns.track
	return httpServer, conns
}

// newHTTPServer is the server of serve and of the worker's health endpoint.
// A client has a bounded time to send its headers and its body and to keep
// an idle connection open; no server-wide write timeout is set, because the
// download handlers bound their own writes. The server's own error lines go
// through the run's logger.
func newHTTPServer(handler http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Handler:           handler,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
}

// readServerSecret reads the server secret and adds it to the redactor the
// moment it is known, before any other step uses it.
func readServerSecret(loaded settings.Settings, redactor *redact.Redactor) ([]byte, error) {
	secret, err := loaded.ReadServerSecret()
	if err != nil {
		return nil, err
	}
	redactor.Add(string(secret))
	return secret, nil
}
