package document

import (
	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// WorstSeverity is the worst severity among the findings the report shows:
// the snapshot's minimum severity applied, raised to attention for the
// executive profile, as BuildView narrows them. It is healthy when the
// report shows no finding.
func WorstSeverity(s snapshot.Snapshot) (string, error) {
	profile, severity, err := resolve(s, Options{})
	if err != nil {
		return "", err
	}
	if profile == "executive" {
		severity = raiseToAttention(severity)
	}
	worst := analysis.SeverityHealthy
	for _, f := range shownFindings(s, severity) {
		if analysis.Rank(f.Severity) > analysis.Rank(worst) {
			worst = f.Severity
		}
	}
	return worst, nil
}
