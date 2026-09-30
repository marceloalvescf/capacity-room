package analysis

import (
	"slices"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// unknownPool is the pool of a node whose pool could not be determined.
const unknownPool = "unknown"

// average and peak read a number of statistics, known only when the
// statistics hold a value.
func average(s collect.Statistics) collect.Number {
	return collect.Number{Value: s.Average, Known: s.State == collect.StateValue}
}

func peak(s collect.Statistics) collect.Number {
	return collect.Number{Value: s.Peak, Known: s.State == collect.StateValue}
}

// higherKnown classifies the known numbers of a and b and returns the
// higher severity, or false when neither is known.
func higherKnown(a, b collect.Number, t config.Thresholds) (string, bool) {
	severity, found := "", false
	for _, n := range []collect.Number{a, b} {
		if !n.Known {
			continue
		}
		if s := Classify(n.Value, t); !found || Rank(s) > Rank(severity) {
			severity, found = s, true
		}
	}
	return severity, found
}

// usageFinding follows the node usage of analyze_nodes: the higher of the
// classifications of the average and of the peak, with the evidence of the
// numbers that are known. resource is CPU or MEMORY.
func usageFinding(resource, node string, averageUsage, peakUsage collect.Number, t config.Thresholds) (Finding, bool) {
	severity, found := higherKnown(averageUsage, peakUsage, t)
	if !found {
		return Finding{}, false
	}
	evidence := []Evidence{}
	if averageUsage.Known {
		evidence = append(evidence, Evidence{Name: "average_percent", Value: Number(averageUsage.Value)})
	}
	if peakUsage.Known {
		evidence = append(evidence, Evidence{Name: "peak_percent", Value: Number(peakUsage.Value)})
	}
	return Finding{
		Code:       "NODE_" + resource + "_USAGE_" + strings.ToUpper(severity),
		Severity:   severity,
		ScopeType:  ScopeNode,
		ScopeName:  node,
		Evidence:   evidence,
		Confidence: discovery.ConfidenceExact,
	}, true
}

// requestHigh follows the request block of analyze_nodes: a request percent
// classified attention or critical. It gives the severity and whether the
// request was high.
func requestHigh(percent collect.Number, t config.Thresholds) (string, bool) {
	if !percent.Known {
		return "", false
	}
	severity := Classify(percent.Value, t)
	return severity, severity != SeverityHealthy
}

// Nodes follows analyze_nodes of the original: per node, in name order,
// the CPU and memory usage, the request high findings, the overrequested
// finding, and the low utilisation finding; then one NODEPOOL_UNKNOWN for
// the nodes whose pool is unknown.
func Nodes(nodes []collect.Node, t config.Thresholds) []Finding {
	ordered := slices.Clone(nodes)
	slices.SortStableFunc(ordered, func(a, b collect.Node) int { return strings.Compare(a.Name, b.Name) })

	findings := []Finding{}
	unknown := 0
	for _, node := range ordered {
		if f, ok := usageFinding("CPU", node.Name, average(node.CPUUsagePercent), peak(node.CPUUsagePercent), t); ok {
			findings = append(findings, f)
		}
		if f, ok := usageFinding("MEMORY", node.Name, average(node.MemoryUsagePercent), peak(node.MemoryUsagePercent), t); ok {
			findings = append(findings, f)
		}

		resources := []struct {
			name    string
			request collect.Number
			usage   collect.Number
		}{
			{"cpu", node.CPURequestPercent, average(node.CPUUsagePercent)},
			{"memory", node.MemoryRequestPercent, average(node.MemoryUsagePercent)},
		}
		overrequested := []string{}
		for _, r := range resources {
			severity, high := requestHigh(r.request, t)
			if !high {
				continue
			}
			findings = append(findings, Finding{
				Code:       "NODE_" + strings.ToUpper(r.name) + "_REQUEST_HIGH",
				Severity:   severity,
				ScopeType:  ScopeNode,
				ScopeName:  node.Name,
				Evidence:   []Evidence{{Name: "percent", Value: Number(r.request.Value)}},
				Confidence: discovery.ConfidenceExact,
			})
			if r.usage.Known && r.usage.Value <= t.LowUtilizationPercent {
				overrequested = append(overrequested, r.name)
			}
		}
		if len(overrequested) > 0 {
			findings = append(findings, Finding{
				Code:       "NODE_OVERREQUESTED",
				Severity:   SeverityAttention,
				ScopeType:  ScopeNode,
				ScopeName:  node.Name,
				Evidence:   []Evidence{{Name: "resources", Value: Text(strings.Join(overrequested, ","))}},
				Confidence: discovery.ConfidenceInferred,
			})
		}

		averages := []float64{}
		for _, n := range []collect.Number{average(node.CPUUsagePercent), average(node.MemoryUsagePercent)} {
			if n.Known {
				averages = append(averages, n.Value)
			}
		}
		if len(averages) > 0 {
			if highest := slices.Max(averages); highest <= t.LowUtilizationPercent {
				findings = append(findings, Finding{
					Code:       "NODE_LOW_UTILIZATION",
					Severity:   SeverityAttention,
					ScopeType:  ScopeNode,
					ScopeName:  node.Name,
					Evidence:   []Evidence{{Name: "maximum_average_percent", Value: Number(highest)}},
					Confidence: discovery.ConfidenceExact,
				})
			}
		}

		if node.Pool == unknownPool || node.PoolConfidence == discovery.ConfidenceUnknown {
			unknown++
		}
	}

	if unknown > 0 {
		findings = append(findings, Finding{
			Code:       "NODEPOOL_UNKNOWN",
			Severity:   SeverityAttention,
			ScopeType:  ScopeNodepool,
			ScopeName:  unknownPool,
			Evidence:   []Evidence{{Name: "affected_nodes", Value: Number(float64(unknown))}},
			Confidence: discovery.ConfidenceUnknown,
		})
	}
	return findings
}
