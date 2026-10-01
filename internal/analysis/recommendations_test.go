package analysis

import (
	"reflect"
	"testing"
)

func finding(code, severity, scopeType, scopeName, confidence string) Finding {
	return Finding{Code: code, Severity: severity, ScopeType: scopeType, ScopeName: scopeName, Confidence: confidence}
}

func recommendationCodes(recs []Recommendation) []string {
	codes := make([]string, 0, len(recs))
	for _, r := range recs {
		codes = append(codes, r.Code)
	}
	return codes
}

func TestRecommendationsSkipHealthy(t *testing.T) {
	got := Recommendations([]Finding{
		finding("NODE_CPU_USAGE_HEALTHY", "healthy", "node", "node-1", "exact"),
		finding("NODE_AUTOSCALING_NEAR_MAX", "attention", "cluster", "cluster", "inferred"),
	})
	want := []Recommendation{{
		Code: "NODE_AUTOSCALING_NEAR_MAX", Severity: "attention", ScopeType: "cluster",
		Scopes: []string{"cluster"}, Confidence: "inferred",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRecommendationsMergeOneCode(t *testing.T) {
	const code = "WORKLOAD_CPU_REQUEST_CRITICAL"
	got := Recommendations([]Finding{
		finding(code, "attention", "workload", "payments/b", "exact"),
		finding(code, "critical", "workload", "payments/a", "inferred"),
		finding(code, "attention", "workload", "payments/b", "exact"),
	})
	want := []Recommendation{{
		Code: code, Severity: "critical", ScopeType: "workload",
		Scopes: []string{"payments/a", "payments/b"}, Confidence: "exact",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRecommendationsOrder(t *testing.T) {
	got := recommendationCodes(Recommendations([]Finding{
		finding("CRITICAL_INFERRED", "critical", "workload", "payments/a", "inferred"),
		finding("ATTENTION", "attention", "workload", "payments/a", "exact"),
		finding("CRITICAL_EXACT", "critical", "workload", "payments/z", "exact"),
	}))
	want := []string{"CRITICAL_EXACT", "CRITICAL_INFERRED", "ATTENTION"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRecommendationsOrderByFirstScopeThenCode(t *testing.T) {
	got := Recommendations([]Finding{
		finding("B", "attention", "namespace", "x", "exact"),
		finding("A", "attention", "namespace", "y", "exact"),
		finding("A", "attention", "namespace", "x", "exact"),
		finding("C", "attention", "namespace", "x", "exact"),
	})
	want := []Recommendation{
		{Code: "A", Severity: "attention", ScopeType: "namespace", Scopes: []string{"x", "y"}, Confidence: "exact"},
		{Code: "B", Severity: "attention", ScopeType: "namespace", Scopes: []string{"x"}, Confidence: "exact"},
		{Code: "C", Severity: "attention", ScopeType: "namespace", Scopes: []string{"x"}, Confidence: "exact"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRecommendationsOfDirectComplete(t *testing.T) {
	got := Recommendations([]Finding{
		finding("NAMESPACE_MEMORY_LIMIT_CRITICAL", "critical", "namespace", "payments", "exact"),
		finding("NAMESPACE_MEMORY_REQUEST_CRITICAL", "critical", "namespace", "payments", "exact"),
		finding("WORKLOAD_MEMORY_LIMIT_CRITICAL", "critical", "workload", "payments/payments-web", "exact"),
		finding("WORKLOAD_MEMORY_REQUEST_CRITICAL", "critical", "workload", "payments/payments-web", "exact"),
		finding("NODE_AUTOSCALING_NEAR_MAX", "attention", "cluster", "cluster", "inferred"),
		finding("NAMESPACE_CPU_REQUEST_ATTENTION", "attention", "namespace", "payments", "exact"),
		finding("HPA_REPLICA_DIVERGENCE", "attention", "hpa", "payments/payments-web", "inferred"),
		finding("WORKLOAD_CPU_REQUEST_ATTENTION", "attention", "workload", "payments/payments-web", "exact"),
		finding("NODE_CPU_USAGE_HEALTHY", "healthy", "node", "node-1", "exact"),
		finding("NODE_MEMORY_USAGE_HEALTHY", "healthy", "node", "node-1", "exact"),
		finding("NAMESPACE_CPU_LIMIT_HEALTHY", "healthy", "namespace", "payments", "exact"),
		finding("WORKLOAD_CPU_LIMIT_HEALTHY", "healthy", "workload", "payments/payments-web", "exact"),
		finding("STATEFULSET_WITHOUT_HPA_INFO", "healthy", "workload", "payments/payments-worker", "exact"),
	})
	want := []Recommendation{
		{Code: "NAMESPACE_MEMORY_LIMIT_CRITICAL", Severity: "critical", ScopeType: "namespace", Scopes: []string{"payments"}, Confidence: "exact"},
		{Code: "NAMESPACE_MEMORY_REQUEST_CRITICAL", Severity: "critical", ScopeType: "namespace", Scopes: []string{"payments"}, Confidence: "exact"},
		{Code: "WORKLOAD_MEMORY_LIMIT_CRITICAL", Severity: "critical", ScopeType: "workload", Scopes: []string{"payments/payments-web"}, Confidence: "exact"},
		{Code: "WORKLOAD_MEMORY_REQUEST_CRITICAL", Severity: "critical", ScopeType: "workload", Scopes: []string{"payments/payments-web"}, Confidence: "exact"},
		{Code: "NAMESPACE_CPU_REQUEST_ATTENTION", Severity: "attention", ScopeType: "namespace", Scopes: []string{"payments"}, Confidence: "exact"},
		{Code: "WORKLOAD_CPU_REQUEST_ATTENTION", Severity: "attention", ScopeType: "workload", Scopes: []string{"payments/payments-web"}, Confidence: "exact"},
		{Code: "NODE_AUTOSCALING_NEAR_MAX", Severity: "attention", ScopeType: "cluster", Scopes: []string{"cluster"}, Confidence: "inferred"},
		{Code: "HPA_REPLICA_DIVERGENCE", Severity: "attention", ScopeType: "hpa", Scopes: []string{"payments/payments-web"}, Confidence: "inferred"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestRecommendationsOfEdgeScalingPutConfidenceBeforeScope(t *testing.T) {
	got := recommendationCodes(Recommendations([]Finding{
		finding("NODE_CPU_USAGE_CRITICAL", "critical", "node", "aks-agentpool-12345678-vmss000000", "exact"),
		finding("NAMESPACE_CPU_LIMIT_CRITICAL", "critical", "namespace", "payments", "inferred"),
		finding("NAMESPACE_CPU_REQUEST_CRITICAL", "critical", "namespace", "payments", "inferred"),
		finding("NAMESPACE_MEMORY_LIMIT_CRITICAL", "critical", "namespace", "payments", "inferred"),
		finding("NAMESPACE_MEMORY_REQUEST_CRITICAL", "critical", "namespace", "payments", "inferred"),
		finding("WORKLOAD_MEMORY_LIMIT_CRITICAL", "critical", "workload", "payments/payments-web", "exact"),
		finding("WORKLOAD_MEMORY_REQUEST_CRITICAL", "critical", "workload", "payments/payments-web", "exact"),
		finding("HPA_MAX_REPLICAS_REACHED", "critical", "hpa", "payments/web", "exact"),
		finding("NODE_CPU_REQUEST_HIGH", "critical", "node", "pool-a-1", "exact"),
	}))
	want := []string{
		"NODE_CPU_USAGE_CRITICAL", "WORKLOAD_MEMORY_LIMIT_CRITICAL", "WORKLOAD_MEMORY_REQUEST_CRITICAL",
		"HPA_MAX_REPLICAS_REACHED", "NODE_CPU_REQUEST_HIGH", "NAMESPACE_CPU_LIMIT_CRITICAL",
		"NAMESPACE_CPU_REQUEST_CRITICAL", "NAMESPACE_MEMORY_LIMIT_CRITICAL", "NAMESPACE_MEMORY_REQUEST_CRITICAL",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRecommendationsEmpty(t *testing.T) {
	got := Recommendations(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, want an empty non-nil slice", got)
	}
}

func TestConfidenceRank(t *testing.T) {
	for confidence, want := range map[string]int{"exact": 0, "inferred": 1, "unknown": 2, "x": 3} {
		if got := ConfidenceRank(confidence); got != want {
			t.Errorf("ConfidenceRank(%q) = %d, want %d", confidence, got, want)
		}
	}
}
