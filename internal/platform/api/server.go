package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/health"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/profiles"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/reports"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
)

// Server builds the handler of serve.
type Server struct {
	Pool     *pgxpool.Pool
	Settings settings.Settings
	Sessions *auth.Sessions
	Users    *auth.Users
	Account  *auth.Account
	Profiles *profiles.Service
	Reports  *reports.Service
	Now      func() time.Time
	Logger   *slog.Logger // nil logs nowhere

	// downloadSlots and downloadDeadline are the number of downloads served
	// at once and the time a download may take to be written, once its body
	// is ready; zero means the defaults.
	downloadSlots    int
	downloadDeadline time.Duration
	slotsOnce        sync.Once
	slots            chan struct{}

	// extraRoutes are routes a test adds; Handler registers them after Routes.
	extraRoutes []Route
}

const (
	defaultDownloadSlots    = 4
	defaultDownloadDeadline = 2 * time.Minute
)

// downloads is the set of download slots: a slot is taken by sending into it.
func (s *Server) downloads() chan struct{} {
	s.slotsOnce.Do(func() {
		n := s.downloadSlots
		if n == 0 {
			n = defaultDownloadSlots
		}
		s.slots = make(chan struct{}, n)
	})
	return s.slots
}

func (s *Server) writeDeadline() time.Duration {
	if s.downloadDeadline == 0 {
		return defaultDownloadDeadline
	}
	return s.downloadDeadline
}

// acquireDownload waits for a download slot until the request's context
// ends. It reports false, taking no slot and writing nothing, when the
// context ended first; otherwise the caller gives the slot back with release.
func (s *Server) acquireDownload(r *http.Request) (release func(), ok bool) {
	slots := s.downloads()
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, true
	case <-r.Context().Done():
		return nil, false
	}
}

// logger is the logger of the server, or one that writes nowhere.
func (s *Server) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return s.Logger
}

// Handler registers the routes, the health routes, the 404 and 405
// answers in the envelope, and wraps everything in SecurityHeaders.
func (s *Server) Handler() http.Handler {
	logger := s.logger()
	mux := http.NewServeMux()
	for _, r := range append(s.Routes(), s.extraRoutes...) {
		mux.HandleFunc(r.Method+" "+r.Pattern, r.Handler)
	}
	checks := health.Handler([]health.Check{health.Database(s.Pool), health.Migrations(s.Pool)})
	mux.Handle("GET /health/live", checks)
	mux.Handle("GET /health/ready", checks)
	return securityHeaders(unmatched{mux}, logger)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	body, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(append(body, '\n'))
}

// unmatched sends a request that matches no pattern through unmatchedWriter,
// which turns the 404 and 405 of the mux into the answers of the contract.
type unmatched struct{ mux *http.ServeMux }

func (u unmatched) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, pattern := u.mux.Handler(r)
	if pattern != "" {
		u.mux.ServeHTTP(w, r)
		return
	}
	h.ServeHTTP(&unmatchedWriter{ResponseWriter: w, path: r.URL.Path}, r)
}

// unmatchedWriter answers the 405 of the mux with the METHOD_NOT_ALLOWED
// envelope, keeping the Allow header, and its 404 with the NOT_FOUND envelope
// under /api/ and plain text elsewhere. It drops the body the mux wrote.
type unmatchedWriter struct {
	http.ResponseWriter
	path     string
	replaced bool
}

func (m *unmatchedWriter) WriteHeader(status int) {
	m.replaced = true
	m.Header().Del("Content-Type")
	switch {
	case status == http.StatusMethodNotAllowed:
		WriteProblem(m.ResponseWriter, &Problem{Code: CodeMethodNotAllowed})
	case status == http.StatusNotFound && strings.HasPrefix(m.path, "/api/"):
		WriteProblem(m.ResponseWriter, &Problem{Code: CodeNotFound})
	case status == http.StatusNotFound:
		m.Header().Set("Content-Type", "text/plain; charset=utf-8")
		m.ResponseWriter.WriteHeader(status)
		_, _ = m.ResponseWriter.Write([]byte("Not Found"))
	default:
		m.replaced = false
		m.ResponseWriter.WriteHeader(status)
	}
}

func (m *unmatchedWriter) Write(b []byte) (int, error) {
	if m.replaced {
		return len(b), nil
	}
	return m.ResponseWriter.Write(b)
}

func (m *unmatchedWriter) Unwrap() http.ResponseWriter { return m.ResponseWriter }
