package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

var wantHeaders = map[string]string{
	"Content-Security-Policy": "default-src 'self'; script-src 'self'; style-src 'self'; font-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'",
	"X-Content-Type-Options":  "nosniff",
	"Referrer-Policy":         "same-origin",
	"X-Frame-Options":         "DENY",
	"Cache-Control":           "no-store",
}

func checkHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	for k, v := range wantHeaders {
		if got := w.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
}

func TestSecurityHeadersOnEveryResponse(t *testing.T) {
	cases := map[string]struct {
		h      http.HandlerFunc
		status int
	}{
		"ok":    {func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("x")) }, 200},
		"404":   {func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(404) }, 404},
		"panic": {func(http.ResponseWriter, *http.Request) { panic("boom") }, 500},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			SecurityHeaders(c.h).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			if w.Code != c.status {
				t.Fatalf("status %d", w.Code)
			}
			checkHeaders(t, w)
			if name == "panic" {
				want := `{"error":{"code":"INTERNAL_ERROR","message":"An unexpected error occurred.","fields":{}}}` + "\n"
				if w.Body.String() != want {
					t.Fatalf("body %q", w.Body.String())
				}
			}
		})
	}
}

func TestSecurityHeadersKeepsHandlerCacheControl(t *testing.T) {
	const value = "private, max-age=0, must-revalidate"
	h := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", value)
		w.WriteHeader(200)
	}
	w := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(h)).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if got := w.Header().Get("Cache-Control"); got != value {
		t.Fatalf("Cache-Control %q", got)
	}
}

func TestNoServerHeader(t *testing.T) {
	w := httptest.NewRecorder()
	SecurityHeaders(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if _, ok := w.Header()["Server"]; ok {
		t.Fatal("Server header present")
	}
}
