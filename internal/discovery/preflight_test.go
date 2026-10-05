package discovery

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

// preflightConfig is the configuration the command builds for a scenario:
// the defaults, the cluster prod with one namespace, and the node
// autoscaling setting of the scenario.
func preflightConfig(url string, scenario promtest.Scenario) config.Config {
	cfg := config.Default()
	cfg.Metrics.URL = url + scenario.APIRoot
	cfg.Metrics.EndpointType = scenario.EndpointType
	cfg.Cluster.Name = "prod"
	cfg.Cluster.Namespaces = []string{"payments"}
	cfg.Analysis.NodeAutoscaling = scenario.NodeAutoscaling
	return cfg
}

func TestRunPreflightAgainstTheFakeServer(t *testing.T) {
	scenario := promtest.Scenarios[0]
	if scenario.Name != "direct-complete" {
		t.Fatalf("first scenario = %q, want direct-complete", scenario.Name)
	}
	path, err := promtest.Path(scenario.Name)
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	ts := httptest.NewServer(promtest.NewServer(fixture))
	defer ts.Close()

	cfg := preflightConfig(ts.URL, scenario)
	got, err := RunPreflight(context.Background(), prom.Options{Metrics: cfg.Metrics}, cfg.Cluster, cfg.Analysis, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("RunPreflight: %v", err)
	}
	if got.Client == nil {
		t.Fatal("Client is nil")
	}
	if got.Client.Kind() != "prometheus" {
		t.Errorf("Client.Kind() = %q, want %q", got.Client.Kind(), "prometheus")
	}
	if got.Scope.ClusterName != "prod" {
		t.Errorf("Scope.ClusterName = %q, want %q", got.Scope.ClusterName, "prod")
	}
	if len(got.Capabilities) != 9 {
		t.Errorf("len(Capabilities) = %d, want 9", len(got.Capabilities))
	}
}

// recordingHandler answers every request with status and body and records
// the path and query of each one.
type recordingHandler struct {
	status int
	body   string

	mu   sync.Mutex
	seen []string
}

func (h *recordingHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mu.Lock()
	h.seen = append(h.seen, r.URL.Path+"?"+r.URL.RawQuery+r.PostForm.Encode())
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(h.status)
	_, _ = w.Write([]byte(h.body))
}

func TestRunPreflightStopsAtTheFirstFailure(t *testing.T) {
	handler := &recordingHandler{status: http.StatusUnauthorized, body: `{}`}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	cfg := config.Default()
	cfg.Metrics.URL = ts.URL + "/api/v1"
	cfg.Metrics.EndpointType = "prometheus"
	cfg.Cluster.Name = "prod"
	cfg.Cluster.Namespaces = []string{"payments"}

	_, err := RunPreflight(context.Background(), prom.Options{Metrics: cfg.Metrics}, cfg.Cluster, cfg.Analysis, time.Unix(1700000000, 0))
	var promErr *prom.Error
	if !errors.As(err, &promErr) {
		t.Fatalf("RunPreflight: err = %v, want a *prom.Error", err)
	}
	if promErr.Kind != prom.KindAuthentication {
		t.Errorf("Kind = %v, want KindAuthentication", promErr.Kind)
	}
	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.seen) != 1 {
		t.Errorf("requests seen = %v, want only the connect probe", handler.seen)
	}
}

func TestRunPreflightInsufficient(t *testing.T) {
	handler := &recordingHandler{
		status: http.StatusOK,
		body:   `{"status":"success","data":{"resultType":"vector","result":[]}}`,
	}
	ts := httptest.NewServer(handler)
	defer ts.Close()

	cfg := config.Default()
	cfg.Metrics.URL = ts.URL + "/api/v1"
	cfg.Metrics.EndpointType = "prometheus"
	cfg.Cluster.Name = "prod"
	cfg.Cluster.Namespaces = []string{"payments"}
	cfg.Cluster.FilterMode = "none"

	_, err := RunPreflight(context.Background(), prom.Options{Metrics: cfg.Metrics}, cfg.Cluster, cfg.Analysis, time.Unix(1700000000, 0))
	if !errors.Is(err, ErrInsufficient) {
		t.Fatalf("RunPreflight: err = %v, want ErrInsufficient", err)
	}
}
