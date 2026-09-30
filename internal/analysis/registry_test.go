package analysis

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

func defaultThresholds() config.Thresholds {
	return config.Default().Thresholds
}

// TestClassifyBoundaries transcribes
// test_threshold_boundaries_are_inclusive_at_attention_and_critical of
// tests/test_capacity_analysis.py.
func TestClassifyBoundaries(t *testing.T) {
	thresholds := defaultThresholds()

	cases := []struct {
		percent float64
		want    string
	}{
		{69.999, SeverityHealthy},
		{70, SeverityAttention},
		{84.999, SeverityAttention},
		{85, SeverityCritical},
	}
	for _, c := range cases {
		if got := Classify(c.percent, thresholds); got != c.want {
			t.Errorf("Classify(%v) = %q, want %q", c.percent, got, c.want)
		}
	}
}

func TestRegistryHoldsSixtySixCodes(t *testing.T) {
	if len(Registry) != 66 {
		t.Fatalf("len(Registry) = %d, want 66", len(Registry))
	}

	seen := make(map[string]bool, len(Registry))
	for _, definition := range Registry {
		if seen[definition.Code] {
			t.Errorf("code %q appears twice in Registry", definition.Code)
		}
		seen[definition.Code] = true
	}

	wantPrefixes := []string{"NAMESPACE", "WORKLOAD"}
	wantResources := []string{"CPU", "MEMORY"}
	wantEndings := []struct {
		suffix   string
		family   string
		severity string
		evidence []string
	}{
		{"REQUEST_HEALTHY", "resource_request_usage", SeverityHealthy, []string{"average_percent", "peak_percent"}},
		{"REQUEST_ATTENTION", "resource_request_usage", SeverityAttention, []string{"average_percent", "peak_percent"}},
		{"REQUEST_CRITICAL", "resource_request_usage", SeverityCritical, []string{"average_percent", "peak_percent"}},
		{"LIMIT_HEALTHY", "resource_limit_usage", SeverityHealthy, []string{"average_percent", "peak_percent"}},
		{"LIMIT_ATTENTION", "resource_limit_usage", SeverityAttention, []string{"average_percent", "peak_percent"}},
		{"LIMIT_CRITICAL", "resource_limit_usage", SeverityCritical, []string{"average_percent", "peak_percent"}},
		{"REQUEST_MISSING", "resource_request_missing", SeverityAttention, []string{}},
		{"LIMIT_MISSING", "resource_limit_missing", SeverityAttention, []string{}},
		{"OVERREQUESTED", "resource_overrequested", SeverityAttention, []string{"average_percent", "peak_percent"}},
		{"BURSTY", "resource_bursty", SeverityAttention, []string{"average", "peak", "gap_percent"}},
	}

	index := 0
	for _, prefix := range wantPrefixes {
		for _, resource := range wantResources {
			for _, e := range wantEndings {
				definition := Registry[index]
				wantCode := prefix + "_" + resource + "_" + e.suffix
				if definition.Code != wantCode {
					t.Errorf("Registry[%d].Code = %q, want %q", index, definition.Code, wantCode)
				}
				if definition.Family != e.family {
					t.Errorf("%s: Family = %q, want %q", wantCode, definition.Family, e.family)
				}
				if !equalStrings(definition.Severities, []string{e.severity}) {
					t.Errorf("%s: Severities = %v, want %v", wantCode, definition.Severities, []string{e.severity})
				}
				if !equalStrings(evidenceNames(definition), e.evidence) {
					t.Errorf("%s: Evidence = %v, want %v", wantCode, evidenceNames(definition), e.evidence)
				}
				index++
			}
		}
	}

	if index != 40 {
		t.Fatalf("checked %d definitions of namespaces and workloads, want 40", index)
	}
	for _, w := range wantClusterDefinitions {
		definition := Registry[index]
		if definition.Code != w.code || definition.Family != w.family || definition.ScopeType != w.scope ||
			!equalStrings(definition.Severities, w.severities) || !equalSpecs(definition.Evidence, w.evidence) {
			t.Errorf("Registry[%d] = %+v, want %+v", index, definition, w)
		}
		index++
	}
}

type wantDefinition struct {
	code, family, scope string
	severities          []string
	evidence            []EvidenceSpec
}

func wantNumbers(names ...string) []EvidenceSpec {
	specs := []EvidenceSpec{}
	for _, name := range names {
		specs = append(specs, EvidenceSpec{Name: name, Kind: KindNumber})
	}
	return specs
}

var (
	wantUsage = []EvidenceSpec{
		{Name: "average_percent", Kind: KindNumber, Optional: true},
		{Name: "peak_percent", Kind: KindNumber, Optional: true},
	}
	wantPressure = []EvidenceSpec{{Name: "node_pressure", Kind: KindBoolean}}
	one          = func(s string) []string { return []string{s} }

	wantClusterDefinitions = []wantDefinition{
		{"NODE_CPU_USAGE_HEALTHY", "node_usage", "node", one("healthy"), wantUsage},
		{"NODE_CPU_USAGE_ATTENTION", "node_usage", "node", one("attention"), wantUsage},
		{"NODE_CPU_USAGE_CRITICAL", "node_usage", "node", one("critical"), wantUsage},
		{"NODE_MEMORY_USAGE_HEALTHY", "node_usage", "node", one("healthy"), wantUsage},
		{"NODE_MEMORY_USAGE_ATTENTION", "node_usage", "node", one("attention"), wantUsage},
		{"NODE_MEMORY_USAGE_CRITICAL", "node_usage", "node", one("critical"), wantUsage},
		{"NODE_CPU_REQUEST_HIGH", "node_request_high", "node", []string{"attention", "critical"}, wantNumbers("percent")},
		{"NODE_MEMORY_REQUEST_HIGH", "node_request_high", "node", []string{"attention", "critical"}, wantNumbers("percent")},
		{"NODE_OVERREQUESTED", "node_overrequested", "node", one("attention"), []EvidenceSpec{{Name: "resources", Kind: KindText}}},
		{"NODE_LOW_UTILIZATION", "node_low_utilization", "node", one("attention"), wantNumbers("maximum_average_percent")},
		{"NODEPOOL_UNKNOWN", "nodepool_unknown", "nodepool", one("attention"), wantNumbers("affected_nodes")},
		{"NODEPOOL_CPU_UNEVEN", "nodepool_uneven", "nodepool", one("attention"), wantNumbers("minimum_peak_percent", "maximum_peak_percent", "spread_percent")},
		{"NODEPOOL_MEMORY_UNEVEN", "nodepool_uneven", "nodepool", one("attention"), wantNumbers("minimum_peak_percent", "maximum_peak_percent", "spread_percent")},
		{"HPA_PINNED_REPLICAS", "hpa_pinned", "hpa", one("healthy"), wantNumbers("spec_min", "spec_max")},
		{"HPA_MAX_REPLICAS_REACHED", "hpa_max_reached", "hpa", one("critical"), wantNumbers("current_peak", "spec_max")},
		{"HPA_NEAR_MAX_REPLICAS", "hpa_near_max", "hpa", one("attention"), wantNumbers("current_average", "spec_max")},
		{"HPA_REPLICA_DIVERGENCE", "hpa_divergence", "hpa", one("attention"), wantNumbers("average_difference")},
		{"DEPLOYMENT_WITHOUT_HPA", "deployment_without_hpa", "workload", one("attention"), wantNumbers()},
		{"STATEFULSET_WITHOUT_HPA_INFO", "statefulset_without_hpa", "workload", one("healthy"), wantNumbers()},
		{"NODE_AUTOSCALING_UNKNOWN", "autoscaling_unknown", "cluster", one("attention"), wantNumbers()},
		{"NODE_AUTOSCALING_DISABLED", "autoscaling_disabled", "cluster", one("attention"), wantPressure},
		{"NODE_AUTOSCALING_DISABLED_PRESSURE", "autoscaling_disabled", "cluster", one("critical"), wantPressure},
		{"NODE_AUTOSCALING_NEAR_MAX", "autoscaling_near_max", "cluster", one("attention"), wantNumbers("current_nodes", "max_nodes")},
		{"NODE_AUTOSCALING_BOUNDS_UNKNOWN", "autoscaling_bounds_unknown", "cluster", one("healthy"), wantNumbers()},
		{"NODE_AUTOSCALING_AT_MIN", "autoscaling_at_min", "cluster", one("healthy"), wantNumbers("min_nodes")},
		{"NODE_AUTOSCALING_ENABLED", "autoscaling_enabled", "cluster", one("healthy"), wantNumbers()},
	}
)

func equalSpecs(a, b []EvidenceSpec) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestRegistryRiskFollowsTheResource covers the twelve LIMIT_ codes of
// usage: their fixed parameter risk is cpu_throttling for CPU and
// oomkill for memory.
func TestRegistryRiskFollowsTheResource(t *testing.T) {
	checked := 0
	for _, definition := range Registry {
		isLimitUsage := definition.Family == "resource_limit_usage"
		if !isLimitUsage {
			continue
		}
		checked++
		want := "cpu_throttling"
		if definition.Params["resource"] == "memory" {
			want = "oomkill"
		}
		if got := definition.Params["risk"]; got != want {
			t.Errorf("%s: risk = %q, want %q", definition.Code, got, want)
		}
	}
	if checked != 12 {
		t.Fatalf("checked %d LIMIT_ codes of usage, want 12", checked)
	}
}

func TestLookupFindsByCode(t *testing.T) {
	definition, ok := Lookup("NAMESPACE_CPU_REQUEST_ATTENTION")
	if !ok {
		t.Fatal("Lookup(NAMESPACE_CPU_REQUEST_ATTENTION) = false, want true")
	}
	if definition.ScopeType != ScopeNamespace {
		t.Errorf("ScopeType = %q, want %q", definition.ScopeType, ScopeNamespace)
	}

	if _, ok := Lookup("NOT_A_CODE"); ok {
		t.Error("Lookup(NOT_A_CODE) = true, want false")
	}
}

func TestRank(t *testing.T) {
	cases := []struct {
		severity string
		want     int
	}{
		{SeverityHealthy, 0},
		{SeverityAttention, 1},
		{SeverityCritical, 2},
		{"unknown", -1},
	}
	for _, c := range cases {
		if got := Rank(c.severity); got != c.want {
			t.Errorf("Rank(%q) = %d, want %d", c.severity, got, c.want)
		}
	}
}

// evidenceNames lists the evidence names of definition, in order.
func evidenceNames(definition Definition) []string {
	names := []string{}
	for _, spec := range definition.Evidence {
		names = append(names, spec.Name)
	}
	return names
}

// TestRegistryHoldsTheKindOfEveryEvidence covers the kind of each evidence
// and the list of severities: the first 40 definitions carry one severity
// and numbers only; the 26 after them are checked against the design in
// TestRegistryHoldsSixtySixCodes.
func TestRegistryHoldsTheKindOfEveryEvidence(t *testing.T) {
	if len(Registry) != 66 {
		t.Fatalf("Registry holds %d definitions, want 66", len(Registry))
	}
	for _, definition := range Registry[:40] {
		if len(definition.Severities) != 1 {
			t.Errorf("%s: Severities = %v, want a list of one", definition.Code, definition.Severities)
		}
		for _, spec := range definition.Evidence {
			if spec.Kind != KindNumber || spec.Optional {
				t.Errorf("%s: evidence %s has kind %q optional %v, want a required number", definition.Code, spec.Name, spec.Kind, spec.Optional)
			}
		}
	}
	for _, definition := range Registry[40:] {
		for _, spec := range definition.Evidence {
			if spec.Optional && definition.Family != "node_usage" {
				t.Errorf("%s: evidence %s is optional outside node_usage", definition.Code, spec.Name)
			}
		}
	}
}

// TestValueConstructors covers the three constructors of Value.
func TestValueConstructors(t *testing.T) {
	cases := []struct {
		name string
		got  Value
		want Value
	}{
		{"number", Number(4.5), Value{Kind: KindNumber, Number: 4.5}},
		{"boolean", Boolean(true), Value{Kind: KindBoolean, Boolean: true}},
		{"text", Text("cpu,memory"), Value{Kind: KindText, Text: "cpu,memory"}},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, c.got, c.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
