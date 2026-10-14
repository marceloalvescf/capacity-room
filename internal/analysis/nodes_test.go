package analysis

import (
	"slices"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
)

func TestNodeUsageTakesTheHigher(t *testing.T) {
	t.Parallel()
	n := testNode("n1")
	n.CPUUsagePercent = valueStats(60, 86)

	f := only(t, byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_CPU_USAGE_CRITICAL"))

	if f.Severity != SeverityCritical || f.ScopeType != ScopeNode || f.ScopeName != "n1" || f.Confidence != discovery.ConfidenceExact {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["average_percent"] != 60 || got["peak_percent"] != 86 || len(got) != 2 {
		t.Errorf("evidence = %v, want average 60 and peak 86", got)
	}
	if !slices.Equal(evidenceList(f), []string{"average_percent", "peak_percent"}) {
		t.Errorf("evidence order = %v", evidenceList(f))
	}
}

func TestNodeUsageWithOneNumber(t *testing.T) {
	t.Parallel()
	f, ok := usageFinding("CPU", "n1", collect.Number{}, known(50), defaultThresholds())
	if !ok {
		t.Fatal("no finding for a known peak")
	}
	if f.Code != "NODE_CPU_USAGE_HEALTHY" {
		t.Errorf("Code = %q, want NODE_CPU_USAGE_HEALTHY", f.Code)
	}
	if !slices.Equal(evidenceList(f), []string{"peak_percent"}) {
		t.Fatalf("evidence = %v, want peak_percent only", evidenceList(f))
	}
	if got := numberEvidence(t, f)["peak_percent"]; got != 50 {
		t.Errorf("peak_percent = %v, want 50", got)
	}
	if _, ok := usageFinding("CPU", "n1", collect.Number{}, collect.Number{}, defaultThresholds()); ok {
		t.Error("a finding was made with no known number")
	}
}

func TestNodeRequestHighReadsTheAverage(t *testing.T) {
	t.Parallel()
	n := testNode("n1")
	n.CPURequestPercent = known(90)

	f := only(t, byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_CPU_REQUEST_HIGH"))

	if f.Severity != SeverityCritical {
		t.Errorf("Severity = %q, want critical", f.Severity)
	}
	if got := numberEvidence(t, f); got["percent"] != 90 || len(got) != 1 {
		t.Errorf("evidence = %v, want percent 90", got)
	}
}

func TestNodeOverrequested(t *testing.T) {
	t.Parallel()
	n := testNode("n1")
	n.CPURequestPercent = known(90)
	n.CPUUsagePercent = valueStats(20, 25)
	n.MemoryRequestPercent = known(72)
	n.MemoryUsagePercent = valueStats(25, 30)

	f := only(t, byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_OVERREQUESTED"))

	if f.Severity != SeverityAttention || f.Confidence != discovery.ConfidenceInferred {
		t.Errorf("finding = %+v", f)
	}
	if len(f.Evidence) != 1 || f.Evidence[0].Name != "resources" || f.Evidence[0].Value != Text("cpu,memory") {
		t.Errorf("evidence = %+v, want resources cpu,memory", f.Evidence)
	}
}

func TestNodeOverrequestedNeedsBoth(t *testing.T) {
	t.Parallel()
	n := testNode("n1")
	n.CPURequestPercent = known(90)
	n.CPUUsagePercent = valueStats(31, 40)

	if got := byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_OVERREQUESTED"); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}

	n.CPURequestPercent = known(50)
	n.CPUUsagePercent = valueStats(20, 25)
	if got := byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_OVERREQUESTED"); len(got) != 0 {
		t.Errorf("request below attention: findings = %+v, want none", got)
	}
}

func TestNodeLowUtilization(t *testing.T) {
	t.Parallel()
	n := testNode("n1")
	n.CPUUsagePercent = valueStats(20, 25)
	n.MemoryUsagePercent = valueStats(25, 30)

	f := only(t, byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_LOW_UTILIZATION"))
	if got := numberEvidence(t, f); got["maximum_average_percent"] != 25 || len(got) != 1 {
		t.Errorf("evidence = %v, want maximum_average_percent 25", got)
	}

	n.MemoryUsagePercent = valueStats(31, 40)
	if got := byCode(Nodes([]collect.Node{n}, defaultThresholds()), "NODE_LOW_UTILIZATION"); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestNodeEmissionOrder(t *testing.T) {
	t.Parallel()
	a, b := testNode("a"), testNode("b")
	for _, n := range []*collect.Node{&a, &b} {
		n.CPUUsagePercent = valueStats(20, 25)
		n.MemoryUsagePercent = valueStats(20, 25)
		n.CPURequestPercent = known(90)
		n.MemoryRequestPercent = known(90)
	}

	got := codesOf(Nodes([]collect.Node{a, b}, defaultThresholds()))

	one := []string{"NODE_CPU_USAGE_HEALTHY", "NODE_MEMORY_USAGE_HEALTHY", "NODE_CPU_REQUEST_HIGH", "NODE_MEMORY_REQUEST_HIGH", "NODE_OVERREQUESTED", "NODE_LOW_UTILIZATION"}
	if want := append(slices.Clone(one), one...); !slices.Equal(got, want) {
		t.Errorf("codes = %v, want %v", got, want)
	}
}

func TestNodepoolUnknownCountsOnce(t *testing.T) {
	t.Parallel()
	a, b, c := testNode("a"), testNode("b"), testNode("c")
	a.Pool, a.PoolConfidence = "unknown", discovery.ConfidenceUnknown
	b.Pool, b.PoolConfidence = "unknown", discovery.ConfidenceUnknown
	c.Pool = "pool-x"

	f := only(t, byCode(Nodes([]collect.Node{a, b, c}, defaultThresholds()), "NODEPOOL_UNKNOWN"))

	if f.ScopeType != ScopeNodepool || f.ScopeName != "unknown" || f.Confidence != discovery.ConfidenceUnknown || f.Severity != SeverityAttention {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["affected_nodes"] != 2 {
		t.Errorf("evidence = %v, want affected_nodes 2", got)
	}
}

// TestNodeReadinessFindings pins the two readiness findings: a known false
// readiness or schedulability is a finding of attention, a known true one
// is none, and an unknown one is none.
func TestNodeReadinessFindings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		ready       collect.Flag
		schedulable collect.Flag
		want        []string
	}{
		{"ready and schedulable", collect.Flag{Value: true, Known: true}, collect.Flag{Value: true, Known: true}, nil},
		{"not ready", collect.Flag{Value: false, Known: true}, collect.Flag{Value: true, Known: true}, []string{"NODE_NOT_READY"}},
		{"cordoned", collect.Flag{Value: true, Known: true}, collect.Flag{Value: false, Known: true}, []string{"NODE_UNSCHEDULABLE"}},
		{"not ready and cordoned", collect.Flag{Value: false, Known: true}, collect.Flag{Value: false, Known: true}, []string{"NODE_NOT_READY", "NODE_UNSCHEDULABLE"}},
		{"unknown", collect.Flag{}, collect.Flag{}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			node := testNode("n1")
			node.Ready = c.ready
			node.Schedulable = c.schedulable
			var got []string
			for _, f := range Nodes([]collect.Node{node}, defaultThresholds()) {
				if f.Code != "NODE_NOT_READY" && f.Code != "NODE_UNSCHEDULABLE" {
					continue
				}
				got = append(got, f.Code)
				if f.Severity != SeverityAttention || f.ScopeType != ScopeNode || f.ScopeName != "n1" ||
					f.Confidence != discovery.ConfidenceExact || len(f.Evidence) != 0 {
					t.Errorf("finding %+v, want attention, node n1, exact, no evidence", f)
				}
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("codes = %v, want %v", got, c.want)
			}
		})
	}
}
