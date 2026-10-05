package api

import (
	"net/http"
)

// Route is one line of the table the server registers and the contract
// test reads.
type Route struct {
	Method  string
	Pattern string // the full path, as the ServeMux pattern without the method
	Handler http.HandlerFunc
}

// Routes returns the table of every route under /api/v1, in a fixed order,
// without the health routes.
func (s *Server) Routes() []Route {
	return []Route{
		{"GET", "/api/v1/auth/session", s.getSession},
		{"POST", "/api/v1/auth/login", s.login},
		{"POST", "/api/v1/auth/logout", s.requireSession(s.requireCSRF(s.logout))},
		{"PATCH", "/api/v1/account", s.requireSession(s.requireCSRF(s.patchAccount))},
		{"GET", "/api/v1/admin/users", s.adminOnly(s.listUsers)},
		{"POST", "/api/v1/admin/users", s.adminOnly(s.requireCSRF(s.createUser))},
		{"GET", "/api/v1/admin/users/{id}", s.adminOnly(s.getUser)},
		{"PATCH", "/api/v1/admin/users/{id}", s.adminOnly(s.requireCSRF(s.patchUser))},
		{"DELETE", "/api/v1/admin/users/{id}", s.adminOnly(s.requireCSRF(s.deleteUser))},
		{"GET", "/api/v1/profiles", s.requireSession(s.listProfileSummaries)},
		{"GET", "/api/v1/admin/profiles", s.adminOnly(s.listProfiles)},
		{"POST", "/api/v1/admin/profiles", s.adminOnly(s.requireCSRF(s.createProfile))},
		{"GET", "/api/v1/admin/profiles/{id}", s.adminOnly(s.getProfile)},
		{"PATCH", "/api/v1/admin/profiles/{id}", s.adminOnly(s.requireCSRF(s.patchProfile))},
		{"DELETE", "/api/v1/admin/profiles/{id}", s.adminOnly(s.requireCSRF(s.deleteProfile))},
		{"POST", "/api/v1/admin/profiles/{id}/test", s.adminOnly(s.requireCSRF(s.testProfile))},
	}
}
