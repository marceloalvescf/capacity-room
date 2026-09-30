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

func TestRegistryHoldsFortyCodes(t *testing.T) {
	if len(Registry) != 40 {
		t.Fatalf("len(Registry) = %d, want 40", len(Registry))
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
// and the list of severities of each of the 40 definitions.
func TestRegistryHoldsTheKindOfEveryEvidence(t *testing.T) {
	if len(Registry) != 40 {
		t.Fatalf("Registry holds %d definitions, want 40", len(Registry))
	}
	for _, definition := range Registry {
		if len(definition.Severities) != 1 {
			t.Errorf("%s: Severities = %v, want a list of one", definition.Code, definition.Severities)
		}
		for _, spec := range definition.Evidence {
			if spec.Kind != KindNumber {
				t.Errorf("%s: evidence %s has kind %q, want %q", definition.Code, spec.Name, spec.Kind, KindNumber)
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
