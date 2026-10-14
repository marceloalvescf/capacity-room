package analysis

import (
	"math"

	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
)

// hpaFinding makes a finding of an HPA of namespace/name.
func hpaFinding(code, severity, scopeName, confidence string, evidence ...Evidence) Finding {
	return Finding{Code: code, Severity: severity, ScopeType: ScopeHPA, ScopeName: scopeName, Evidence: evidence, Confidence: confidence}
}

// HPAs follows analyze_hpas of the original: per HPA, the first of pinned,
// maximum reached, and near maximum that holds, then the divergence
// independently; then per workload without an HPA, a finding for a
// Deployment or a StatefulSet.
func HPAs(hpas []collect.HPA, uncovered []collect.WorkloadRef, t config.Thresholds) []Finding {
	findings := []Finding{}
	for _, hpa := range hpas {
		name := hpa.Namespace + "/" + hpa.Name
		current := hpa.Current.State == collect.StateValue
		desired := hpa.Desired.State == collect.StateValue
		lowest, highest := hpa.SpecMin, hpa.SpecMax

		switch {
		case lowest.Known && highest.Known && !math.IsInf(lowest.Value, 0) && !math.IsNaN(lowest.Value) &&
			!math.IsInf(highest.Value, 0) && !math.IsNaN(highest.Value) && lowest.Value == highest.Value:
			findings = append(findings, hpaFinding("HPA_PINNED_REPLICAS", SeverityHealthy, name, discovery.ConfidenceExact,
				Evidence{Name: "spec_min", Value: Number(lowest.Value)}, Evidence{Name: "spec_max", Value: Number(highest.Value)}))
		case highest.Known && current && hpa.Current.Peak >= highest.Value:
			findings = append(findings, hpaFinding("HPA_MAX_REPLICAS_REACHED", SeverityCritical, name, discovery.ConfidenceExact,
				Evidence{Name: "current_peak", Value: Number(hpa.Current.Peak)}, Evidence{Name: "spec_max", Value: Number(highest.Value)}))
		case highest.Known && highest.Value > 0 && current && hpa.Current.Average/highest.Value*100 >= t.AutoscalerNearMaxPercent:
			findings = append(findings, hpaFinding("HPA_NEAR_MAX_REPLICAS", SeverityAttention, name, discovery.ConfidenceExact,
				Evidence{Name: "current_average", Value: Number(hpa.Current.Average)}, Evidence{Name: "spec_max", Value: Number(highest.Value)}))
		}

		if current && desired {
			if divergence := math.Abs(hpa.Current.Average - hpa.Desired.Average); divergence >= t.HPAReplicaDivergence {
				findings = append(findings, hpaFinding("HPA_REPLICA_DIVERGENCE", SeverityAttention, name, discovery.ConfidenceInferred,
					Evidence{Name: "average_difference", Value: Number(divergence)}))
			}
		}
	}

	for _, workload := range uncovered {
		code, severity := "", ""
		switch workload.Kind {
		case "Deployment":
			code, severity = "DEPLOYMENT_WITHOUT_HPA", SeverityAttention
		case "StatefulSet":
			code, severity = "STATEFULSET_WITHOUT_HPA_INFO", SeverityHealthy
		default:
			continue
		}
		findings = append(findings, Finding{
			Code:       code,
			Severity:   severity,
			ScopeType:  ScopeWorkload,
			ScopeKind:  workload.Kind,
			ScopeName:  workload.Namespace + "/" + workload.Name,
			Evidence:   []Evidence{},
			Confidence: discovery.ConfidenceExact,
		})
	}
	return findings
}
