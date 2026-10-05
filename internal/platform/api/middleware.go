package api

import (
	"errors"
	"net/http"
	"slices"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

// SessionHandler is a handler of a route that needs a signed-in reader.
type SessionHandler func(w http.ResponseWriter, r *http.Request, a auth.Authenticated)

// fail answers with the problem of err and logs an unexpected one once, with
// the route and the text of the error.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := problemOf(err)
	if p.Code == CodeInternalError && s.Logger != nil {
		s.Logger.Error("request failed", "route", r.Method+" "+r.URL.Path, "error", err.Error())
	}
	WriteProblem(w, p)
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
		if err := s.Sessions.VerifyCSRF(r.Context(), a.SessionID, r.Header.Get(CSRFHeader)); err != nil {
			s.fail(w, r, err)
			return
		}
		next(w, r, a)
	}
}
