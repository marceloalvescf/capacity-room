package document

import (
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// workloadFinding returns a finding of a workload with its evidence.
func workloadFinding(code, severity, namespace, name, kind string, evidence ...snapshot.Evidence) snapshot.Finding {
	return snapshot.Finding{
		Code: code, Severity: severity, ScopeType: "workload", ScopeKind: kind,
		ScopeName: namespace + "/" + name, Evidence: evidence, Confidence: "exact",
	}
}

func TestReasonFragmentOfNodesUnchanged(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	tests := []struct {
		finding snapshot.Finding
		want    string
	}{
		{nodeFinding("NODE_CPU_USAGE_ATTENTION", "attention", "n", number("average_percent", 55), number("peak_percent", 78)), "CPU: peak of 78% utilization"},
		{nodeFinding("NODE_MEMORY_USAGE_HEALTHY", "healthy", "n", number("average_percent", 40.25)), "Memory: average of 40.2% utilization"},
		{nodeFinding("NODE_MEMORY_USAGE_CRITICAL", "critical", "n", number("peak_percent", 91)), "Memory: peak of 91% utilization"},
		{nodeFinding("NODE_CPU_REQUEST_HIGH", "attention", "n", number("percent", 75.5)), "CPU: 75.5% of the allocatable requested"},
		{nodeFinding("NODE_MEMORY_REQUEST_HIGH", "critical", "n", number("percent", 90)), "Memory: 90% of the allocatable requested"},
		{nodeFinding("NODE_OVERREQUESTED", "attention", "n", snapshot.Evidence{Name: "resources", Value: snapshot.Value{Kind: "text", Text: "cpu"}}), "High requests coexist with low observed utilization"},
		{nodeFinding("NODE_LOW_UTILIZATION", "attention", "n", number("maximum_average_percent", 12)), "The node showed low average utilization"},
	}
	for _, tt := range tests {
		t.Run(tt.finding.Code, func(t *testing.T) {
			if got := reasonFragment(b, tt.finding); got != tt.want {
				t.Errorf("reasonFragment() = %q, want %q", got, tt.want)
			}
		})
	}
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
}

func TestReasonFragmentOfWorkloads(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	tests := []struct {
		name    string
		finding snapshot.Finding
		want    string
	}{
		{"request peak", workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", "critical", "p", "api", "Deployment", number("average_percent", 88), number("peak_percent", 97)), reason(t, "request_peak", "cpu", "97%")},
		{"request average", workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", "critical", "p", "api", "Deployment", number("average_percent", 72)), reason(t, "request_average", "cpu", "72%")},
		{"limit peak", workloadFinding("WORKLOAD_MEMORY_LIMIT_ATTENTION", "attention", "p", "api", "Deployment", number("peak_percent", 78)), reason(t, "limit_peak", "memory", "78%")},
		{"overrequested", workloadFinding("WORKLOAD_CPU_OVERREQUESTED", "attention", "p", "api", "Deployment", number("average_percent", 7.1)), reason(t, "request_average", "cpu", "7,1%")},
		{"bursty", workloadFinding("WORKLOAD_CPU_BURSTY", "attention", "p", "api", "Deployment", number("gap_percent", 96.1)), reason(t, "bursty", "cpu", "96,1%")},
		{"request missing", workloadFinding("WORKLOAD_CPU_REQUEST_MISSING", "attention", "p", "api", "Deployment"), pt(t, catalog.Reason("request_missing", "cpu"), nil)},
		{"limit missing", workloadFinding("WORKLOAD_MEMORY_LIMIT_MISSING", "attention", "p", "api", "Deployment"), pt(t, catalog.Reason("limit_missing", "memory"), nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reasonFragment(b, tt.finding); got != tt.want {
				t.Errorf("reasonFragment() = %q, want %q", got, tt.want)
			}
		})
	}
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
}

func TestReasonFragmentRefusesACodeWithoutText(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	f := snapshot.Finding{Code: "HPA_NEAR_MAX_REPLICAS", Severity: "attention", ScopeType: "hpa", ScopeName: "p/api"}
	if got := reasonFragment(b, f); got != "" {
		t.Errorf("reasonFragment() = %q, want empty", got)
	}
	if b.err == nil || b.err.Error() != "finding HPA_NEAR_MAX_REPLICAS has no reason text" {
		t.Fatalf("builder error = %v", b.err)
	}
}

func missingFindings(codes ...string) []snapshot.Finding {
	var out []snapshot.Finding
	for _, code := range codes {
		out = append(out, workloadFinding(code, "attention", "p", "api", "Deployment"))
	}
	return out
}

func TestWorkloadReasonCollapsesTheFourMissing(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	findings := missingFindings("WORKLOAD_CPU_LIMIT_MISSING", "WORKLOAD_CPU_REQUEST_MISSING", "WORKLOAD_MEMORY_LIMIT_MISSING")
	findings = append(findings,
		workloadFinding("WORKLOAD_CPU_BURSTY", "attention", "p", "api", "Deployment", number("gap_percent", 50)),
		workloadFinding("WORKLOAD_MEMORY_REQUEST_MISSING", "attention", "p", "api", "Deployment"))
	want := pt(t, catalog.SectionsExecutiveReasonAllMissing, nil) + "; " + reason(t, "bursty", "cpu", "50%")
	if got := workloadReason(b, findings); got != want {
		t.Errorf("workloadReason() = %q, want %q", got, want)
	}
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
}

func TestWorkloadReasonKeepsThreeMissingApart(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	findings := missingFindings("WORKLOAD_CPU_LIMIT_MISSING", "WORKLOAD_CPU_REQUEST_MISSING", "WORKLOAD_MEMORY_LIMIT_MISSING")
	want := strings.Join([]string{
		pt(t, catalog.Reason("limit_missing", "cpu"), nil),
		pt(t, catalog.Reason("request_missing", "cpu"), nil),
		pt(t, catalog.Reason("limit_missing", "memory"), nil),
	}, "; ")
	got := workloadReason(b, findings)
	if got != want {
		t.Errorf("workloadReason() = %q, want %q", got, want)
	}
	if strings.Contains(got, pt(t, catalog.SectionsExecutiveReasonAllMissing, nil)) {
		t.Errorf("workloadReason() = %q holds the collapsed sentence", got)
	}
}

func TestWorkloadReasonDeduplicates(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	findings := []snapshot.Finding{
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", "critical", "p", "api", "Deployment", number("peak_percent", 97)),
		workloadFinding("WORKLOAD_CPU_REQUEST_ATTENTION", "attention", "p", "api", "Deployment", number("peak_percent", 97)),
	}
	if got, want := workloadReason(b, findings), reason(t, "request_peak", "cpu", "97%"); got != want {
		t.Errorf("workloadReason() = %q, want %q", got, want)
	}
}
