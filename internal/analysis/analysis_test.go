package analysis

import (
	"slices"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
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
// code, then scope kind.
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
		"critical/a/StatefulSet/A_CODE",
		"critical/a/Deployment/A_CODE_2",
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

	findings := slices.DeleteFunc(All(collected, defaultThresholds()), func(f Finding) bool { return f.ScopeType == ScopeCluster })

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

// clusterFindings builds inputs that give each of the 26 codes of HPAs,
// nodes, pools, and autoscaling, and returns the findings of all of them.
func clusterFindings() []Finding {
	thresholds := defaultThresholds()
	node := func(name, pool, confidence string, cpu, memory collect.Statistics) collect.Node {
		return collect.Node{Name: name, Pool: pool, PoolConfidence: confidence, CPUUsagePercent: cpu, MemoryUsagePercent: memory}
	}
	n1 := node("n1", "p1", discovery.ConfidenceExact, valueStats(50, 50), valueStats(75, 75))
	n2 := node("n2", "p1", discovery.ConfidenceExact, valueStats(75, 75), valueStats(90, 90))
	n3 := node("n3", "p1", discovery.ConfidenceExact, valueStats(90, 90), valueStats(50, 50))
	n4 := node("n4", "p2", discovery.ConfidenceExact, valueStats(20, 20), valueStats(25, 25))
	n4.CPURequestPercent = known(90)
	n4.MemoryRequestPercent = known(72)
	n5 := node("n5", "unknown", discovery.ConfidenceUnknown, collect.Statistics{}, collect.Statistics{})
	n5.Ready = collect.Flag{Known: true}
	n5.Schedulable = collect.Flag{Known: true}
	nodes := []collect.Node{n1, n2, n3, n4, n5}

	hpas := []collect.HPA{
		{Namespace: "a", Name: "pinned", SpecMin: known(3), SpecMax: known(3), Current: valueStats(3, 3)},
		{Namespace: "a", Name: "max", SpecMin: known(1), SpecMax: known(4), Current: valueStats(2, 4)},
		{Namespace: "a", Name: "near", SpecMin: known(1), SpecMax: known(5), Current: valueStats(4, 4), Desired: valueStats(2, 2)},
	}
	uncovered := []collect.WorkloadRef{
		{Namespace: "a", Kind: "Deployment", Name: "d"},
		{Namespace: "a", Kind: "StatefulSet", Name: "s"},
	}

	findings := HPAs(hpas, uncovered, thresholds)
	nodeFindings := Nodes(nodes, thresholds)
	poolFindings := Pools(collect.Pools(nodes), nodes, thresholds)
	findings = append(findings, nodeFindings...)
	findings = append(findings, poolFindings...)

	none := collect.Number{}
	for _, e := range []struct {
		evidence collect.AutoscalingEvidence
		pressure bool
	}{
		{evidenceOf("unknown", discovery.ConfidenceUnknown, none, none, none), false},
		{evidenceOf("disabled", discovery.ConfidenceExact, none, none, none), false},
		{evidenceOf("disabled", discovery.ConfidenceExact, none, none, none), true},
		{evidenceOf("enabled", discovery.ConfidenceInferred, known(4), known(1), known(5)), false},
		{evidenceOf("enabled", discovery.ConfidenceExact, none, none, none), false},
		{evidenceOf("enabled", discovery.ConfidenceInferred, known(2), known(2), known(10)), false},
		{evidenceOf("enabled", discovery.ConfidenceInferred, known(3), known(2), known(10)), false},
	} {
		findings = append(findings, Autoscaling(e.evidence, thresholds, e.pressure)...)
	}
	return findings
}

// matchesSpecs reports whether the evidence of a finding follows specs: the
// same names in order, where a spec marked optional may be left out.
func matchesSpecs(evidence []Evidence, specs []EvidenceSpec) bool {
	i := 0
	for _, spec := range specs {
		if i < len(evidence) && evidence[i].Name == spec.Name {
			if evidence[i].Value.Kind != spec.Kind {
				return false
			}
			i++
		} else if !spec.Optional {
			return false
		}
	}
	return i == len(evidence)
}

// TestEveryFindingFollowsItsDefinition builds rows that give each of the
// 68 codes and checks that each finding agrees with its registry
// definition in severity, scope type, and evidence names and kinds. A
// code of nodes may come from more than one node.
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
	findings = append(findings, clusterFindings()...)

	for _, definition := range Registry {
		var matches []Finding
		for _, f := range findings {
			if f.Code == definition.Code {
				matches = append(matches, f)
			}
		}
		if len(matches) == 0 {
			t.Errorf("%s: no finding", definition.Code)
		}
		if len(matches) != 1 && !strings.HasPrefix(definition.Code, "NODE_") {
			t.Errorf("%s: found %d findings, want 1", definition.Code, len(matches))
		}
		for _, got := range matches {
			if !slices.Contains(definition.Severities, got.Severity) {
				t.Errorf("%s: Severity = %q, want one of %v", definition.Code, got.Severity, definition.Severities)
			}
			if got.ScopeType != definition.ScopeType {
				t.Errorf("%s: ScopeType = %q, want %q", definition.Code, got.ScopeType, definition.ScopeType)
			}
			if !matchesSpecs(got.Evidence, definition.Evidence) {
				t.Errorf("%s: Evidence = %+v, want specs %v", definition.Code, got.Evidence, definition.Evidence)
			}
		}
	}
}

// TestNodeRequestHighSeverities covers both severities the two request
// high codes may carry.
func TestNodeRequestHighSeverities(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		percent  float64
		severity string
	}{{72, SeverityAttention}, {90, SeverityCritical}} {
		n := testNode("n1")
		n.MemoryRequestPercent = known(c.percent)
		f := only(t, byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_MEMORY_REQUEST_HIGH"))
		if f.Severity != c.severity {
			t.Errorf("percent %v: Severity = %q, want %q", c.percent, f.Severity, c.severity)
		}
	}
	n := testNode("n1")
	n.CPURequestPercent = known(69)
	if got := Nodes([]collect.Node{n}, defaultThresholds()); len(got) != 0 {
		t.Errorf("request 69: findings = %+v, want none", got)
	}
}

// TestAllOrdersSixGroups gives one finding of each group and checks that
// All sorts them together, and that the node pressure is worked out from
// the node and pool findings only.
func TestAllOrdersSixGroups(t *testing.T) {
	t.Parallel()
	collected := collect.Collected{
		Namespaces: collect.Result[collect.Namespace]{Rows: []collect.Namespace{
			{Name: "ns", Resources: collect.Resources{CPURequestPercent: valueStats(90, 95)}},
		}},
		Workloads: collect.Result[collect.Workload]{Rows: []collect.Workload{
			{Namespace: "w", Kind: "Deployment", Name: "x", Resources: collect.Resources{MemoryRequestPercent: valueStats(40, 40)}},
		}},
		HPAs: collect.Result[collect.HPA]{Rows: []collect.HPA{
			{Namespace: "a", Name: "h", SpecMin: known(1), SpecMax: known(4), Current: valueStats(2, 4)},
		}},
		Nodes: collect.Result[collect.Node]{Rows: []collect.Node{
			{Name: "n1", Pool: "p", PoolConfidence: discovery.ConfidenceExact, CPUUsagePercent: valueStats(50, 50)},
		}},
		Autoscaling: evidenceOf("disabled", discovery.ConfidenceExact, collect.Number{}, collect.Number{}, collect.Number{}),
	}

	got := codesOf(All(collected, defaultThresholds()))

	want := []string{"HPA_MAX_REPLICAS_REACHED", "NAMESPACE_CPU_REQUEST_CRITICAL", "NODE_AUTOSCALING_DISABLED", "NODE_CPU_USAGE_HEALTHY", "WORKLOAD_MEMORY_REQUEST_HEALTHY"}
	if !slices.Equal(got, want) {
		t.Errorf("without pressure: codes = %v, want %v", got, want)
	}

	collected.Nodes.Rows = append(collected.Nodes.Rows,
		collect.Node{Name: "n2", Pool: "p", PoolConfidence: discovery.ConfidenceExact, CPUUsagePercent: valueStats(10, 10)})
	got = codesOf(All(collected, defaultThresholds()))
	if !slices.Contains(got, "NODE_AUTOSCALING_DISABLED_PRESSURE") || slices.Contains(got, "NODE_AUTOSCALING_DISABLED") {
		t.Errorf("with pressure: codes = %v, want DISABLED_PRESSURE", got)
	}
	if got[0] != "HPA_MAX_REPLICAS_REACHED" || got[1] != "NODE_AUTOSCALING_DISABLED_PRESSURE" || got[2] != "NAMESPACE_CPU_REQUEST_CRITICAL" {
		t.Errorf("with pressure: critical findings first, sorted by scope: %v", got)
	}
}

// TestSortRanksTheCodeBeforeTheKind covers two workloads of different kinds
// that share a name: the code decides before the kind does.
func TestSortRanksTheCodeBeforeTheKind(t *testing.T) {
	t.Parallel()
	findings := []Finding{
		{Code: "WORKLOAD_MEMORY_REQUEST_ATTENTION", Severity: SeverityAttention, ScopeName: "ns/web", ScopeKind: "Deployment"},
		{Code: "WORKLOAD_CPU_REQUEST_ATTENTION", Severity: SeverityAttention, ScopeName: "ns/web", ScopeKind: "StatefulSet"},
	}

	Sort(findings)

	if got, want := findings[0].Code, "WORKLOAD_CPU_REQUEST_ATTENTION"; got != want {
		t.Errorf("findings[0].Code = %q, want %q", got, want)
	}
}

// TestReadinessFindingsAreNotNodePressure covers a cordoned node and a node
// that is not ready: the autoscaling finding is the one of a cluster
// without pressure.
func TestReadinessFindingsAreNotNodePressure(t *testing.T) {
	t.Parallel()
	yes, no := collect.Flag{Value: true, Known: true}, collect.Flag{Value: false, Known: true}
	disabled := evidenceOf("disabled", discovery.ConfidenceExact, collect.Number{}, collect.Number{}, collect.Number{})
	atMin := evidenceOf("enabled", discovery.ConfidenceInferred, known(2), known(2), known(10))
	cases := []struct {
		name        string
		evidence    collect.AutoscalingEvidence
		ready       collect.Flag
		schedulable collect.Flag
		code        string
		severity    string
	}{
		{"disabled, cordoned", disabled, yes, no, "NODE_AUTOSCALING_DISABLED", SeverityAttention},
		{"disabled, not ready", disabled, no, yes, "NODE_AUTOSCALING_DISABLED", SeverityAttention},
		{"at minimum, cordoned", atMin, yes, no, "NODE_AUTOSCALING_AT_MIN", SeverityHealthy},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			collected := collect.Collected{
				Nodes: collect.Result[collect.Node]{Rows: []collect.Node{
					{Name: "n1", Pool: "p", PoolConfidence: discovery.ConfidenceExact, CPUUsagePercent: valueStats(50, 50), Ready: c.ready, Schedulable: c.schedulable},
				}},
				Autoscaling: c.evidence,
			}
			var cluster []Finding
			readiness := 0
			for _, f := range All(collected, defaultThresholds()) {
				if f.ScopeType == ScopeCluster {
					cluster = append(cluster, f)
				}
				if f.Code == "NODE_NOT_READY" || f.Code == "NODE_UNSCHEDULABLE" {
					readiness++
				}
			}
			if readiness != 1 {
				t.Fatalf("readiness findings = %d, want 1", readiness)
			}
			f := only(t, cluster)
			if f.Code != c.code || f.Severity != c.severity {
				t.Errorf("cluster finding = %s %s, want %s %s", f.Code, f.Severity, c.code, c.severity)
			}
			if c.code == "NODE_AUTOSCALING_DISABLED" && (len(f.Evidence) != 1 || f.Evidence[0].Name != "node_pressure" || f.Evidence[0].Value != Boolean(false)) {
				t.Errorf("evidence = %+v, want node_pressure false", f.Evidence)
			}
		})
	}
}
