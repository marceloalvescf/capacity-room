package api

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
)

// Server builds the handler of serve.
type Server struct {
	Pool     *pgxpool.Pool
	Settings settings.Settings
	Sessions *auth.Sessions
	Users    *auth.Users
	Account  *auth.Account
	Now      func() time.Time
	Logger   *slog.Logger // nil logs nowhere

	// extraRoutes are routes a test adds; Handler registers them after Routes.
	extraRoutes []Route
}

// Handler registers the routes, the health routes, the 404 and 405
// answers in the envelope, and wraps everything in SecurityHeaders.
func (s *Server) Handler() http.Handler {
	logger := s.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	mux := http.NewServeMux()
	for _, r := range append(s.Routes(), s.extraRoutes...) {
		mux.HandleFunc(r.Method+" "+r.Pattern, r.Handler)
	}
	mux.HandleFunc("GET /health/live", s.live)
	mux.HandleFunc("GET /health/ready", s.ready)
	return securityHeaders(unmatched{mux}, logger)
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
	m.Header().Del("X-Content-Type-Options")
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
