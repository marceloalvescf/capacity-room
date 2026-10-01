package document

import (
	"fmt"
	"slices"
	"strings"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/arith"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// unknownPool is the name a node gets when no pool could be told.
const unknownPool = "unknown"

// buildNodepoolsSection assembles section 4: a numbered heading, then the
// summary and the profile of the pools whose own findings clear severity,
// then the detail of the nodes whose own findings do. The two filters
// differ: a pool is judged by the findings of the pool, a node by those of
// the node.
func buildNodepoolsSection(b *builder, s snapshot.Snapshot, thresholds config.Thresholds, severity string) []Block {
	minRank := minimumRank(severity)
	nodes := make(map[string]snapshot.Node, len(s.Nodes))
	for _, n := range s.Nodes {
		nodes[n.Name] = n
	}

	var pools []snapshot.NodePool
	for _, pool := range s.Nodepools {
		if analysis.Rank(scopeSeverity(s.Findings, analysis.ScopeNodepool, pool.Name, "")) >= minRank {
			pools = append(pools, pool)
		}
	}

	blocks := []Block{Heading{Level: 1, Numbered: true, Runs: []Run{Text(b.text(catalog.SectionsNodepoolsHeading, nil))}}}
	blocks = append(blocks, poolSummary(b, pools, nodes, thresholds)...)
	blocks = append(blocks, poolProfile(b, pools, nodes)...)
	blocks = append(blocks, nodeDetail(b, s, nodes, minRank, thresholds)...)
	return blocks
}

// numberedHeading returns a numbered level-2 heading holding one run.
func numberedHeading(b *builder, key catalog.Key) Heading {
	return Heading{Level: 2, Numbered: true, Runs: []Run{Text(b.text(key, nil))}}
}

// poolRun writes a pool's name as a real name, or as the placeholder the
// language names when the pool is the unknown one.
func poolRun(b *builder, name string) Run {
	if name == unknownPool {
		return Placeholder(b.text(catalog.PlaceholderUndetermined, nil))
	}
	return Ident(name)
}

// poolNodes returns the nodes a pool lists, in its order.
func poolNodes(pool snapshot.NodePool, nodes map[string]snapshot.Node) []snapshot.Node {
	out := make([]snapshot.Node, 0, len(pool.Nodes))
	for _, name := range pool.Nodes {
		if n, ok := nodes[name]; ok {
			out = append(out, n)
		}
	}
	return out
}

// known returns the number of a statistic that carries a value.
func known(stat snapshot.Statistics, value *float64) (float64, bool) {
	if stat.State == stateValue && value != nil {
		return *value, true
	}
	return 0, false
}

// mean returns the mean of values, or nil when there is none.
func mean(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	m := arith.Mean(values)
	return &m
}

// maximum returns the largest of values, or nil when there is none.
func maximum(values []float64) *float64 {
	if len(values) == 0 {
		return nil
	}
	m := slices.Max(values)
	return &m
}

// percentRun judges a percentage, or marks it unavailable when it is nil.
func percentRun(b *builder, value *float64, thresholds config.Thresholds) Run {
	if value == nil {
		return StatusText("unavailable", b.text(catalog.PlaceholderUnavailable, nil))
	}
	return StatusText(analysis.Classify(*value, thresholds), b.cat.Percent(*value))
}

// poolSummary builds the summary heading and its table, or its paragraph
// when no pool is left.
func poolSummary(b *builder, pools []snapshot.NodePool, nodes map[string]snapshot.Node, thresholds config.Thresholds) []Block {
	heading := numberedHeading(b, catalog.SectionsNodepoolsSummaryHeading)
	if len(pools) == 0 {
		return []Block{heading, Paragraph{Runs: []Run{Text(b.text(catalog.SectionsNodepoolsEmptySummary, nil))}}}
	}

	rows := make([][][]Run, len(pools))
	for i, pool := range pools {
		members := poolNodes(pool, nodes)
		var cpuAverage, cpuPeak, memoryAverage, memoryPeak []float64
		for _, n := range members {
			if v, ok := known(n.CPUUsagePercent, n.CPUUsagePercent.Average); ok {
				cpuAverage = append(cpuAverage, v)
			}
			if v, ok := known(n.CPUUsagePercent, n.CPUUsagePercent.Peak); ok {
				cpuPeak = append(cpuPeak, v)
			}
			if v, ok := known(n.MemoryUsagePercent, n.MemoryUsagePercent.Average); ok {
				memoryAverage = append(memoryAverage, v)
			}
			if v, ok := known(n.MemoryUsagePercent, n.MemoryUsagePercent.Peak); ok {
				memoryPeak = append(memoryPeak, v)
			}
		}
		rows[i] = [][]Run{
			{poolRun(b, pool.Name)},
			{Text(b.cat.Integer(len(members)))},
			{percentRun(b, mean(cpuAverage), thresholds)},
			{percentRun(b, maximum(cpuPeak), thresholds)},
			{percentRun(b, mean(memoryAverage), thresholds)},
			{percentRun(b, maximum(memoryPeak), thresholds)},
			{Text(b.text(catalog.Confidence(pool.Confidence), nil))},
		}
	}
	return []Block{heading, Table{
		ID: "nodepool-summary",
		Headers: []string{
			b.text(catalog.SectionsNodepoolsHeaderPool, nil),
			b.text(catalog.SectionsNodepoolsHeaderNodes, nil),
			b.text(catalog.SectionsNodepoolsHeaderCPUAverage, nil),
			b.text(catalog.SectionsNodepoolsHeaderCPUPeak, nil),
			b.text(catalog.SectionsNodepoolsHeaderMemoryAverage, nil),
			b.text(catalog.SectionsNodepoolsHeaderMemoryPeak, nil),
			b.text(catalog.SectionsNodepoolsHeaderConfidence, nil),
		},
		Rows: rows,
	}}
}

// meanRun writes the mean of a column with format, or the unavailable
// placeholder when no node contributed to it.
func meanRun(b *builder, values []float64, format func(float64) string) Run {
	m := mean(values)
	if m == nil {
		return Text(b.text(catalog.PlaceholderUnavailable, nil))
	}
	return Text(format(*m))
}

// resourceMeans returns the three means of one resource over the nodes of a
// pool: the allocatable, the amount used, and the amount requested. Each
// mean runs over the nodes that hold every number it needs. The amounts are
// the allocatable times the percentage, divided by 100, in that order.
func resourceMeans(members []snapshot.Node, allocatable func(snapshot.Node) *float64, usage func(snapshot.Node) *float64, request func(snapshot.Node) *float64) (allocated, used, requested []float64) {
	for _, n := range members {
		a := allocatable(n)
		if a == nil {
			continue
		}
		allocated = append(allocated, *a)
		if u := usage(n); u != nil {
			used = append(used, *a**u/100)
		}
		if r := request(n); r != nil {
			requested = append(requested, *a**r/100)
		}
	}
	return allocated, used, requested
}

// usageAverage returns the average of a node's usage statistic when it
// carries a value.
func usageAverage(stat snapshot.Statistics) *float64 {
	if stat.State == stateValue {
		return stat.Average
	}
	return nil
}

// poolProfile builds the profile heading and its table, or its paragraph
// when no pool is left.
func poolProfile(b *builder, pools []snapshot.NodePool, nodes map[string]snapshot.Node) []Block {
	heading := numberedHeading(b, catalog.SectionsNodepoolsProfileHeading)
	if len(pools) == 0 {
		return []Block{heading, Paragraph{Runs: []Run{Text(b.text(catalog.SectionsNodepoolsEmptyProfile, nil))}}}
	}

	rows := make([][][]Run, len(pools))
	for i, pool := range pools {
		members := poolNodes(pool, nodes)
		cpuAllocatable, cpuUsed, cpuRequested := resourceMeans(members,
			func(n snapshot.Node) *float64 { return n.CPUAllocatable },
			func(n snapshot.Node) *float64 { return usageAverage(n.CPUUsagePercent) },
			func(n snapshot.Node) *float64 { return n.CPURequestPercent })
		memoryAllocatable, memoryUsed, memoryRequested := resourceMeans(members,
			func(n snapshot.Node) *float64 { return n.MemoryAllocatable },
			func(n snapshot.Node) *float64 { return usageAverage(n.MemoryUsagePercent) },
			func(n snapshot.Node) *float64 { return n.MemoryRequestPercent })
		rows[i] = [][]Run{
			{poolRun(b, pool.Name)},
			{meanRun(b, cpuAllocatable, b.cat.CPU)},
			{meanRun(b, cpuUsed, b.cat.CPU)},
			{meanRun(b, cpuRequested, b.cat.CPU)},
			{meanRun(b, memoryAllocatable, b.cat.Memory)},
			{meanRun(b, memoryUsed, b.cat.Memory)},
			{meanRun(b, memoryRequested, b.cat.Memory)},
		}
	}
	return []Block{heading, Table{
		ID: "nodepool-profile",
		Headers: []string{
			b.text(catalog.SectionsNodepoolsHeaderPool, nil),
			b.text(catalog.SectionsNodepoolsHeaderCPUAllocatable, nil),
			b.text(catalog.SectionsNodepoolsHeaderCPUUsed, nil),
			b.text(catalog.SectionsNodepoolsHeaderCPURequested, nil),
			b.text(catalog.SectionsNodepoolsHeaderMemoryAllocatable, nil),
			b.text(catalog.SectionsNodepoolsHeaderMemoryUsed, nil),
			b.text(catalog.SectionsNodepoolsHeaderMemoryRequested, nil),
		},
		Rows: rows,
	}}
}

// nodeDetail builds the detail heading, then for every pool with a node
// that clears minRank a heading and a table; or one paragraph when no pool
// has one.
func nodeDetail(b *builder, s snapshot.Snapshot, nodes map[string]snapshot.Node, minRank int, thresholds config.Thresholds) []Block {
	blocks := []Block{numberedHeading(b, catalog.SectionsNodepoolsDetailHeading)}
	shown := false
	for _, pool := range s.Nodepools {
		var rows [][][]Run
		for _, n := range poolNodes(pool, nodes) {
			if analysis.Rank(scopeSeverity(s.Findings, analysis.ScopeNode, n.Name, "")) < minRank {
				continue
			}
			rows = append(rows, nodeRow(b, n, s.Findings, thresholds))
		}
		if len(rows) == 0 {
			continue
		}
		shown = true
		blocks = append(blocks,
			Heading{Level: 3, Numbered: false, Runs: []Run{
				Text(b.text(catalog.SectionsNodepoolsPoolLabel, nil) + " "),
				poolRun(b, pool.Name),
			}},
			Table{
				ID: "node-detail",
				Headers: []string{
					b.text(catalog.SectionsNodepoolsHeaderNode, nil),
					b.text(catalog.SectionsNodepoolsHeaderCPUAvgPeak, nil),
					b.text(catalog.SectionsNodepoolsHeaderMemoryAvgPeak, nil),
					b.text(catalog.SectionsNodepoolsHeaderReason, nil),
					b.text(catalog.SectionsNodepoolsHeaderState, nil),
				},
				Rows: rows,
			})
	}
	if !shown {
		blocks = append(blocks, Paragraph{Runs: []Run{Text(b.text(catalog.SectionsNodepoolsEmptyDetail, nil))}})
	}
	return blocks
}

// nodeRow builds one row of the "node-detail" table.
func nodeRow(b *builder, n snapshot.Node, findings []snapshot.Finding, thresholds config.Thresholds) [][]Run {
	return [][]Run{
		{Ident(n.Name)},
		averagePeakCell(b, n.CPUUsagePercent, thresholds),
		averagePeakCell(b, n.MemoryUsagePercent, thresholds),
		{nodeReason(b, n.Name, findings)},
		{Text(stateWord(b, n.Ready, catalog.NodeStateReady, catalog.NodeStateNotReady) + "; " + stateWord(b, n.Schedulable, catalog.NodeStateSchedulable, catalog.NodeStateUnschedulable))},
	}
}

// averagePeakCell writes the average and the peak of a usage statistic as
// two judged percentages with a slash between them.
func averagePeakCell(b *builder, stat snapshot.Statistics, thresholds config.Thresholds) []Run {
	var average, peak *float64
	if v, ok := known(stat, stat.Average); ok {
		average = &v
	}
	if v, ok := known(stat, stat.Peak); ok {
		peak = &v
	}
	return []Run{percentRun(b, average, thresholds), Text(" / "), percentRun(b, peak, thresholds)}
}

// stateWord writes yes or no by flag, or the unavailable placeholder when
// the flag is unknown.
func stateWord(b *builder, flag *bool, yes, no catalog.Key) string {
	switch {
	case flag == nil:
		return b.text(catalog.PlaceholderUnavailable, nil)
	case *flag:
		return b.text(yes, nil)
	default:
		return b.text(no, nil)
	}
}

// nodeReason writes why a node is in the table: no finding gives the
// catalog's text; otherwise the fragments of the findings of the highest
// severity, each once, joined by a semicolon, marked with that severity.
func nodeReason(b *builder, name string, findings []snapshot.Finding) Run {
	var own []snapshot.Finding
	severity := analysis.SeverityHealthy
	for _, f := range findings {
		if f.ScopeType != analysis.ScopeNode || f.ScopeName != name {
			continue
		}
		own = append(own, f)
		if analysis.Rank(f.Severity) > analysis.Rank(severity) {
			severity = f.Severity
		}
	}
	if len(own) == 0 {
		return Text(b.text(catalog.SectionsNodepoolsNoFinding, nil))
	}

	var fragments []string
	for _, f := range own {
		if f.Severity != severity {
			continue
		}
		if fragment := reasonFragment(b, f); !slices.Contains(fragments, fragment) {
			fragments = append(fragments, fragment)
		}
	}
	return StatusText(severity, strings.Join(fragments, "; "))
}

// evidenceNumber returns the number a finding carries under name.
func evidenceNumber(f snapshot.Finding, name string) (float64, bool) {
	for _, e := range f.Evidence {
		if e.Name == name && e.Value.Kind == "number" {
			return e.Value.Number, true
		}
	}
	return 0, false
}

// reasonFragment writes the short reason of a node finding. A usage finding
// gives the peak, or the average when it has no peak; a request finding
// gives its percentage; the two findings without a number give their summary
// without its trailing periods.
func reasonFragment(b *builder, f snapshot.Finding) string {
	resource := "cpu"
	if strings.Contains(f.Code, "_MEMORY_") {
		resource = "memory"
	}

	switch {
	case strings.HasSuffix(f.Code, "_REQUEST_HIGH"):
		if percent, ok := evidenceNumber(f, "percent"); ok {
			return b.text(catalog.Reason("request_high", resource), catalog.Params{"Percent": b.cat.Percent(percent)})
		}
	case strings.Contains(f.Code, "_USAGE_"):
		if peak, ok := evidenceNumber(f, "peak_percent"); ok {
			return b.text(catalog.Reason("usage_peak", resource), catalog.Params{"Percent": b.cat.Percent(peak)})
		}
		if average, ok := evidenceNumber(f, "average_percent"); ok {
			return b.text(catalog.Reason("usage_average", resource), catalog.Params{"Percent": b.cat.Percent(average)})
		}
	case f.Code == "NODE_OVERREQUESTED":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_overrequested", ""), nil), ".")
	case f.Code == "NODE_LOW_UTILIZATION":
		return strings.TrimRight(b.text(catalog.FindingSummary("node_low_utilization", ""), nil), ".")
	}
	if b.err == nil {
		b.err = fmt.Errorf("finding %s has no reason text", f.Code)
	}
	return ""
}
