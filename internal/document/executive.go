package document

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// The two codes of the coverage inventory: a workload without an HPA is
// the subject of the replica ceiling table, not of the priority tables.
const (
	deploymentWithoutHPA  = "DEPLOYMENT_WITHOUT_HPA"
	statefulSetWithoutHPA = "STATEFULSET_WITHOUT_HPA_INFO"
)

// raiseToAttention returns severity, or attention when it is all: the
// executive body never shows a healthy row.
func raiseToAttention(severity string) string {
	if severity == "all" {
		return analysis.SeverityAttention
	}
	return severity
}

// executiveMinimum is the rank the executive body and the action plan
// clear: the minimum severity, raised to at least attention.
func executiveMinimum(severity string) int {
	return minimumRank(raiseToAttention(severity))
}

// buildExecutiveSection assembles the executive body: the numbered
// heading, the intro, and the seven level-2 numbered subsections, each a
// heading and a table or a fallback paragraph, in the order of the
// original: critical workloads, attention workloads, namespaces at risk,
// replica ceiling, pool situation, action plan, material limitations. The
// minimum is raised to at least attention. In the executive profile the
// note of the HPA data follows the replica ceiling table when it has
// rows; in both profiles it follows the fallback sentence when it has
// none.
func buildExecutiveSection(b *builder, s snapshot.Snapshot, thresholds config.Thresholds, severity string, executiveProfile bool) []Block {
	minRank := executiveMinimum(severity)
	period := b.period(b.time(s.Report.PeriodStart), b.time(s.Report.PeriodEnd))

	summaries, err := workloadSummaries(s, minRank)
	if err != nil && b.err == nil {
		b.err = err
	}

	blocks := []Block{
		Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.SectionsExecutiveHeading, nil))}},
		Paragraph{Runs: []Run{Text(b.text(catalog.SectionsExecutiveIntro, catalog.Params{"Period": period}))}},
	}
	blocks = append(blocks, tableOrFallback(b, catalog.SectionsExecutiveCriticalHeading, catalog.SectionsExecutiveCriticalEmpty, priorityTable(b, priorityRows(b, summaries, analysis.SeverityCritical)))...)

	// The attention table is built only when the raised minimum is
	// attention: at critical it prints its fallback.
	var attention [][][]Run
	if minRank <= analysis.Rank(analysis.SeverityAttention) {
		attention = priorityRows(b, summaries, analysis.SeverityAttention)
	}
	blocks = append(blocks, tableOrFallback(b, catalog.SectionsExecutiveAttentionHeading, catalog.SectionsExecutiveAttentionEmpty, priorityTable(b, attention))...)

	blocks = append(blocks, namespacesAtRisk(b, s, summaries, thresholds)...)
	blocks = append(blocks, replicaCeiling(b, s, minRank, executiveProfile)...)
	blocks = append(blocks, nodepoolStatus(b, s)...)
	blocks = append(blocks, actionPlan(b, s, minRank)...)
	blocks = append(blocks, materialLimitations(b, s)...)
	return blocks
}

// tableOrFallback returns a numbered level-2 heading, then the table
// built by table when it has rows, else the paragraph of the empty key. A
// table with no row is nil.
func tableOrFallback(b *builder, heading, empty catalog.Key, table *Table) []Block {
	blocks := []Block{numberedHeading(b, heading)}
	if table == nil {
		return append(blocks, Paragraph{Runs: []Run{Text(b.text(empty, nil))}})
	}
	return append(blocks, *table)
}

// newTable returns a table of id with the headers named by keys, or nil
// when it has no row.
func newTable(b *builder, id string, headers []catalog.Key, rows [][][]Run) *Table {
	if len(rows) == 0 {
		return nil
	}
	texts := make([]string, len(headers))
	for i, key := range headers {
		texts[i] = b.text(key, nil)
	}
	return &Table{ID: id, Headers: texts, Rows: rows}
}

// priorityTable returns the table of the priority workloads, or nil.
func priorityTable(b *builder, rows [][][]Run) *Table {
	return newTable(b, "priority-workloads", []catalog.Key{
		catalog.SectionsExecutiveHeaderNamespace, catalog.SectionsExecutiveHeaderWorkload,
		catalog.SectionsExecutiveHeaderPods, catalog.SectionsExecutiveHeaderReason,
	}, rows)
}

// workloadSummary is one workload of the priority tables.
type workloadSummary struct {
	Namespace, Kind, Name string
	Pods                  *float64
	Severity              string
	Findings              []snapshot.Finding // severity descending, then code
}

// workloadSummaries groups the workload findings at or above attention,
// the two coverage codes left out, by scope name and kind, in that
// order; keeps the groups whose highest severity reaches minRank; names
// each by the row of the snapshot it matches, or by the finding's own
// kind with no pods when no observed row matches; and returns them
// sorted by namespace, kind, name. It follows _workload_summaries of the
// original.
func workloadSummaries(s snapshot.Snapshot, minRank int) ([]workloadSummary, error) {
	type scope struct{ name, kind string }
	groups := make(map[scope][]snapshot.Finding)
	for _, f := range s.Findings {
		if f.ScopeType != analysis.ScopeWorkload || analysis.Rank(f.Severity) < analysis.Rank(analysis.SeverityAttention) {
			continue
		}
		if f.Code == deploymentWithoutHPA || f.Code == statefulSetWithoutHPA {
			continue
		}
		if !strings.Contains(f.ScopeName, "/") {
			return nil, fmt.Errorf("finding %s names no namespace", f.Code)
		}
		key := scope{f.ScopeName, f.ScopeKind}
		groups[key] = append(groups[key], f)
	}

	scopes := make([]scope, 0, len(groups))
	for key := range groups {
		scopes = append(scopes, key)
	}
	slices.SortFunc(scopes, func(a, c scope) int {
		return cmp.Or(strings.Compare(a.name, c.name), strings.Compare(a.kind, c.kind))
	})

	var summaries []workloadSummary
	for _, key := range scopes {
		findings := groups[key]
		sort.SliceStable(findings, func(i, j int) bool {
			if ri, rj := analysis.Rank(findings[i].Severity), analysis.Rank(findings[j].Severity); ri != rj {
				return ri > rj
			}
			return findings[i].Code < findings[j].Code
		})
		severity := findings[0].Severity
		if analysis.Rank(severity) < minRank {
			continue
		}

		namespace, name, _ := strings.Cut(key.name, "/")
		summary := workloadSummary{Namespace: namespace, Kind: key.kind, Name: name, Severity: severity, Findings: findings}
		for _, w := range s.Workloads {
			if w.ObservedUsage && w.Namespace == namespace && w.Name == name && w.Kind == key.kind {
				summary.Pods = w.Pods
			}
		}
		summaries = append(summaries, summary)
	}

	slices.SortStableFunc(summaries, func(a, c workloadSummary) int {
		return cmp.Or(strings.Compare(a.Namespace, c.Namespace), strings.Compare(a.Kind, c.Kind), strings.Compare(a.Name, c.Name))
	})
	return summaries, nil
}

// priorityRows writes the rows of the summaries whose severity is severity:
// the namespace, the workload, its pods, and its reason. A workload whose
// name another summary shares in the same namespace carries its kind.
func priorityRows(b *builder, summaries []workloadSummary, severity string) [][][]Run {
	type identity struct{ namespace, name string }
	counts := make(map[identity]int)
	for _, w := range summaries {
		counts[identity{w.Namespace, w.Name}]++
	}

	var rows [][][]Run
	for _, w := range summaries {
		if w.Severity != severity {
			continue
		}
		name := w.Name
		if counts[identity{w.Namespace, w.Name}] > 1 {
			name += " (" + w.Kind + ")"
		}
		pods := Text(b.text(catalog.PlaceholderEmptyCell, nil))
		if w.Pods != nil {
			pods = Text(b.cat.Count(*w.Pods))
		}
		rows = append(rows, [][]Run{
			{Ident(w.Namespace)},
			{Ident(name)},
			{pods},
			{Text(workloadReason(b, w.Findings))},
		})
	}
	return rows
}

// namespaceRisk is one namespace of the namespaces-at-risk table.
type namespaceRisk struct {
	name        string
	dominant    workloadSummary
	critical    int
	attention   int
	cpu, memory *float64
	largest     float64
	severity    string
}

// namespacesAtRisk groups the summaries by namespace and writes one row
// for each: the two limit peaks, the reason of the dominant workload, the
// count of workloads by severity, and the priority. Highest severity
// first, then the largest available limit peak, then the name.
func namespacesAtRisk(b *builder, s snapshot.Snapshot, summaries []workloadSummary, thresholds config.Thresholds) []Block {
	var risks []*namespaceRisk
	byName := make(map[string]*namespaceRisk)
	for _, w := range summaries {
		risk, ok := byName[w.Namespace]
		if !ok {
			risk = &namespaceRisk{name: w.Namespace, dominant: w, severity: w.Severity}
			byName[w.Namespace] = risk
			risks = append(risks, risk)
		}
		if analysis.Rank(w.Severity) > analysis.Rank(risk.dominant.Severity) {
			risk.dominant, risk.severity = w, w.Severity
		}
		if w.Severity == analysis.SeverityCritical {
			risk.critical++
		} else {
			risk.attention++
		}
	}

	for _, risk := range risks {
		risk.largest = math.Inf(-1)
		for _, n := range s.Namespaces {
			if n.Name != risk.name {
				continue
			}
			if v, ok := known(n.CPULimitPercent, n.CPULimitPercent.Peak); ok {
				risk.cpu = &v
			}
			if v, ok := known(n.MemoryLimitPercent, n.MemoryLimitPercent.Peak); ok {
				risk.memory = &v
			}
		}
		for _, v := range []*float64{risk.cpu, risk.memory} {
			if v != nil {
				risk.largest = math.Max(risk.largest, *v)
			}
		}
	}
	slices.SortStableFunc(risks, func(a, c *namespaceRisk) int {
		return cmp.Or(
			-cmp.Compare(analysis.Rank(a.severity), analysis.Rank(c.severity)),
			-cmp.Compare(a.largest, c.largest),
			strings.Compare(a.name, c.name))
	})

	var rows [][][]Run
	for _, risk := range risks {
		rows = append(rows, [][]Run{
			{Ident(risk.name)},
			{percentRun(b, risk.cpu, thresholds)},
			{percentRun(b, risk.memory, thresholds)},
			{Text(workloadReason(b, risk.dominant.Findings))},
			{Text(riskStatus(b, risk.critical, risk.attention))},
			{StatusText(risk.severity, b.text(catalog.Priority(risk.severity), nil))},
		})
	}
	return tableOrFallback(b, catalog.SectionsExecutiveNamespacesHeading, catalog.SectionsExecutiveNamespacesEmpty,
		newTable(b, "namespaces-at-risk", []catalog.Key{
			catalog.SectionsExecutiveHeaderNamespace, catalog.SectionsExecutiveHeaderCPULimit,
			catalog.SectionsExecutiveHeaderMemoryLimit, catalog.SectionsExecutiveHeaderReason,
			catalog.SectionsExecutiveHeaderStatus, catalog.SectionsExecutiveHeaderPriority,
		}, rows))
}

// riskStatus writes the counts of critical and attention workloads of a
// namespace, each when it is not zero, joined by the entry for both.
func riskStatus(b *builder, critical, attention int) string {
	count := func(severity string, n int) string {
		if n == 1 {
			return b.text(catalog.RiskCount(severity, 1), nil)
		}
		return b.text(catalog.RiskCount(severity, float64(n)), catalog.Params{"Count": b.cat.Integer(n)})
	}
	switch {
	case critical > 0 && attention > 0:
		return b.text(catalog.SectionsExecutiveRiskStatusBoth, catalog.Params{
			"Critical":  count(analysis.SeverityCritical, critical),
			"Attention": count(analysis.SeverityAttention, attention),
		})
	case critical > 0:
		return count(analysis.SeverityCritical, critical)
	case attention > 0:
		return count(analysis.SeverityAttention, attention)
	}
	return ""
}

// replicaCeiling writes the HPAs at their maximum, then, when the raised
// minimum is attention, the Deployments no HPA targets. The note of the
// HPA data follows the table in the executive profile and the fallback
// sentence in both. It follows _scaling_risk_rows of the original.
func replicaCeiling(b *builder, s snapshot.Snapshot, minRank int, executiveProfile bool) []Block {
	type identity struct{ namespace, kind, name string }
	type candidate struct {
		hpa        snapshot.HPA
		kind, name string
		unknown    bool
	}
	unavailable := b.text(catalog.PlaceholderUnavailable, nil)

	var candidates []candidate
	for _, h := range s.HPAs {
		if !hpaAtMax(h) {
			continue
		}
		c := candidate{hpa: h, kind: h.TargetKind, name: h.TargetName}
		if h.TargetKind == "" || h.TargetName == "" {
			c.kind, c.unknown = unavailable, true
			c.name = h.Name + " " + b.text(catalog.SectionsExecutiveTargetUnknown, nil)
		}
		candidates = append(candidates, c)
	}
	slices.SortStableFunc(candidates, func(a, c candidate) int {
		return cmp.Or(
			strings.Compare(a.hpa.Namespace, c.hpa.Namespace),
			strings.Compare(a.kind, c.kind),
			strings.Compare(a.name, c.name),
			strings.Compare(a.hpa.Name, c.hpa.Name))
	})

	var rows [][][]Run
	seen := make(map[identity]bool)
	for _, c := range candidates {
		id := identity{c.hpa.Namespace, c.kind, c.name}
		if seen[id] {
			continue
		}
		seen[id] = true
		target := []Run{Ident(c.name)}
		if c.unknown {
			target = []Run{Ident(c.hpa.Name), Text(" "), Placeholder(b.text(catalog.SectionsExecutiveTargetUnknown, nil))}
		}
		rows = append(rows, [][]Run{
			{Ident(c.hpa.Namespace)},
			target,
			{Text(c.kind)},
			{Text(b.text(catalog.SectionsExecutiveReplicasObserved, catalog.Params{
				"Peak": b.cat.Count(*c.hpa.Current.Peak),
				"Max":  b.cat.Count(*c.hpa.SpecMax),
			}))},
			{StatusText("critical", b.text(catalog.SectionsExecutiveSituationHPAMax, nil))},
		})
	}

	if minRank <= analysis.Rank(analysis.SeverityAttention) {
		for _, w := range s.Workloads {
			id := identity{w.Namespace, w.Kind, w.Name}
			if w.HPACoverage != snapshot.CoverageUncovered || w.Kind != deploymentKind || seen[id] {
				continue
			}
			seen[id] = true
			rows = append(rows, [][]Run{
				{Ident(w.Namespace)},
				{Ident(w.Name)},
				{Text(w.Kind)},
				{Text(activeReplicas(b, w.Pods))},
				{StatusText("attention", b.text(catalog.SectionsHPAsSituationNoHPA, nil))},
			})
		}
	}

	table := newTable(b, "replica-ceiling", []catalog.Key{
		catalog.SectionsExecutiveHeaderNamespace, catalog.SectionsExecutiveHeaderWorkload,
		catalog.SectionsExecutiveHeaderKind, catalog.SectionsExecutiveHeaderReplicas,
		catalog.SectionsExecutiveHeaderSituation,
	}, rows)
	blocks := tableOrFallback(b, catalog.SectionsExecutiveScalingHeading, catalog.SectionsExecutiveScalingEmpty, table)
	if table == nil || executiveProfile {
		if note := hpaNote(b, s); note != nil {
			blocks = append(blocks, *note)
		}
	}
	return blocks
}

// activeReplicas writes the replicas of a workload no HPA targets: the pod
// count with its unit, or the unavailable placeholder.
func activeReplicas(b *builder, pods *float64) string {
	switch {
	case pods == nil:
		return b.text(catalog.PlaceholderUnavailable, nil)
	case math.RoundToEven(*pods) == 1:
		return b.text(catalog.SectionsExecutiveActiveReplicasSingular, nil)
	default:
		return b.text(catalog.SectionsExecutiveActiveReplicasPlural, catalog.Params{"Count": b.cat.Count(*pods)})
	}
}

// nodepoolStatus writes one row for every pool: its name, the highest
// severity among its own findings and those of its nodes, and the
// observations of the findings at that severity. Highest severity first,
// then the name as the reader sees it. No minimum filters it.
func nodepoolStatus(b *builder, s snapshot.Snapshot) []Block {
	type poolRow struct {
		name, displayed string
		severity        string
		cells           [][]Run
	}
	var pools []poolRow
	for _, pool := range s.Nodepools {
		var own []snapshot.Finding
		severity := analysis.SeverityHealthy
		for _, f := range s.Findings {
			inPool := (f.ScopeType == analysis.ScopeNodepool && f.ScopeName == pool.Name) ||
				(f.ScopeType == analysis.ScopeNode && slices.Contains(pool.Nodes, f.ScopeName))
			if !inPool {
				continue
			}
			own = append(own, f)
			if analysis.Rank(f.Severity) > analysis.Rank(severity) {
				severity = f.Severity
			}
		}

		var observations []string
		if severity != analysis.SeverityHealthy {
			for _, f := range own {
				if f.Severity != severity {
					continue
				}
				if text := poolObservation(b, f); !slices.Contains(observations, text) {
					observations = append(observations, text)
				}
			}
		}

		label := catalog.SectionsExecutivePoolStable
		switch severity {
		case analysis.SeverityAttention:
			label = catalog.SectionsExecutivePoolAttention
		case analysis.SeverityCritical:
			label = catalog.SectionsExecutivePoolCritical
		}
		remark := b.text(catalog.PlaceholderEmptyCell, nil)
		if len(observations) > 0 {
			remark = strings.Join(observations, "; ")
		}
		displayed := pool.Name
		if pool.Name == unknownPool {
			displayed = b.text(catalog.PlaceholderUndetermined, nil)
		}
		pools = append(pools, poolRow{pool.Name, displayed, severity, [][]Run{
			{poolRun(b, pool.Name)},
			{StatusText(severity, b.text(label, nil))},
			{Text(remark)},
		}})
	}
	slices.SortStableFunc(pools, func(a, c poolRow) int {
		return cmp.Or(-cmp.Compare(analysis.Rank(a.severity), analysis.Rank(c.severity)), strings.Compare(a.displayed, c.displayed))
	})

	rows := make([][][]Run, len(pools))
	for i, p := range pools {
		rows[i] = p.cells
	}
	return tableOrFallback(b, catalog.SectionsExecutiveNodepoolsHeading, catalog.SectionsNodepoolsEmptySummary,
		newTable(b, "nodepool-status", []catalog.Key{
			catalog.SectionsExecutiveHeaderPool, catalog.SectionsExecutiveHeaderPoolSituation,
			catalog.SectionsExecutiveHeaderRemark,
		}, rows))
}

// poolObservation writes the short observation of a node or pool finding,
// from the entry of its family. A family with no observation keeps an
// error.
func poolObservation(b *builder, f snapshot.Finding) string {
	definition, ok := analysis.Lookup(f.Code)
	if !ok {
		if b.err == nil {
			b.err = fmt.Errorf("finding %s is not in the registry", f.Code)
		}
		return ""
	}
	if !slices.Contains(catalog.ObservationFamilies, definition.Family) {
		if b.err == nil {
			b.err = fmt.Errorf("finding %s has no observation", f.Code)
		}
		return ""
	}
	return b.text(catalog.FindingObservation(definition.Family, definition.Params["resource"]), nil)
}

// planRecommendations returns the recommendations of the action plan: those
// at or above minRank that are not about a node or a pool, severity first,
// then code, then scopes.
func planRecommendations(s snapshot.Snapshot, minRank int) []snapshot.Recommendation {
	var included []snapshot.Recommendation
	for _, r := range s.Recommendations {
		if analysis.Rank(r.Severity) < minRank || strings.HasPrefix(r.Code, "NODE_") || strings.HasPrefix(r.Code, "NODEPOOL_") {
			continue
		}
		included = append(included, r)
	}
	slices.SortStableFunc(included, func(a, c snapshot.Recommendation) int {
		return cmp.Or(
			-cmp.Compare(analysis.Rank(a.Severity), analysis.Rank(c.Severity)),
			strings.Compare(a.Code, c.Code),
			slices.Compare(a.Scopes, c.Scopes))
	})
	return included
}

// actionPlan writes the rows of planRecommendations. It follows
// _recommendation_rows of the original.
func actionPlan(b *builder, s snapshot.Snapshot, minRank int) []Block {
	rows := actionRows(b, planRecommendations(s, minRank))
	return tableOrFallback(b, catalog.SectionsExecutiveActionsHeading, catalog.SectionsExecutiveActionsEmpty,
		newTable(b, "action-plan", []catalog.Key{
			catalog.VerdictPlanOrder, catalog.VerdictPlanPriority,
			catalog.VerdictPlanAction, catalog.VerdictPlanScope,
		}, rows))
}

// actionRows writes one row for each recommendation: its number, priority,
// action, and scope.
func actionRows(b *builder, included []snapshot.Recommendation) [][][]Run {
	actions := make([]string, len(included))
	for i, r := range included {
		definition, ok := analysis.Lookup(r.Code)
		if !ok {
			if b.err == nil {
				b.err = fmt.Errorf("recommendation %s is not in the registry", r.Code)
			}
			return nil
		}
		actions[i] = b.text(catalog.FindingAction(definition.Family, definition.Params["resource"]), nil)
	}

	var rows [][][]Run
	for i, r := range included {
		rows = append(rows, [][]Run{
			{Text(b.cat.Integer(i + 1))},
			{StatusText(r.Severity, b.text(catalog.Priority(r.Severity), nil))},
			{Text(actions[i])},
			{recommendationScope(b, r)},
		})
	}
	return rows
}

// recommendationScope writes the scopes of a recommendation: with up to
// three, their full names as one identifier; with more, a summary of how many
// there are.
func recommendationScope(b *builder, r snapshot.Recommendation) Run {
	if len(r.Scopes) <= 3 {
		return Ident(strings.Join(r.Scopes, ", "))
	}

	kind := r.ScopeType
	if kind == analysis.ScopeWorkload && strings.HasPrefix(r.Code, "DEPLOYMENT_") {
		kind = "deployment"
	}
	// A StatefulSet recommendation has no summary: its only code is healthy
	// and never recommended.
	if !slices.Contains(catalog.ScopeSummaries, kind) || strings.HasPrefix(r.Code, "STATEFULSET_") {
		if b.err == nil {
			b.err = fmt.Errorf("recommendation %s of scope type %q has no summary", r.Code, r.ScopeType)
		}
		return Placeholder("")
	}
	return Placeholder(b.text(catalog.ScopeSummary(kind), catalog.Params{"Count": b.cat.Integer(len(r.Scopes))}))
}

// materialLimitations writes one row for every capability that is not
// available, then one for every warning whose text no row printed. No
// minimum filters it.
func materialLimitations(b *builder, s snapshot.Snapshot) []Block {
	var rows [][][]Run
	var remarks []string
	for _, c := range effectiveCapabilities(s) {
		if c.State == discovery.StateAvailable {
			continue
		}
		remark := appendixRemark(b, c.Reasons)
		remarks = append(remarks, remark)
		rows = append(rows, [][]Run{
			{Text(b.text(catalog.CapabilityName(c.Name), nil))},
			{StatusText(appendixStatus(c.State), b.text(catalog.State(c.State), nil))},
			{Text(remark)},
		})
	}
	for _, w := range s.Quality.Warnings {
		text := warningText(b, w.Code)
		if slices.Contains(remarks, text) {
			continue
		}
		rows = append(rows, [][]Run{
			{Text(b.text(catalog.SectionsExecutiveInterpretation, nil))},
			{StatusText("attention", b.text(catalog.State("partial"), nil))},
			{Text(text)},
		})
	}
	return tableOrFallback(b, catalog.SectionsExecutiveLimitationsHeading, catalog.SectionsExecutiveLimitationsEmpty,
		newTable(b, "material-limitations", []catalog.Key{
			catalog.SectionsExecutiveHeaderCapability, catalog.SectionsExecutiveHeaderCoverage,
			catalog.SectionsExecutiveHeaderRemark,
		}, rows))
}
