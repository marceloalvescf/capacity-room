// Package promtest serves the recorded answers of a Prometheus-compatible
// API for tests, and for cmd/fake-prometheus. It imports nothing of this
// module, so it never reaches the production binary.
package promtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

// Request is one scripted answer of a scenario file.
type Request struct {
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	QueryContains string            `json:"query_contains"`
	Authorization string            `json:"authorization"`
	Status        int               `json:"status"`
	Headers       map[string]string `json:"headers"`
	Body          json.RawMessage   `json:"body"`
}

// Fixture is a scenario file.
type Fixture struct {
	Requests []Request `json:"requests"`
}

// Load reads and checks a scenario file. An unknown key is an error, and so
// is a file whose top level holds no "requests" key.
func Load(path string) (Fixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Fixture{}, fmt.Errorf("reading %s: %w", path, err)
	}

	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Fixture{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if _, ok := probe["requests"]; !ok {
		return Fixture{}, fmt.Errorf("%s: fixture has no \"requests\" key", path)
	}

	var fixture Fixture
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return Fixture{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return fixture, nil
}

// Seen is one request the server received.
type Seen struct {
	Method, Path, Query, Authorization string
	Form                               url.Values
	Matched                            bool
}

// Server answers from a copy of the fixture. Each scripted request answers
// once. It is safe for use by several goroutines.
type Server struct {
	mu      sync.Mutex
	pending []Request
	seen    []Seen
}

// NewServer copies the requests of f, so that the caller's Fixture is never
// mutated by serving it.
func NewServer(f Fixture) *Server {
	pending := make([]Request, len(f.Requests))
	copy(pending, f.Requests)
	return &Server{pending: pending}
}

// fixtureError is the body of a request the server refuses, shaped like a
// Prometheus API error.
type fixtureError struct {
	Status    string `json:"status"`
	ErrorType string `json:"errorType"`
	Error     string `json:"error"`
}

// ServeHTTP answers r from the first remaining scripted request that
// matches it, removing that request. A scripted request matches when its
// method is equal (a missing method means GET), its path is equal, its
// query_contains is empty or is part of the query field of the form, and
// its authorization is empty or equals the Authorization header. The query
// is read from the form of the request body, never from the address: a
// query string on the address itself is not read.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	query := r.PostFormValue("query")
	authorization := r.Header.Get("Authorization")
	method := r.Method
	path := r.URL.Path

	s.mu.Lock()
	matched, ok, message := s.consume(method, path, query, authorization)
	s.seen = append(s.seen, Seen{
		Method:        method,
		Path:          path,
		Query:         query,
		Authorization: authorization,
		Form:          r.Form,
		Matched:       ok,
	})
	s.mu.Unlock()

	if !ok {
		writeFixtureError(w, http.StatusBadRequest, message)
		return
	}

	for key, value := range matched.Headers {
		w.Header().Set(key, value)
	}
	w.Header().Set("Content-Type", "application/json")
	status := matched.Status
	if status == 0 {
		status = http.StatusOK
	}
	w.WriteHeader(status)
	_, _ = w.Write(matched.Body)
}

// consume looks for the first pending request that matches the given
// request, in order, and removes it when found. The caller must hold s.mu.
// When nothing matches, it also builds the message the caller answers with:
// "unexpected request" when nothing is left, "expected query for <path>"
// when a remaining request shares the path, and "unexpected path <path>"
// otherwise.
func (s *Server) consume(method, path, query, authorization string) (Request, bool, string) {
	for i, candidate := range s.pending {
		if !requestMatches(candidate, method, path, query, authorization) {
			continue
		}
		s.pending = append(s.pending[:i:i], s.pending[i+1:]...)
		return candidate, true, ""
	}

	if len(s.pending) == 0 {
		return Request{}, false, "unexpected request"
	}
	for _, candidate := range s.pending {
		if candidate.Path == path {
			return Request{}, false, fmt.Sprintf("expected query for %q", path)
		}
	}
	return Request{}, false, fmt.Sprintf("unexpected path %q", path)
}

// requestMatches reports whether a scripted request matches an incoming
// one, by the rule documented on ServeHTTP.
func requestMatches(candidate Request, method, path, query, authorization string) bool {
	expectedMethod := candidate.Method
	if expectedMethod == "" {
		expectedMethod = http.MethodGet
	}
	if expectedMethod != method {
		return false
	}
	if candidate.Path != path {
		return false
	}
	if candidate.QueryContains != "" && !strings.Contains(query, candidate.QueryContains) {
		return false
	}
	if candidate.Authorization != "" && candidate.Authorization != authorization {
		return false
	}
	return true
}

// writeFixtureError answers with the Prometheus-shaped error body the
// original fake server used.
func writeFixtureError(w http.ResponseWriter, status int, message string) {
	body, err := json.Marshal(fixtureError{Status: "error", ErrorType: "fixture", Error: message})
	if err != nil {
		// fixtureError holds only strings; json.Marshal cannot fail on it.
		panic(err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

// Seen returns every request the server received, in the order it received
// them.
func (s *Server) Seen() []Seen {
	s.mu.Lock()
	defer s.mu.Unlock()
	seen := make([]Seen, len(s.seen))
	copy(seen, s.seen)
	return seen
}

// Remaining returns the number of scripted requests not yet answered.
func (s *Server) Remaining() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// Merge returns one fixture holding the requests of each fixture in turn.
func Merge(fixtures ...Fixture) Fixture {
	var merged Fixture
	for _, f := range fixtures {
		merged.Requests = append(merged.Requests, f.Requests...)
	}
	return merged
}
