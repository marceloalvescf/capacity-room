package analysis

import (
	"sort"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

// prefixOf returns the code prefix of scopeType: NAMESPACE or WORKLOAD.
func prefixOf(scopeType string) string {
	if scopeType == ScopeWorkload {
		return "WORKLOAD"
	}
	return "NAMESPACE"
}

// rowFindings judges CPU then memory of one row, following the loop of
// analyze_namespaces and analyze_workloads of the original.
func rowFindings(scopeType, scopeName, scopeKind string, resources collect.Resources, requestsLimitsState string, approximate bool, t config.Thresholds) []Finding {
	prefix := prefixOf(scopeType)
	findings := resourceFindings(prefix, scopeType, scopeName, scopeKind, "cpu",
		resources.CPUUsage, resources.CPURequests, resources.CPULimits, resources.CPURequestPercent, resources.CPULimitPercent,
		requestsLimitsState, approximate, t)
	findings = append(findings, resourceFindings(prefix, scopeType, scopeName, scopeKind, "memory",
		resources.MemoryUsage, resources.MemoryRequests, resources.MemoryLimits, resources.MemoryRequestPercent, resources.MemoryLimitPercent,
		requestsLimitsState, approximate, t)...)
	return findings
}

// Namespaces follows analyze_namespaces of the original: it judges every
// row and returns the findings, sorted.
func Namespaces(rows []collect.Namespace, t config.Thresholds) []Finding {
	findings := []Finding{}
	for _, row := range rows {
		findings = append(findings, rowFindings(ScopeNamespace, row.Name, "", row.Resources, row.RequestsLimitsState, row.Approximate, t)...)
	}
	Sort(findings)
	return findings
}

// Workloads follows analyze_workloads of the original: it judges every
// row and returns the findings, sorted. The scope of a workload is
// namespace/name, with its kind.
func Workloads(rows []collect.Workload, t config.Thresholds) []Finding {
	findings := []Finding{}
	for _, row := range rows {
		scopeName := row.Namespace + "/" + row.Name
		findings = append(findings, rowFindings(ScopeWorkload, scopeName, row.Kind, row.Resources, row.RequestsLimitsState, row.Approximate, t)...)
	}
	Sort(findings)
	return findings
}

// All gives the findings of the namespaces and of the workloads of c,
// sorted together.
func All(c collect.Collected, t config.Thresholds) []Finding {
	findings := append(Namespaces(c.Namespaces.Rows, t), Workloads(c.Workloads.Rows, t)...)
	Sort(findings)
	return findings
}

// Sort orders findings by severity from the highest, then scope name,
// then scope kind, then code, following _ordered of the original with the
// scope kind added so that two workloads of one name keep a fixed order.
func Sort(findings []Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i], findings[j]
		if ra, rb := Rank(a.Severity), Rank(b.Severity); ra != rb {
			return ra > rb
		}
		if a.ScopeName != b.ScopeName {
			return a.ScopeName < b.ScopeName
		}
		if a.ScopeKind != b.ScopeKind {
			return a.ScopeKind < b.ScopeKind
		}
		return a.Code < b.Code
	})
}
