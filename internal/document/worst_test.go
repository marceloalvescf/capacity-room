package document

import (
	"slices"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/analysis"
	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
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

// withFindings returns a copy of a golden snapshot that holds only the
// findings of the given severities, with its counts made to agree.
func withFindings(t *testing.T, profile, minSeverity string, keep ...string) snapshot.Snapshot {
	t.Helper()
	s := goldenSnapshot(t, "edge-scaling")
	s.Report.Profile, s.Report.MinSeverity = profile, minSeverity
	var findings []snapshot.Finding
	for _, f := range s.Findings {
		if slices.Contains(keep, f.Severity) {
			findings = append(findings, f)
		}
	}
	s.Findings = findings
	var healthy, attention float64
	for _, f := range findings {
		switch f.Severity {
		case "healthy":
			healthy++
		case "attention":
			attention++
		}
	}
	s.Summary.Findings, s.Summary.Healthy, s.Summary.Attention, s.Summary.Critical = float64(len(findings)), healthy, attention, 0
	afs := make([]analysis.Finding, len(findings))
	for i, f := range findings {
		afs[i] = analysis.Finding{Code: f.Code, Severity: f.Severity, ScopeType: f.ScopeType, ScopeKind: f.ScopeKind, ScopeName: f.ScopeName, Confidence: f.Confidence}
	}
	s.Recommendations = []snapshot.Recommendation{}
	for _, r := range analysis.Recommendations(afs) {
		s.Recommendations = append(s.Recommendations, snapshot.Recommendation{Code: r.Code, Severity: r.Severity, ScopeType: r.ScopeType, Scopes: r.Scopes, Confidence: r.Confidence})
	}
	s.Summary.Recommendations = float64(len(s.Recommendations))
	if healthy == 0 || (len(keep) > 1 && attention == 0) {
		t.Fatalf("the golden snapshot lacks the findings %v", keep)
	}
	return s
}

func TestWorstSeverityAppliesTheFilters(t *testing.T) {
	tests := []struct {
		name, profile, minSeverity string
		keep                       []string
		want                       string
	}{
		{"executive, healthy findings only", "executive", "all", []string{"healthy"}, "healthy"},
		{"detailed, minimum critical hides attention", "detailed", "critical", []string{"attention", "healthy"}, "healthy"},
		{"detailed, minimum all shows attention", "detailed", "all", []string{"attention", "healthy"}, "attention"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := withFindings(t, tt.profile, tt.minSeverity, tt.keep...)
			got, err := WorstSeverity(s)
			if err != nil || got != tt.want {
				t.Errorf("WorstSeverity = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
