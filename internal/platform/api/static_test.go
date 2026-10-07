package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestUnmatchedRoutes(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("stub"))
	})
	cases := []struct {
		method, path string
		status       int
		body         string
	}{
		{"GET", "/api/v1/nothing", 404, `{"error":{"code":"NOT_FOUND","message":"The requested resource was not found.","fields":{}}}` + "\n"},
		{"GET", "/api", 404, "Not Found"},
		{"GET", "/health/nothing", 404, "Not Found"},
		{"POST", "/account", 404, "Not Found"},
		{"GET", "/account", 200, "stub"},
		{"HEAD", "/account", 200, "stub"},
	}
	for _, c := range cases {
		t.Run(c.method+" "+c.path, func(t *testing.T) {
			w := get(t, &Server{Static: stub}, c.method, c.path)
			if w.Code != c.status || w.Body.String() != c.body {
				t.Fatalf("%d %q, want %d %q", w.Code, w.Body.String(), c.status, c.body)
			}
		})
	}
}

func TestStaticCarriesSecurityHeaders(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write([]byte("stub"))
	})
	w := httptest.NewRecorder()
	(&Server{Static: stub}).Handler().ServeHTTP(w, httptest.NewRequest("GET", "/account", nil))
	for k, v := range wantHeaders {
		if k == "Cache-Control" {
			v = "no-cache"
		}
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestUnmatchedUncleanPathsRedirect(t *testing.T) {
	stub := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("stub"))
	})
	cases := map[string]string{
		"//api/v1/nothing":     "/api/v1/nothing",
		"/x/../api/v1/nothing": "/api/v1/nothing",
		"/x/../health/nothing": "/health/nothing",
		"/a/../account":        "/account",
	}
	for path, want := range cases {
		t.Run(path, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.URL.Path = path
			w := httptest.NewRecorder()
			(&Server{Static: stub}).Handler().ServeHTTP(w, r)
			if w.Code != http.StatusTemporaryRedirect || w.Header().Get("Location") != want || w.Body.String() == "stub" {
				t.Fatalf("%d %q %q, want 307 to %q", w.Code, w.Header().Get("Location"), w.Body.String(), want)
			}
		})
	}
}
