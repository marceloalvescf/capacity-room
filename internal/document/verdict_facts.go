package document

import (
	"cmp"
	"math"
	"slices"

	"github.com/marceloalvescf/capacity-room/internal/analysis"
	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// GaugeHazards are the severities a hazard of the depth gauge can hold.
var GaugeHazards = []string{analysis.SeverityCritical, analysis.SeverityAttention}

// verdictFacts are the decisions of the verdict, before any word: the
// indices point into the findings at or above the minimum and into the
// warnings of the data.
type verdictFacts struct {
	ClusterFinding  int // -1 when the cluster has no finding
	EvidenceWarning int // -1 when the evidence of the nodes is complete
	Gauge           *gaugeFacts
	Pressure        []pressureFacts
	Scopes          []scopeFacts
}

type gaugeFacts struct {
	Current, Max int
	Min          *int
	Hazard       string // "", "attention", or "critical"
}

type pressureFacts struct {
	ScopeType, Severity string
	Lead                int // index of the first finding of Severity in the type
	Critical, Attention int // rows that reach each severity
}

// scopeFacts counts the findings of one scope type, never its rows.
type scopeFacts struct {
	ScopeType, Severity          string
	Lead                         int // index of the first finding of Severity in the type
	Critical, Attention, Healthy int
}

// shownFindings are the findings at or above the floor, in order.
func shownFindings(s snapshot.Snapshot, minRank int) []snapshot.Finding {
	findings := make([]snapshot.Finding, 0, len(s.Findings))
	for _, f := range s.Findings {
		if analysis.Rank(f.Severity) >= minRank {
			findings = append(findings, f)
		}
	}
	return findings
}

// severityCount is the number of shown findings of one severity.
type severityCount struct {
	Severity string
	Count    int
}

// shownTotals counts the findings by the severities at or above the floor,
// in the order critical, attention, healthy.
func shownTotals(findings []snapshot.Finding, minRank int) []severityCount {
	var counts []severityCount
	for _, severity := range []string{analysis.SeverityCritical, analysis.SeverityAttention, analysis.SeverityHealthy} {
		if analysis.Rank(severity) < minRank {
			continue
		}
		n := 0
		for _, f := range findings {
			if f.Severity == severity {
				n++
			}
		}
		counts = append(counts, severityCount{severity, n})
	}
	return counts
}

// totalCount is the sum of counts.
func totalCount(counts []severityCount) int {
	total := 0
	for _, c := range counts {
		total += c.Count
	}
	return total
}

// decideVerdict makes the decisions of the verdict over the shown findings.
func decideVerdict(s snapshot.Snapshot, findings []snapshot.Finding) verdictFacts {
	facts := verdictFacts{
		ClusterFinding: worstClusterFinding(findings),
		EvidenceWarning: slices.IndexFunc(s.Quality.Warnings, func(w snapshot.Message) bool {
			return w.Code == collect.WarningNodeCapacityEvidenceIncomplete
		}),
	}
	a := s.Summary.Autoscaling
	if a.Status == collect.AutoscalingEnabled && a.CurrentNodes != nil && a.MaxNodes != nil && *a.MaxNodes > 0 {
		hazard := ""
		if facts.ClusterFinding >= 0 && findings[facts.ClusterFinding].Severity != analysis.SeverityHealthy {
			hazard = findings[facts.ClusterFinding].Severity
		}
		facts.Gauge = decideGauge(*a.CurrentNodes, *a.MaxNodes, a.MinNodes, hazard)
	}
	for _, scopeType := range verdictScopeTypes {
		if row, ok := decidePressure(scopeType, findings); ok {
			facts.Pressure = append(facts.Pressure, row)
		}
	}
	facts.Scopes = decideScopes(findings)
	slices.SortStableFunc(facts.Pressure, func(a, c pressureFacts) int {
		return analysis.Rank(c.Severity) - analysis.Rank(a.Severity)
	})
	return facts
}

// decideGauge counts whole nodes as the gauge of the web does: the maximum
// is rounded and the level is held between none and the maximum. It is nil
// when the rounded maximum is not above zero. The minimum is kept only
// between none and the maximum.
func decideGauge(current, maximum float64, minimum *float64, hazard string) *gaugeFacts {
	rounded := int(math.Round(maximum))
	if rounded <= 0 {
		return nil
	}
	level := min(max(int(math.Round(current)), 0), rounded)
	gauge := &gaugeFacts{Current: level, Max: rounded, Hazard: hazard}
	if minimum != nil {
		floor := int(math.Round(*minimum))
		if floor > 0 && floor < rounded {
			gauge.Min = &floor
		}
	}
	return gauge
}

// worstClusterFinding returns the index of the first finding of the cluster
// at the worst severity, or -1 when the cluster has none.
func worstClusterFinding(findings []snapshot.Finding) int {
	worst := -1
	for i, f := range findings {
		if f.ScopeType != analysis.ScopeCluster {
			continue
		}
		if worst < 0 || analysis.Rank(f.Severity) > analysis.Rank(findings[worst].Severity) {
			worst = i
		}
	}
	return worst
}

// scopeKey names the row a finding belongs to: its type, its kind for a
// workload alone, and its name.
type scopeKey struct{ scopeType, kind, name string }

func keyOf(f snapshot.Finding) scopeKey {
	kind := ""
	if f.ScopeType == analysis.ScopeWorkload {
		kind = f.ScopeKind
	}
	return scopeKey{f.ScopeType, kind, f.ScopeName}
}

// decidePressure summarises one scope type: its worst severity over its
// rows, the first finding of that severity, and how many rows reach each
// severity above healthy. A type whose worst is healthy has no row.
func decidePressure(scopeType string, findings []snapshot.Finding) (pressureFacts, bool) {
	worstByRow := make(map[scopeKey]string)
	lead := make(map[string]int)
	for i, f := range findings {
		if f.ScopeType != scopeType {
			continue
		}
		if _, seen := lead[f.Severity]; !seen {
			lead[f.Severity] = i
		}
		key := keyOf(f)
		held, seen := worstByRow[key]
		if !seen || analysis.Rank(f.Severity) > analysis.Rank(held) {
			worstByRow[key] = f.Severity
		}
	}

	worst := analysis.SeverityHealthy
	for _, severity := range worstByRow {
		if analysis.Rank(severity) > analysis.Rank(worst) {
			worst = severity
		}
	}
	if worst == analysis.SeverityHealthy {
		return pressureFacts{}, false
	}
	row := pressureFacts{ScopeType: scopeType, Severity: worst, Lead: lead[worst]}
	for _, severity := range worstByRow {
		switch severity {
		case analysis.SeverityCritical:
			row.Critical++
		case analysis.SeverityAttention:
			row.Attention++
		}
	}
	return row, true
}

// decideScopes counts the findings of each scope type but the cluster, by
// severity, with its worst severity and the first finding of that severity.
// A type with no finding has no entry. The worst come first, by severity,
// then by critical and attention findings, then by the order of the types.
func decideScopes(findings []snapshot.Finding) []scopeFacts {
	var scopes []scopeFacts
	for _, scopeType := range verdictScopeTypes {
		row := scopeFacts{ScopeType: scopeType, Lead: -1}
		lead := make(map[string]int)
		for i, f := range findings {
			if f.ScopeType != scopeType {
				continue
			}
			if _, seen := lead[f.Severity]; !seen {
				lead[f.Severity] = i
			}
			switch f.Severity {
			case analysis.SeverityCritical:
				row.Critical++
			case analysis.SeverityAttention:
				row.Attention++
			default:
				row.Healthy++
			}
		}
		for severity, at := range lead {
			if row.Lead < 0 || analysis.Rank(severity) > analysis.Rank(row.Severity) {
				row.Severity, row.Lead = severity, at
			}
		}
		if row.Lead >= 0 {
			scopes = append(scopes, row)
		}
	}
	slices.SortStableFunc(scopes, func(a, c scopeFacts) int {
		return cmp.Or(
			analysis.Rank(c.Severity)-analysis.Rank(a.Severity),
			c.Critical-a.Critical,
			c.Attention-a.Attention,
		)
	})
	return scopes
}

// recommendationFinding is the index in findings of the finding an action
// comes from: the same code and scope type, on the first scope of the
// action. It is nil when there is none.
func recommendationFinding(r snapshot.Recommendation, findings []snapshot.Finding) *int {
	if len(r.Scopes) == 0 {
		return nil
	}
	at := slices.IndexFunc(findings, func(f snapshot.Finding) bool {
		return f.Code == r.Code && f.ScopeType == r.ScopeType && f.ScopeName == r.Scopes[0]
	})
	if at < 0 {
		return nil
	}
	return &at
}
