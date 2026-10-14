package analysis

import (
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
)

func poolOf(peaks ...float64) ([]collect.Pool, []collect.Node) {
	nodes := []collect.Node{}
	for i, peak := range peaks {
		n := testNode(string(rune('a' + i)))
		n.CPUUsagePercent = valueStats(peak, peak)
		nodes = append(nodes, n)
	}
	return collect.Pools(nodes), nodes
}

func TestNodepoolUneven(t *testing.T) {
	t.Parallel()
	pools, nodes := poolOf(20, 60)

	f := only(t, Pools(pools, nodes, defaultThresholds()))

	if f.Code != "NODEPOOL_CPU_UNEVEN" || f.ScopeType != ScopeNodepool || f.ScopeName != "p" || f.Confidence != discovery.ConfidenceExact {
		t.Errorf("finding = %+v", f)
	}
	got := numberEvidence(t, f)
	if got["minimum_peak_percent"] != 20 || got["maximum_peak_percent"] != 60 || got["spread_percent"] != 40 {
		t.Errorf("evidence = %v", got)
	}

	pools, nodes = poolOf(20, 49)
	if got := Pools(pools, nodes, defaultThresholds()); len(got) != 0 {
		t.Errorf("spread 29: findings = %+v, want none", got)
	}
	pools, nodes = poolOf(20, 50)
	if got := Pools(pools, nodes, defaultThresholds()); len(got) != 1 {
		t.Errorf("spread 30 is inclusive: findings = %+v, want one", got)
	}
	pools, nodes = poolOf(20)
	if got := Pools(pools, nodes, defaultThresholds()); len(got) != 0 {
		t.Errorf("one node: findings = %+v, want none", got)
	}
}

func TestNodepoolUnevenMemoryAndConfidence(t *testing.T) {
	t.Parallel()
	a, b := testNode("a"), testNode("b")
	a.Pool, a.PoolConfidence = "unknown", discovery.ConfidenceInferred
	b.Pool, b.PoolConfidence = "unknown", discovery.ConfidenceExact
	a.MemoryUsagePercent = valueStats(10, 10)
	b.MemoryUsagePercent = valueStats(10, 80)
	nodes := []collect.Node{a, b}

	f := only(t, Pools(collect.Pools(nodes), nodes, defaultThresholds()))

	if f.Code != "NODEPOOL_MEMORY_UNEVEN" || f.ScopeName != "unknown" || f.Confidence != discovery.ConfidenceInferred {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["minimum_peak_percent"] != 10 || got["maximum_peak_percent"] != 80 || got["spread_percent"] != 70 {
		t.Errorf("evidence = %v", got)
	}
}
