package api

import (
	"crypto/subtle"
	"errors"
	"net/http"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
)

const (
	maxUsername = 256
	maxPassword = 1024
)

func principalOf(p auth.Principal) Principal {
	return Principal{Id: p.ID, Username: p.Username, Role: Role(p.Role), Language: Language(p.Language)}
}

func signedIn(p auth.Principal, csrf string, expiresAt time.Time) SignedInSession {
	return SignedInSession{
		Authenticated: true,
		Principal:     principalOf(p),
		CsrfToken:     csrf,
		ExpiresAt:     expiresAt.UTC(),
	}
}

// anonymous answers the anonymous shape with a fresh sign-in CSRF token,
// and clears the session cookie when the request carried one.
func (s *Server) anonymous(w http.ResponseWriter, r *http.Request, clearSession bool) {
	token, err := auth.NewToken()
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if clearSession {
		http.SetCookie(w, sessionCookie(SessionCookie, "", -1))
	}
	http.SetCookie(w, sessionCookie(LoginCSRFCookie, token, 0))
	writeJSON(w, http.StatusOK, AnonymousSession{Authenticated: false, LoginCsrf: token})
}

// getSession answers who the caller is. It is the one route that handles
// both states itself: a missing, expired, or revoked session, and an
// inactive account, answer as an anonymous caller.
func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	// A request from another site, such as a top-level navigation that
	// carries the Lax cookie, must not rotate the token of an open page.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		WriteProblem(w, &Problem{Code: CodeCSRFRejected})
		return
	}
	cookie, err := r.Cookie(SessionCookie)
	if err != nil || cookie.Value == "" {
		s.anonymous(w, r, false)
		return
	}
	a, err := s.Sessions.Authenticate(r.Context(), cookie.Value)
	var csrf string
	if err == nil {
		csrf, err = s.Sessions.RotateCSRF(r.Context(), a.SessionID)
	}
	if errors.Is(err, auth.ErrUnauthenticated) {
		s.anonymous(w, r, true)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, signedIn(a.Principal, csrf, a.ExpiresAt))
}

// login checks the sign-in CSRF pair before it reads the body, so that the
// throttle never answers a caller who is not the page.
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(LoginCSRFCookie)
	header := r.Header.Get(CSRFHeaderName)
	if err != nil || cookie.Value == "" || header == "" ||
		subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(header)) != 1 {
		WriteProblem(w, &Problem{Code: CodeCSRFRejected})
		return
	}
	var body LoginRequest
	if p := DecodeBody(r, &body, []string{"username", "password"}, map[string]int{"username": maxUsername, "password": maxPassword}); p != nil {
		WriteProblem(w, p)
		return
	}
	presented := ""
	if c, err := r.Cookie(SessionCookie); err == nil {
		presented = c.Value
	}
	issued, err := s.Sessions.Login(r.Context(), body.Username, body.Password, ClientAddress(r, s.Settings.TrustedProxies), presented)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, sessionCookie(SessionCookie, issued.Token, int(s.Sessions.TTL.Seconds())))
	http.SetCookie(w, sessionCookie(LoginCSRFCookie, "", -1))
	writeJSON(w, http.StatusOK, signedIn(issued.Principal, issued.CSRFToken, issued.ExpiresAt))
}

// logout revokes the session of the request and clears its cookie.
func (s *Server) logout(w http.ResponseWriter, r *http.Request, _ auth.Authenticated) {
	cookie, err := r.Cookie(SessionCookie)
	if err != nil {
		unauthenticated(w)
		return
	}
	if err := s.Sessions.Logout(r.Context(), cookie.Value); err != nil {
		s.fail(w, r, err)
		return
	}
	http.SetCookie(w, sessionCookie(SessionCookie, "", -1))
	w.WriteHeader(http.StatusNoContent)
}
