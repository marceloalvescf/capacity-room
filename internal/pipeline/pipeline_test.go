package pipeline_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/goldentest"
	"github.com/marceloalvescf/capacity-room/internal/pipeline"
	"github.com/marceloalvescf/capacity-room/internal/prom"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

const (
	goldenID      = "00000000-0000-4000-8000-000000000001"
	goldenVersion = "0.0.0-golden"
	goldenSpan    = 42 * time.Second
)

// fixture is one scenario served in this process, with the run that
// goldentest makes of it.
type fixture struct {
	run      pipeline.Run
	nowCalls *int
	idCalls  *int
	steps    *[]pipeline.Step
}

func scenarioNamed(t *testing.T, name string) promtest.Scenario {
	t.Helper()
	for _, s := range promtest.Scenarios {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("no scenario %q", name)
	return promtest.Scenario{}
}

func newRun(t *testing.T, scenario promtest.Scenario, serverURL string) fixture {
	t.Helper()
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	emittedAt := time.Date(2026, time.July, 15, 14, 30, 0, 0, loc)

	cfg := config.Default()
	cfg.Metrics.URL = serverURL + scenario.APIRoot
	cfg.Cluster.Name = "prod"
	cfg.Cluster.Namespaces = []string{"payments"}
	cfg.Analysis.NodeAutoscaling = scenario.NodeAutoscaling
	cfg.Analysis.ExcludedContainers = []string{"istio-proxy"}
	cfg.Report.Organization = "ACME"
	cfg.Report.Classification = "Uso interno"
	cfg.Report.Author = "Time SRE"

	window, err := collect.NewWindow(cfg.Analysis, emittedAt)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}

	nowCalls, idCalls := 0, 0
	var steps []pipeline.Step
	f := fixture{nowCalls: &nowCalls, idCalls: &idCalls, steps: &steps}
	f.run = pipeline.Run{
		Options:   prom.Options{Metrics: cfg.Metrics, Token: scenario.Token},
		Config:    cfg,
		Window:    window,
		EmittedAt: emittedAt,
		Now: func() time.Time {
			nowCalls++
			return emittedAt.Add(goldenSpan)
		},
		NewID: func() (string, error) {
			idCalls++
			return goldenID, nil
		},
		Version: goldenVersion,
		Step: func(_ context.Context, s pipeline.Step) error {
			steps = append(steps, s)
			return nil
		},
	}
	return f
}

func serveScenario(t *testing.T, name string) fixture {
	t.Helper()
	scenario := scenarioNamed(t, name)
	recorded, err := promtest.LoadScenario(name)
	if err != nil {
		t.Fatalf("promtest.Load: %v", err)
	}
	server := httptest.NewServer(promtest.NewServer(recorded))
	t.Cleanup(server.Close)
	return newRun(t, scenario, server.URL)
}

// preflightRequests returns the number of requests of the preflight of
// direct-complete. They are the requests that the golden list of queries of
// the scenario (testdata/queries/direct-complete.txt) holds before its first
// range query, because the collectors begin with a range query.
func preflightRequests(t *testing.T) int {
	t.Helper()
	path, err := goldentest.QueriesPath("direct-complete")
	if err != nil {
		t.Fatalf("QueriesPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.HasPrefix(line, "/api/v1/query_range\t") {
			break
		}
		count++
	}
	return count
}

func TestPreflightRequestsComeFromTheRecordedQueries(t *testing.T) {
	if got := preflightRequests(t); got != 29 {
		t.Fatalf("preflightRequests = %d, want 29", got)
	}
}

// emptyCollectors answers the preflight of direct-complete as recorded,
// except the probe of the ConfigMaps, and every later
// request with an empty result.
func emptyCollectors(t *testing.T) fixture {
	t.Helper()
	scenario := scenarioNamed(t, "direct-complete")
	recorded, err := promtest.LoadScenario("direct-complete")
	if err != nil {
		t.Fatalf("promtest.Load: %v", err)
	}
	fake := promtest.NewServer(recorded)
	preflight := preflightRequests(t)
	var mu sync.Mutex
	received := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		received++
		inPreflight := received <= preflight
		mu.Unlock()
		if inPreflight && !strings.Contains(r.PostFormValue("query"), "kube_configmap_info") {
			fake.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resultType := "vector"
		if strings.HasSuffix(r.URL.Path, "query_range") {
			resultType = "matrix"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":%q,"result":[]}}`, resultType)
	}))
	t.Cleanup(server.Close)
	return newRun(t, scenario, server.URL)
}

func TestCollectEqualsTheGoldenSnapshot(t *testing.T) {
	f := serveScenario(t, "direct-complete")

	got, err := pipeline.Collect(context.Background(), f.run)
	if err != nil {
		t.Fatalf("Collect: %v", err)
	}
	data, err := snapshot.Encode(got.Snapshot)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	path, err := goldentest.SnapshotPath("direct-complete")
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.Equal(data, want) {
		t.Fatalf("the encoded snapshot differs from %s", path)
	}
	if len(got.Capabilities) == 0 {
		t.Fatalf("Capabilities is empty")
	}
}

func TestCollectCallsTheStepsInOrder(t *testing.T) {
	f := serveScenario(t, "direct-complete")

	if _, err := pipeline.Collect(context.Background(), f.run); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	want := []pipeline.Step{"collecting", "analyzing", "snapshotting"}
	if !slices.Equal(*f.steps, want) {
		t.Fatalf("steps = %v, want %v", *f.steps, want)
	}
}

func TestCollectStopsOnAStepError(t *testing.T) {
	f := serveScenario(t, "direct-complete")
	boom := errors.New("stop here")
	f.run.Step = func(_ context.Context, s pipeline.Step) error {
		*f.steps = append(*f.steps, s)
		if s == pipeline.StepAnalyzing {
			return boom
		}
		return nil
	}

	got, err := pipeline.Collect(context.Background(), f.run)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want the hook's error", err)
	}
	if got.Snapshot.Report.ID != "" {
		t.Fatalf("a snapshot was built: %q", got.Snapshot.Report.ID)
	}
	if want := []pipeline.Step{"collecting", "analyzing"}; !slices.Equal(*f.steps, want) {
		t.Fatalf("steps = %v, want %v", *f.steps, want)
	}
	if *f.nowCalls != 1 || *f.idCalls != 1 {
		t.Fatalf("Now read %d times and NewID called %d times, want 1 and 1", *f.nowCalls, *f.idCalls)
	}
}

func TestCollectReadsTheClockOnceAndTheIDOnce(t *testing.T) {
	f := serveScenario(t, "direct-complete")

	if _, err := pipeline.Collect(context.Background(), f.run); err != nil {
		t.Fatalf("Collect: %v", err)
	}
	if *f.nowCalls != 1 || *f.idCalls != 1 {
		t.Fatalf("Now read %d times and NewID called %d times, want 1 and 1", *f.nowCalls, *f.idCalls)
	}
}

func TestCollectReturnsNothingToReport(t *testing.T) {
	f := emptyCollectors(t)

	_, err := pipeline.Collect(context.Background(), f.run)
	if !errors.Is(err, collect.ErrNoUsefulSections) {
		t.Fatalf("err = %v, want collect.ErrNoUsefulSections", err)
	}
	if *f.nowCalls != 0 || *f.idCalls != 0 {
		t.Fatalf("Now read %d times and NewID called %d times, want 0 and 0", *f.nowCalls, *f.idCalls)
	}
	if want := []pipeline.Step{"collecting"}; !slices.Equal(*f.steps, want) {
		t.Fatalf("steps = %v, want %v", *f.steps, want)
	}
}
