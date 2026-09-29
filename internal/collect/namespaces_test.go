package collect

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
)

// rangeCase answers a range query whose text holds every one of Contains,
// with Series.
type rangeCase struct {
	Contains []string
	Series   []prom.Series
}

// rangeTable builds a range answer function from cases, checked in the
// order given: the first case whose Contains all match the query text
// answers it. A query no case matches gets an empty answer, following the
// fake server of internal/promtest.
func rangeTable(cases ...rangeCase) func(string) ([]prom.Series, error) {
	return func(query string) ([]prom.Series, error) {
		for _, c := range cases {
			matches := true
			for _, s := range c.Contains {
				if !strings.Contains(query, s) {
					matches = false
					break
				}
			}
			if matches {
				return c.Series, nil
			}
		}
		return nil, nil
	}
}

// scriptedQuerier is a Querier that records every query it is asked, in
// order, and answers a range query with rangeFunc and an instant query
// with queryFunc. When failAfter is not zero, the call at that position
// answers failErr instead.
type scriptedQuerier struct {
	rangeFunc func(query string) ([]prom.Series, error)
	queryFunc func(query string) ([]prom.Sample, error)
	queries   []string
	failAfter int
	failErr   error
}

func (s *scriptedQuerier) QueryRange(_ context.Context, query string, _, _ time.Time, _ string) ([]prom.Series, error) {
	s.queries = append(s.queries, query)
	if s.failAfter != 0 && len(s.queries) == s.failAfter {
		return nil, s.failErr
	}
	if s.rangeFunc == nil {
		return nil, nil
	}
	return s.rangeFunc(query)
}

func (s *scriptedQuerier) Query(_ context.Context, query string, _ time.Time) ([]prom.Sample, error) {
	s.queries = append(s.queries, query)
	if s.failAfter != 0 && len(s.queries) == s.failAfter {
		return nil, s.failErr
	}
	if s.queryFunc == nil {
		return nil, nil
	}
	return s.queryFunc(query)
}

// defaultCluster is the cluster the tests use when the scenario needs
// nothing more than one configured namespace.
func defaultCluster() config.Cluster {
	return config.Cluster{Name: "prod", Namespaces: []string{"payments"}, Label: "cluster", FilterMode: "required"}
}

// defaultAnalysis is the analysis configuration of every test, one
// excluded container.
func defaultAnalysis() config.Analysis {
	return config.Analysis{ExcludedContainers: []string{"istio-proxy"}}
}

// testScope is the resolved cluster scope of the tests: a "required" match
// on cluster "prod", with the matcher on or off as asked.
func testScope(matcherEnabled bool) discovery.Scope {
	return discovery.Scope{
		ClusterName:      "prod",
		ClusterLabel:     "cluster",
		FilterMode:       "required",
		MatcherEnabled:   matcherEnabled,
		Confidence:       discovery.ConfidenceExact,
		DiscoveredValues: []string{"prod"},
	}
}

// testCapability builds the requests_limits capability with the given
// state and evidence. A nil evidence gets the two unified metric names.
func testCapability(state string, evidence []string) discovery.Capability {
	strategy := discovery.StrategyRawMetrics
	if state == discovery.StateUnavailable {
		strategy = discovery.StrategyNone
	}
	if evidence == nil {
		evidence = []string{"kube_pod_container_resource_requests", "kube_pod_container_resource_limits"}
	}
	return discovery.Capability{
		Name:     "requests_limits",
		State:    state,
		Strategy: strategy,
		Evidence: evidence,
		Reasons:  []string{},
	}
}

// testInput builds the Input of a test: the window of the original's
// tests, start 1, end 2, step "30s".
func testInput(scope discovery.Scope, cluster config.Cluster, analysis config.Analysis, capability discovery.Capability) Input {
	return Input{
		Scope:        scope,
		Cluster:      cluster,
		Analysis:     analysis,
		Capabilities: []discovery.Capability{capability},
		Window: Window{
			Start: time.Unix(1, 0),
			End:   time.Unix(2, 0),
			Step:  "30s",
		},
	}
}

// hasWarning reports whether warnings holds one of the given code.
func hasWarning(warnings []Message, code string) bool {
	for _, w := range warnings {
		if w.Code == code {
			return true
		}
	}
	return false
}

// TestNamespaceQueriesInOrder transcribes
// test_namespace_collector_applies_scope_and_builds_every_configured_namespace
// of tests/test_namespace_collector.py: the seven query texts equal the
// original's, letter for letter.
func TestNamespaceQueriesInOrder(t *testing.T) {
	q := &scriptedQuerier{}
	cluster := config.Cluster{Name: "prod", Namespaces: []string{"payments", "orders"}, Label: "cluster", FilterMode: "required"}
	in := testInput(testScope(true), cluster, defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	if _, err := Namespaces(context.Background(), q, in); err != nil {
		t.Fatalf("Namespaces: %v", err)
	}

	want := []string{
		"sum by (namespace) (rate(container_cpu_usage_seconds_total{cluster=\"prod\",namespace=~\"(?:payments|orders)\",container!~\"(?:|POD|istio\\\\-proxy)\"}[30s]))",
		"sum by (namespace) (max by (namespace, pod, container) (container_memory_working_set_bytes{cluster=\"prod\",namespace=~\"(?:payments|orders)\",container!~\"(?:|POD|istio\\\\-proxy)\"}))",
		"sum by (namespace) (kube_pod_container_resource_requests{cluster=\"prod\",resource=\"cpu\",namespace=~\"(?:payments|orders)\",container!~\"(?:|POD|istio\\\\-proxy)\"})",
		"sum by (namespace) (kube_pod_container_resource_requests{cluster=\"prod\",resource=\"memory\",namespace=~\"(?:payments|orders)\",container!~\"(?:|POD|istio\\\\-proxy)\"})",
		"sum by (namespace) (kube_pod_container_resource_limits{cluster=\"prod\",resource=\"cpu\",namespace=~\"(?:payments|orders)\",container!~\"(?:|POD|istio\\\\-proxy)\"})",
		"sum by (namespace) (kube_pod_container_resource_limits{cluster=\"prod\",resource=\"memory\",namespace=~\"(?:payments|orders)\",container!~\"(?:|POD|istio\\\\-proxy)\"})",
		"sum by (namespace) (kube_pod_status_phase{cluster=\"prod\",phase=\"Running\",namespace=~\"(?:payments|orders)\"} == 1)",
	}
	if len(q.queries) != len(want) {
		t.Fatalf("len(queries) = %d, want %d\nqueries: %v", len(q.queries), len(want), q.queries)
	}
	for i, w := range want {
		if q.queries[i] != w {
			t.Errorf("queries[%d] =\n%q, want\n%q", i, q.queries[i], w)
		}
	}
}

// TestNamespaceQueriesWithoutScopeMatcher covers a scope whose matcher is
// off: no query holds a cluster matcher.
func TestNamespaceQueriesWithoutScopeMatcher(t *testing.T) {
	q := &scriptedQuerier{}
	in := testInput(testScope(false), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	if _, err := Namespaces(context.Background(), q, in); err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	for _, query := range q.queries {
		if strings.Contains(query, "cluster=") {
			t.Errorf("query %q holds a cluster matcher", query)
		}
	}
}

// TestNamespaceSkipsRequestsAndLimits covers requests_limits unavailable:
// three queries, none of them holding a requests or limits metric, the
// warning of unavailable requests and limits, and a partial state.
func TestNamespaceSkipsRequestsAndLimits(t *testing.T) {
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: func(string) ([]prom.Series, error) { return usage, nil }}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateUnavailable, []string{}))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if len(q.queries) != 3 {
		t.Fatalf("len(queries) = %d, want 3\nqueries: %v", len(q.queries), q.queries)
	}
	for _, query := range q.queries {
		if strings.Contains(query, "resource_requests") || strings.Contains(query, "resource_limits") {
			t.Errorf("query %q holds a resource requests or limits metric", query)
		}
	}
	if result.State != discovery.StatePartial {
		t.Errorf("State = %q, want %q", result.State, discovery.StatePartial)
	}
	if !hasWarning(result.Warnings, WarningNamespaceRequestsLimitsUnavailable) {
		t.Errorf("Warnings = %v, want it to hold %q", result.Warnings, WarningNamespaceRequestsLimitsUnavailable)
	}
}

// TestNamespaceUsesLegacyNames covers evidence that names the four legacy
// metrics: the four resource queries name them and hold no resource=
// matcher.
func TestNamespaceUsesLegacyNames(t *testing.T) {
	legacyEvidence := []string{
		"kube_pod_container_resource_requests_cpu_cores",
		"kube_pod_container_resource_requests_memory_bytes",
		"kube_pod_container_resource_limits_cpu_cores",
		"kube_pod_container_resource_limits_memory_bytes",
	}
	q := &scriptedQuerier{}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, legacyEvidence))

	if _, err := Namespaces(context.Background(), q, in); err != nil {
		t.Fatalf("Namespaces: %v", err)
	}

	resourceQueries := 0
	for _, query := range q.queries {
		if !strings.Contains(query, "resource_requests") && !strings.Contains(query, "resource_limits") {
			continue
		}
		resourceQueries++
		if strings.Contains(query, `resource="`) {
			t.Errorf("query %q holds a resource= matcher", query)
		}
	}
	if resourceQueries != 4 {
		t.Fatalf("resource queries = %d, want 4\nqueries: %v", resourceQueries, q.queries)
	}
	for _, name := range legacyEvidence {
		found := false
		for _, query := range q.queries {
			if strings.Contains(query, name) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("no query names %q\nqueries: %v", name, q.queries)
		}
	}
}

// TestNamespaceRowForEachConfigured covers two configured namespaces,
// answered for one of them: two rows, sorted, the second with every
// statistic unavailable and PodsKnown false.
func TestNamespaceRowForEachConfigured(t *testing.T) {
	cluster := config.Cluster{Name: "prod", Namespaces: []string{"payments", "paymets"}, Label: "cluster", FilterMode: "required"}
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: func(string) ([]prom.Series, error) { return usage, nil }}
	in := testInput(testScope(true), cluster, defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(result.Rows))
	}
	if result.Rows[0].Name != "payments" || result.Rows[1].Name != "paymets" {
		t.Fatalf("Rows = %q, %q, want payments, paymets", result.Rows[0].Name, result.Rows[1].Name)
	}

	second := result.Rows[1]
	if second.PodsKnown {
		t.Errorf("Rows[1].PodsKnown = true, want false")
	}
	stats := map[string]Statistics{
		"CPUUsage": second.Resources.CPUUsage, "MemoryUsage": second.Resources.MemoryUsage,
		"CPURequests": second.Resources.CPURequests, "MemoryRequests": second.Resources.MemoryRequests,
		"CPULimits": second.Resources.CPULimits, "MemoryLimits": second.Resources.MemoryLimits,
		"CPURequestPercent": second.Resources.CPURequestPercent, "MemoryRequestPercent": second.Resources.MemoryRequestPercent,
		"CPULimitPercent": second.Resources.CPULimitPercent, "MemoryLimitPercent": second.Resources.MemoryLimitPercent,
	}
	for name, s := range stats {
		if s.State != StateUnavailable {
			t.Errorf("Rows[1].Resources.%s.State = %q, want %q", name, s.State, StateUnavailable)
		}
	}
}

// TestNamespaceRowsAreSortedAndUnique covers a configuration that repeats
// a namespace: the rows are sorted and each name appears once.
func TestNamespaceRowsAreSortedAndUnique(t *testing.T) {
	cluster := config.Cluster{Name: "prod", Namespaces: []string{"b", "a", "b"}, Label: "cluster", FilterMode: "required"}
	q := &scriptedQuerier{}
	in := testInput(testScope(true), cluster, defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if len(result.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(result.Rows))
	}
	if result.Rows[0].Name != "a" || result.Rows[1].Name != "b" {
		t.Errorf("Rows = %q, %q, want a, b", result.Rows[0].Name, result.Rows[1].Name)
	}
}

// TestNamespaceNumbersOfTheScenario transcribes the recorded answers of
// the namespace "payments" in the "direct-complete" scenario
// (testdata/prometheus/direct-complete.json, requests 48 to 53 for the
// range queries and 29 for the instant one, by their 0-based index) and
// asserts the whole row against the numbers the original recorded for it
// in .cache/phase-2/part-2/python/out/direct-complete.json,
// namespaces[0]. The scenario file itself is not read here; a later task
// runs it.
func TestNamespaceNumbersOfTheScenario(t *testing.T) {
	const t1, t2 = 1784050200, 1784093400

	series := func(value1, value2 float64) []prom.Series {
		return []prom.Series{{
			Labels: map[string]string{"namespace": "payments"},
			Points: []prom.Point{
				{Time: t1, Value: value1, Valid: true},
				{Time: t2, Value: value2, Valid: true},
			},
		}}
	}
	cpuUsage := series(0.2, 0.4)
	memoryUsage := series(100, 120)
	cpuRequests := series(0.5, 0.5)
	memoryRequests := series(0.5, 0.5)
	cpuLimits := series(1.0, 1.0)
	memoryLimits := series(1.0, 1.0)
	podSamples := []prom.Sample{{Labels: map[string]string{"namespace": "payments"}, Value: 3, Valid: true}}

	q := &scriptedQuerier{
		rangeFunc: rangeTable(
			rangeCase{[]string{"container_cpu_usage_seconds_total"}, cpuUsage},
			rangeCase{[]string{"container_memory_working_set_bytes"}, memoryUsage},
			rangeCase{[]string{"kube_pod_container_resource_requests", `resource="cpu"`}, cpuRequests},
			rangeCase{[]string{"kube_pod_container_resource_requests", `resource="memory"`}, memoryRequests},
			rangeCase{[]string{"kube_pod_container_resource_limits", `resource="cpu"`}, cpuLimits},
			rangeCase{[]string{"kube_pod_container_resource_limits", `resource="memory"`}, memoryLimits},
		),
		queryFunc: func(string) ([]prom.Sample, error) { return podSamples, nil },
	}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
	row := result.Rows[0]
	res := row.Resources

	checkStatistics(t, res.CPUUsage, Statistics{State: StateValue, Average: 0.30000000000000004, Minimum: 0.2, Peak: 0.4, Current: 0.4})
	checkStatistics(t, res.MemoryUsage, Statistics{State: StateValue, Average: 110.0, Minimum: 100.0, Peak: 120.0, Current: 120.0})
	checkStatistics(t, res.CPURequests, Statistics{State: StateValue, Average: 0.5, Minimum: 0.5, Peak: 0.5, Current: 0.5})
	checkStatistics(t, res.MemoryRequests, Statistics{State: StateValue, Average: 0.5, Minimum: 0.5, Peak: 0.5, Current: 0.5})
	checkStatistics(t, res.CPULimits, Statistics{State: StateValue, Average: 1.0, Minimum: 1.0, Peak: 1.0, Current: 1.0})
	checkStatistics(t, res.MemoryLimits, Statistics{State: StateValue, Average: 1.0, Minimum: 1.0, Peak: 1.0, Current: 1.0})
	checkStatistics(t, res.CPURequestPercent, Statistics{State: StateValue, Average: 60.0, Minimum: 40.0, Peak: 80.0, Current: 80.0})
	checkStatistics(t, res.MemoryRequestPercent, Statistics{State: StateValue, Average: 22000.0, Minimum: 20000.0, Peak: 24000.0, Current: 24000.0})
	checkStatistics(t, res.CPULimitPercent, Statistics{State: StateValue, Average: 30.0, Minimum: 20.0, Peak: 40.0, Current: 40.0})
	checkStatistics(t, res.MemoryLimitPercent, Statistics{State: StateValue, Average: 11000.0, Minimum: 10000.0, Peak: 12000.0, Current: 12000.0})

	if row.Pods != 3 {
		t.Errorf("Pods = %d, want 3", row.Pods)
	}
	if !row.PodsKnown {
		t.Errorf("PodsKnown = false, want true")
	}
	if row.Approximate {
		t.Errorf("Approximate = true, want false")
	}
	if row.RequestsLimitsState != discovery.StateAvailable {
		t.Errorf("RequestsLimitsState = %q, want %q", row.RequestsLimitsState, discovery.StateAvailable)
	}
}

// TestNamespaceNoBaseWhenAvailable covers usage present with requests and
// limits answered empty while requests_limits is available: the four
// ratios read no_base, and the four requests and limits statistics read
// unavailable.
func TestNamespaceNoBaseWhenAvailable(t *testing.T) {
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: rangeTable(
		rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage},
		rangeCase{[]string{"container_memory_working_set_bytes"}, usage},
	)}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	res := result.Rows[0].Resources
	ratios := map[string]Statistics{
		"CPURequestPercent": res.CPURequestPercent, "MemoryRequestPercent": res.MemoryRequestPercent,
		"CPULimitPercent": res.CPULimitPercent, "MemoryLimitPercent": res.MemoryLimitPercent,
	}
	for name, s := range ratios {
		if s.State != StateNoBase {
			t.Errorf("%s.State = %q, want %q", name, s.State, StateNoBase)
		}
	}
	bases := map[string]Statistics{
		"CPURequests": res.CPURequests, "MemoryRequests": res.MemoryRequests,
		"CPULimits": res.CPULimits, "MemoryLimits": res.MemoryLimits,
	}
	for name, s := range bases {
		if s.State != StateUnavailable {
			t.Errorf("%s.State = %q, want %q", name, s.State, StateUnavailable)
		}
	}
}

// TestNamespaceUnavailableWhenPartial covers the same answers as
// TestNamespaceNoBaseWhenAvailable, but requests_limits partial: the four
// ratios read unavailable, and the warning names the incomplete requests
// and limits.
func TestNamespaceUnavailableWhenPartial(t *testing.T) {
	usage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: rangeTable(
		rangeCase{[]string{"container_cpu_usage_seconds_total"}, usage},
		rangeCase{[]string{"container_memory_working_set_bytes"}, usage},
	)}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StatePartial, []string{"kube_pod_container_resource_requests"}))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	res := result.Rows[0].Resources
	ratios := map[string]Statistics{
		"CPURequestPercent": res.CPURequestPercent, "MemoryRequestPercent": res.MemoryRequestPercent,
		"CPULimitPercent": res.CPULimitPercent, "MemoryLimitPercent": res.MemoryLimitPercent,
	}
	for name, s := range ratios {
		if s.State != StateUnavailable {
			t.Errorf("%s.State = %q, want %q", name, s.State, StateUnavailable)
		}
	}
	if !hasWarning(result.Warnings, WarningNamespaceRequestsLimitsIncomplete) {
		t.Errorf("Warnings = %v, want it to hold %q", result.Warnings, WarningNamespaceRequestsLimitsIncomplete)
	}
}

// TestNamespaceApproximate covers two namespaces whose CPU request base
// lacks one timestamp of the usage for one of them: Approximate is true
// only on that row.
func TestNamespaceApproximate(t *testing.T) {
	cluster := config.Cluster{Name: "prod", Namespaces: []string{"orders", "payments"}, Label: "cluster", FilterMode: "required"}
	cpuUsage := []prom.Series{
		{Labels: map[string]string{"namespace": "orders"}, Points: []prom.Point{{Time: 1, Value: 10, Valid: true}, {Time: 2, Value: 20, Valid: true}}},
		{Labels: map[string]string{"namespace": "payments"}, Points: []prom.Point{{Time: 1, Value: 10, Valid: true}, {Time: 2, Value: 20, Valid: true}}},
	}
	cpuRequests := []prom.Series{
		{Labels: map[string]string{"namespace": "orders"}, Points: []prom.Point{{Time: 1, Value: 100, Valid: true}, {Time: 2, Value: 100, Valid: true}}},
		{Labels: map[string]string{"namespace": "payments"}, Points: []prom.Point{{Time: 1, Value: 100, Valid: true}}},
	}
	q := &scriptedQuerier{rangeFunc: rangeTable(
		rangeCase{[]string{"container_cpu_usage_seconds_total"}, cpuUsage},
		rangeCase{[]string{"kube_pod_container_resource_requests", `resource="cpu"`}, cpuRequests},
	)}
	in := testInput(testScope(true), cluster, defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	for _, row := range result.Rows {
		want := row.Name == "payments"
		if row.Approximate != want {
			t.Errorf("Rows[%q].Approximate = %v, want %v", row.Name, row.Approximate, want)
		}
	}
}

// TestNamespaceUsageIncomplete covers a memory usage answer left empty:
// the warning names the incomplete usage, and the state is partial.
func TestNamespaceUsageIncomplete(t *testing.T) {
	cpuUsage := []prom.Series{{
		Labels: map[string]string{"namespace": "payments"},
		Points: []prom.Point{{Time: 1, Value: 1, Valid: true}},
	}}
	q := &scriptedQuerier{rangeFunc: rangeTable(
		rangeCase{[]string{"container_cpu_usage_seconds_total"}, cpuUsage},
	)}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if result.State != discovery.StatePartial {
		t.Errorf("State = %q, want %q", result.State, discovery.StatePartial)
	}
	if !hasWarning(result.Warnings, WarningNamespaceUsageIncomplete) {
		t.Errorf("Warnings = %v, want it to hold %q", result.Warnings, WarningNamespaceUsageIncomplete)
	}
}

// TestNamespaceNothingAnswered covers both usage answers left empty: the
// state is unavailable, and the configured namespace still gets a row.
func TestNamespaceNothingAnswered(t *testing.T) {
	q := &scriptedQuerier{}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	result, err := Namespaces(context.Background(), q, in)
	if err != nil {
		t.Fatalf("Namespaces: %v", err)
	}
	if result.State != discovery.StateUnavailable {
		t.Errorf("State = %q, want %q", result.State, discovery.StateUnavailable)
	}
	if len(result.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(result.Rows))
	}
}

// TestNamespaceStopsAtTheFirstError covers the third query failing: that
// error comes back, and no query after it is sent.
func TestNamespaceStopsAtTheFirstError(t *testing.T) {
	wantErr := errors.New("boom")
	q := &scriptedQuerier{failAfter: 3, failErr: wantErr}
	in := testInput(testScope(true), defaultCluster(), defaultAnalysis(), testCapability(discovery.StateAvailable, nil))

	_, err := Namespaces(context.Background(), q, in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if len(q.queries) != 3 {
		t.Fatalf("len(queries) = %d, want 3\nqueries: %v", len(q.queries), q.queries)
	}
}
