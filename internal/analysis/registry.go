// Package analysis turns the rows of the namespace and workload
// collectors into findings, following analysis/capacity.py of the
// original.
package analysis

import (
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

// The severities a finding can carry.
const (
	SeverityHealthy   = "healthy"
	SeverityAttention = "attention"
	SeverityCritical  = "critical"
)

// Severities lists the severities, from the lowest.
var Severities = []string{SeverityHealthy, SeverityAttention, SeverityCritical}

// The scope types a finding can carry.
const (
	ScopeNamespace = "namespace"
	ScopeWorkload  = "workload"
)

// Evidence is one named number of a finding.
type Evidence struct {
	Name  string
	Value float64
}

// Finding is one judgment of the analysis. It carries no sentence.
type Finding struct {
	Code       string
	Severity   string
	ScopeType  string
	ScopeKind  string // the kind of a workload; empty for a namespace
	ScopeName  string // the namespace, or namespace/name for a workload
	Evidence   []Evidence
	Confidence string
}

// Definition is what the registry knows of one code.
type Definition struct {
	Code      string
	Family    string
	Severity  string
	ScopeType string
	Params    map[string]string
	Evidence  []string
}

// ending describes one of the ten endings of a code, in the order the
// registry lists them.
type ending struct {
	suffix   string
	family   string
	severity string
	hasRisk  bool
	evidence []string
}

// endings holds the ten endings of the table of the design, in order.
var endings = []ending{
	{"REQUEST_HEALTHY", "resource_request_usage", SeverityHealthy, false, []string{"average_percent", "peak_percent"}},
	{"REQUEST_ATTENTION", "resource_request_usage", SeverityAttention, false, []string{"average_percent", "peak_percent"}},
	{"REQUEST_CRITICAL", "resource_request_usage", SeverityCritical, false, []string{"average_percent", "peak_percent"}},
	{"LIMIT_HEALTHY", "resource_limit_usage", SeverityHealthy, true, []string{"average_percent", "peak_percent"}},
	{"LIMIT_ATTENTION", "resource_limit_usage", SeverityAttention, true, []string{"average_percent", "peak_percent"}},
	{"LIMIT_CRITICAL", "resource_limit_usage", SeverityCritical, true, []string{"average_percent", "peak_percent"}},
	{"REQUEST_MISSING", "resource_request_missing", SeverityAttention, false, []string{}},
	{"LIMIT_MISSING", "resource_limit_missing", SeverityAttention, false, []string{}},
	{"OVERREQUESTED", "resource_overrequested", SeverityAttention, false, []string{"average_percent", "peak_percent"}},
	{"BURSTY", "resource_bursty", SeverityAttention, false, []string{"average", "peak", "gap_percent"}},
}

// prefixes holds the two prefixes of a code, in order, each with the
// scope type its rows carry.
var prefixes = []struct {
	code      string
	scopeType string
}{
	{"NAMESPACE", ScopeNamespace},
	{"WORKLOAD", ScopeWorkload},
}

// resources holds the two resources a code names, in order, each with the
// risk its limit findings name.
var resources = []struct {
	name string
	risk string
}{
	{"cpu", "cpu_throttling"},
	{"memory", "oomkill"},
}

// buildRegistry lists the 40 codes in the order of the design: prefix
// NAMESPACE then WORKLOAD; resource CPU then MEMORY; ending in the order
// of endings.
func buildRegistry() []Definition {
	registry := make([]Definition, 0, len(prefixes)*len(resources)*len(endings))
	for _, prefix := range prefixes {
		for _, resource := range resources {
			for _, e := range endings {
				params := map[string]string{"resource": resource.name}
				if e.hasRisk {
					params["risk"] = resource.risk
				}
				evidence := append([]string{}, e.evidence...)
				registry = append(registry, Definition{
					Code:      prefix.code + "_" + strings.ToUpper(resource.name) + "_" + e.suffix,
					Family:    e.family,
					Severity:  e.severity,
					ScopeType: prefix.scopeType,
					Params:    params,
					Evidence:  evidence,
				})
			}
		}
	}
	return registry
}

// Registry lists every code this version can write.
var Registry = buildRegistry()

// Lookup finds a definition by its code.
func Lookup(code string) (Definition, bool) {
	for _, definition := range Registry {
		if definition.Code == code {
			return definition, true
		}
	}
	return Definition{}, false
}

// Classify follows classify_percent of the original: at or above the
// critical threshold, critical; at or above the warning threshold,
// attention; below, healthy.
func Classify(percent float64, t config.Thresholds) string {
	switch {
	case percent >= t.CriticalPercent:
		return SeverityCritical
	case percent >= t.WarningPercent:
		return SeverityAttention
	default:
		return SeverityHealthy
	}
}

// Rank numbers a severity as the original numbers Severity: 0 for
// healthy, 1 for attention, 2 for critical. It returns -1 for any other
// text.
func Rank(severity string) int {
	switch severity {
	case SeverityHealthy:
		return 0
	case SeverityAttention:
		return 1
	case SeverityCritical:
		return 2
	default:
		return -1
	}
}
