package api

import "net/http"

// The cookies and the header of the sign-in and the session.
const (
	SessionCookie   = "capacityroom_session"
	LoginCSRFCookie = "capacityroom_login_csrf"
	CSRFHeaderName  = "X-CSRF-Token"
	CookiePath      = "/api"
)

// sessionCookie builds a cookie with the given Max-Age in seconds; 0 means
// none (a browser-session cookie) and -1 clears it.
func sessionCookie(name, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     CookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}
