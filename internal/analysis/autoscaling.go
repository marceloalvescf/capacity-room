package analysis

import (
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// NodePressure follows node_pressure of analyze_evidence: true when any
// finding of the node or pool lists carries attention or above. The
// readiness findings of a node (not ready, cordoned) are left out: they say
// nothing of how loaded the nodes are.
func NodePressure(nodeFindings, poolFindings []Finding) bool {
	for _, list := range [][]Finding{nodeFindings, poolFindings} {
		for _, f := range list {
			if f.Code == "NODE_NOT_READY" || f.Code == "NODE_UNSCHEDULABLE" {
				continue
			}
			if Rank(f.Severity) >= Rank(SeverityAttention) {
				return true
			}
		}
	}
	return false
}

// Autoscaling follows analyze_autoscaling of the original: no finding when
// the evidence holds no status, as when the original has no evidence;
// otherwise one finding, the
// first that holds of unknown, disabled, near maximum, bounds unknown, at
// minimum, and enabled.
func Autoscaling(a collect.AutoscalingEvidence, t config.Thresholds, nodePressure bool) []Finding {
	finding := func(code, severity string, evidence ...Evidence) []Finding {
		return []Finding{{
			Code:       code,
			Severity:   severity,
			ScopeType:  ScopeCluster,
			ScopeName:  "cluster",
			Evidence:   evidence,
			Confidence: a.Confidence,
		}}
	}

	switch a.Status {
	case "":
		return nil
	case "unknown":
		f := finding("NODE_AUTOSCALING_UNKNOWN", SeverityAttention)
		f[0].Confidence = discovery.ConfidenceUnknown
		return f
	case "disabled":
		evidence := Evidence{Name: "node_pressure", Value: Boolean(nodePressure)}
		if nodePressure {
			return finding("NODE_AUTOSCALING_DISABLED_PRESSURE", SeverityCritical, evidence)
		}
		return finding("NODE_AUTOSCALING_DISABLED", SeverityAttention, evidence)
	}

	current, lowest, highest := a.CurrentNodes, a.MinNodes, a.MaxNodes
	if highest.Known && highest.Value > 0 && current.Known && current.Value/highest.Value*100 >= t.AutoscalerNearMaxPercent {
		return finding("NODE_AUTOSCALING_NEAR_MAX", SeverityAttention,
			Evidence{Name: "current_nodes", Value: Number(current.Value)}, Evidence{Name: "max_nodes", Value: Number(highest.Value)})
	}
	if !highest.Known || !lowest.Known {
		return finding("NODE_AUTOSCALING_BOUNDS_UNKNOWN", SeverityHealthy)
	}
	if current.Known && current.Value == lowest.Value && !nodePressure {
		return finding("NODE_AUTOSCALING_AT_MIN", SeverityHealthy, Evidence{Name: "min_nodes", Value: Number(lowest.Value)})
	}
	return finding("NODE_AUTOSCALING_ENABLED", SeverityHealthy)
}
