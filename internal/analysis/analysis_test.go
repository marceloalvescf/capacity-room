package analysis

import (
	"slices"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// TestWorkloadScope covers the scope a workload row carries: scope type
// workload, name namespace/name, kind the workload's own.
func TestWorkloadScope(t *testing.T) {
	t.Parallel()
	row := collect.Workload{
		Namespace: "payments",
		Kind:      "Deployment",
		Name:      "api",
		Resources: collect.Resources{CPURequestPercent: valueStats(72, 78)},
	}

	findings := Workloads([]collect.Workload{row}, defaultThresholds())

	if len(findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1: %+v", len(findings), findings)
	}
	got := findings[0]
	if got.ScopeType != ScopeWorkload {
		t.Errorf("ScopeType = %q, want %q", got.ScopeType, ScopeWorkload)
	}
	if got.ScopeName != "payments/api" {
		t.Errorf("ScopeName = %q, want payments/api", got.ScopeName)
	}
	if got.ScopeKind != "Deployment" {
		t.Errorf("ScopeKind = %q, want Deployment", got.ScopeKind)
	}
}

// TestTwoWorkloadsOfOneName covers two workloads that share a namespace
// and a name but differ in kind: each finding carries the kind of its own
// row.
func TestTwoWorkloadsOfOneName(t *testing.T) {
	t.Parallel()
	critical := collect.Workload{
		Namespace: "payments", Kind: "Deployment", Name: "api",
		Resources: collect.Resources{CPURequestPercent: valueStats(90, 95)},
	}
	healthy := collect.Workload{
		Namespace: "payments", Kind: "StatefulSet", Name: "api",
		Resources: collect.Resources{CPURequestPercent: valueStats(40, 50)},
	}

	findings := Workloads([]collect.Workload{critical, healthy}, defaultThresholds())

	if len(findings) != 2 {
		t.Fatalf("len(findings) = %d, want 2: %+v", len(findings), findings)
	}

	var deployment, statefulSet *Finding
	for i := range findings {
		switch findings[i].ScopeKind {
		case "Deployment":
			deployment = &findings[i]
		case "StatefulSet":
			statefulSet = &findings[i]
		}
	}
	if deployment == nil || deployment.Code != "WORKLOAD_CPU_REQUEST_CRITICAL" {
		t.Errorf("Deployment finding = %+v, want WORKLOAD_CPU_REQUEST_CRITICAL", deployment)
	}
	if statefulSet == nil || statefulSet.Code != "WORKLOAD_CPU_REQUEST_HEALTHY" {
		t.Errorf("StatefulSet finding = %+v, want WORKLOAD_CPU_REQUEST_HEALTHY", statefulSet)
	}
	if deployment != nil && statefulSet != nil && deployment.ScopeName != statefulSet.ScopeName {
		t.Errorf("ScopeName differs: %q vs %q", deployment.ScopeName, statefulSet.ScopeName)
	}
}

// TestSortOrder covers findings of three severities, two scopes, and two
// kinds: Sort orders by severity from the highest, then scope name, then
// scope kind, then code.
func TestSortOrder(t *testing.T) {
	t.Parallel()
	findings := []Finding{
		{Code: "B_CODE", Severity: SeverityHealthy, ScopeName: "b", ScopeKind: "Deployment"},
		{Code: "A_CODE", Severity: SeverityCritical, ScopeName: "a", ScopeKind: "StatefulSet"},
		{Code: "A_CODE_2", Severity: SeverityCritical, ScopeName: "a", ScopeKind: "Deployment"},
		{Code: "A_CODE", Severity: SeverityAttention, ScopeName: "a", ScopeKind: "Deployment"},
		{Code: "B_CODE", Severity: SeverityCritical, ScopeName: "b", ScopeKind: "Deployment"},
	}

	Sort(findings)

	want := []string{
		"critical/a/Deployment/A_CODE_2",
		"critical/a/StatefulSet/A_CODE",
		"critical/b/Deployment/B_CODE",
		"attention/a/Deployment/A_CODE",
		"healthy/b/Deployment/B_CODE",
	}
	for i, w := range want {
		got := findings[i].Severity + "/" + findings[i].ScopeName + "/" + findings[i].ScopeKind + "/" + findings[i].Code
		if got != w {
			t.Errorf("findings[%d] = %q, want %q", i, got, w)
		}
	}
}

// TestFindingsOfDirectComplete transcribes the numbers
// TestNamespaceNumbersOfTheScenario of internal/collect/namespaces_test.go
// records for the namespace "payments" of the "direct-complete" scenario;
// its single workload, "payments-web", consumes the whole namespace, so
// it carries the same numbers, following the rule of the program applied
// to the answers of the scenario.
func TestFindingsOfDirectComplete(t *testing.T) {
	t.Parallel()
	resources := collect.Resources{
		CPUUsage:             valueStats(0.30000000000000004, 0.4),
		MemoryUsage:          valueStats(110.0, 120.0),
		CPURequests:          valueStats(0.5, 0.5),
		MemoryRequests:       valueStats(0.5, 0.5),
		CPULimits:            valueStats(1.0, 1.0),
		MemoryLimits:         valueStats(1.0, 1.0),
		CPURequestPercent:    valueStats(60.0, 80.0),
		MemoryRequestPercent: valueStats(22000.0, 24000.0),
		CPULimitPercent:      valueStats(30.0, 40.0),
		MemoryLimitPercent:   valueStats(11000.0, 12000.0),
	}

	namespaces := []collect.Namespace{{
		Name: "payments", RequestsLimitsState: discovery.StateAvailable, Resources: resources,
	}}
	workloads := []collect.Workload{{
		Namespace: "payments", Kind: "Deployment", Name: "payments-web",
		RequestsLimitsState: discovery.StateAvailable, Resources: resources,
	}}
	collected := collect.Collected{
		Namespaces: collect.Result[collect.Namespace]{Rows: namespaces},
		Workloads:  collect.Result[collect.Workload]{Rows: workloads},
	}

	findings := All(collected, defaultThresholds())

	want := []struct {
		code    string
		average float64
		peak    float64
	}{
		{"NAMESPACE_MEMORY_LIMIT_CRITICAL", 11000.0, 12000.0},
		{"NAMESPACE_MEMORY_REQUEST_CRITICAL", 22000.0, 24000.0},
		{"WORKLOAD_MEMORY_LIMIT_CRITICAL", 11000.0, 12000.0},
		{"WORKLOAD_MEMORY_REQUEST_CRITICAL", 22000.0, 24000.0},
		{"NAMESPACE_CPU_REQUEST_ATTENTION", 60.0, 80.0},
		{"WORKLOAD_CPU_REQUEST_ATTENTION", 60.0, 80.0},
		{"NAMESPACE_CPU_LIMIT_HEALTHY", 30.0, 40.0},
		{"WORKLOAD_CPU_LIMIT_HEALTHY", 30.0, 40.0},
	}
	if len(findings) != len(want) {
		t.Fatalf("len(findings) = %d, want %d: %+v", len(findings), len(want), findings)
	}
	for i, w := range want {
		got := findings[i]
		if got.Code != w.code {
			t.Errorf("findings[%d].Code = %q, want %q", i, got.Code, w.code)
		}
		if average, ok := findEvidence(got.Evidence, "average_percent"); !ok || average != w.average {
			t.Errorf("%s: average_percent = %v, %v, want %v, true", w.code, average, ok, w.average)
		}
		if peak, ok := findEvidence(got.Evidence, "peak_percent"); !ok || peak != w.peak {
			t.Errorf("%s: peak_percent = %v, %v, want %v, true", w.code, peak, ok, w.peak)
		}
	}
}

// healthyMissingOverBurstyResources gives a row all four of a healthy
// ratio, a missing configuration, an overrequested request, and a bursty
// usage, for both CPU and memory.
func healthyMissingOverBurstyResources() collect.Resources {
	unavailable := collect.Statistics{State: collect.StateUnavailable}
	usage := valueStats(1, 3)
	ratio := valueStats(10, 20)
	return collect.Resources{
		CPUUsage: usage, CPURequests: unavailable, CPULimits: unavailable,
		CPURequestPercent: ratio, CPULimitPercent: ratio,
		MemoryUsage: usage, MemoryRequests: unavailable, MemoryLimits: unavailable,
		MemoryRequestPercent: ratio, MemoryLimitPercent: ratio,
	}
}

// attentionResources gives a row a request and a limit ratio classified
// attention, for both CPU and memory, and nothing else.
func attentionResources() collect.Resources {
	value := valueStats(1, 1)
	unavailable := collect.Statistics{State: collect.StateUnavailable}
	ratio := valueStats(60, 80)
	return collect.Resources{
		CPUUsage: unavailable, CPURequests: value, CPULimits: value,
		CPURequestPercent: ratio, CPULimitPercent: ratio,
		MemoryUsage: unavailable, MemoryRequests: value, MemoryLimits: value,
		MemoryRequestPercent: ratio, MemoryLimitPercent: ratio,
	}
}

// criticalResources gives a row a request and a limit ratio classified
// critical, for both CPU and memory, and nothing else.
func criticalResources() collect.Resources {
	value := valueStats(1, 1)
	unavailable := collect.Statistics{State: collect.StateUnavailable}
	ratio := valueStats(90, 95)
	return collect.Resources{
		CPUUsage: unavailable, CPURequests: value, CPULimits: value,
		CPURequestPercent: ratio, CPULimitPercent: ratio,
		MemoryUsage: unavailable, MemoryRequests: value, MemoryLimits: value,
		MemoryRequestPercent: ratio, MemoryLimitPercent: ratio,
	}
}

// TestEveryFindingFollowsItsDefinition builds rows that give each of the
// 40 codes and checks that each finding agrees with its registry
// definition in severity, scope type, and evidence names.
func TestEveryFindingFollowsItsDefinition(t *testing.T) {
	t.Parallel()
	thresholds := defaultThresholds()

	namespaces := []collect.Namespace{
		{Name: "ns-healthy", RequestsLimitsState: discovery.StateAvailable, Resources: healthyMissingOverBurstyResources()},
		{Name: "ns-attention", RequestsLimitsState: discovery.StateAvailable, Resources: attentionResources()},
		{Name: "ns-critical", RequestsLimitsState: discovery.StateAvailable, Resources: criticalResources()},
	}
	workloads := []collect.Workload{
		{Namespace: "wl", Kind: "Deployment", Name: "healthy", RequestsLimitsState: discovery.StateAvailable, Resources: healthyMissingOverBurstyResources()},
		{Namespace: "wl", Kind: "Deployment", Name: "attention", RequestsLimitsState: discovery.StateAvailable, Resources: attentionResources()},
		{Namespace: "wl", Kind: "Deployment", Name: "critical", RequestsLimitsState: discovery.StateAvailable, Resources: criticalResources()},
	}

	findings := append(Namespaces(namespaces, thresholds), Workloads(workloads, thresholds)...)

	for _, definition := range Registry {
		var matches []Finding
		for _, f := range findings {
			if f.Code == definition.Code {
				matches = append(matches, f)
			}
		}
		if len(matches) != 1 {
			t.Errorf("%s: found %d findings, want 1", definition.Code, len(matches))
			continue
		}
		got := matches[0]
		if !slices.Contains(definition.Severities, got.Severity) {
			t.Errorf("%s: Severity = %q, want one of %v", definition.Code, got.Severity, definition.Severities)
		}
		if got.ScopeType != definition.ScopeType {
			t.Errorf("%s: ScopeType = %q, want %q", definition.Code, got.ScopeType, definition.ScopeType)
		}
		if len(got.Evidence) != len(definition.Evidence) {
			t.Errorf("%s: Evidence = %+v, want specs %v", definition.Code, got.Evidence, definition.Evidence)
			continue
		}
		for i, spec := range definition.Evidence {
			if got.Evidence[i].Name != spec.Name {
				t.Errorf("%s: Evidence[%d].Name = %q, want %q", definition.Code, i, got.Evidence[i].Name, spec.Name)
			}
			if got.Evidence[i].Value.Kind != spec.Kind {
				t.Errorf("%s: Evidence[%d].Kind = %q, want %q", definition.Code, i, got.Evidence[i].Value.Kind, spec.Kind)
			}
		}
	}
}
