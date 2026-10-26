package document

import (
	"github.com/marceloalvescf/capacity-room/internal/analysis"
	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
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
// floor, the quality warnings, and the action plan.
func buildVerdict(b *builder, s snapshot.Snapshot, floor int) Verdict {
	findings := shownFindings(s, floor)
	facts := decideVerdict(s, findings)

	return Verdict{
		Totals:   verdictTotals(b, findings, floor),
		Quality:  verdictQuality(b, s),
		Growth:   verdictGrowth(b, s, findings, facts),
		Pressure: verdictPressure(b, findings, facts, floor),
		Findings: verdictFindings(b),
		Plan:     verdictPlan(b, s),
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
	return VerdictCount{Status: status, Label: statusLabel(b, status), Count: b.cat.Count(count), Value: int(count)}
}

// verdictTotals counts the findings shown, by the severities at or above the
// floor; when there is none it holds the sentence that says nothing needs
// attention instead.
func verdictTotals(b *builder, findings []snapshot.Finding, floor int) VerdictTotals {
	counts := shownTotals(findings, floor)
	if totalCount(counts) == 0 {
		none := b.text(catalog.VerdictNone, nil)
		return VerdictTotals{None: &none}
	}
	items := make([]VerdictCount, 0, len(counts))
	for _, c := range counts {
		items = append(items, verdictCount(b, c.Severity, float64(c.Count)))
	}
	return VerdictTotals{Items: items}
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
// maximum of the autoscaler, drawn only when the status is enabled and both
// counts are known, otherwise the state word with the counts that are known;
// the worst finding of the cluster; and the warning that the evidence of the
// nodes is incomplete.
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

	if a.Status != collect.AutoscalingEnabled || a.CurrentNodes == nil || a.MaxNodes == nil {
		state := b.text(catalog.VerdictGrowthState(a.Status), nil)
		growth.State = &state
		if a.Status == collect.AutoscalingEnabled {
			growth.Facts = knownFacts(b, vb, []growthFact{
				{catalog.VerdictGrowthCurrentNodes, a.CurrentNodes},
				{catalog.VerdictGrowthUnschedulablePods, a.UnschedulablePods},
				{catalog.VerdictGrowthScaleEvents, a.ScaleEvents},
			})
		}
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
		if g.Hazard != "" {
			growth.Gauge.Status = &g.Hazard
		}
	}
	growth.Facts = knownFacts(b, vb, []growthFact{
		{catalog.VerdictGrowthMinNodes, a.MinNodes},
		{catalog.VerdictGrowthUnschedulablePods, a.UnschedulablePods},
		{catalog.VerdictGrowthScaleEvents, a.ScaleEvents},
	})
	return growth
}

// growthFact is a count the verdict may state beside the growth of the
// cluster, with the key of its label.
type growthFact struct {
	label catalog.Key
	value *float64
}

// knownFacts words the facts whose count is known, in order, and leaves out
// the others: a report from kube-state-metrics holds no minimum.
func knownFacts(b *builder, vb *viewBuilder, facts []growthFact) []VerdictFact {
	words := []VerdictFact{}
	for _, f := range facts {
		if f.value == nil {
			continue
		}
		words = append(words, VerdictFact{Label: b.text(f.label, nil), Value: vb.number(f.value, b.cat.Count).Text})
	}
	return words
}

// verdictPressure words the scope types that hold a finding, the worst
// first, with a bar of their findings. The healthy count is kept only when
// the floor lets a healthy finding through.
func verdictPressure(b *builder, findings []snapshot.Finding, facts verdictFacts, floor int) VerdictPressure {
	pressure := VerdictPressure{Title: b.text(catalog.VerdictPressureTitle, nil)}
	showHealthy := floor <= analysis.Rank(analysis.SeverityHealthy)
	scale := 0
	for _, f := range facts.Scopes {
		scale = max(scale, f.Critical+f.Attention+f.Healthy)
	}
	for _, f := range facts.Scopes {
		pressure.Rows = append(pressure.Rows, pressureRow(b, f, findings[f.Lead], showHealthy, scale))
	}
	if len(pressure.Rows) == 0 {
		empty := b.text(catalog.VerdictPressureEmpty, nil)
		pressure.Empty = &empty
	}
	return pressure
}

// pressureRow words one scope type from its facts and its lead finding.
func pressureRow(b *builder, facts scopeFacts, lead snapshot.Finding, showHealthy bool, scale int) VerdictPressureRow {
	row := VerdictPressureRow{
		Status: facts.Severity,
		Label:  b.text(catalog.VerdictPressureScope(facts.ScopeType), nil),
		Bar:    VerdictBar{Critical: facts.Critical, Attention: facts.Attention, Healthy: facts.Healthy, Scale: scale},
	}
	if definition, ok := findingDefinition(b, lead.Code); ok {
		row.Sentence = findingSentence(b, definition)
	}
	row.Counts = append(row.Counts,
		verdictCount(b, analysis.SeverityCritical, float64(facts.Critical)),
		verdictCount(b, analysis.SeverityAttention, float64(facts.Attention)))
	if showHealthy {
		row.Counts = append(row.Counts, verdictCount(b, analysis.SeverityHealthy, float64(facts.Healthy)))
	}
	return row
}

// verdictFindings words the donut of the findings.
func verdictFindings(b *builder) VerdictFindings {
	return VerdictFindings{
		Title:      b.text(catalog.VerdictFindingsTitle, nil),
		TotalLabel: b.text(catalog.VerdictFindingsTotal, nil),
	}
}

// verdictPlan holds the first actions of the executive action plan, and the
// line that says how many there are in all when they are more.
func verdictPlan(b *builder, s snapshot.Snapshot) VerdictPlan {
	plan := VerdictPlan{Title: b.text(catalog.VerdictPlanTitle, nil)}
	included := planRecommendations(s, attentionRank)
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
