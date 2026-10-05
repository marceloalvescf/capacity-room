package api

import "time"

// LoginRequest is the body of POST /auth/login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// AccountChange is the body of PATCH /account.
type AccountChange struct {
	CurrentPassword *string `json:"current_password"`
	Password        *string `json:"password"`
	Language        *string `json:"language"`
}

// Principal is the account of a signed-in reader.
type Principal struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Language string `json:"language"`
}

// AnonymousSession is the answer of GET /auth/session without a session.
type AnonymousSession struct {
	Authenticated bool   `json:"authenticated"`
	LoginCSRF     string `json:"login_csrf"`
}

// SignedInSession is the answer of GET /auth/session with a session and of
// a successful sign-in.
type SignedInSession struct {
	Authenticated bool      `json:"authenticated"`
	Principal     Principal `json:"principal"`
	CSRFToken     string    `json:"csrf_token"`
	ExpiresAt     time.Time `json:"expires_at"`
}
