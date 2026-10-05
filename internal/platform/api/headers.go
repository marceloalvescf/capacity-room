package api

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

// SecurityHeaders wraps next: every response carries the five security
// headers (Cache-Control only when the handler set none) and a recovered
// panic answers 500 INTERNAL_ERROR with the same headers, or, when the
// response was already written, is logged.
func SecurityHeaders(next http.Handler) http.Handler {
	return securityHeaders(next, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

// securityHeaders is SecurityHeaders with a logger, which records every 5xx
// once.
func securityHeaders(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Frame-Options", "DENY")
		sw := &statusWriter{ResponseWriter: w}
		defer func() {
			route := r.Pattern
			if route == "" {
				route = r.URL.Path
			}
			if rec := recover(); rec != nil {
				if rec == http.ErrAbortHandler {
					panic(rec)
				}
				if sw.wrote {
					// No envelope can be written any more; the log keeps
					// the failure.
					logger.Error("panic after the response was written",
						"method", r.Method, "route", route, "panic", fmt.Sprint(rec))
					return
				}
				WriteProblem(sw, &Problem{Code: CodeInternalError})
			}
			if sw.status >= 500 {
				attrs := []any{"method", r.Method, "route", route, "status", sw.status, "code", string(sw.code)}
				if sw.err != nil {
					attrs = append(attrs, "err", sw.err.Error())
				}
				logger.Error("request failed", attrs...)
			}
		}()
		next.ServeHTTP(sw, r)
	})
}

// statusWriter fills in the default Cache-Control, remembers the status and
// the code of the answer.
type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
	code   Code
	err    error
}

func (s *statusWriter) noteCode(c Code) { s.code = c }

func (s *statusWriter) noteError(err error) { s.err = err }

func (s *statusWriter) WriteHeader(status int) {
	if s.wrote {
		return
	}
	s.wrote = true
	s.status = status
	if s.Header().Get("Cache-Control") == "" {
		s.Header().Set("Cache-Control", "no-store")
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusWriter) Write(b []byte) (int, error) {
	if !s.wrote {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }
