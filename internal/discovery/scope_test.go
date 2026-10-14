package discovery

import (
	"context"
	"errors"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/prom"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
)

// queryCall is one recorded call to a fakeQuerier.
type queryCall struct {
	query string
	at    time.Time
}

// fakeQuerier answers by query text and records every query it receives.
type fakeQuerier struct {
	answers map[string][]prom.Sample
	err     error
	queries []queryCall
}

func (f *fakeQuerier) Query(_ context.Context, query string, at time.Time) ([]prom.Sample, error) {
	f.queries = append(f.queries, queryCall{query: query, at: at})
	if f.err != nil {
		return nil, f.err
	}
	return f.answers[query], nil
}

// series builds a sample carrying labels and nothing else. Its value is
// never valid data: a series counts toward the discovered values whatever
// its sample value is, so the tests never give it one.
func series(labels map[string]string) prom.Sample {
	return prom.Sample{Labels: labels}
}

// testCluster returns a config.Cluster for the scope tests, with mode and
// name overridable.
func testCluster(name, label, filterMode string) config.Cluster {
	return config.Cluster{
		Name:       name,
		Namespaces: []string{"payments"},
		Label:      label,
		FilterMode: filterMode,
	}
}

func TestScope(t *testing.T) {
	at := time.Unix(1700000000, 0)

	tests := []struct {
		name    string
		cluster config.Cluster
		primary []prom.Sample
		// fallback is the answer to the `up` query. It is only reached when
		// primary yields no value, and most rows leave it nil because the
		// primary answer alone already decides the case.
		fallback   []prom.Sample
		want       Scope
		wantErrMsg string
	}{
		{
			name:    "required finds the configured cluster",
			cluster: testCluster("prod", "cluster", "required"),
			primary: []prom.Sample{series(map[string]string{"cluster": "prod"}), series(map[string]string{"cluster": "dev"})},
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "required",
				MatcherEnabled:   true,
				Confidence:       ConfidenceExact,
				DiscoveredValues: []string{"dev", "prod"},
			},
		},
		{
			name:       "required rejects a missing cluster",
			cluster:    testCluster("prod", "cluster", "required"),
			primary:    []prom.Sample{series(map[string]string{"cluster": "dev"})},
			wantErrMsg: `cluster "prod" was not found in label "cluster"; visible values: dev`,
		},
		{
			name:       "none rejects a mix of labeled and unlabeled groups",
			cluster:    testCluster("prod", "cluster", "none"),
			primary:    []prom.Sample{series(map[string]string{"cluster": "prod"}), series(map[string]string{})},
			wantErrMsg: "labeled and unlabeled cluster groups are visible while filter mode is none",
		},
		{
			name:       "none rejects more than one visible value",
			cluster:    testCluster("prod", "cluster", "none"),
			primary:    []prom.Sample{series(map[string]string{"cluster": "prod"}), series(map[string]string{"cluster": "dev"})},
			wantErrMsg: "multiple cluster values are visible while filter mode is none",
		},
		{
			name:    "none accepts a single visible value",
			cluster: testCluster("prod", "cluster", "none"),
			primary: []prom.Sample{series(map[string]string{"cluster": "prod"})},
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "none",
				MatcherEnabled:   false,
				Confidence:       ConfidenceExact,
				DiscoveredValues: []string{"prod"},
			},
		},
		{
			name:    "auto finds the configured cluster",
			cluster: testCluster("prod", "cluster", "auto"),
			primary: []prom.Sample{series(map[string]string{"cluster": "prod"})},
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "auto",
				MatcherEnabled:   true,
				Confidence:       ConfidenceExact,
				DiscoveredValues: []string{"prod"},
			},
		},
		{
			name:       "auto rejects a different visible cluster",
			cluster:    testCluster("prod", "cluster", "auto"),
			primary:    []prom.Sample{series(map[string]string{"cluster": "dev"})},
			wantErrMsg: `cluster "prod" was not found in visible label values: dev`,
		},
		{
			name:    "auto infers when nothing is visible",
			cluster: testCluster("prod", "cluster", "auto"),
			primary: nil,
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "auto",
				MatcherEnabled:   false,
				Confidence:       ConfidenceInferred,
				DiscoveredValues: []string{},
			},
		},
		{
			name:       "required rejects when nothing is visible",
			cluster:    testCluster("prod", "cluster", "required"),
			primary:    nil,
			fallback:   nil,
			wantErrMsg: `cluster "prod" was not found in label "cluster"; visible values: none`,
		},
		{
			name:       "required rejects when only unlabeled series are visible",
			cluster:    testCluster("prod", "cluster", "required"),
			primary:    []prom.Sample{series(map[string]string{})},
			fallback:   []prom.Sample{series(map[string]string{})},
			wantErrMsg: `cluster "prod" was not found in label "cluster"; visible values: none`,
		},
		{
			name:     "none accepts when nothing is visible",
			cluster:  testCluster("prod", "cluster", "none"),
			primary:  nil,
			fallback: nil,
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "none",
				MatcherEnabled:   false,
				Confidence:       ConfidenceExact,
				DiscoveredValues: []string{},
			},
		},
		{
			name:     "none accepts when only unlabeled series are visible",
			cluster:  testCluster("prod", "cluster", "none"),
			primary:  []prom.Sample{series(map[string]string{})},
			fallback: []prom.Sample{series(map[string]string{})},
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "none",
				MatcherEnabled:   false,
				Confidence:       ConfidenceExact,
				DiscoveredValues: []string{},
			},
		},
		{
			name:     "auto infers when only unlabeled series are visible",
			cluster:  testCluster("prod", "cluster", "auto"),
			primary:  []prom.Sample{series(map[string]string{})},
			fallback: []prom.Sample{series(map[string]string{})},
			want: Scope{
				ClusterName:      "prod",
				ClusterLabel:     "cluster",
				FilterMode:       "auto",
				MatcherEnabled:   false,
				Confidence:       ConfidenceInferred,
				DiscoveredValues: []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := &fakeQuerier{answers: map[string][]prom.Sample{
				"count by (cluster) (kube_node_info)": tt.primary,
				"count by (cluster) (up)":             tt.fallback,
			}}

			got, err := ResolveScope(context.Background(), q, tt.cluster, at)

			if tt.wantErrMsg != "" {
				var scopeErr *ScopeError
				if !errors.As(err, &scopeErr) {
					t.Fatalf("ResolveScope: err = %v, want a *ScopeError", err)
				}
				if scopeErr.Error() != tt.wantErrMsg {
					t.Errorf("ResolveScope: message = %q, want %q", scopeErr.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("ResolveScope: unexpected error: %v", err)
			}
			if got.ClusterName != tt.want.ClusterName ||
				got.ClusterLabel != tt.want.ClusterLabel ||
				got.FilterMode != tt.want.FilterMode ||
				got.MatcherEnabled != tt.want.MatcherEnabled ||
				got.Confidence != tt.want.Confidence {
				t.Errorf("ResolveScope() = %+v, want %+v", got, tt.want)
			}
			if !slices.Equal(got.DiscoveredValues, tt.want.DiscoveredValues) {
				t.Errorf("DiscoveredValues = %v, want %v", got.DiscoveredValues, tt.want.DiscoveredValues)
			}
			if got.DiscoveredValues == nil {
				t.Error("DiscoveredValues is nil, want a non-nil slice")
			}
		})
	}
}

func TestScopeAsksWithTheConfiguredLabel(t *testing.T) {
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		"count by (k8s_cluster) (kube_node_info)": {series(map[string]string{"k8s_cluster": "prod"})},
	}}
	cluster := testCluster("prod", "k8s_cluster", "required")

	if _, err := ResolveScope(context.Background(), q, cluster, time.Now()); err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}

	if len(q.queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1", len(q.queries))
	}
	want := "count by (k8s_cluster) (kube_node_info)"
	if q.queries[0].query != want {
		t.Errorf("query = %q, want %q", q.queries[0].query, want)
	}
}

func TestScopeFallsBackToUp(t *testing.T) {
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		"count by (cluster) (kube_node_info)": nil,
		"count by (cluster) (up)":             {series(map[string]string{"cluster": "prod"})},
	}}
	cluster := testCluster("prod", "cluster", "required")

	got, err := ResolveScope(context.Background(), q, cluster, time.Now())
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}

	if len(q.queries) != 2 {
		t.Fatalf("len(queries) = %d, want 2", len(q.queries))
	}
	if !slices.Equal(got.DiscoveredValues, []string{"prod"}) {
		t.Errorf("DiscoveredValues = %v, want [prod]", got.DiscoveredValues)
	}
}

func TestScopeDoesNotFallBackWhenValuesExist(t *testing.T) {
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		"count by (cluster) (kube_node_info)": {series(map[string]string{"cluster": "dev"})},
		"count by (cluster) (up)":             {series(map[string]string{"cluster": "prod"})},
	}}
	cluster := testCluster("dev", "cluster", "required")

	if _, err := ResolveScope(context.Background(), q, cluster, time.Now()); err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}

	if len(q.queries) != 1 {
		t.Fatalf("len(queries) = %d, want 1", len(q.queries))
	}
}

func TestScopeFallbackReplacesTheFirstResult(t *testing.T) {
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		"count by (cluster) (kube_node_info)": {series(map[string]string{})},
		"count by (cluster) (up)":             {series(map[string]string{"cluster": "prod"})},
	}}
	cluster := testCluster("prod", "cluster", "none")

	got, err := ResolveScope(context.Background(), q, cluster, time.Now())
	if err != nil {
		t.Fatalf("ResolveScope: unexpected error: %v", err)
	}
	if got.MatcherEnabled {
		t.Error("MatcherEnabled = true, want false")
	}
	if got.Confidence != ConfidenceExact {
		t.Errorf("Confidence = %q, want %q", got.Confidence, ConfidenceExact)
	}
	if !slices.Equal(got.DiscoveredValues, []string{"prod"}) {
		t.Errorf("DiscoveredValues = %v, want [prod]", got.DiscoveredValues)
	}
}

func TestScopeSortsAndDeduplicatesValues(t *testing.T) {
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		"count by (cluster) (kube_node_info)": {
			series(map[string]string{"cluster": "prod"}),
			series(map[string]string{"cluster": "dev"}),
			series(map[string]string{"cluster": "prod"}),
		},
	}}
	cluster := testCluster("prod", "cluster", "required")

	got, err := ResolveScope(context.Background(), q, cluster, time.Now())
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}
	if !slices.Equal(got.DiscoveredValues, []string{"dev", "prod"}) {
		t.Errorf("DiscoveredValues = %v, want [dev prod]", got.DiscoveredValues)
	}
}

func TestScopeReturnsClientError(t *testing.T) {
	wantErr := &prom.Error{Kind: prom.KindQuery, Message: "metrics endpoint returned HTTP 500"}
	q := &fakeQuerier{err: wantErr}
	cluster := testCluster("prod", "cluster", "required")

	_, err := ResolveScope(context.Background(), q, cluster, time.Now())

	var promErr *prom.Error
	if !errors.As(err, &promErr) {
		t.Fatalf("ResolveScope: err = %v, want a *prom.Error", err)
	}
	if promErr != wantErr {
		t.Errorf("ResolveScope: err = %v, want %v", promErr, wantErr)
	}
}

func TestScopePassesTheTime(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	q := &fakeQuerier{answers: map[string][]prom.Sample{
		"count by (cluster) (kube_node_info)": nil,
		"count by (cluster) (up)":             {series(map[string]string{"cluster": "prod"})},
	}}
	cluster := testCluster("prod", "cluster", "required")

	if _, err := ResolveScope(context.Background(), q, cluster, at); err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}

	if len(q.queries) != 2 {
		t.Fatalf("len(queries) = %d, want 2", len(q.queries))
	}
	for i, call := range q.queries {
		if !call.at.Equal(at) {
			t.Errorf("queries[%d].at = %v, want %v", i, call.at, at)
		}
	}
}

func TestMatchers(t *testing.T) {
	tests := []struct {
		name  string
		scope Scope
		want  map[string]string
	}{
		{
			name:  "matcher on",
			scope: Scope{ClusterLabel: "cluster", ClusterName: "prod", MatcherEnabled: true},
			want:  map[string]string{"cluster": "prod"},
		},
		{
			name:  "matcher off",
			scope: Scope{ClusterLabel: "cluster", ClusterName: "prod", MatcherEnabled: false},
			want:  map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.scope.Matchers()
			if got == nil {
				t.Fatal("Matchers() returned nil, want a non-nil map")
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Matchers() = %v, want %v", got, tt.want)
			}
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("Matchers()[%q] = %q, want %q", k, got[k], v)
				}
			}
		})
	}
}

func TestScopeScenario(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			path, err := promtest.Path(scenario.Name)
			if err != nil {
				t.Fatalf("Path: %v", err)
			}
			fixture, err := promtest.Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			server := promtest.NewServer(fixture)
			ts := httptest.NewServer(server)
			defer ts.Close()

			client, err := prom.Connect(context.Background(), prom.Options{
				Metrics: config.Metrics{
					URL:            ts.URL + scenario.APIRoot,
					EndpointType:   scenario.EndpointType,
					TLSVerify:      true,
					TimeoutSeconds: 5,
				},
				Token: scenario.Token,
			})
			if err != nil {
				t.Fatalf("Connect: %v", err)
			}

			cluster := testCluster("prod", "cluster", "required")
			got, err := ResolveScope(context.Background(), client, cluster, time.Now())
			if err != nil {
				t.Fatalf("ResolveScope: %v", err)
			}
			if !got.MatcherEnabled {
				t.Error("MatcherEnabled = false, want true")
			}
			if got.Confidence != ConfidenceExact {
				t.Errorf("Confidence = %q, want %q", got.Confidence, ConfidenceExact)
			}

			want := []string{"prod"}
			if scenario.Name == "thanos-multicluster" {
				want = []string{"dev", "prod"}
			}
			if !slices.Equal(got.DiscoveredValues, want) {
				t.Errorf("DiscoveredValues = %v, want %v", got.DiscoveredValues, want)
			}
		})
	}
}

func TestScopeScenarioMultipleClusters(t *testing.T) {
	path, err := promtest.Path("thanos-multicluster")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	server := promtest.NewServer(fixture)
	ts := httptest.NewServer(server)
	defer ts.Close()

	client, err := prom.Connect(context.Background(), prom.Options{
		Metrics: config.Metrics{
			URL:            ts.URL + "/api/v1",
			EndpointType:   "prometheus",
			TLSVerify:      true,
			TimeoutSeconds: 5,
		},
	})
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	cluster := testCluster("prod", "cluster", "none")
	_, err = ResolveScope(context.Background(), client, cluster, time.Now())

	var scopeErr *ScopeError
	if !errors.As(err, &scopeErr) {
		t.Fatalf("ResolveScope: err = %v, want a *ScopeError", err)
	}
	want := "multiple cluster values are visible while filter mode is none"
	if scopeErr.Error() != want {
		t.Errorf("ResolveScope: message = %q, want %q", scopeErr.Error(), want)
	}
}
