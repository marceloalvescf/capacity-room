package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoutesAreUnique(t *testing.T) {
	s := &Server{}
	seen := map[string]bool{}
	for _, r := range append(s.Routes(), s.extraRoutes...) {
		key := r.Method + " " + r.Pattern
		if seen[key] {
			t.Errorf("duplicate route %s", key)
		}
		seen[key] = true
		if !strings.HasPrefix(r.Pattern, "/api/v1/") {
			t.Errorf("route %s outside /api/v1/", key)
		}
	}
}

func get(t *testing.T, s *Server, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestUnknownAPIPathIsEnvelope(t *testing.T) {
	w := get(t, &Server{}, "GET", "/api/v1/nothing")
	checkHeaders(t, w)
	want := `{"error":{"code":"NOT_FOUND","message":"The requested resource was not found.","fields":{}}}` + "\n"
	if w.Code != 404 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestUnknownOtherPathIsPlain(t *testing.T) {
	w := get(t, &Server{}, "GET", "/nothing")
	checkHeaders(t, w)
	if w.Code != 404 || w.Header().Get("Content-Type") != "text/plain; charset=utf-8" || w.Body.String() != "Not Found" {
		t.Fatalf("%d %q %q", w.Code, w.Header().Get("Content-Type"), w.Body.String())
	}
}

func TestWrongMethodIsEnvelope(t *testing.T) {
	s := &Server{extraRoutes: []Route{{Method: "GET", Pattern: "/api/v1/auth/session", Handler: func(http.ResponseWriter, *http.Request) {}}}}
	w := get(t, s, "DELETE", "/api/v1/auth/session")
	checkHeaders(t, w)
	want := `{"error":{"code":"METHOD_NOT_ALLOWED","message":"This method is not allowed for this resource.","fields":{}}}` + "\n"
	if w.Code != 405 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Header().Get("Allow"), "GET") {
		t.Fatalf("Allow %q", w.Header().Get("Allow"))
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type %q", w.Header().Get("Content-Type"))
	}
}

func TestHealthLive(t *testing.T) {
	w := get(t, &Server{}, "GET", "/health/live")
	if w.Code != 200 || w.Body.String() != `{"status":"live"}`+"\n" {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestHealthRoutesNeedNoSession(t *testing.T) {
	for _, path := range []string{"/health/live", "/health/ready"} {
		if w := get(t, &Server{}, "GET", path); w.Code == 401 {
			t.Errorf("%s answered 401", path)
		}
	}
}
