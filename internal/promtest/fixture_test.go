package promtest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
)

func TestLoadReadsEachScenario(t *testing.T) {
	cases := []struct {
		name     string
		requests int
	}{
		{"direct-complete", 70},
		{"grafana-authenticated", 70},
		{"thanos-multicluster", 70},
		{"raw-fallback", 73},
		{"partial-data", 63},
		{"value-edge-cases", 70},
		{"edge-values", 70},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path, err := scenarioPath(c.name)
			if err != nil {
				t.Fatalf("scenarioPath(%q): %v", c.name, err)
			}
			fixture, err := Load(path)
			if err != nil {
				t.Fatalf("Load(%q): %v", path, err)
			}
			if got := len(fixture.Requests); got != c.requests {
				t.Fatalf("len(fixture.Requests) = %d, want %d", got, c.requests)
			}
		})
	}
}

func writeFixtureFile(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.json")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("writing fixture file: %v", err)
	}
	return path
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := writeFixtureFile(t, `{"requests": [{"path": "/api/v1/query", "delay": 1}]}`)

	_, err := Load(path)
	if err == nil {
		t.Fatalf("Load(%q) = nil error, want one naming \"delay\"", path)
	}
	if !strings.Contains(err.Error(), "delay") {
		t.Fatalf("Load(%q) error = %q, want it to name \"delay\"", path, err)
	}
}

func TestLoadRejectsFileWithoutRequests(t *testing.T) {
	path := writeFixtureFile(t, `{}`)

	if _, err := Load(path); err == nil {
		t.Fatalf("Load(%q) = nil error, want one", path)
	}
}

// postQuery builds a POST request whose form body carries the given
// PromQL query, exactly as a Prometheus client sends one.
func postQuery(path, query string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader("query="+query))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestServerMatchesOnFormQuery(t *testing.T) {
	server := NewServer(Fixture{Requests: []Request{
		{Method: "POST", Path: "/api/v1/query", QueryContains: "vector(1)", Status: 200, Body: json.RawMessage(`{"status":"success"}`)},
	}})

	w := httptest.NewRecorder()
	server.ServeHTTP(w, postQuery("/api/v1/query", "vector(1)"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := w.Body.String(); got != `{"status":"success"}` {
		t.Fatalf("body = %q, want the scripted body", got)
	}
}

func TestServerIgnoresQueryInAddress(t *testing.T) {
	server := NewServer(Fixture{Requests: []Request{
		{Method: "POST", Path: "/api/v1/query", QueryContains: "vector(1)", Status: 200, Body: json.RawMessage(`{}`)},
	}})

	r := httptest.NewRequest(http.MethodPost, "/api/v1/query?query=vector(1)", strings.NewReader(""))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	server.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: a query in the address must not be read", w.Code, http.StatusBadRequest)
	}
}

func TestServerAnswersEachRequestOnce(t *testing.T) {
	server := NewServer(Fixture{Requests: []Request{
		{Method: "POST", Path: "/api/v1/query", QueryContains: "vector(1)", Status: 200, Body: json.RawMessage(`{}`)},
	}})

	w1 := httptest.NewRecorder()
	server.ServeHTTP(w1, postQuery("/api/v1/query", "vector(1)"))
	if w1.Code != http.StatusOK {
		t.Fatalf("first answer status = %d, want %d", w1.Code, http.StatusOK)
	}

	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, postQuery("/api/v1/query", "vector(1)"))
	if w2.Code != http.StatusBadRequest {
		t.Fatalf("second answer status = %d, want %d", w2.Code, http.StatusBadRequest)
	}
}

func TestServerMatchesOutOfOrder(t *testing.T) {
	server := NewServer(Fixture{Requests: []Request{
		{Method: "POST", Path: "/api/v1/query", QueryContains: "vector(1)", Status: 200, Body: json.RawMessage(`{}`)},
		{Method: "POST", Path: "/api/v1/query", QueryContains: "kube_node_info", Status: 200, Body: json.RawMessage(`{}`)},
	}})

	w := httptest.NewRecorder()
	server.ServeHTTP(w, postQuery("/api/v1/query", "kube_node_info"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if got := server.Remaining(); got != 1 {
		t.Fatalf("Remaining() = %d, want 1", got)
	}
}

func TestServerRequiresAuthorization(t *testing.T) {
	path, err := scenarioPath("grafana-authenticated")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	fixture, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	server := NewServer(fixture)

	w := httptest.NewRecorder()
	server.ServeHTTP(w, postQuery("/api/datasources/uid/prom/resources/api/v1/query", "vector(1)"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: the request carried no Authorization header", w.Code, http.StatusBadRequest)
	}
}

func TestServerErrorMessages(t *testing.T) {
	newServer := func() *Server {
		return NewServer(Fixture{Requests: []Request{
			{Method: "POST", Path: "/api/v1/query", QueryContains: "vector(1)", Status: 200, Body: json.RawMessage(`{}`)},
		}})
	}

	t.Run("nothing left", func(t *testing.T) {
		server := NewServer(Fixture{})
		w := httptest.NewRecorder()
		server.ServeHTTP(w, postQuery("/api/v1/query", "vector(1)"))
		assertFixtureError(t, w, "unexpected request")
	})

	t.Run("known path with another query", func(t *testing.T) {
		server := newServer()
		w := httptest.NewRecorder()
		server.ServeHTTP(w, postQuery("/api/v1/query", "kube_node_info"))
		assertFixtureError(t, w, `expected query for "/api/v1/query"`)
	})

	t.Run("unknown path", func(t *testing.T) {
		server := newServer()
		w := httptest.NewRecorder()
		server.ServeHTTP(w, postQuery("/api/v1/other", "vector(1)"))
		assertFixtureError(t, w, `unexpected path "/api/v1/other"`)
	})
}

func assertFixtureError(t *testing.T, w *httptest.ResponseRecorder, wantMessage string) {
	t.Helper()
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var body fixtureError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decoding error body: %v", err)
	}
	want := fixtureError{Status: "error", ErrorType: "fixture", Error: wantMessage}
	if body != want {
		t.Fatalf("error body = %+v, want %+v", body, want)
	}
}

func TestServerRecordsWhatItSaw(t *testing.T) {
	server := NewServer(Fixture{Requests: []Request{
		{Method: "POST", Path: "/api/v1/query", QueryContains: "vector(1)", Status: 200, Body: json.RawMessage(`{}`)},
	}})

	server.ServeHTTP(httptest.NewRecorder(), postQuery("/api/v1/query", "vector(1)"))
	server.ServeHTTP(httptest.NewRecorder(), postQuery("/api/v1/query", "vector(1)"))

	seen := server.Seen()
	if len(seen) != 2 {
		t.Fatalf("len(Seen()) = %d, want 2", len(seen))
	}
	if !seen[0].Matched {
		t.Fatalf("seen[0].Matched = false, want true")
	}
	if seen[1].Matched {
		t.Fatalf("seen[1].Matched = true, want false")
	}
	for i, s := range seen {
		if s.Path != "/api/v1/query" {
			t.Fatalf("seen[%d].Path = %q, want /api/v1/query", i, s.Path)
		}
	}
}

func TestServerIsSafeForConcurrentUse(t *testing.T) {
	const total = 50
	requests := make([]Request, total)
	// Each query is fixed-width so that none is ever a substring of
	// another (e.g. "q01" inside "q010"), which would let one goroutine's
	// request consume a slot scripted for a different one.
	queryFor := func(i int) string { return fmt.Sprintf("q%02d", i) }

	for i := range requests {
		requests[i] = Request{
			Method:        "POST",
			Path:          "/api/v1/query",
			QueryContains: queryFor(i),
			Status:        200,
			Body:          json.RawMessage(`{}`),
		}
	}
	server := NewServer(Fixture{Requests: requests})

	var wg sync.WaitGroup
	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			server.ServeHTTP(httptest.NewRecorder(), postQuery("/api/v1/query", queryFor(i)))
		}(i)
	}
	wg.Wait()

	if got := server.Remaining(); got != 0 {
		t.Fatalf("Remaining() = %d, want 0", got)
	}
	if got := len(server.Seen()); got != total {
		t.Fatalf("len(Seen()) = %d, want %d", got, total)
	}
}

// TestScenariosMatchTheFiles checks, for each folder a scenario names, that
// Scenarios holds exactly the scenarios whose file sits in that folder.
func TestScenariosMatchTheFiles(t *testing.T) {
	byFolder := map[string][]string{}
	for _, s := range Scenarios {
		byFolder[s.Folder] = append(byFolder[s.Folder], s.Name)
	}

	var folders []string
	for folder := range byFolder {
		folders = append(folders, folder)
	}
	sort.Strings(folders)

	for _, folder := range folders {
		names := byFolder[folder]
		t.Run(folder, func(t *testing.T) {
			anyPath, err := scenarioPath(names[0])
			if err != nil {
				t.Fatalf("Path: %v", err)
			}
			dir := filepath.Dir(anyPath)

			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("reading %s: %v", dir, err)
			}

			var files []string
			for _, entry := range entries {
				if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
					continue
				}
				files = append(files, strings.TrimSuffix(entry.Name(), ".json"))
			}
			sort.Strings(files)
			sort.Strings(names)

			if len(names) != len(files) {
				t.Fatalf("Scenarios has %d entries for folder %s, folder has %d files: %v vs %v", len(names), folder, len(files), names, files)
			}
			for i := range names {
				if names[i] != files[i] {
					t.Fatalf("Scenarios %v does not match folder %v", names, files)
				}
			}
		})
	}
}

func TestMergeKeepsTheOrderOfEachFixture(t *testing.T) {
	a := Fixture{Requests: []Request{{Path: "/a1"}, {Path: "/a2"}}}
	b := Fixture{Requests: []Request{{Path: "/b1"}}}
	got := Merge(a, b)
	var paths []string
	for _, r := range got.Requests {
		paths = append(paths, r.Path)
	}
	if want := []string{"/a1", "/a2", "/b1"}; !slices.Equal(paths, want) {
		t.Fatalf("paths = %q, want %q", paths, want)
	}
}
