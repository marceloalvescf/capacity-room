package document

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// findingsOf returns n findings of a scope type at a severity, each on a
// row of its own.
func findingsOf(scopeType, severity string, n int) []snapshot.Finding {
	out := make([]snapshot.Finding, 0, n)
	for i := range n {
		out = append(out, snapshot.Finding{
			ScopeType: scopeType, ScopeName: fmt.Sprintf("%s-%s-%d", scopeType, severity, i), Severity: severity,
		})
	}
	return out
}

func joinFindings(groups ...[]snapshot.Finding) []snapshot.Finding {
	var out []snapshot.Finding
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}

// scopeFindings is a set of findings that holds every severity of four scope
// types and one finding of the cluster.
func scopeFindings() []snapshot.Finding {
	return joinFindings(
		findingsOf("node", "critical", 2),
		findingsOf("node", "attention", 1),
		findingsOf("node", "healthy", 1),
		findingsOf("workload", "critical", 1),
		findingsOf("workload", "attention", 4),
		findingsOf("workload", "healthy", 3),
		findingsOf("hpa", "attention", 5),
		findingsOf("namespace", "healthy", 2),
		findingsOf("cluster", "critical", 1),
	)
}

func TestDecideScopesCountsFindings(t *testing.T) {
	findings := scopeFindings()
	want := []scopeFacts{
		{ScopeType: "node", Severity: "critical", Lead: 0, Critical: 2, Attention: 1, Healthy: 1},
		{ScopeType: "workload", Severity: "critical", Lead: 4, Critical: 1, Attention: 4, Healthy: 3},
		{ScopeType: "hpa", Severity: "attention", Lead: 12, Critical: 0, Attention: 5, Healthy: 0},
		{ScopeType: "namespace", Severity: "healthy", Lead: 17, Critical: 0, Attention: 0, Healthy: 2},
	}
	if got := decideScopes(findings); !reflect.DeepEqual(got, want) {
		t.Errorf("decideScopes = %+v, want %+v", got, want)
	}
}

func TestDecideScopesOrdersByCounts(t *testing.T) {
	findings := joinFindings(findingsOf("hpa", "attention", 3), findingsOf("node", "attention", 5))
	got := decideScopes(findings)
	if len(got) != 2 || got[0].ScopeType != "node" || got[1].ScopeType != "hpa" {
		t.Errorf("order = %+v, want node then hpa", got)
	}

	tied := joinFindings(findingsOf("hpa", "attention", 2), findingsOf("workload", "attention", 2), findingsOf("nodepool", "attention", 2))
	got = decideScopes(tied)
	order := []string{}
	for _, s := range got {
		order = append(order, s.ScopeType)
	}
	if want := []string{"nodepool", "workload", "hpa"}; !reflect.DeepEqual(order, want) {
		t.Errorf("tied order = %v, want %v", order, want)
	}
}

func TestDecideScopesEmpty(t *testing.T) {
	if got := decideScopes(findingsOf("cluster", "critical", 2)); len(got) != 0 {
		t.Errorf("cluster only: %+v, want none", got)
	}
	if got := decideScopes(nil); len(got) != 0 {
		t.Errorf("no findings: %+v, want none", got)
	}
}
