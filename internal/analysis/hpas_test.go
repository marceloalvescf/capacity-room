package analysis

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

func testHPA(specMin, specMax collect.Number, current, desired collect.Statistics) collect.HPA {
	return collect.HPA{Namespace: "payments", Name: "web", SpecMin: specMin, SpecMax: specMax, Current: current, Desired: desired}
}

func currentOf(average, peak float64) collect.Statistics { return valueStats(average, peak) }

func TestHPAPinnedWinsOverMax(t *testing.T) {
	t.Parallel()
	h := testHPA(known(3), known(3), currentOf(3, 3), collect.Statistics{})

	f := only(t, HPAs([]collect.HPA{h}, nil, defaultThresholds()))

	if f.Code != "HPA_PINNED_REPLICAS" || f.Severity != SeverityHealthy || f.ScopeType != ScopeHPA || f.ScopeName != "payments/web" || f.Confidence != discovery.ConfidenceExact {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["spec_min"] != 3 || got["spec_max"] != 3 {
		t.Errorf("evidence = %v", got)
	}
}

func TestHPAMaxReached(t *testing.T) {
	t.Parallel()
	h := testHPA(known(1), known(4), currentOf(2, 4), collect.Statistics{})

	f := only(t, HPAs([]collect.HPA{h}, nil, defaultThresholds()))

	if f.Code != "HPA_MAX_REPLICAS_REACHED" || f.Severity != SeverityCritical {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["current_peak"] != 4 || got["spec_max"] != 4 {
		t.Errorf("evidence = %v", got)
	}
	if !equalStrings(evidenceList(f), []string{"current_peak", "spec_max"}) {
		t.Errorf("order = %v", evidenceList(f))
	}
}

func TestHPAMaxReachedAtZero(t *testing.T) {
	t.Parallel()
	h := testHPA(known(0), known(0), currentOf(0, 0), collect.Statistics{})
	h.SpecMin = collect.Number{}

	f := only(t, HPAs([]collect.HPA{h}, nil, defaultThresholds()))

	if f.Code != "HPA_MAX_REPLICAS_REACHED" || f.Severity != SeverityCritical {
		t.Errorf("finding = %+v", f)
	}
}

func TestHPANearMax(t *testing.T) {
	t.Parallel()
	h := testHPA(known(1), known(5), currentOf(4, 4), collect.Statistics{})

	f := only(t, HPAs([]collect.HPA{h}, nil, defaultThresholds()))

	if f.Code != "HPA_NEAR_MAX_REPLICAS" || f.Severity != SeverityAttention {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["current_average"] != 4 || got["spec_max"] != 5 {
		t.Errorf("evidence = %v", got)
	}

	h = testHPA(known(0), known(0), currentOf(4, -1), collect.Statistics{})
	h.SpecMin = collect.Number{}
	if got := byCode(HPAs([]collect.HPA{h}, nil, defaultThresholds()), "HPA_NEAR_MAX_REPLICAS"); len(got) != 0 {
		t.Errorf("max 0: findings = %+v, want none", got)
	}
}

func TestHPADivergence(t *testing.T) {
	t.Parallel()
	h := testHPA(known(1), known(10), currentOf(1, 2), currentOf(2, 2))

	f := only(t, HPAs([]collect.HPA{h}, nil, defaultThresholds()))

	if f.Code != "HPA_REPLICA_DIVERGENCE" || f.Confidence != discovery.ConfidenceInferred || f.Severity != SeverityAttention {
		t.Errorf("finding = %+v", f)
	}
	if got := numberEvidence(t, f); got["average_difference"] != 1 {
		t.Errorf("evidence = %v", got)
	}

	h = testHPA(known(1), known(4), currentOf(1, 4), currentOf(2, 2))
	got := codesOf(HPAs([]collect.HPA{h}, nil, defaultThresholds()))
	if !equalStrings(got, []string{"HPA_MAX_REPLICAS_REACHED", "HPA_REPLICA_DIVERGENCE"}) {
		t.Errorf("codes = %v, want max reached then divergence", got)
	}
}

func TestHPAUnknownSpecSkips(t *testing.T) {
	t.Parallel()
	h := testHPA(known(1), collect.Number{}, currentOf(9, 9), collect.Statistics{})

	if got := HPAs([]collect.HPA{h}, nil, defaultThresholds()); len(got) != 0 {
		t.Errorf("findings = %+v, want none", got)
	}
}

func TestWorkloadsWithoutHPA(t *testing.T) {
	t.Parallel()
	uncovered := []collect.WorkloadRef{
		{Namespace: "ns", Kind: "Deployment", Name: "api"},
		{Namespace: "ns", Kind: "StatefulSet", Name: "queue"},
		{Namespace: "ns", Kind: "DaemonSet", Name: "d"},
	}

	got := HPAs(nil, uncovered, defaultThresholds())

	if len(got) != 2 {
		t.Fatalf("findings = %+v, want 2", got)
	}
	want := []struct{ code, severity, kind, name string }{
		{"DEPLOYMENT_WITHOUT_HPA", SeverityAttention, "Deployment", "ns/api"},
		{"STATEFULSET_WITHOUT_HPA_INFO", SeverityHealthy, "StatefulSet", "ns/queue"},
	}
	for i, w := range want {
		f := got[i]
		if f.Code != w.code || f.Severity != w.severity || f.ScopeKind != w.kind || f.ScopeName != w.name || f.ScopeType != ScopeWorkload || f.Confidence != discovery.ConfidenceExact || len(f.Evidence) != 0 {
			t.Errorf("findings[%d] = %+v, want %+v", i, f, w)
		}
	}
}
