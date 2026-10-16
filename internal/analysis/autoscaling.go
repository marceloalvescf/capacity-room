package analysis

import (
	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
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

// Autoscaling gives no finding when the evidence holds no status, as when
// the original has no evidence; otherwise one finding, the first that holds
// of unknown, disabled, and enabled. The registry keeps the codes of near
// maximum, bounds unknown, and at minimum for reports stored before; they are
// no longer given.
func Autoscaling(a collect.AutoscalingEvidence, nodePressure bool) []Finding {
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

	return finding("NODE_AUTOSCALING_ENABLED", SeverityHealthy)
}
