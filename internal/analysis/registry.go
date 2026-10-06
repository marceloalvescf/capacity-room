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
	ScopeHPA       = "hpa"
	ScopeNode      = "node"
	ScopeNodepool  = "nodepool"
	ScopeCluster   = "cluster"
)

// ValueKind names the kind an evidence value carries.
type ValueKind string

// The kinds of an evidence value.
const (
	KindNumber  ValueKind = "number"
	KindBoolean ValueKind = "boolean"
	KindText    ValueKind = "text"
)

// Value is the value of an evidence: a number, a boolean, or a text.
type Value struct {
	Kind    ValueKind
	Number  float64
	Boolean bool
	Text    string
}

// Number makes a Value of kind number.
func Number(v float64) Value { return Value{Kind: KindNumber, Number: v} }

// Boolean makes a Value of kind boolean.
func Boolean(b bool) Value { return Value{Kind: KindBoolean, Boolean: b} }

// Text makes a Value of kind text.
func Text(s string) Value { return Value{Kind: KindText, Text: s} }

// Evidence is one named value of a finding.
type Evidence struct {
	Name  string
	Value Value
}

// EvidenceSpec names one evidence of a definition and the kind it carries.
type EvidenceSpec struct {
	Name     string
	Kind     ValueKind
	Optional bool // a finding may omit it when its number is unknown
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

// ConfidenceRule says which confidence a finding of a code may carry. When
// Fixed is empty, exactly one of the three sources decides.
type ConfidenceRule struct {
	Fixed           []string // the confidences allowed whatever the row
	FromApproximate bool     // exact when the row is not approximate, inferred when it is
	FromPool        bool     // equal to the confidence of the node pool in scope
	FromAutoscaling bool     // equal to the confidence of the cluster autoscaling
}

// Definition is what the registry knows of one code.
type Definition struct {
	Code       string
	Family     string
	Severities []string // the severities the code may carry, in Severities order
	ScopeType  string
	Params     map[string]string
	Evidence   []EvidenceSpec
	Confidence ConfidenceRule
}

// The confidence rules the codes of the registry share.
var (
	fixedExact      = ConfidenceRule{Fixed: []string{"exact"}}
	fixedInferred   = ConfidenceRule{Fixed: []string{"inferred"}}
	fixedUnknown    = ConfidenceRule{Fixed: []string{"unknown"}}
	fromApproximate = ConfidenceRule{FromApproximate: true}
	fromPool        = ConfidenceRule{FromPool: true}
	fromAutoscaling = ConfidenceRule{FromAutoscaling: true}
)

// ending describes one of the ten endings of a code, in the order the
// registry lists them.
type ending struct {
	suffix   string
	family   string
	severity string
	hasRisk  bool
	evidence []string
	rule     ConfidenceRule
}

// endings holds the ten endings of the table of the design, in order.
var endings = []ending{
	{"REQUEST_HEALTHY", "resource_request_usage", SeverityHealthy, false, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"REQUEST_ATTENTION", "resource_request_usage", SeverityAttention, false, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"REQUEST_CRITICAL", "resource_request_usage", SeverityCritical, false, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"LIMIT_HEALTHY", "resource_limit_usage", SeverityHealthy, true, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"LIMIT_ATTENTION", "resource_limit_usage", SeverityAttention, true, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"LIMIT_CRITICAL", "resource_limit_usage", SeverityCritical, true, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"REQUEST_MISSING", "resource_request_missing", SeverityAttention, false, []string{}, fixedExact},
	{"LIMIT_MISSING", "resource_limit_missing", SeverityAttention, false, []string{}, fixedExact},
	{"OVERREQUESTED", "resource_overrequested", SeverityAttention, false, []string{"average_percent", "peak_percent"}, fromApproximate},
	{"BURSTY", "resource_bursty", SeverityAttention, false, []string{"average", "peak", "gap_percent"}, fixedExact},
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

// buildRegistry lists the 40 codes of namespaces and workloads in the order
// of the design: prefix NAMESPACE then WORKLOAD; resource CPU then MEMORY;
// ending in the order of endings. The 26 codes of HPAs, nodes, pools, and
// autoscaling follow.
func buildRegistry() []Definition {
	registry := make([]Definition, 0, 66)
	for _, prefix := range prefixes {
		for _, resource := range resources {
			for _, e := range endings {
				params := map[string]string{"resource": resource.name}
				if e.hasRisk {
					params["risk"] = resource.risk
				}
				evidence := make([]EvidenceSpec, 0, len(e.evidence))
				for _, name := range e.evidence {
					evidence = append(evidence, EvidenceSpec{Name: name, Kind: KindNumber})
				}
				registry = append(registry, Definition{
					Code:       prefix.code + "_" + strings.ToUpper(resource.name) + "_" + e.suffix,
					Family:     e.family,
					Severities: []string{e.severity},
					ScopeType:  prefix.scopeType,
					Params:     params,
					Evidence:   evidence,
					Confidence: e.rule,
				})
			}
		}
	}
	return append(registry, clusterDefinitions()...)
}

// numbers makes the evidence specs of a definition, all of kind number.
func numbers(names ...string) []EvidenceSpec {
	specs := make([]EvidenceSpec, 0, len(names))
	for _, name := range names {
		specs = append(specs, EvidenceSpec{Name: name, Kind: KindNumber})
	}
	return specs
}

// usageEvidence lists the evidence of a node usage code: the average and
// the peak, each optional.
func usageEvidence() []EvidenceSpec {
	return []EvidenceSpec{
		{Name: "average_percent", Kind: KindNumber, Optional: true},
		{Name: "peak_percent", Kind: KindNumber, Optional: true},
	}
}

// clusterDefinitions lists the 26 codes of nodes, pools, HPAs, and
// autoscaling, in the order of the design.
func clusterDefinitions() []Definition {
	definitions := []Definition{}
	for _, resource := range []string{"cpu", "memory"} {
		for _, severity := range Severities {
			definitions = append(definitions, Definition{
				Code:       "NODE_" + strings.ToUpper(resource) + "_USAGE_" + strings.ToUpper(severity),
				Family:     "node_usage",
				Severities: []string{severity},
				ScopeType:  ScopeNode,
				Params:     map[string]string{"resource": resource},
				Evidence:   usageEvidence(),
				Confidence: fixedExact,
			})
		}
	}
	for _, resource := range []string{"cpu", "memory"} {
		definitions = append(definitions, Definition{
			Code:       "NODE_" + strings.ToUpper(resource) + "_REQUEST_HIGH",
			Family:     "node_request_high",
			Severities: []string{SeverityAttention, SeverityCritical},
			ScopeType:  ScopeNode,
			Params:     map[string]string{"resource": resource},
			Evidence:   numbers("percent"),
			Confidence: fixedExact,
		})
	}
	definitions = append(definitions,
		Definition{Code: "NODE_OVERREQUESTED", Family: "node_overrequested", Severities: []string{SeverityAttention}, ScopeType: ScopeNode, Params: map[string]string{},
			Evidence: []EvidenceSpec{{Name: "resources", Kind: KindText}}, Confidence: fixedInferred},
		Definition{Code: "NODE_LOW_UTILIZATION", Family: "node_low_utilization", Severities: []string{SeverityAttention}, ScopeType: ScopeNode, Params: map[string]string{},
			Evidence: numbers("maximum_average_percent"), Confidence: fixedExact},
		Definition{Code: "NODEPOOL_UNKNOWN", Family: "nodepool_unknown", Severities: []string{SeverityAttention}, ScopeType: ScopeNodepool, Params: map[string]string{},
			Evidence: numbers("affected_nodes"), Confidence: fixedUnknown},
	)
	for _, resource := range []string{"cpu", "memory"} {
		definitions = append(definitions, Definition{
			Code:       "NODEPOOL_" + strings.ToUpper(resource) + "_UNEVEN",
			Family:     "nodepool_uneven",
			Severities: []string{SeverityAttention},
			ScopeType:  ScopeNodepool,
			Params:     map[string]string{"resource": resource},
			Evidence:   numbers("minimum_peak_percent", "maximum_peak_percent", "spread_percent"),
			Confidence: fromPool,
		})
	}
	plain := func(code, family, severity, scopeType string, evidence []EvidenceSpec, rule ConfidenceRule) Definition {
		return Definition{Code: code, Family: family, Severities: []string{severity}, ScopeType: scopeType, Params: map[string]string{}, Evidence: evidence, Confidence: rule}
	}
	pressure := []EvidenceSpec{{Name: "node_pressure", Kind: KindBoolean}}
	return append(definitions,
		plain("HPA_PINNED_REPLICAS", "hpa_pinned", SeverityHealthy, ScopeHPA, numbers("spec_min", "spec_max"), fixedExact),
		plain("HPA_MAX_REPLICAS_REACHED", "hpa_max_reached", SeverityCritical, ScopeHPA, numbers("current_peak", "spec_max"), fixedExact),
		plain("HPA_NEAR_MAX_REPLICAS", "hpa_near_max", SeverityAttention, ScopeHPA, numbers("current_average", "spec_max"), fixedExact),
		plain("HPA_REPLICA_DIVERGENCE", "hpa_divergence", SeverityAttention, ScopeHPA, numbers("average_difference"), fixedInferred),
		plain("DEPLOYMENT_WITHOUT_HPA", "deployment_without_hpa", SeverityAttention, ScopeWorkload, numbers(), fixedExact),
		plain("STATEFULSET_WITHOUT_HPA_INFO", "statefulset_without_hpa", SeverityHealthy, ScopeWorkload, numbers(), fixedExact),
		plain("NODE_AUTOSCALING_UNKNOWN", "autoscaling_unknown", SeverityAttention, ScopeCluster, numbers(), fixedUnknown),
		plain("NODE_AUTOSCALING_DISABLED", "autoscaling_disabled", SeverityAttention, ScopeCluster, pressure, fromAutoscaling),
		plain("NODE_AUTOSCALING_DISABLED_PRESSURE", "autoscaling_disabled", SeverityCritical, ScopeCluster, pressure, fromAutoscaling),
		plain("NODE_AUTOSCALING_NEAR_MAX", "autoscaling_near_max", SeverityAttention, ScopeCluster, numbers("current_nodes", "max_nodes"), fromAutoscaling),
		plain("NODE_AUTOSCALING_BOUNDS_UNKNOWN", "autoscaling_bounds_unknown", SeverityHealthy, ScopeCluster, numbers(), fromAutoscaling),
		plain("NODE_AUTOSCALING_AT_MIN", "autoscaling_at_min", SeverityHealthy, ScopeCluster, numbers("min_nodes"), fromAutoscaling),
		plain("NODE_AUTOSCALING_ENABLED", "autoscaling_enabled", SeverityHealthy, ScopeCluster, numbers(), fromAutoscaling),
	)
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
