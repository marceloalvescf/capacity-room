package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

func newServer() *promtest.Server {
	return promtest.NewServer(promtest.Fixture{Requests: []promtest.Request{{
		Method:        "POST",
		Path:          "/api/v1/query",
		QueryContains: "up",
		Body:          json.RawMessage(`{"status":"success"}`),
	}}})
}

func queryRequest() *http.Request {
	request := httptest.NewRequest("POST", "/api/v1/query", strings.NewReader("query=up"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return request
}

func TestReadyRouteSpendsNothing(t *testing.T) {
	server := newServer()
	recorder := httptest.NewRecorder()
	handler(server, 0).ServeHTTP(recorder, httptest.NewRequest("GET", "/-/ready", nil))
	if recorder.Code != 204 {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if server.Remaining() != 1 {
		t.Fatalf("remaining = %d, want 1", server.Remaining())
	}
	if len(server.Seen()) != 0 {
		t.Fatalf("seen = %v, want none", server.Seen())
	}
}

func TestDelayHoldsEachAnswer(t *testing.T) {
	server := newServer()
	recorder := httptest.NewRecorder()
	start := time.Now()
	handler(server, 50*time.Millisecond).ServeHTTP(recorder, queryRequest())
	if elapsed := time.Since(start); elapsed < 50*time.Millisecond {
		t.Fatalf("elapsed = %v, want at least 50ms", elapsed)
	}
	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != `{"status":"success"}` {
		t.Fatalf("body = %q", body)
	}
	if server.Remaining() != 0 {
		t.Fatalf("remaining = %d, want 0", server.Remaining())
	}
}

func TestReadyIsNotDelayed(t *testing.T) {
	recorder := httptest.NewRecorder()
	start := time.Now()
	handler(newServer(), 2*time.Second).ServeHTTP(recorder, httptest.NewRequest("GET", "/-/ready", nil))
	if recorder.Code != 204 {
		t.Fatalf("status = %d, want 204", recorder.Code)
	}
	if elapsed := time.Since(start); elapsed >= time.Second {
		t.Fatalf("elapsed = %v, want under 1s", elapsed)
	}
}

func TestProbe(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(204)
	}))
	defer ok.Close()
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer failing.Close()

	tests := []struct {
		name string
		url  string
		code int
	}{
		{"2xx", ok.URL + "/-/ready", 0},
		{"500", failing.URL + "/-/ready", 1},
		{"closed", "http://127.0.0.1:1/-/ready", 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			if code := probe(test.url, &stderr); code != test.code {
				t.Fatalf("code = %d, want %d", code, test.code)
			}
			if test.code == 0 {
				return
			}
			line := stderr.String()
			if !strings.HasPrefix(line, "fake-prometheus: ") || strings.Count(line, "\n") != 1 {
				t.Fatalf("stderr = %q, want one line starting fake-prometheus: ", line)
			}
		})
	}
}

func TestRunRefusesFixtureAndProbeTogether(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"--probe", "http://x/-/ready", "--fixture", "f.json"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("code = %d, want 2", code)
	}
	if !strings.HasPrefix(stderr.String(), "fake-prometheus: ") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}
