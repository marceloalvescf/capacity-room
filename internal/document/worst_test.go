package document

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

func TestWorstSeverityIsTheWorstTheViewShows(t *testing.T) {
	for _, name := range goldenNames {
		for _, profile := range []string{"detailed", "executive"} {
			s := goldenSnapshot(t, name)
			s.Report.Profile = profile
			view := mustView(t, s, catalog.English)
			want := analysis.SeverityHealthy
			for _, f := range view.Findings {
				if analysis.Rank(f.Severity) > analysis.Rank(want) {
					want = f.Severity
				}
			}
			got, err := WorstSeverity(s)
			if err != nil || got != want {
				t.Errorf("%s %s: WorstSeverity = %q, %v; the view shows %q", name, profile, got, err, want)
			}
		}
	}
}

func TestWorstSeverityLiterals(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	s.Report.Profile = "detailed"
	if got, err := WorstSeverity(s); err != nil || got != "critical" {
		t.Errorf("edge-scaling detailed = %q, %v", got, err)
	}
	s.Findings = []snapshot.Finding{}
	s.Summary.Findings, s.Summary.Healthy, s.Summary.Attention, s.Summary.Critical = 0, 0, 0, 0
	s.Recommendations = []snapshot.Recommendation{}
	s.Summary.Recommendations = 0
	if got, err := WorstSeverity(s); err != nil || got != "healthy" {
		t.Errorf("no findings = %q, %v; want healthy", got, err)
	}
}
