package analysis

import (
	"slices"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

// Pools follows analyze_nodepools of the original: per pool, in the order
// given, CPU then memory; when the known usage peaks of its nodes are at
// least two and their spread reaches the threshold, one finding. The pool
// unknown is judged like any other.
func Pools(pools []collect.Pool, nodes []collect.Node, t config.Thresholds) []Finding {
	byName := make(map[string]collect.Node, len(nodes))
	for _, node := range nodes {
		byName[node.Name] = node
	}

	findings := []Finding{}
	for _, pool := range pools {
		for _, resource := range []string{"cpu", "memory"} {
			peaks := []float64{}
			for _, name := range pool.Nodes {
				node := byName[name]
				usage := node.CPUUsagePercent
				if resource == "memory" {
					usage = node.MemoryUsagePercent
				}
				if p := peak(usage); p.Known {
					peaks = append(peaks, p.Value)
				}
			}
			if len(peaks) < 2 {
				continue
			}
			lowest, highest := slices.Min(peaks), slices.Max(peaks)
			spread := highest - lowest
			if spread < t.NodepoolSpreadPoints {
				continue
			}
			findings = append(findings, Finding{
				Code:      "NODEPOOL_" + strings.ToUpper(resource) + "_UNEVEN",
				Severity:  SeverityAttention,
				ScopeType: ScopeNodepool,
				ScopeName: pool.Name,
				Evidence: []Evidence{
					{Name: "minimum_peak_percent", Value: Number(lowest)},
					{Name: "maximum_peak_percent", Value: Number(highest)},
					{Name: "spread_percent", Value: Number(spread)},
				},
				Confidence: pool.Confidence,
			})
		}
	}
	return findings
}
