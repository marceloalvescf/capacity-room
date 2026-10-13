package document

import (
	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// The three states a snapshot.Statistics carries, spec 8.1. internal/collect
// keeps the same three constants, unexported here for the same reason
// build.go's own severities is not read from internal/snapshot: a stored
// snapshot must be readable by a version that never imports that package's
// unexported list.
const (
	stateValue       = "value"
	stateNoBase      = "no_base"
	stateUnavailable = "unavailable"
)

// thresholdsFromSnapshot builds a config.Thresholds from the seven fields of
// a snapshot.Thresholds, field by field: the two types look alike and are
// deliberately not one, the same way snapshot.Scope is not discovery.Scope.
func thresholdsFromSnapshot(t snapshot.Thresholds) config.Thresholds {
	return config.Thresholds{
		WarningPercent:           t.WarningPercent,
		CriticalPercent:          t.CriticalPercent,
		LowUtilizationPercent:    t.LowUtilizationPercent,
		AutoscalerNearMaxPercent: t.AutoscalerNearMaxPercent,
		NodepoolSpreadPoints:     t.NodepoolSpreadPoints,
		BurstyGapPercent:         t.BurstyGapPercent,
		HPAReplicaDivergence:     t.HPAReplicaDivergence,
	}
}

// minimumRank turns a report's minimum severity into the lowest
// analysis.Rank a row must reach to be shown: "all" is the rank of healthy,
// so every row clears it.
func minimumRank(severity string) int {
	switch severity {
	case "critical":
		return analysis.Rank(analysis.SeverityCritical)
	case "attention":
		return analysis.Rank(analysis.SeverityAttention)
	default:
		return analysis.Rank(analysis.SeverityHealthy)
	}
}

// scopeSeverity returns the highest severity of the findings whose scope
// type, name, and kind equal scopeType, scopeName, and scopeKind, and
// healthy when none does.
func scopeSeverity(findings []snapshot.Finding, scopeType, scopeName, scopeKind string) string {
	severity := analysis.SeverityHealthy
	for _, f := range findings {
		if f.ScopeType != scopeType || f.ScopeName != scopeName || f.ScopeKind != scopeKind {
			continue
		}
		if analysis.Rank(f.Severity) > analysis.Rank(severity) {
			severity = f.Severity
		}
	}
	return severity
}

// namespaceHeading returns the level-2 heading a namespace's own table, or
// tables, stand under: the entry sections.namespace_label, one space, and
// the name as a real name.
func namespaceHeading(b *builder, name string) Heading {
	return Heading{Level: 2, Numbered: false, Runs: []Run{
		Text(b.text(catalog.SectionsNamespaceLabel, nil) + " "),
		Ident(name),
	}}
}

// quantityText writes value with format, or the unavailable placeholder
// when stat does not carry a value, or value itself is nil.
func quantityText(b *builder, stat snapshot.Statistics, value *float64, format func(float64) string) string {
	if stat.State == stateValue && value != nil {
		return format(*value)
	}
	return b.text(catalog.PlaceholderUnavailable, nil)
}

// quantityRun is quantityText as a plain run.
func quantityRun(b *builder, stat snapshot.Statistics, value *float64, format func(float64) string) Run {
	return Text(quantityText(b, stat, value, format))
}

// podsText writes a pod count by Count, or the unavailable placeholder when
// it is unknown.
func podsText(b *builder, pods *float64) string {
	if pods == nil {
		return b.text(catalog.PlaceholderUnavailable, nil)
	}
	return b.cat.Count(*pods)
}

// podsRun is podsText as a plain run.
func podsRun(b *builder, pods *float64) Run {
	return Text(podsText(b, pods))
}

// ratioText judges value, the figure a ratio statistic stands for, and
// returns its status and text: a classified percentage when the ratio has
// a value, the "no base" text when it has none to measure against, and the
// unavailable placeholder when the ratio itself was never measured.
func ratioText(b *builder, stat snapshot.Statistics, value *float64, thresholds config.Thresholds) (status, text string) {
	switch {
	case stat.State == stateValue && value != nil:
		return analysis.Classify(*value, thresholds), b.cat.Percent(*value)
	case stat.State == stateNoBase:
		return "undefined", b.text(catalog.PlaceholderNoBase, nil)
	default:
		return "unavailable", b.text(catalog.PlaceholderUnavailable, nil)
	}
}

// ratioRun judges a ratio's peak and returns its one status run.
func ratioRun(b *builder, stat snapshot.Statistics, thresholds config.Thresholds) Run {
	status, text := ratioText(b, stat, stat.Peak, thresholds)
	return StatusValue(status, text, statusWord(b, status))
}

// buildNamespacesSection assembles section 1: a numbered heading, then one
// heading and table for every row whose severity clears severity, in the
// order rows holds them; or, when none clears it, one paragraph that says
// there is no data or that no row reaches the minimum.
func buildNamespacesSection(b *builder, rows []snapshot.Namespace, findings []snapshot.Finding, thresholds config.Thresholds, severity string) []Block {
	heading := Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.SectionsNamespacesHeading, nil))}}

	minRank := minimumRank(severity)
	var shown []Block
	for _, row := range rows {
		if analysis.Rank(scopeSeverity(findings, analysis.ScopeNamespace, row.Name, "")) < minRank {
			continue
		}
		shown = append(shown, namespaceHeading(b, row.Name), namespaceTable(b, row, thresholds))
	}
	if len(shown) == 0 {
		key := catalog.SectionsNamespacesBelowMinimum
		if len(rows) == 0 {
			key = catalog.SectionsNamespacesEmpty
		}
		return []Block{heading, Paragraph{Runs: []Run{Text(b.text(key, nil))}}}
	}
	return append([]Block{heading}, shown...)
}

// namespaceTable builds one "namespace-capacity" table: two rows, CPU then
// memory.
func namespaceTable(b *builder, row snapshot.Namespace, thresholds config.Thresholds) Table {
	return Table{
		ID: "namespace-capacity",
		Headers: []string{
			b.text(catalog.SectionsNamespacesHeaderResource, nil),
			b.text(catalog.SectionsNamespacesHeaderAverage, nil),
			b.text(catalog.SectionsNamespacesHeaderPeak, nil),
			b.text(catalog.SectionsNamespacesHeaderRequest, nil),
			b.text(catalog.SectionsNamespacesHeaderLimit, nil),
			b.text(catalog.SectionsNamespacesHeaderPeakRequest, nil),
			b.text(catalog.SectionsNamespacesHeaderPeakLimit, nil),
		},
		Rows: [][][]Run{
			capacityRow(b, catalog.SectionsNamespacesRowCPU, row.CPUUsage, row.CPURequests, row.CPULimits, row.CPURequestPercent, row.CPULimitPercent, thresholds, b.cat.CPU),
			capacityRow(b, catalog.SectionsNamespacesRowMemory, row.MemoryUsage, row.MemoryRequests, row.MemoryLimits, row.MemoryRequestPercent, row.MemoryLimitPercent, thresholds, b.cat.Memory),
		},
	}
}

// capacityRow builds one row of the "namespace-capacity" table: a resource
// label, the average and the peak of usage, the average of requests and of
// limits, and the two ratio cells.
func capacityRow(
	b *builder, labelKey catalog.Key,
	usage, requests, limits, requestPercent, limitPercent snapshot.Statistics,
	thresholds config.Thresholds, format func(float64) string,
) [][]Run {
	return [][]Run{
		{Text(b.text(labelKey, nil))},
		{quantityRun(b, usage, usage.Average, format)},
		{quantityRun(b, usage, usage.Peak, format)},
		{quantityRun(b, requests, requests.Average, format)},
		{quantityRun(b, limits, limits.Average, format)},
		{ratioRun(b, requestPercent, thresholds)},
		{ratioRun(b, limitPercent, thresholds)},
	}
}

// buildWorkloadsSection assembles section 2: a numbered heading, then one
// heading and table for every namespace that has at least one row whose
// severity clears severity, in the order those namespaces first appear
// among rows; or, when none clears it, the section's own empty paragraph.
func buildWorkloadsSection(b *builder, rows []snapshot.Workload, findings []snapshot.Finding, thresholds config.Thresholds, severity string) []Block {
	heading := Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.SectionsWorkloadsHeading, nil))}}

	minRank := minimumRank(severity)
	var order []string
	grouped := make(map[string][][][]Run)
	for _, row := range rows {
		// A row with no observed usage comes from the coverage inventory
		// alone and has no number to show.
		if !row.ObservedUsage {
			continue
		}
		// A namespace and a workload name are both refused if either holds a
		// slash (internal/collect, internal/snapshot), so this join is the
		// same reversible one internal/analysis builds a finding's scope
		// name from: the two can never collide with a different pair.
		scopeName := row.Namespace + "/" + row.Name
		if analysis.Rank(scopeSeverity(findings, analysis.ScopeWorkload, scopeName, row.Kind)) < minRank {
			continue
		}
		if _, ok := grouped[row.Namespace]; !ok {
			order = append(order, row.Namespace)
		}
		grouped[row.Namespace] = append(grouped[row.Namespace], workloadRow(b, row, thresholds))
	}
	if len(order) == 0 {
		key := catalog.SectionsWorkloadsBelowMinimum
		if len(rows) == 0 {
			key = catalog.SectionsWorkloadsEmpty
		}
		return []Block{heading, Paragraph{Runs: []Run{Text(b.text(key, nil))}}}
	}

	blocks := []Block{heading}
	for _, namespace := range order {
		blocks = append(blocks, namespaceHeading(b, namespace), workloadTable(b, grouped[namespace]))
	}
	return blocks
}

// workloadTable builds one "workload-resources" table out of rows, one per
// workload of a namespace.
func workloadTable(b *builder, rows [][][]Run) Table {
	return Table{
		ID: "workload-resources",
		Headers: []string{
			b.text(catalog.SectionsWorkloadsHeaderWorkload, nil),
			b.text(catalog.SectionsWorkloadsHeaderPods, nil),
			b.text(catalog.SectionsWorkloadsHeaderCPUAverage, nil),
			b.text(catalog.SectionsWorkloadsHeaderCPUPeak, nil),
			b.text(catalog.SectionsWorkloadsHeaderCPURequest, nil),
			b.text(catalog.SectionsWorkloadsHeaderCPULimit, nil),
			b.text(catalog.SectionsWorkloadsHeaderMemoryAverage, nil),
			b.text(catalog.SectionsWorkloadsHeaderMemoryPeak, nil),
			b.text(catalog.SectionsWorkloadsHeaderMemoryRequest, nil),
			b.text(catalog.SectionsWorkloadsHeaderMemoryLimit, nil),
		},
		Rows: rows,
	}
}

// workloadRow builds one row of the "workload-resources" table: the
// workload's name, its pod count, the average and the peak of CPU usage,
// the two CPU ratio cells, the average and the peak of memory usage, and
// the two memory ratio cells.
func workloadRow(b *builder, row snapshot.Workload, thresholds config.Thresholds) [][]Run {
	return [][]Run{
		{Ident(row.Name)},
		{podsRun(b, row.Pods)},
		{quantityRun(b, row.CPUUsage, row.CPUUsage.Average, b.cat.CPU)},
		{quantityRun(b, row.CPUUsage, row.CPUUsage.Peak, b.cat.CPU)},
		{ratioRun(b, row.CPURequestPercent, thresholds)},
		{ratioRun(b, row.CPULimitPercent, thresholds)},
		{quantityRun(b, row.MemoryUsage, row.MemoryUsage.Average, b.cat.Memory)},
		{quantityRun(b, row.MemoryUsage, row.MemoryUsage.Peak, b.cat.Memory)},
		{ratioRun(b, row.MemoryRequestPercent, thresholds)},
		{ratioRun(b, row.MemoryLimitPercent, thresholds)},
	}
}
