package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func serve(h http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func answer(ok bool) func(context.Context) bool {
	return func(context.Context) bool { return ok }
}

func TestHealthLive(t *testing.T) {
	w := serve(Handler(nil), "GET", "/health/live")
	if w.Code != 200 || w.Body.String() != `{"status":"live"}`+"\n" {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type %q", w.Header().Get("Content-Type"))
	}
}

func TestHealthReady(t *testing.T) {
	h := Handler([]Check{{"database", answer(true)}, {"worker", answer(true)}})
	w := serve(h, "GET", "/health/ready")
	want := `{"status":"ready","checks":{"database":true,"worker":true}}` + "\n"
	if w.Code != 200 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}

	h = Handler([]Check{{"database", answer(true)}, {"worker", answer(false)}})
	w = serve(h, "GET", "/health/ready")
	want = `{"status":"not_ready","checks":{"database":true,"worker":false}}` + "\n"
	if w.Code != 503 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestHealthNotFound(t *testing.T) {
	for _, tc := range []struct{ method, path string }{
		{"GET", "/health"}, {"GET", "/health/other"}, {"GET", "/"}, {"POST", "/health/live"},
	} {
		w := serve(Handler(nil), tc.method, tc.path)
		if w.Code != 404 || w.Body.String() != `{"status":"not_found"}`+"\n" {
			t.Errorf("%s %s: %d %q", tc.method, tc.path, w.Code, w.Body.String())
		}
	}
}

func TestHealthReadyBound(t *testing.T) {
	block := func(ctx context.Context) bool {
		<-ctx.Done()
		return true
	}
	h := Handler([]Check{{"database", answer(true)}, {"slow", block}})
	start := time.Now()
	w := serve(h, "GET", "/health/ready")
	if took := time.Since(start); took > 2500*time.Millisecond {
		t.Fatalf("took %v", took)
	}
	want := `{"status":"not_ready","checks":{"database":true,"slow":false}}` + "\n"
	if w.Code != 503 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestHealthReadyBoundIgnoresContext(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	stuck := func(context.Context) bool {
		<-release
		return true
	}
	start := time.Now()
	w := serve(Handler([]Check{{"stuck", stuck}}), "GET", "/health/ready")
	if took := time.Since(start); took > 2500*time.Millisecond {
		t.Fatalf("took %v", took)
	}
	if w.Code != 503 {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestHealthHead(t *testing.T) {
	h := Handler([]Check{{"database", answer(true)}})
	for path, want := range map[string]int{"/health/live": 200, "/health/ready": 200} {
		w := serve(h, "HEAD", path)
		if w.Code != want || w.Body.Len() != 0 {
			t.Errorf("HEAD %s: %d %q", path, w.Code, w.Body.String())
		}
	}
	w := serve(Handler([]Check{{"database", answer(false)}}), "HEAD", "/health/ready")
	if w.Code != 503 || w.Body.Len() != 0 {
		t.Errorf("HEAD ready down: %d %q", w.Code, w.Body.String())
	}
}
