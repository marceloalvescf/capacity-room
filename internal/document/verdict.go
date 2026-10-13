package document

import (
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
	findings := shownFindings(s, severity)
	facts := decideVerdict(s, findings)

	return Verdict{
		Totals:   verdictTotals(b, s.Summary),
		Quality:  verdictQuality(b, s),
		Growth:   verdictGrowth(b, s, findings, facts),
		Pressure: verdictPressure(b, findings, facts),
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
func verdictGrowth(b *builder, s snapshot.Snapshot, findings []snapshot.Finding, facts verdictFacts) VerdictGrowth {
	a := s.Summary.Autoscaling
	vb := &viewBuilder{b: b}
	growth := VerdictGrowth{Title: b.text(catalog.VerdictGrowthTitle, nil)}

	if facts.ClusterFinding >= 0 {
		worst := findings[facts.ClusterFinding]
		if definition, ok := findingDefinition(b, worst.Code); ok {
			growth.Finding = &VerdictFinding{
				Status:   worst.Severity,
				Label:    statusLabel(b, worst.Severity),
				Sentence: findingSentence(b, definition),
			}
		}
	}
	if facts.EvidenceWarning >= 0 {
		text := warningText(b, s.Quality.Warnings[facts.EvidenceWarning].Code)
		growth.Warning = &text
	}

	if a.Status != collect.AutoscalingEnabled {
		state := b.text(catalog.VerdictGrowthState(a.Status), nil)
		growth.State = &state
		return growth
	}

	current := vb.number(a.CurrentNodes, b.cat.Count).Text
	growth.Current = &current
	maximum := vb.number(a.MaxNodes, b.cat.Count).Text
	growth.Max = &maximum
	caption := b.text(catalog.VerdictGrowthCaption, nil)
	growth.Caption = &caption
	if g := facts.Gauge; g != nil {
		growth.Gauge = &VerdictGauge{Current: g.Current, Max: g.Max, Min: g.Min, Hazard: g.Hazard != ""}
	}
	growth.Facts = []VerdictFact{
		{Label: b.text(catalog.VerdictGrowthMinNodes, nil), Value: vb.number(a.MinNodes, b.cat.Count).Text},
		{Label: b.text(catalog.VerdictGrowthUnschedulablePods, nil), Value: vb.number(a.UnschedulablePods, b.cat.Count).Text},
		{Label: b.text(catalog.VerdictGrowthScaleEvents, nil), Value: vb.number(a.ScaleEvents, b.cat.Count).Text},
	}
	return growth
}

// verdictPressure words the scope types under pressure, the worst first.
func verdictPressure(b *builder, findings []snapshot.Finding, facts verdictFacts) VerdictPressure {
	pressure := VerdictPressure{Title: b.text(catalog.VerdictPressureTitle, nil)}
	for _, f := range facts.Pressure {
		pressure.Rows = append(pressure.Rows, pressureRow(b, f, findings[f.Lead]))
	}
	if len(pressure.Rows) == 0 {
		empty := b.text(catalog.VerdictPressureEmpty, nil)
		pressure.Empty = &empty
	}
	return pressure
}

// pressureRow words one scope type from its facts and its lead finding.
func pressureRow(b *builder, facts pressureFacts, lead snapshot.Finding) VerdictPressureRow {
	row := VerdictPressureRow{
		Status: facts.Severity,
		Label:  statusWord(b, facts.Severity),
		Scope:  b.text(catalog.VerdictPressureScope(facts.ScopeType), nil),
	}
	if definition, ok := findingDefinition(b, lead.Code); ok {
		row.Sentence = findingSentence(b, definition)
	}
	row.Name = []Run{Ident(lead.ScopeName)}
	if facts.ScopeType == analysis.ScopeNodepool {
		row.Name = []Run{poolRun(b, lead.ScopeName)}
	}
	for _, e := range lead.Evidence {
		if e.Name == "peak_percent" {
			row.Peak = &VerdictFact{Label: b.text(catalog.VerdictPressurePeak, nil), Value: evidenceView(b, e).Text}
			break
		}
	}
	if facts.Critical > 0 {
		row.Counts = append(row.Counts, verdictCount(b, analysis.SeverityCritical, float64(facts.Critical)))
	}
	if facts.Attention > 0 {
		row.Counts = append(row.Counts, verdictCount(b, analysis.SeverityAttention, float64(facts.Attention)))
	}
	return row
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
