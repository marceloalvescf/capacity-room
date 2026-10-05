package api

import "net/http"

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
	return []Route{}
}
