package document

import (
	"github.com/marceloalvescf/capacity-room/internal/analysis"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// WorstSeverity is the worst severity among the findings the report shows:
// those at or above the floor of the snapshot's profile, as BuildView
// narrows them. It is healthy when the report shows no finding.
func WorstSeverity(s snapshot.Snapshot) (string, error) {
	profile, err := resolve(s, Options{})
	if err != nil {
		return "", err
	}
	worst := analysis.SeverityHealthy
	for _, f := range shownFindings(s, profileFloor(profile)) {
		if analysis.Rank(f.Severity) > analysis.Rank(worst) {
			worst = f.Severity
		}
	}
	return worst, nil
}
