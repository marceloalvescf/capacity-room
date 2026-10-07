package api

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

// SessionHandler is a handler of a route that needs a signed-in reader.
type SessionHandler func(w http.ResponseWriter, r *http.Request, a auth.Authenticated)

// fail answers with the problem of err and hands err to the writer of
// SecurityHeaders, which logs a 5xx once. A request its client cancelled gets
// no answer and a debug line.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	if r.Context().Err() != nil && errors.Is(err, context.Canceled) {
		// The client left; nobody reads an envelope.
		s.logger().Debug("request canceled by the client", "method", r.Method, "path", r.URL.Path, "error", err)
		return
	}
	if n, ok := w.(errorNoter); ok {
		n.noteError(err)
	}
	WriteProblem(w, problemOf(err))
}

// unauthenticated answers 401 and clears the session cookie.
func unauthenticated(w http.ResponseWriter) {
	http.SetCookie(w, sessionCookie(SessionCookie, "", -1))
	WriteProblem(w, &Problem{Code: CodeUnauthenticated})
}

// requireSession authenticates the request and hands the Authenticated to
// next; 401 otherwise, clearing the cookie.
func (s *Server) requireSession(next SessionHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookie)
		if err != nil || cookie.Value == "" {
			unauthenticated(w)
			return
		}
		a, err := s.Sessions.Authenticate(r.Context(), cookie.Value)
		if errors.Is(err, auth.ErrUnauthenticated) {
			unauthenticated(w)
			return
		}
		if err != nil {
			s.fail(w, r, err)
			return
		}
		next(w, r, a)
	}
}

// requireRole wraps requireSession and answers 403 when the role is not
// one of roles.
func (s *Server) requireRole(roles []auth.Role, next SessionHandler) http.HandlerFunc {
	return s.requireSession(func(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
		if !slices.Contains(roles, a.Principal.Role) {
			WriteProblem(w, &Problem{Code: CodeForbidden})
			return
		}
		next(w, r, a)
	})
}

// requireCSRF wraps a SessionHandler: it verifies X-CSRF-Token against the
// session before calling next; 403 otherwise.
func (s *Server) requireCSRF(next SessionHandler) SessionHandler {
	return func(w http.ResponseWriter, r *http.Request, a auth.Authenticated) {
		if err := s.Sessions.VerifyCSRF(r.Context(), a.SessionID, r.Header.Get(CSRFHeaderName)); err != nil {
			s.fail(w, r, err)
			return
		}
		next(w, r, a)
	}
}
