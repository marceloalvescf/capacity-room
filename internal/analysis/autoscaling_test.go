package analysis

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

func evidenceOf(status, confidence string, current, lowest, highest collect.Number) collect.AutoscalingEvidence {
	return collect.AutoscalingEvidence{Status: status, Confidence: confidence, CurrentNodes: current, MinNodes: lowest, MaxNodes: highest}
}

func TestNodePressure(t *testing.T) {
	t.Parallel()
	low := []Finding{{Code: "NODE_LOW_UTILIZATION", Severity: SeverityAttention, ScopeType: ScopeNode}}
	healthy := []Finding{{Code: "NODE_CPU_USAGE_HEALTHY", Severity: SeverityHealthy, ScopeType: ScopeNode}}
	pool := []Finding{{Code: "NODEPOOL_CPU_UNEVEN", Severity: SeverityAttention, ScopeType: ScopeNodepool}}

	if !NodePressure(low, nil) {
		t.Error("a low utilization finding is pressure")
	}
	if NodePressure(healthy, nil) {
		t.Error("a healthy usage finding is not pressure")
	}
	if !NodePressure(healthy, pool) {
		t.Error("a pool finding at attention is pressure")
	}
	readiness := []Finding{
		{Code: "NODE_NOT_READY", Severity: SeverityAttention, ScopeType: ScopeNode},
		{Code: "NODE_UNSCHEDULABLE", Severity: SeverityAttention, ScopeType: ScopeNode},
	}
	if NodePressure(readiness, nil) {
		t.Error("readiness findings are not pressure")
	}
}

func TestAutoscalingChain(t *testing.T) {
	t.Parallel()
	none := collect.Number{}
	cases := []struct {
		name       string
		evidence   collect.AutoscalingEvidence
		pressure   bool
		code       string
		severity   string
		confidence string
		boolean    *bool
		numbers    map[string]float64
	}{
		{"unknown", evidenceOf("unknown", discovery.ConfidenceUnknown, none, none, none), false, "NODE_AUTOSCALING_UNKNOWN", SeverityAttention, discovery.ConfidenceUnknown, nil, nil},
		{"disabled", evidenceOf("disabled", discovery.ConfidenceExact, none, none, none), false, "NODE_AUTOSCALING_DISABLED", SeverityAttention, discovery.ConfidenceExact, ptr(false), nil},
		{"disabled under pressure", evidenceOf("disabled", discovery.ConfidenceExact, none, none, none), true, "NODE_AUTOSCALING_DISABLED_PRESSURE", SeverityCritical, discovery.ConfidenceExact, ptr(true), nil},
		{"near max", evidenceOf("enabled", discovery.ConfidenceInferred, known(4), known(1), known(5)), false, "NODE_AUTOSCALING_NEAR_MAX", SeverityAttention, discovery.ConfidenceInferred, nil, map[string]float64{"current_nodes": 4, "max_nodes": 5}},
		{"bounds unknown", evidenceOf("enabled", discovery.ConfidenceExact, none, none, none), false, "NODE_AUTOSCALING_BOUNDS_UNKNOWN", SeverityHealthy, discovery.ConfidenceExact, nil, nil},
		{"at min", evidenceOf("enabled", discovery.ConfidenceInferred, known(2), known(2), known(10)), false, "NODE_AUTOSCALING_AT_MIN", SeverityHealthy, discovery.ConfidenceInferred, nil, map[string]float64{"min_nodes": 2}},
		{"at min under pressure", evidenceOf("enabled", discovery.ConfidenceInferred, known(2), known(2), known(10)), true, "NODE_AUTOSCALING_ENABLED", SeverityHealthy, discovery.ConfidenceInferred, nil, nil},
		{"enabled", evidenceOf("enabled", discovery.ConfidenceInferred, known(3), known(2), known(10)), false, "NODE_AUTOSCALING_ENABLED", SeverityHealthy, discovery.ConfidenceInferred, nil, nil},
		{"near max under pressure", evidenceOf("enabled", discovery.ConfidenceInferred, known(4), known(1), known(5)), true, "NODE_AUTOSCALING_NEAR_MAX", SeverityAttention, discovery.ConfidenceInferred, nil, map[string]float64{"current_nodes": 4, "max_nodes": 5}},
	}
	for _, c := range cases {
		f := only(t, Autoscaling(c.evidence, defaultThresholds(), c.pressure))
		if f.Code != c.code || f.Severity != c.severity || f.Confidence != c.confidence || f.ScopeType != ScopeCluster || f.ScopeName != "cluster" || f.ScopeKind != "" {
			t.Errorf("%s: finding = %+v", c.name, f)
		}
		if c.boolean != nil {
			if len(f.Evidence) != 1 || f.Evidence[0].Name != "node_pressure" || f.Evidence[0].Value != Boolean(*c.boolean) {
				t.Errorf("%s: evidence = %+v", c.name, f.Evidence)
			}
			continue
		}
		got := numberEvidence(t, f)
		if len(got) != len(c.numbers) {
			t.Errorf("%s: evidence = %v, want %v", c.name, got, c.numbers)
		}
		for name, v := range c.numbers {
			if got[name] != v {
				t.Errorf("%s: %s = %v, want %v", c.name, name, got[name], v)
			}
		}
	}
}

func ptr(b bool) *bool { return &b }

func TestAutoscalingNearMaxIsIntegerDivision(t *testing.T) {
	t.Parallel()
	e := evidenceOf("enabled", discovery.ConfidenceInferred, known(3), known(1), known(3))

	f := only(t, Autoscaling(e, defaultThresholds(), false))

	if f.Code != "NODE_AUTOSCALING_NEAR_MAX" {
		t.Errorf("Code = %q, want NODE_AUTOSCALING_NEAR_MAX", f.Code)
	}
	if got := numberEvidence(t, f); got["current_nodes"] != 3 || got["max_nodes"] != 3 {
		t.Errorf("evidence = %v", got)
	}
}

func TestAutoscalingWithoutStatusGivesNothing(t *testing.T) {
	t.Parallel()
	if got := Autoscaling(collect.AutoscalingEvidence{}, defaultThresholds(), false); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}
