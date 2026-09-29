package analysis

import (
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// resourceFindings judges one resource of one row, following
// _resource_findings of the original, in this order: the request ratio,
// the limit ratio, REQUEST_MISSING, LIMIT_MISSING, OVERREQUESTED, and
// BURSTY.
func resourceFindings(
	prefix, scopeType, scopeName, scopeKind, resource string,
	usage, requests, limits, requestPercent, limitPercent collect.Statistics,
	requestsLimitsState string,
	approximate bool,
	t config.Thresholds,
) []Finding {
	findings := []Finding{}
	confidence := discovery.ConfidenceExact
	if approximate {
		confidence = discovery.ConfidenceInferred
	}
	code := prefix + "_" + strings.ToUpper(resource)

	if requestPercent.State == collect.StateValue {
		severity := higherSeverity(requestPercent.Average, requestPercent.Peak, t)
		findings = append(findings, Finding{
			Code:       code + "_REQUEST_" + strings.ToUpper(severity),
			Severity:   severity,
			ScopeType:  scopeType,
			ScopeKind:  scopeKind,
			ScopeName:  scopeName,
			Evidence:   percentEvidence(requestPercent),
			Confidence: confidence,
		})
	}

	if limitPercent.State == collect.StateValue {
		severity := higherSeverity(limitPercent.Average, limitPercent.Peak, t)
		findings = append(findings, Finding{
			Code:       code + "_LIMIT_" + strings.ToUpper(severity),
			Severity:   severity,
			ScopeType:  scopeType,
			ScopeKind:  scopeKind,
			ScopeName:  scopeName,
			Evidence:   percentEvidence(limitPercent),
			Confidence: confidence,
		})
	}

	if requestsLimitsState == discovery.StateAvailable && requests.State != collect.StateValue {
		findings = append(findings, Finding{
			Code:       code + "_REQUEST_MISSING",
			Severity:   SeverityAttention,
			ScopeType:  scopeType,
			ScopeKind:  scopeKind,
			ScopeName:  scopeName,
			Evidence:   []Evidence{},
			Confidence: discovery.ConfidenceExact,
		})
	}

	if requestsLimitsState == discovery.StateAvailable && limits.State != collect.StateValue {
		findings = append(findings, Finding{
			Code:       code + "_LIMIT_MISSING",
			Severity:   SeverityAttention,
			ScopeType:  scopeType,
			ScopeKind:  scopeKind,
			ScopeName:  scopeName,
			Evidence:   []Evidence{},
			Confidence: discovery.ConfidenceExact,
		})
	}

	if requestPercent.State == collect.StateValue && requestPercent.Average <= t.LowUtilizationPercent {
		findings = append(findings, Finding{
			Code:       code + "_OVERREQUESTED",
			Severity:   SeverityAttention,
			ScopeType:  scopeType,
			ScopeKind:  scopeKind,
			ScopeName:  scopeName,
			Evidence:   percentEvidence(requestPercent),
			Confidence: confidence,
		})
	}

	if usage.State == collect.StateValue && usage.Average > 0 {
		gapPercent := (usage.Peak - usage.Average) / usage.Average * 100.0
		if gapPercent >= t.BurstyGapPercent {
			findings = append(findings, Finding{
				Code:      code + "_BURSTY",
				Severity:  SeverityAttention,
				ScopeType: scopeType,
				ScopeKind: scopeKind,
				ScopeName: scopeName,
				Evidence: []Evidence{
					{Name: "average", Value: usage.Average},
					{Name: "peak", Value: usage.Peak},
					{Name: "gap_percent", Value: gapPercent},
				},
				Confidence: discovery.ConfidenceExact,
			})
		}
	}

	return findings
}

// higherSeverity classifies average and peak, and returns the higher of
// the two severities, following _percent_severity of the original.
func higherSeverity(average, peak float64, t config.Thresholds) string {
	averageSeverity := Classify(average, t)
	peakSeverity := Classify(peak, t)
	if Rank(peakSeverity) > Rank(averageSeverity) {
		return peakSeverity
	}
	return averageSeverity
}

// percentEvidence carries the average and the peak of statistics, in that
// order.
func percentEvidence(statistics collect.Statistics) []Evidence {
	return []Evidence{
		{Name: "average_percent", Value: statistics.Average},
		{Name: "peak_percent", Value: statistics.Peak},
	}
}
