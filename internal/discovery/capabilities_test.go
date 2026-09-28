package discovery

import (
	"context"
	"errors"
	"math"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

// capStub answers Detect's queries by exact text and records every query it
// receives, in order. failAt, when non-zero, makes the call of that number
// (1-indexed) fail with failErr instead of being answered, and no call
// after it is recorded.
type capStub struct {
	answers map[string][]prom.Sample
	failAt  int
	failErr error
	queries []string
}

func (s *capStub) Query(_ context.Context, query string, _ time.Time) ([]prom.Sample, error) {
	if s.failAt != 0 && len(s.queries)+1 == s.failAt {
		s.queries = append(s.queries, query)
		return nil, s.failErr
	}
	s.queries = append(s.queries, query)
	return s.answers[query], nil
}

// present builds one valid sample above zero: the shape every "found"
// answer in these tests carries.
func present() []prom.Sample {
	return []prom.Sample{{Value: 1, Valid: true}}
}

// noMatcherScope is the scope most tests run against: matcher off, so the
// queries built are the bare `count(metric)` and `count(metric{resource=...})`
// forms.
func noMatcherScope() Scope {
	return Scope{ClusterName: "prod", ClusterLabel: "cluster", FilterMode: "auto", MatcherEnabled: false, Confidence: ConfidenceInferred}
}

// matcherScope is a scope with the matcher on, for the tests that check the
// matcher is carried and escaped.
func matcherScope(name string) Scope {
	return Scope{ClusterName: name, ClusterLabel: "cluster", FilterMode: "required", MatcherEnabled: true, Confidence: ConfidenceExact}
}

func byName(capabilities []Capability, name string) Capability {
	for _, c := range capabilities {
		if c.Name == name {
			return c
		}
	}
	return Capability{}
}

func TestDetectReturnsNineInOrder(t *testing.T) {
	answers := map[string][]prom.Sample{
		"count(container_cpu_usage_seconds_total)":                       present(),
		"count(container_memory_working_set_bytes)":                      present(),
		"count(namespace_workload_pod:kube_pod_owner:relabel)":           present(),
		"count(kube_pod_owner)":                                          present(),
		`count(kube_pod_container_resource_requests{resource="cpu"})`:    present(),
		`count(kube_pod_container_resource_requests{resource="memory"})`: present(),
		`count(kube_pod_container_resource_limits{resource="cpu"})`:      present(),
		`count(kube_pod_container_resource_limits{resource="memory"})`:   present(),
		"count(kube_pod_container_resource_requests_cpu_cores)":          present(),
		"count(kube_pod_container_resource_requests_memory_bytes)":       present(),
		"count(kube_pod_container_resource_limits_cpu_cores)":            present(),
		"count(kube_pod_container_resource_limits_memory_bytes)":         present(),
		"count(kube_horizontalpodautoscaler_info)":                       present(),
		"count(kube_horizontalpodautoscaler_spec_min_replicas)":          present(),
		"count(kube_horizontalpodautoscaler_spec_max_replicas)":          present(),
		"count(kube_horizontalpodautoscaler_status_current_replicas)":    present(),
		"count(kube_horizontalpodautoscaler_status_desired_replicas)":    present(),
		"count(kube_deployment_labels)":                                  present(),
		"count(kube_statefulset_labels)":                                 present(),
		"count(kube_node_info)":                                          present(),
		"count(kube_node_labels)":                                        present(),
		"count(instance:node_cpu_utilisation:rate5m)":                    present(),
		"count(instance:node_memory_utilisation:ratio)":                  present(),
		"count(node_cpu_seconds_total)":                                  present(),
		"count(node_memory_MemAvailable_bytes)":                          present(),
		"count(node_memory_MemTotal_bytes)":                              present(),
		`count({__name__=~"cluster_autoscaler_.+"})`:                     present(),
	}
	stub := &capStub{answers: answers}

	caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(caps) != len(Names) {
		t.Fatalf("len(caps) = %d, want %d", len(caps), len(Names))
	}
	for i, c := range caps {
		if c.Name != Names[i] {
			t.Errorf("caps[%d].Name = %q, want %q", i, c.Name, Names[i])
		}
	}
}

func TestNamespaceUsage(t *testing.T) {
	tests := []struct {
		name         string
		cpu, memory  bool
		wantState    string
		wantEvidence []string
		wantReasons  []string
	}{
		{"both present", true, true, StateAvailable, []string{metricContainerCPUUsageSecondsTotal, metricContainerMemoryWorkingSetBytes}, []string{}},
		{"CPU only", true, false, StatePartial, []string{metricContainerCPUUsageSecondsTotal}, []string{reasonMemoryUsageMetricNotFound}},
		{"memory only", false, true, StatePartial, []string{metricContainerMemoryWorkingSetBytes}, []string{reasonCPUUsageMetricNotFound}},
		{"none", false, false, StateUnavailable, []string{}, []string{reasonUsageMetricsNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			if tt.cpu {
				answers["count(container_cpu_usage_seconds_total)"] = present()
			}
			if tt.memory {
				answers["count(container_memory_working_set_bytes)"] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "namespace_usage")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.Evidence, tt.wantEvidence) {
				t.Errorf("Evidence = %v, want %v", got.Evidence, tt.wantEvidence)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
			if got.Strategy != StrategyRawMetrics && tt.wantState != StateUnavailable {
				t.Errorf("Strategy = %q, want %q", got.Strategy, StrategyRawMetrics)
			}
			if tt.wantState == StateUnavailable && got.Strategy != StrategyNone {
				t.Errorf("Strategy = %q, want %q", got.Strategy, StrategyNone)
			}
		})
	}
}

func TestWorkloadUsage(t *testing.T) {
	tests := []struct {
		name                    string
		rule, raw, cpu, memory  bool
		wantState, wantStrategy string
		wantReasons             []string
	}{
		{"rule and both usage", true, false, true, true, StateAvailable, StrategyRecordingRule, []string{}},
		{"rule and CPU only", true, false, true, false, StatePartial, StrategyRecordingRule, []string{reasonMemoryUsageMetricNotFound}},
		{"raw owner and both", false, true, true, true, StatePartial, StrategyRawMetrics, []string{reasonWorkloadHistoryInexact}},
		{"raw owner and CPU only", false, true, true, false, StatePartial, StrategyRawMetrics, []string{reasonWorkloadHistoryInexact, reasonMemoryUsageMetricNotFound}},
		{"usage and no owner", false, false, true, true, StatePartial, StrategyRawMetrics, []string{reasonWorkloadAssociationUnavailable}},
		{"owner and no usage", true, false, false, false, StateUnavailable, StrategyNone, []string{reasonUsageMetricsNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			if tt.rule {
				answers["count(namespace_workload_pod:kube_pod_owner:relabel)"] = present()
			}
			if tt.raw {
				answers["count(kube_pod_owner)"] = present()
			}
			if tt.cpu {
				answers["count(container_cpu_usage_seconds_total)"] = present()
			}
			if tt.memory {
				answers["count(container_memory_working_set_bytes)"] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "workload_usage")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if got.Strategy != tt.wantStrategy {
				t.Errorf("Strategy = %q, want %q", got.Strategy, tt.wantStrategy)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestRequestsLimits(t *testing.T) {
	unifiedCPUReq := `count(kube_pod_container_resource_requests{resource="cpu"})`
	unifiedMemReq := `count(kube_pod_container_resource_requests{resource="memory"})`
	unifiedCPULim := `count(kube_pod_container_resource_limits{resource="cpu"})`
	unifiedMemLim := `count(kube_pod_container_resource_limits{resource="memory"})`
	legacyReqCPU := "count(kube_pod_container_resource_requests_cpu_cores)"
	legacyReqMem := "count(kube_pod_container_resource_requests_memory_bytes)"
	legacyLimCPU := "count(kube_pod_container_resource_limits_cpu_cores)"
	legacyLimMem := "count(kube_pod_container_resource_limits_memory_bytes)"

	tests := []struct {
		name        string
		present     []string
		wantState   string
		wantReasons []string
	}{
		{"four unified", []string{unifiedCPUReq, unifiedMemReq, unifiedCPULim, unifiedMemLim}, StateAvailable, []string{}},
		{"four legacy", []string{legacyReqCPU, legacyReqMem, legacyLimCPU, legacyLimMem}, StateAvailable, []string{}},
		{"three unified", []string{unifiedCPUReq, unifiedMemReq, unifiedCPULim}, StatePartial, []string{reasonRequestsLimitsIncomplete}},
		{"two legacy", []string{legacyReqCPU, legacyReqMem}, StatePartial, []string{reasonLegacyMetricsPartial}},
		{"none", nil, StateUnavailable, []string{reasonMetricsNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			for _, q := range tt.present {
				answers[q] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "requests_limits")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestHPAHistory(t *testing.T) {
	queries := []string{
		"count(kube_horizontalpodautoscaler_info)",
		"count(kube_horizontalpodautoscaler_spec_min_replicas)",
		"count(kube_horizontalpodautoscaler_spec_max_replicas)",
		"count(kube_horizontalpodautoscaler_status_current_replicas)",
		"count(kube_horizontalpodautoscaler_status_desired_replicas)",
	}
	names := []string{
		metricKubeHorizontalPodAutoscalerInfo,
		metricKubeHorizontalPodAutoscalerSpecMin,
		metricKubeHorizontalPodAutoscalerSpecMax,
		metricKubeHorizontalPodAutoscalerStatusCurrent,
		metricKubeHorizontalPodAutoscalerStatusDesired,
	}
	tests := []struct {
		name        string
		count       int // how many of queries/names, from the front, are present
		wantState   string
		wantReasons []string
	}{
		{"five", 5, StateAvailable, []string{}},
		{"three", 3, StatePartial, []string{reasonHPAHistoryIncomplete}},
		{"none", 0, StateUnavailable, []string{reasonMetricsNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			for i := 0; i < tt.count; i++ {
				answers[queries[i]] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "hpa_history")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
			if tt.count == 3 && !slices.Equal(got.Evidence, names[:3]) {
				t.Errorf("Evidence = %v, want %v", got.Evidence, names[:3])
			}
		})
	}
}

func TestHPAMetricsAreProbedOnceEach(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	for _, name := range []string{
		"kube_horizontalpodautoscaler_info",
		"kube_horizontalpodautoscaler_spec_min_replicas",
		"kube_horizontalpodautoscaler_spec_max_replicas",
		"kube_horizontalpodautoscaler_status_current_replicas",
		"kube_horizontalpodautoscaler_status_desired_replicas",
	} {
		count := 0
		for _, q := range stub.queries {
			if q == "count("+name+")" {
				count++
			}
		}
		if count != 1 {
			t.Errorf("count of %q = %d, want 1", name, count)
		}
	}
}

func TestWorkloadsWithoutHPA(t *testing.T) {
	tests := []struct {
		name        string
		present     []string
		wantState   string
		wantReasons []string
	}{
		{
			"labels of both and HPA info",
			[]string{"count(kube_deployment_labels)", "count(kube_statefulset_labels)", "count(kube_horizontalpodautoscaler_info)"},
			StateAvailable, []string{},
		},
		{
			"replicas of both and HPA info",
			[]string{"count(kube_deployment_spec_replicas)", "count(kube_statefulset_replicas)", "count(kube_horizontalpodautoscaler_info)"},
			StateAvailable, []string{},
		},
		{
			"deployment labels only",
			[]string{"count(kube_deployment_labels)"},
			StatePartial, []string{reasonWorkloadInventoryIncomplete},
		},
		{
			"none",
			nil,
			StateUnavailable, []string{reasonMetricsNotFound},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			for _, q := range tt.present {
				answers[q] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "workloads_without_hpa")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestInventoryShortCut(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{
		"count(kube_deployment_labels)": present(),
	}}

	if _, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	for _, q := range stub.queries {
		if q == "count(kube_deployment_spec_replicas)" {
			t.Error("kube_deployment_spec_replicas was queried, want it skipped")
		}
	}
}

func TestIPNodeMapping(t *testing.T) {
	tests := []struct {
		name         string
		present      []string
		wantState    string
		wantEvidence []string
		wantReasons  []string
	}{
		{"node info", []string{"count(kube_node_info)"}, StateAvailable, []string{metricKubeNodeInfo}, []string{}},
		{
			"no node info and the rules",
			[]string{"count(instance:node_cpu_utilisation:rate5m)", "count(instance:node_memory_utilisation:ratio)"},
			StatePartial, []string{metricInstanceNodeCPUUtilisationRate5m, metricInstanceNodeMemoryUtilisationRatio}, []string{reasonNodeAddressMappingIncomplete},
		},
		{
			"no node info and raw",
			[]string{"count(node_cpu_seconds_total)", "count(node_memory_MemAvailable_bytes)", "count(node_memory_MemTotal_bytes)"},
			StatePartial, []string{metricNodeCPUSecondsTotal, metricNodeMemoryMemAvailableBytes, metricNodeMemoryMemTotalBytes}, []string{reasonNodeAddressMappingIncomplete},
		},
		{"none", nil, StateUnavailable, []string{}, []string{reasonMetricsNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			for _, q := range tt.present {
				answers[q] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "ip_node_mapping")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.Evidence, tt.wantEvidence) {
				t.Errorf("Evidence = %v, want %v", got.Evidence, tt.wantEvidence)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestNodepoolDiscovery(t *testing.T) {
	tests := []struct {
		name         string
		present      []string
		wantState    string
		wantEvidence []string
		wantReasons  []string
	}{
		{"node labels", []string{"count(kube_node_labels)"}, StateAvailable, []string{metricKubeNodeLabels}, []string{}},
		{"node info only", []string{"count(kube_node_info)"}, StatePartial, []string{metricKubeNodeInfo}, []string{reasonNodepoolLabelsNotFound}},
		{"none", nil, StateUnavailable, []string{}, []string{reasonMetricsNotFound}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			for _, q := range tt.present {
				answers[q] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "nodepool_discovery")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if !slices.Equal(got.Evidence, tt.wantEvidence) {
				t.Errorf("Evidence = %v, want %v", got.Evidence, tt.wantEvidence)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestNodeUtilization(t *testing.T) {
	bothRules := []string{"count(instance:node_cpu_utilisation:rate5m)", "count(instance:node_memory_utilisation:ratio)"}
	threeRaw := []string{"count(node_cpu_seconds_total)", "count(node_memory_MemAvailable_bytes)", "count(node_memory_MemTotal_bytes)"}

	tests := []struct {
		name         string
		present      []string
		wantState    string
		wantStrategy string
		wantEvidence []string
	}{
		{"both rules", bothRules, StateAvailable, StrategyRecordingRule, bothRules[:0]},
		{
			"one rule and three raw",
			append([]string{"count(instance:node_cpu_utilisation:rate5m)"}, threeRaw...),
			StateAvailable, StrategyRawMetrics, nil,
		},
		{"three raw", threeRaw, StateAvailable, StrategyRawMetrics, []string{metricNodeCPUSecondsTotal, metricNodeMemoryMemAvailableBytes, metricNodeMemoryMemTotalBytes}},
		{"two raw", threeRaw[:2], StateUnavailable, StrategyNone, []string{}},
		{"none", nil, StateUnavailable, StrategyNone, []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			for _, q := range tt.present {
				answers[q] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "node_utilization")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if got.Strategy != tt.wantStrategy {
				t.Errorf("Strategy = %q, want %q", got.Strategy, tt.wantStrategy)
			}
			if tt.name == "three raw" && !slices.Equal(got.Evidence, tt.wantEvidence) {
				t.Errorf("Evidence = %v, want %v", got.Evidence, tt.wantEvidence)
			}
		})
	}
}

func TestNodeUtilizationShortCut(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	for _, q := range stub.queries {
		if q == "count(instance:node_memory_utilisation:ratio)" {
			t.Error("the memory rule was queried, want it skipped when the CPU rule is absent")
		}
	}
}

func TestNodeAutoscaling(t *testing.T) {
	tests := []struct {
		name              string
		configured        string
		autoscalerPresent bool
		wantState         string
		wantStrategy      string
		wantEvidence      []string
		wantReasons       []string
	}{
		{"configured enabled, metrics absent", "enabled", false, StateAvailable, StrategyConfigured, []string{}, []string{}},
		{"configured disabled, metrics present", "disabled", true, StateAvailable, StrategyConfigured, []string{}, []string{}},
		{"auto, present", "auto", true, StateAvailable, StrategyRawMetrics, []string{"cluster_autoscaler_*"}, []string{}},
		{"auto, absent", "auto", false, StatePartial, StrategyNone, []string{}, []string{reasonNodeAutoscalingUndetermined}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answers := map[string][]prom.Sample{}
			if tt.autoscalerPresent {
				answers[`count({__name__=~"cluster_autoscaler_.+"})`] = present()
			}
			stub := &capStub{answers: answers}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: tt.configured}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "node_autoscaling")
			if got.State != tt.wantState {
				t.Errorf("State = %q, want %q", got.State, tt.wantState)
			}
			if got.Strategy != tt.wantStrategy {
				t.Errorf("Strategy = %q, want %q", got.Strategy, tt.wantStrategy)
			}
			if !slices.Equal(got.Evidence, tt.wantEvidence) {
				t.Errorf("Evidence = %v, want %v", got.Evidence, tt.wantEvidence)
			}
			if !slices.Equal(got.Reasons, tt.wantReasons) {
				t.Errorf("Reasons = %v, want %v", got.Reasons, tt.wantReasons)
			}
		})
	}
}

func TestNodeAutoscalingIsProbedWhenConfigured(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "enabled"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	found := false
	for _, q := range stub.queries {
		if contains(q, "cluster_autoscaler_") {
			found = true
		}
	}
	if !found {
		t.Error("no query held cluster_autoscaler_, want the probe asked even when configured")
	}
}

func TestProbesCarryTheMatcher(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, matcherScope("prod"), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(stub.queries) == 0 {
		t.Fatal("no queries were issued")
	}
	for _, q := range stub.queries {
		if contains(q, "count(") && !contains(q, `cluster="prod"`) {
			t.Errorf("query %q does not carry the matcher", q)
		}
	}
}

func TestProbesEscapeTheMatcher(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, matcherScope(`prod"west\one`), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	if len(stub.queries) == 0 {
		t.Fatal("no queries were issued")
	}
	want := `cluster="prod\"west\\one"`
	for _, q := range stub.queries {
		if !contains(q, want) {
			t.Errorf("query %q does not hold %q", q, want)
		}
	}
}

func TestProbesWithoutMatcher(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	for _, q := range stub.queries {
		if contains(q, "cluster=") {
			t.Errorf("query %q holds a matcher, want none", q)
		}
	}
}

func TestProbeSortsMatchers(t *testing.T) {
	stub := &capStub{answers: map[string][]prom.Sample{}}

	if _, err := Detect(context.Background(), stub, matcherScope("prod"), config.Analysis{NodeAutoscaling: "auto"}, time.Now()); err != nil {
		t.Fatalf("Detect: %v", err)
	}

	want := `{cluster="prod",resource="cpu"}`
	found := false
	for _, q := range stub.queries {
		if contains(q, want) {
			found = true
		}
	}
	if !found {
		t.Errorf("no query held %q", want)
	}
}

func TestPresentNeedsAValueAboveZero(t *testing.T) {
	tests := []struct {
		name    string
		samples []prom.Sample
	}{
		{"the count is 0", []prom.Sample{{Value: 0, Valid: true}}},
		{"NaN", []prom.Sample{{Value: math.NaN(), Valid: false}}},
		{"no sample", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &capStub{answers: map[string][]prom.Sample{
				"count(container_cpu_usage_seconds_total)":  tt.samples,
				"count(container_memory_working_set_bytes)": present(),
			}}

			caps, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			got := byName(caps, "namespace_usage")
			if got.State != StatePartial {
				t.Errorf("State = %q, want %q (CPU should read as absent)", got.State, StatePartial)
			}
			if !slices.Contains(got.Reasons, reasonCPUUsageMetricNotFound) {
				t.Errorf("Reasons = %v, want to hold %q", got.Reasons, reasonCPUUsageMetricNotFound)
			}
		})
	}
}

func TestDetectReturnsClientError(t *testing.T) {
	wantErr := &prom.Error{Kind: prom.KindQuery, Message: "metrics endpoint returned HTTP 500"}
	stub := &capStub{answers: map[string][]prom.Sample{}, failAt: 3, failErr: wantErr}

	_, err := Detect(context.Background(), stub, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())

	var promErr *prom.Error
	if !errors.As(err, &promErr) {
		t.Fatalf("Detect: err = %v, want a *prom.Error", err)
	}
	if promErr != wantErr {
		t.Errorf("Detect: err = %v, want %v", promErr, wantErr)
	}
	if len(stub.queries) != 3 {
		t.Errorf("len(queries) = %d, want 3 (no probe after the failing one)", len(stub.queries))
	}
}

func TestEveryReasonIsInTheRegistry(t *testing.T) {
	scenarios := []struct {
		name    string
		answers map[string][]prom.Sample
	}{
		{"everything absent", map[string][]prom.Sample{}},
		{"memory only", map[string][]prom.Sample{"count(container_memory_working_set_bytes)": present()}},
		{"cpu only", map[string][]prom.Sample{"count(container_cpu_usage_seconds_total)": present()}},
		{
			"raw ownership, both usage",
			map[string][]prom.Sample{
				"count(container_cpu_usage_seconds_total)":  present(),
				"count(container_memory_working_set_bytes)": present(),
				"count(kube_pod_owner)":                     present(),
			},
		},
		{
			"usage with no ownership",
			map[string][]prom.Sample{
				"count(container_cpu_usage_seconds_total)":  present(),
				"count(container_memory_working_set_bytes)": present(),
			},
		},
		{
			"partial unified requests_limits",
			map[string][]prom.Sample{`count(kube_pod_container_resource_requests{resource="cpu"})`: present()},
		},
		{
			"partial legacy requests_limits",
			map[string][]prom.Sample{"count(kube_pod_container_resource_requests_cpu_cores)": present()},
		},
		{
			"partial hpa_history",
			map[string][]prom.Sample{"count(kube_horizontalpodautoscaler_info)": present()},
		},
		{
			"partial workloads_without_hpa",
			map[string][]prom.Sample{"count(kube_deployment_labels)": present()},
		},
		{
			"partial ip_node_mapping",
			map[string][]prom.Sample{"count(instance:node_cpu_utilisation:rate5m)": present(), "count(instance:node_memory_utilisation:ratio)": present()},
		},
		{
			"partial nodepool_discovery",
			map[string][]prom.Sample{"count(kube_node_info)": present()},
		},
		{
			"partial node_autoscaling",
			map[string][]prom.Sample{},
		},
	}

	found := map[string]struct{}{}
	for _, s := range scenarios {
		caps, err := Detect(context.Background(), &capStub{answers: s.answers}, noMatcherScope(), config.Analysis{NodeAutoscaling: "auto"}, time.Now())
		if err != nil {
			t.Fatalf("%s: Detect: %v", s.name, err)
		}
		for _, c := range caps {
			for _, reason := range c.Reasons {
				if !slices.Contains(ReasonCodes, reason) {
					t.Errorf("%s: capability %s carries %q, which is not in ReasonCodes", s.name, c.Name, reason)
				}
				found[reason] = struct{}{}
			}
		}
	}

	for _, code := range ReasonCodes {
		if _, ok := found[code]; !ok {
			t.Errorf("ReasonCodes holds %q, which no scenario produced", code)
		}
	}
}

func TestCheckSufficient(t *testing.T) {
	allAvailable := func() []Capability {
		caps := make([]Capability, len(Names))
		for i, name := range Names {
			caps[i] = Capability{Name: name, State: StateAvailable, Strategy: StrategyRawMetrics, Evidence: []string{}, Reasons: []string{}}
		}
		return caps
	}
	withState := func(base []Capability, states map[string]string) []Capability {
		out := make([]Capability, len(base))
		copy(out, base)
		for i, c := range out {
			if state, ok := states[c.Name]; ok {
				c.State = state
				out[i] = c
			}
		}
		return out
	}

	tests := []struct {
		name    string
		caps    []Capability
		wantErr bool
	}{
		{"all nine available", allAvailable(), false},
		{
			"the five unavailable and autoscaling partial",
			withState(allAvailable(), map[string]string{
				"namespace_usage": StateUnavailable, "workload_usage": StateUnavailable,
				"hpa_history": StateUnavailable, "workloads_without_hpa": StateUnavailable,
				"node_utilization": StateUnavailable, "node_autoscaling": StatePartial,
			}),
			true,
		},
		{
			"the five unavailable and autoscaling available",
			withState(allAvailable(), map[string]string{
				"namespace_usage": StateUnavailable, "workload_usage": StateUnavailable,
				"hpa_history": StateUnavailable, "workloads_without_hpa": StateUnavailable,
				"node_utilization": StateUnavailable, "node_autoscaling": StateAvailable,
			}),
			false,
		},
		{
			"four unavailable and one partial",
			withState(allAvailable(), map[string]string{
				"namespace_usage": StateUnavailable, "workload_usage": StateUnavailable,
				"hpa_history": StateUnavailable, "workloads_without_hpa": StateUnavailable,
				"node_utilization": StatePartial,
			}),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckSufficient(tt.caps)
			if tt.wantErr {
				if !errors.Is(err, ErrInsufficient) {
					t.Errorf("CheckSufficient: err = %v, want ErrInsufficient", err)
				}
			} else if err != nil {
				t.Errorf("CheckSufficient: err = %v, want nil", err)
			}
		})
	}
}

// scenarioClient connects a fresh server serving name's fixture, and
// returns the client and its scope, resolved with a required "prod"
// cluster, the way every scenario file was built.
func scenarioClient(t *testing.T, scenario promtest.Scenario) (*prom.Client, Scope) {
	t.Helper()
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
	t.Cleanup(ts.Close)

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

	cluster := config.Cluster{Name: "prod", Namespaces: []string{"payments"}, Label: "cluster", FilterMode: "required"}
	scope, err := ResolveScope(context.Background(), client, cluster, time.Now())
	if err != nil {
		t.Fatalf("ResolveScope: %v", err)
	}
	return client, scope
}

func TestDetectScenario(t *testing.T) {
	for _, scenario := range promtest.Scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			client, scope := scenarioClient(t, scenario)

			caps, err := Detect(context.Background(), client, scope, config.Analysis{NodeAutoscaling: scenario.NodeAutoscaling}, time.Now())
			if err != nil {
				t.Fatalf("Detect: %v", err)
			}
			if len(caps) != len(Names) {
				t.Fatalf("len(caps) = %d, want %d", len(caps), len(Names))
			}

			want := scenarioWant(scenario.Name)
			for _, wc := range want {
				got := byName(caps, wc.Name)
				if got.State != wc.State || got.Strategy != wc.Strategy {
					t.Errorf("%s: State/Strategy = %s/%s, want %s/%s", wc.Name, got.State, got.Strategy, wc.State, wc.Strategy)
				}
				if !slices.Equal(got.Reasons, wc.Reasons) {
					t.Errorf("%s: Reasons = %v, want %v", wc.Name, got.Reasons, wc.Reasons)
				}
			}
		})
	}
}

// requestMatchesScripted replays promtest's own matching rule (documented
// on promtest.Server.ServeHTTP): a scripted request matches a seen one when
// its method is equal (a missing method means GET), its path is equal, its
// query_contains is empty or held by the seen query, and its authorization
// is empty or equal to the seen one. It is reimplemented here, rather than
// exported from promtest, because internal/promtest is not to be touched
// while its own reviewer reads it live.
func requestMatchesScripted(c promtest.Request, s promtest.Seen) bool {
	method := c.Method
	if method == "" {
		method = "GET"
	}
	if method != s.Method {
		return false
	}
	if c.Path != s.Path {
		return false
	}
	if c.QueryContains != "" && !strings.Contains(s.Query, c.QueryContains) {
		return false
	}
	if c.Authorization != "" && c.Authorization != s.Authorization {
		return false
	}
	return true
}

// unusedScriptedRequests replays the server's own consumption order against
// the requests it actually saw, and returns the query_contains text of
// every one of the first windowLen scripted requests that no seen request
// ever consumed. This is computed from data promtest already exposes
// (Fixture.Requests and Server.Seen(), which records every request
// received, matched or not, in the order it arrived) rather than from a
// change to promtest itself: a scripted request is consumed by the first
// still-pending one it structurally matches, exactly as
// promtest.Server.consume does, so replaying that order here reproduces
// which scripted requests promtest itself left on the table.
func unusedScriptedRequests(scripted []promtest.Request, seen []promtest.Seen, windowLen int) []string {
	pending := make([]int, len(scripted))
	for i := range scripted {
		pending[i] = i
	}
	consumed := make(map[int]bool, len(scripted))
	for _, s := range seen {
		for pi, idx := range pending {
			if !requestMatchesScripted(scripted[idx], s) {
				continue
			}
			consumed[idx] = true
			pending = append(pending[:pi:pi], pending[pi+1:]...)
			break
		}
	}

	unused := []string{}
	for i := 0; i < windowLen && i < len(scripted); i++ {
		if !consumed[i] {
			unused = append(unused, scripted[i].QueryContains)
		}
	}
	return unused
}

func TestDetectScenarioUsesTheExpectedRequests(t *testing.T) {
	want := map[string]int{
		"direct-complete":       29,
		"grafana-authenticated": 29,
		"thanos-multicluster":   29,
		// the CPU rule (instance:node_cpu_utilisation:rate5m) is present but
		// the memory rule (instance:node_memory_utilisation:ratio) answers
		// absent; node_utilization still falls back to raw_metrics, so no
		// further probe is skipped.
		"raw-fallback": 30,
		// the CPU rule (instance:node_cpu_utilisation:rate5m) is absent, so
		// the memory rule (instance:node_memory_utilisation:ratio) is left
		// out; node_cpu_seconds_total is also absent, so the other two raw
		// node metrics (node_memory_MemAvailable_bytes,
		// node_memory_MemTotal_bytes) are left out too.
		"partial-data":     26,
		"value-edge-cases": 29,
	}

	// windowLen is how many of the fixture's own scripted requests (in its
	// own order, smoke test and scope query included) hold the capability
	// block for that scenario: 29 for every scenario whose last capability
	// probe (cluster_autoscaler_) is scripted at index 28, and 31 for
	// raw-fallback, whose block is two probes longer because neither
	// kube_deployment_labels nor kube_statefulset_labels is present there
	// (so the short-circuited *_spec_replicas probes are not skipped), which
	// pushes its own cluster_autoscaler_ entry to index 31.
	windowLen := map[string]int{
		"direct-complete":       29,
		"grafana-authenticated": 29,
		"thanos-multicluster":   29,
		"raw-fallback":          31,
		"partial-data":          29,
		"value-edge-cases":      29,
	}

	// wantUnused is the query_contains text of every scripted request within
	// windowLen that Detect's run never asked for, in the fixture's own
	// order. Besides the metric each scenario's short cut leaves out, every
	// fixture also scripts a collection-phase request or two inside this
	// same window (kube_pod_status_phase; a second
	// namespace_workload_pod:kube_pod_owner:relabel) that Detect, which only
	// ever asks for it once, never consumes either — real, verified facts of
	// the fixture files, not something this test's code chooses to ignore.
	wantUnused := map[string][]string{
		"direct-complete":       {},
		"grafana-authenticated": {},
		"thanos-multicluster":   {},
		"raw-fallback":          {"kube_pod_status_phase", "instance:node_memory_utilisation:ratio"},
		"partial-data":          {"instance:node_memory_utilisation:ratio", "node_memory_MemAvailable_bytes", "node_memory_MemTotal_bytes"},
		"value-edge-cases":      {},
	}

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

			cluster := config.Cluster{Name: "prod", Namespaces: []string{"payments"}, Label: "cluster", FilterMode: "required"}
			scope, err := ResolveScope(context.Background(), client, cluster, time.Now())
			if err != nil {
				t.Fatalf("ResolveScope: %v", err)
			}
			if _, err := Detect(context.Background(), client, scope, config.Analysis{NodeAutoscaling: scenario.NodeAutoscaling}, time.Now()); err != nil {
				t.Fatalf("Detect: %v", err)
			}

			used := 0
			for _, s := range server.Seen() {
				if s.Matched {
					used++
				}
			}
			if used != want[scenario.Name] {
				t.Errorf("used = %d, want %d", used, want[scenario.Name])
			}

			unused := unusedScriptedRequests(fixture.Requests, server.Seen(), windowLen[scenario.Name])
			if !slices.Equal(unused, wantUnused[scenario.Name]) {
				t.Errorf("unused (within the first %d scripted requests) = %v, want %v", windowLen[scenario.Name], unused, wantUnused[scenario.Name])
			}
		})
	}
}

// scenarioCapability is one expected row of a scenario's result, checked
// against State, Strategy, and Reasons; evidence is left to the unit
// tests above, which pin it exactly.
type scenarioCapability struct {
	Name, State, Strategy string
	Reasons               []string
}

// scenarioWant returns the expected result for every one of the nine
// capabilities of scenario name, from the table of task-8-brief.md.
// "Every capability that is not listed is available", with the strategy
// the three baseline scenarios name for workload_usage and
// node_utilization (recording_rule); every other unlisted capability is
// raw_metrics, the only strategy its available case can carry.
func scenarioWant(name string) []scenarioCapability {
	baseline := []scenarioCapability{
		{"namespace_usage", StateAvailable, StrategyRawMetrics, []string{}},
		{"workload_usage", StateAvailable, StrategyRecordingRule, []string{}},
		{"requests_limits", StateAvailable, StrategyRawMetrics, []string{}},
		{"hpa_history", StateAvailable, StrategyRawMetrics, []string{}},
		{"workloads_without_hpa", StateAvailable, StrategyRawMetrics, []string{}},
		{"ip_node_mapping", StateAvailable, StrategyRawMetrics, []string{}},
		{"nodepool_discovery", StateAvailable, StrategyRawMetrics, []string{}},
		{"node_utilization", StateAvailable, StrategyRecordingRule, []string{}},
		{"node_autoscaling", StateAvailable, StrategyRawMetrics, []string{}},
	}

	set := func(caps []scenarioCapability, changes map[string]scenarioCapability) []scenarioCapability {
		out := make([]scenarioCapability, len(caps))
		copy(out, caps)
		for i, c := range out {
			if replacement, ok := changes[c.Name]; ok {
				out[i] = replacement
			}
		}
		return out
	}

	switch name {
	case "direct-complete", "grafana-authenticated", "thanos-multicluster":
		return baseline
	case "value-edge-cases":
		return set(baseline, map[string]scenarioCapability{
			"node_autoscaling": {"node_autoscaling", StateAvailable, StrategyConfigured, []string{}},
		})
	case "raw-fallback":
		return set(baseline, map[string]scenarioCapability{
			"workload_usage":        {"workload_usage", StatePartial, StrategyRawMetrics, []string{reasonWorkloadHistoryInexact}},
			"workloads_without_hpa": {"workloads_without_hpa", StateAvailable, StrategyRawMetrics, []string{}},
			"node_utilization":      {"node_utilization", StateAvailable, StrategyRawMetrics, []string{}},
			"node_autoscaling":      {"node_autoscaling", StatePartial, StrategyNone, []string{reasonNodeAutoscalingUndetermined}},
		})
	case "partial-data":
		return set(baseline, map[string]scenarioCapability{
			"hpa_history":           {"hpa_history", StateUnavailable, StrategyNone, []string{reasonMetricsNotFound}},
			"workloads_without_hpa": {"workloads_without_hpa", StatePartial, StrategyRawMetrics, []string{reasonWorkloadInventoryIncomplete}},
			"nodepool_discovery":    {"nodepool_discovery", StatePartial, StrategyRawMetrics, []string{reasonNodepoolLabelsNotFound}},
			"node_utilization":      {"node_utilization", StateUnavailable, StrategyNone, []string{reasonMetricsNotFound}},
			"node_autoscaling":      {"node_autoscaling", StatePartial, StrategyNone, []string{reasonNodeAutoscalingUndetermined}},
		})
	default:
		return baseline
	}
}

// contains reports whether s holds substr. A tiny local helper so this file
// does not need to import strings only for one call site repeated many
// times.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (substr == "" || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
