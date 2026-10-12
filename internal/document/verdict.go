package document

import (
	"math"
	"slices"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// The verdict is the first page of the document. It words the same rules as
// the overview of the web report (its growth block, its cluster verdict, its
// depth gauge, and its pressure by scope), so the page and the screen give
// the same answer for the same snapshot.

const (
	// verdictWarnings is the number of data-quality warnings listed in words.
	verdictWarnings = 3
	// verdictActions is the number of actions of the plan the page holds.
	verdictActions = 5
)

// verdictScopeTypes are the scope types the pressure block summarises, in
// the order that ties are kept.
var verdictScopeTypes = catalog.VerdictPressureScopes

// buildVerdict assembles the first page from the findings at or above the
// minimum severity, the summary, the quality warnings, and the action plan.
func buildVerdict(b *builder, s snapshot.Snapshot, severity string) Verdict {
	minRank := minimumRank(severity)
	findings := make([]snapshot.Finding, 0, len(s.Findings))
	for _, f := range s.Findings {
		if analysis.Rank(f.Severity) >= minRank {
			findings = append(findings, f)
		}
	}

	return Verdict{
		Totals:   verdictTotals(b, s.Summary),
		Quality:  verdictQuality(b, s),
		Growth:   verdictGrowth(b, s, findings),
		Pressure: verdictPressure(b, findings),
		Plan:     verdictPlan(b, s, severity),
		Legend:   verdictLegend(b),
	}
}

// statusLabel writes the word of a status of the totals and the pressure.
func statusLabel(b *builder, status string) string {
	switch status {
	case analysis.SeverityCritical:
		return b.text(catalog.StatusCritical, nil)
	case analysis.SeverityAttention:
		return b.text(catalog.StatusAttention, nil)
	default:
		return b.text(catalog.StatusHealthy, nil)
	}
}

// verdictCount writes one status and a count of it.
func verdictCount(b *builder, status string, count float64) VerdictCount {
	return VerdictCount{Status: status, Label: statusLabel(b, status), Count: b.cat.Count(count)}
}

func verdictTotals(b *builder, m snapshot.Summary) VerdictTotals {
	if m.Critical == 0 && m.Attention == 0 && m.Healthy == 0 {
		none := b.text(catalog.VerdictNone, nil)
		return VerdictTotals{None: &none}
	}
	return VerdictTotals{Items: []VerdictCount{
		verdictCount(b, analysis.SeverityCritical, m.Critical),
		verdictCount(b, analysis.SeverityAttention, m.Attention),
		verdictCount(b, analysis.SeverityHealthy, m.Healthy),
	}}
}

// verdictQuality lists the first warnings of the data in words, or is nil
// when the data carries none.
func verdictQuality(b *builder, s snapshot.Snapshot) *VerdictQuality {
	warnings := s.Quality.Warnings
	if len(warnings) == 0 {
		return nil
	}
	quality := &VerdictQuality{
		Label: b.text(catalog.VerdictQuality, nil),
		Count: b.cat.Integer(len(warnings)),
	}
	for _, w := range warnings[:min(len(warnings), verdictWarnings)] {
		quality.Items = append(quality.Items, warningText(b, w.Code))
	}
	if len(warnings) > verdictWarnings {
		more := b.text(catalog.VerdictMoreWarnings, catalog.Params{"Count": b.cat.Integer(len(warnings) - verdictWarnings)})
		quality.More = &more
	}
	return quality
}

// verdictGrowth words whether the cluster can grow: its nodes against the
// maximum of the autoscaler, the worst finding of the cluster, and the
// warning that the evidence of the nodes is incomplete.
func verdictGrowth(b *builder, s snapshot.Snapshot, findings []snapshot.Finding) VerdictGrowth {
	a := s.Summary.Autoscaling
	vb := &viewBuilder{b: b}
	growth := VerdictGrowth{Title: b.text(catalog.VerdictGrowthTitle, nil)}

	worst := worstClusterFinding(findings)
	if worst != nil {
		if definition, ok := findingDefinition(b, worst.Code); ok {
			growth.Finding = &VerdictFinding{
				Status:   worst.Severity,
				Label:    statusLabel(b, worst.Severity),
				Sentence: findingSentence(b, definition),
			}
		}
	}
	for _, w := range s.Quality.Warnings {
		if w.Code == collect.WarningNodeCapacityEvidenceIncomplete {
			text := warningText(b, w.Code)
			growth.Warning = &text
			break
		}
	}

	if a.Status != collect.AutoscalingEnabled {
		state := b.text(catalog.VerdictGrowthState(a.Status), nil)
		growth.State = &state
		return growth
	}

	if a.CurrentNodes != nil {
		current := vb.number(a.CurrentNodes, b.cat.Count).Text
		growth.Current = &current
	}
	if a.MaxNodes != nil {
		maximum := vb.number(a.MaxNodes, b.cat.Count).Text
		growth.Max = &maximum
	}
	if a.CurrentNodes != nil && a.MaxNodes != nil && *a.MaxNodes > 0 {
		hazard := worst != nil && worst.Severity != analysis.SeverityHealthy
		growth.Gauge = depthGauge(*a.CurrentNodes, *a.MaxNodes, a.MinNodes, hazard)
	}
	if growth.Gauge != nil {
		caption := b.text(catalog.VerdictGrowthCaption, nil)
		growth.Caption = &caption
	}
	growth.Facts = []VerdictFact{
		{Label: b.text(catalog.VerdictGrowthMinNodes, nil), Value: vb.number(a.MinNodes, b.cat.Count).Text},
		{Label: b.text(catalog.VerdictGrowthUnschedulablePods, nil), Value: vb.number(a.UnschedulablePods, b.cat.Count).Text},
		{Label: b.text(catalog.VerdictGrowthScaleEvents, nil), Value: vb.number(a.ScaleEvents, b.cat.Count).Text},
	}
	return growth
}

// depthGauge counts whole nodes as the gauge of the web does: the maximum
// is rounded and the level is held between none and the maximum. It is nil
// when the rounded maximum is not above zero.
func depthGauge(current, maximum float64, minimum *float64, hazard bool) *VerdictGauge {
	rounded := int(math.Round(maximum))
	if rounded <= 0 {
		return nil
	}
	level := min(max(int(math.Round(current)), 0), rounded)
	gauge := &VerdictGauge{Current: level, Max: rounded, Hazard: hazard}
	if minimum != nil {
		floor := int(math.Round(*minimum))
		gauge.Min = &floor
	}
	return gauge
}

// worstClusterFinding returns the first finding of the cluster at the worst
// severity, or nil when the cluster has none.
func worstClusterFinding(findings []snapshot.Finding) *snapshot.Finding {
	var worst *snapshot.Finding
	for i, f := range findings {
		if f.ScopeType != analysis.ScopeCluster {
			continue
		}
		if worst == nil || analysis.Rank(f.Severity) > analysis.Rank(worst.Severity) {
			worst = &findings[i]
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

// verdictPressure lists the scope types with a finding above healthy, the
// worst first and in a fixed order among equals.
func verdictPressure(b *builder, findings []snapshot.Finding) VerdictPressure {
	pressure := VerdictPressure{Title: b.text(catalog.VerdictPressureTitle, nil)}
	for _, scopeType := range verdictScopeTypes {
		if row, ok := pressureRow(b, scopeType, findings); ok {
			pressure.Rows = append(pressure.Rows, row)
		}
	}
	slices.SortStableFunc(pressure.Rows, func(a, c VerdictPressureRow) int {
		return analysis.Rank(c.Status) - analysis.Rank(a.Status)
	})
	if len(pressure.Rows) == 0 {
		empty := b.text(catalog.VerdictPressureEmpty, nil)
		pressure.Empty = &empty
	}
	return pressure
}

// pressureRow summarises one scope type: its worst severity over its rows,
// the first finding of that severity, and how many rows reach each severity
// above healthy. A type whose worst is healthy has no row.
func pressureRow(b *builder, scopeType string, findings []snapshot.Finding) (VerdictPressureRow, bool) {
	worstByRow := make(map[scopeKey]string)
	var own []snapshot.Finding
	for _, f := range findings {
		if f.ScopeType != scopeType {
			continue
		}
		own = append(own, f)
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
		return VerdictPressureRow{}, false
	}
	leadAt := slices.IndexFunc(own, func(f snapshot.Finding) bool { return f.Severity == worst })
	lead := own[leadAt]

	row := VerdictPressureRow{
		Status: worst,
		Scope:  b.text(catalog.VerdictPressureScope(scopeType), nil),
	}
	if definition, ok := findingDefinition(b, lead.Code); ok {
		row.Sentence = findingSentence(b, definition)
	}
	row.Name = []Run{Ident(lead.ScopeName)}
	if scopeType == analysis.ScopeNodepool {
		row.Name = []Run{poolRun(b, lead.ScopeName)}
	}
	for _, e := range lead.Evidence {
		if e.Name == "peak_percent" {
			row.Peak = &VerdictFact{Label: b.text(catalog.VerdictPressurePeak, nil), Value: evidenceView(b, e).Text}
			break
		}
	}
	for _, status := range []string{analysis.SeverityCritical, analysis.SeverityAttention} {
		count := 0
		for _, severity := range worstByRow {
			if severity == status {
				count++
			}
		}
		if count > 0 {
			row.Counts = append(row.Counts, verdictCount(b, status, float64(count)))
		}
	}
	return row, true
}

// verdictPlan holds the first actions of the executive action plan, and the
// line that says how many there are in all when they are more.
func verdictPlan(b *builder, s snapshot.Snapshot, severity string) VerdictPlan {
	plan := VerdictPlan{Title: b.text(catalog.VerdictPlanTitle, nil)}
	included := planRecommendations(s, executiveMinimum(severity))
	shown := included[:min(len(included), verdictActions)]
	plan.Table = newTable(b, "verdict-plan", []catalog.Key{
		catalog.VerdictPlanOrder, catalog.VerdictPlanPriority,
		catalog.VerdictPlanAction, catalog.VerdictPlanScope,
	}, actionRows(b, shown))
	switch {
	case len(included) == 0:
		empty := b.text(catalog.VerdictPlanEmpty, nil)
		plan.Empty = &empty
	case len(included) > len(shown):
		more := b.text(catalog.VerdictPlanMore, catalog.Params{
			"Shown": b.cat.Integer(len(shown)),
			"Total": b.cat.Integer(len(included)),
		})
		plan.More = &more
	}
	return plan
}

// verdictLegend words the five statuses in the order of the legend.
func verdictLegend(b *builder) []VerdictLegendItem {
	items := make([]VerdictLegendItem, len(legendOrder))
	for i, row := range legendOrder {
		items[i] = VerdictLegendItem{Status: row.status, Label: b.text(row.key, nil)}
	}
	return items
}
