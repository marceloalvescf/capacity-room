package document

import (
	"slices"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// deriveRecommendations gives the recommendations a list of findings
// implies, in the form the snapshot stores them.
func deriveRecommendations(findings []snapshot.Finding) []snapshot.Recommendation {
	all := make([]analysis.Finding, len(findings))
	for i, f := range findings {
		all[i] = analysis.Finding{
			Code: f.Code, Severity: f.Severity, ScopeType: f.ScopeType,
			ScopeKind: f.ScopeKind, ScopeName: f.ScopeName, Confidence: f.Confidence,
		}
	}
	out := []snapshot.Recommendation{}
	for _, r := range analysis.Recommendations(all) {
		out = append(out, snapshot.Recommendation{
			Code: r.Code, Severity: r.Severity, ScopeType: r.ScopeType,
			Scopes: append([]string(nil), r.Scopes...), Confidence: r.Confidence,
		})
	}
	return out
}

// executiveSnapshot returns the base snapshot holding workloads and
// findings, with the recommendations the findings imply.
func executiveSnapshot(t *testing.T, workloads []snapshot.Workload, findings ...snapshot.Finding) snapshot.Snapshot {
	t.Helper()
	s := baseSnapshot(t)
	s.Workloads = workloads
	s.Findings = findings
	s.Recommendations = deriveRecommendations(findings)
	return s
}

// podsWorkload returns an observed workload row with a pod count.
func podsWorkload(namespace, kind, name string, pods float64) snapshot.Workload {
	w := unavailableWorkload(namespace, kind, name)
	w.Pods = floatPtr(pods)
	return w
}

// executiveBlocks builds the executive section of s.
func executiveBlocks(t *testing.T, s snapshot.Snapshot, lang catalog.Language, severity string, executiveProfile bool) []Block {
	t.Helper()
	b := &builder{cat: mustCatalog(t, lang)}
	blocks := buildExecutiveSection(b, s, config.Default().Thresholds, severity, executiveProfile)
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
	return blocks
}

// subsection returns the blocks of the n-th level-2 heading of blocks,
// counted from zero, the heading included.
func subsection(t *testing.T, blocks []Block, n int) []Block {
	t.Helper()
	var starts []int
	for i, blk := range blocks {
		if h, ok := blk.(Heading); ok && h.Level == 2 {
			starts = append(starts, i)
		}
	}
	if n >= len(starts) {
		t.Fatalf("section has %d level-2 headings, want one at %d", len(starts), n)
	}
	end := len(blocks)
	if n+1 < len(starts) {
		end = starts[n+1]
	}
	return blocks[starts[n]:end]
}

// catText reads key through cat.
func catText(t *testing.T, cat *catalog.Catalog, key catalog.Key, params catalog.Params) string {
	t.Helper()
	text, err := cat.Text(key, params)
	if err != nil {
		t.Fatalf("Text(%q): %v", key, err)
	}
	return text
}

// pt reads key through the Portuguese catalog.
func pt(t *testing.T, key catalog.Key, params catalog.Params) string {
	t.Helper()
	return catText(t, mustCatalog(t, catalog.PortugueseBrazil), key, params)
}

// reason reads the reason of a family and resource with the percent
// given, through the Portuguese catalog.
func reason(t *testing.T, family, resource, percent string) string {
	t.Helper()
	return pt(t, catalog.Reason(family, resource), catalog.Params{"Percent": percent})
}

// runString shows a run as kind, status, and text.
func runString(r Run) string {
	s := r.kind
	if r.status != "" {
		s += "/" + r.status
	}
	if r.text != nil {
		s += ":" + *r.text
	}
	return s
}

// rowString shows a row as its cells, each the runs joined by a bar.
func rowString(row [][]Run) []string {
	out := make([]string, len(row))
	for i, cell := range row {
		parts := make([]string, len(cell))
		for j, r := range cell {
			parts[j] = runString(r)
		}
		out[i] = strings.Join(parts, "|")
	}
	return out
}

// rowsOf returns the rows of the table of a subsection.
func rowsOf(t *testing.T, sub []Block) [][]string {
	t.Helper()
	table, ok := sub[1].(Table)
	if !ok {
		t.Fatalf("sub[1] is %T, want Table", sub[1])
	}
	out := make([][]string, len(table.Rows))
	for i, row := range table.Rows {
		out[i] = rowString(row)
	}
	return out
}

// assertRows fails unless the table of sub holds want.
func assertRows(t *testing.T, sub []Block, want ...[]string) {
	t.Helper()
	got := rowsOf(t, sub)
	if len(got) != len(want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("row %d = %q, want %q", i, got[i], want[i])
		}
	}
}

// assertFallback fails unless sub is a heading and the paragraph of key.
func assertFallback(t *testing.T, cat *catalog.Catalog, sub []Block, key catalog.Key) {
	t.Helper()
	if len(sub) != 2 {
		t.Fatalf("subsection holds %v, want a heading and a paragraph", blockTypes(sub))
	}
	p, ok := sub[1].(Paragraph)
	if !ok {
		t.Fatalf("sub[1] is %T, want Paragraph", sub[1])
	}
	if got, want := runString(p.Runs[0]), "text:"+catText(t, cat, key, nil); got != want {
		t.Errorf("fallback = %q, want %q", got, want)
	}
}

func TestExecutiveOrder(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	_, s := paritySnapshot(t)
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "all", false)

	h, ok := blocks[0].(Heading)
	if !ok || h.Level != 1 || !h.Numbered || runString(h.Runs[0]) != "text:"+pt(t, catalog.SectionsExecutiveHeading, nil) {
		t.Errorf("blocks[0] = %+v, want the numbered level-1 heading Resumo executivo", blocks[0])
	}
	p, ok := blocks[1].(Paragraph)
	if !ok {
		t.Fatalf("blocks[1] is %T, want Paragraph", blocks[1])
	}
	if want := catText(t, cat, catalog.SectionsExecutiveIntro, catalog.Params{"Period": "24 horas"}); runString(p.Runs[0]) != "text:"+want {
		t.Errorf("intro = %q, want %q", runString(p.Runs[0]), want)
	}

	want := []catalog.Key{
		catalog.SectionsExecutiveCriticalHeading, catalog.SectionsExecutiveAttentionHeading,
		catalog.SectionsExecutiveNamespacesHeading, catalog.SectionsExecutiveScalingHeading,
		catalog.SectionsExecutiveNodepoolsHeading, catalog.SectionsExecutiveActionsHeading,
		catalog.SectionsExecutiveLimitationsHeading,
	}
	var got []string
	for _, blk := range blocks {
		if h, ok := blk.(Heading); ok && h.Level == 2 {
			if !h.Numbered {
				t.Errorf("heading %q is not numbered", runString(h.Runs[0]))
			}
			got = append(got, runString(h.Runs[0]))
		}
	}
	if len(got) != len(want) {
		t.Fatalf("level-2 headings = %q, want %d", got, len(want))
	}
	for i, key := range want {
		if got[i] != "text:"+catText(t, cat, key, nil) {
			t.Errorf("heading %d = %q, want the text of %s", i, got[i], key)
		}
	}
}

func TestExecutiveRaisesAllToAttention(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 3)},
		workloadFinding("WORKLOAD_CPU_REQUEST_HEALTHY", analysis.SeverityHealthy, "payments", "api", "Deployment", parityEvidence(40, 50)...))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "all", false)

	assertFallback(t, cat, subsection(t, blocks, 0), catalog.SectionsExecutiveCriticalEmpty)
	assertFallback(t, cat, subsection(t, blocks, 1), catalog.SectionsExecutiveAttentionEmpty)
	assertFallback(t, cat, subsection(t, blocks, 2), catalog.SectionsExecutiveNamespacesEmpty)
	for _, blk := range blocks {
		if table, ok := blk.(Table); ok {
			for _, row := range table.Rows {
				for _, cell := range row {
					for _, r := range cell {
						if r.status == "healthy" {
							t.Errorf("table %s holds a healthy run %s", table.ID, runString(r))
						}
					}
				}
			}
		}
	}
}

func TestExecutiveAtCriticalOmitsAttention(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 4), podsWorkload("payments", "Deployment", "worker", 2)},
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", analysis.SeverityCritical, "payments", "api", "Deployment", parityEvidence(88, 97)...),
		workloadFinding("WORKLOAD_MEMORY_REQUEST_ATTENTION", analysis.SeverityAttention, "payments", "worker", "Deployment", parityEvidence(60, 75)...))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "critical", false)

	assertRows(t, subsection(t, blocks, 0), []string{"ident:payments", "ident:api", "text:4", "text:" + reason(t, "request_peak", "cpu", "97%")})
	assertFallback(t, cat, subsection(t, blocks, 1), catalog.SectionsExecutiveAttentionEmpty)
	rows := rowsOf(t, subsection(t, blocks, 2))
	if len(rows) != 1 || rows[0][4] != "text:"+pt(t, catalog.RiskCount("critical", 1), nil) {
		t.Errorf("namespaces rows = %q, want one row counting 1 crítica", rows)
	}
}

func TestPriorityRowsConsolidateOneWorkload(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 50)},
		workloadFinding("WORKLOAD_MEMORY_REQUEST_CRITICAL", analysis.SeverityCritical, "payments", "api", "Deployment", number("peak_percent", 131.2)),
		workloadFinding("WORKLOAD_CPU_BURSTY", analysis.SeverityAttention, "payments", "api", "Deployment", number("gap_percent", 96.1)),
		workloadFinding("WORKLOAD_CPU_OVERREQUESTED", analysis.SeverityAttention, "payments", "api", "Deployment", number("average_percent", 7.1)))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	assertRows(t, subsection(t, blocks, 0), []string{
		"ident:payments", "ident:api", "text:50",
		"text:" + strings.Join([]string{reason(t, "request_peak", "memory", "131,2%"), reason(t, "bursty", "cpu", "96,1%"), reason(t, "request_average", "cpu", "7,1%")}, "; "),
	})
	assertFallback(t, cat, subsection(t, blocks, 1), catalog.SectionsExecutiveAttentionEmpty)
}

func TestPriorityRowsTellHomonymsApart(t *testing.T) {
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 4), podsWorkload("payments", "StatefulSet", "api", 3)},
		workloadFinding("WORKLOAD_MEMORY_LIMIT_CRITICAL", analysis.SeverityCritical, "payments", "api", "Deployment", number("peak_percent", 95)),
		workloadFinding("WORKLOAD_CPU_REQUEST_ATTENTION", analysis.SeverityAttention, "payments", "api", "StatefulSet", number("average_percent", 72)))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	assertRows(t, subsection(t, blocks, 0), []string{"ident:payments", "ident:api (Deployment)", "text:4", "text:" + reason(t, "limit_peak", "memory", "95%")})
	assertRows(t, subsection(t, blocks, 1), []string{"ident:payments", "ident:api (StatefulSet)", "text:3", "text:" + reason(t, "request_average", "cpu", "72%")})
}

func TestPriorityRowsWithoutPods(t *testing.T) {
	jobs := unavailableWorkload("payments", "CronJob", "jobs")
	jobs.ObservedUsage = false
	jobs.HPACoverage = snapshot.CoverageUncovered
	s := executiveSnapshot(t, []snapshot.Workload{jobs},
		workloadFinding("WORKLOAD_MEMORY_LIMIT_ATTENTION", analysis.SeverityAttention, "payments", "jobs", "CronJob", number("peak_percent", 78)))

	summaries, err := workloadSummaries(s, minimumRank("attention"))
	if err != nil {
		t.Fatalf("workloadSummaries() error = %v", err)
	}
	if len(summaries) != 1 || summaries[0].Kind != "CronJob" || summaries[0].Pods != nil {
		t.Fatalf("summaries = %+v, want one CronJob with nil pods", summaries)
	}

	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)
	assertRows(t, subsection(t, blocks, 1), []string{"ident:payments", "ident:jobs", "text:" + pt(t, catalog.PlaceholderEmptyCell, nil), "text:" + reason(t, "limit_peak", "memory", "78%")})
}

func TestWorkloadSummariesRefuseAScopeWithoutNamespace(t *testing.T) {
	s := executiveSnapshot(t, nil, snapshot.Finding{
		Code: "WORKLOAD_CPU_REQUEST_CRITICAL", Severity: analysis.SeverityCritical,
		ScopeType: analysis.ScopeWorkload, ScopeKind: "Deployment", ScopeName: "api", Confidence: "exact",
	})
	_, err := workloadSummaries(s, minimumRank("attention"))
	if err == nil || err.Error() != "finding WORKLOAD_CPU_REQUEST_CRITICAL names no namespace" {
		t.Errorf("workloadSummaries() error = %v", err)
	}
}

func TestCoverageCodesNeverReachThePriorityTables(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	worker := podsWorkload("payments", "Deployment", "worker", 2)
	worker.HPACoverage = snapshot.CoverageUncovered
	s := executiveSnapshot(t, []snapshot.Workload{worker},
		workloadFinding("DEPLOYMENT_WITHOUT_HPA", analysis.SeverityAttention, "payments", "worker", "Deployment"))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	assertFallback(t, cat, subsection(t, blocks, 0), catalog.SectionsExecutiveCriticalEmpty)
	assertFallback(t, cat, subsection(t, blocks, 1), catalog.SectionsExecutiveAttentionEmpty)
	assertFallback(t, cat, subsection(t, blocks, 2), catalog.SectionsExecutiveNamespacesEmpty)
}

// namespaceWithLimits returns a namespace row whose limit percent peaks
// are cpu and memory.
func namespaceWithLimits(name string, cpu, memory float64) snapshot.Namespace {
	n := unavailableNamespace(name)
	n.CPULimitPercent = valueStat(cpu/2, cpu)
	n.MemoryLimitPercent = valueStat(memory/2, memory)
	return n
}

func TestNamespacesAtRiskRows(t *testing.T) {
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 4), podsWorkload("stock", "Deployment", "worker", 2)},
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", analysis.SeverityCritical, "payments", "api", "Deployment", number("peak_percent", 97)),
		workloadFinding("WORKLOAD_CPU_REQUEST_ATTENTION", analysis.SeverityAttention, "stock", "worker", "Deployment", number("peak_percent", 75)))
	s.Namespaces = []snapshot.Namespace{namespaceWithLimits("payments", 49.9, 76.8), namespaceWithLimits("stock", 37.9, 45.4)}
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	assertRows(t, subsection(t, blocks, 2),
		[]string{"ident:payments", "status/healthy:49,9%", "status/attention:76,8%", "text:" + reason(t, "request_peak", "cpu", "97%"), "text:1 crítica", "status/critical:" + pt(t, catalog.Priority("critical"), nil)},
		[]string{"ident:stock", "status/healthy:37,9%", "status/healthy:45,4%", "text:" + reason(t, "request_peak", "cpu", "75%"), "text:" + pt(t, catalog.RiskCount("attention", 1), nil), "status/attention:" + pt(t, catalog.Priority("attention"), nil)})
}

func TestNamespacesAtRiskSortsByLargestPeak(t *testing.T) {
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("a", "Deployment", "x", 1), podsWorkload("b", "Deployment", "y", 1)},
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", analysis.SeverityCritical, "a", "x", "Deployment", number("peak_percent", 97)),
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", analysis.SeverityCritical, "b", "y", "Deployment", number("peak_percent", 97)))
	s.Namespaces = []snapshot.Namespace{namespaceWithLimits("a", 90, 10), namespaceWithLimits("b", 20, 95)}
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	rows := rowsOf(t, subsection(t, blocks, 2))
	if len(rows) != 2 || rows[0][0] != "ident:b" || rows[1][0] != "ident:a" {
		t.Errorf("rows = %q, want b before a", rows)
	}
}

func TestNamespacesAtRiskCountsBoth(t *testing.T) {
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 1), podsWorkload("payments", "Deployment", "queue", 1)},
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", analysis.SeverityCritical, "payments", "api", "Deployment", number("peak_percent", 97)),
		workloadFinding("WORKLOAD_CPU_REQUEST_ATTENTION", analysis.SeverityAttention, "payments", "queue", "Deployment", number("peak_percent", 75)))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	rows := rowsOf(t, subsection(t, blocks, 2))
	if len(rows) != 1 || rows[0][4] != "text:"+pt(t, catalog.SectionsExecutiveRiskStatusBoth, catalog.Params{"Critical": pt(t, catalog.RiskCount("critical", 1), nil), "Attention": pt(t, catalog.RiskCount("attention", 1), nil)}) || rows[0][3] != "text:"+reason(t, "request_peak", "cpu", "97%") || rows[0][5] != "status/critical:"+pt(t, catalog.Priority("critical"), nil) {
		t.Errorf("rows = %q, want one row counting 1 crítica · 1 alta, led by api", rows)
	}
}

func TestNamespacesAtRiskUnavailablePercent(t *testing.T) {
	s := executiveSnapshot(t,
		[]snapshot.Workload{podsWorkload("payments", "Deployment", "api", 1)},
		workloadFinding("WORKLOAD_CPU_REQUEST_CRITICAL", analysis.SeverityCritical, "payments", "api", "Deployment", number("peak_percent", 97)))
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	rows := rowsOf(t, subsection(t, blocks, 2))
	if len(rows) != 1 || rows[0][1] != "status/unavailable:"+pt(t, catalog.PlaceholderUnavailable, nil) || rows[0][2] != "status/unavailable:"+pt(t, catalog.PlaceholderUnavailable, nil) {
		t.Errorf("rows = %q, want two unavailable cells", rows)
	}
}

// maxedHPA returns an HPA with the bounds lowest and highest and the peak.
func maxedHPA(name, targetKind, targetName string, lowest, highest, peak float64) snapshot.HPA {
	return snapshot.HPA{
		Namespace: "payments", Name: name, TargetKind: targetKind, TargetName: targetName,
		SpecMin: floatPtr(lowest), SpecMax: floatPtr(highest),
		Current: valueStat(peak-1, peak), Desired: snapshot.Statistics{State: stateUnavailable},
	}
}

// uncoveredDeployment returns a Deployment row no HPA targets.
func uncoveredDeployment(name string, pods *float64, observed bool) snapshot.Workload {
	w := unavailableWorkload("payments", "Deployment", name)
	w.HPACoverage = snapshot.CoverageUncovered
	w.Pods = pods
	w.ObservedUsage = observed
	return w
}

func TestReplicaCeilingRows(t *testing.T) {
	s := executiveSnapshot(t, []snapshot.Workload{podsWorkload("payments", "Deployment", "api", 4), uncoveredDeployment("worker", floatPtr(2), true)})
	s.HPAs = []snapshot.HPA{maxedHPA("api", "Deployment", "api", 2, 10, 10)}
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	assertRows(t, subsection(t, blocks, 3),
		[]string{"ident:payments", "ident:api", "text:Deployment", "text:" + pt(t, catalog.SectionsExecutiveReplicasObserved, catalog.Params{"Peak": "10", "Max": "10"}), "status/critical:" + pt(t, catalog.SectionsExecutiveSituationHPAMax, nil)},
		[]string{"ident:payments", "ident:worker", "text:Deployment", "text:" + pt(t, catalog.SectionsExecutiveActiveReplicasPlural, catalog.Params{"Count": "2"}), "status/attention:" + pt(t, catalog.SectionsHPAsSituationNoHPA, nil)})
}

func TestReplicaCeilingSingularActive(t *testing.T) {
	s := executiveSnapshot(t, []snapshot.Workload{uncoveredDeployment("worker", floatPtr(1), true)})
	rows := rowsOf(t, subsection(t, executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false), 3))
	if len(rows) != 1 || rows[0][3] != "text:"+pt(t, catalog.SectionsExecutiveActiveReplicasSingular, nil) {
		t.Errorf("rows = %q, want the replicas 1 ativo", rows)
	}
}

func TestReplicaCeilingUncoveredWithoutPods(t *testing.T) {
	s := executiveSnapshot(t, []snapshot.Workload{uncoveredDeployment("api", nil, false)})
	rows := rowsOf(t, subsection(t, executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false), 3))
	if len(rows) != 1 || rows[0][3] != "text:"+pt(t, catalog.PlaceholderUnavailable, nil) {
		t.Errorf("rows = %q, want the replicas Indisponível", rows)
	}
}

func TestReplicaCeilingSkipsPinnedAndStatefulSets(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	sts := unavailableWorkload("payments", "StatefulSet", "db")
	sts.HPACoverage = snapshot.CoverageUncovered
	s := executiveSnapshot(t, []snapshot.Workload{sts})
	s.HPAs = []snapshot.HPA{maxedHPA("pinned", "Deployment", "pinned", 1, 1, 1)}
	assertFallback(t, cat, subsection(t, executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false), 3), catalog.SectionsExecutiveScalingEmpty)
}

func TestReplicaCeilingUnidentifiedTarget(t *testing.T) {
	s := executiveSnapshot(t, nil)
	// Listed out of name order on purpose.
	s.HPAs = []snapshot.HPA{maxedHPA("api-hpa", "", "", 2, 10, 10), maxedHPA("api-cpu", "", "svc", 2, 10, 10)}
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false)

	assertRows(t, subsection(t, blocks, 3),
		[]string{"ident:payments", "ident:api-cpu|text: |placeholder:(alvo não identificado)", "text:" + pt(t, catalog.PlaceholderUnavailable, nil), "text:" + pt(t, catalog.SectionsExecutiveReplicasObserved, catalog.Params{"Peak": "10", "Max": "10"}), "status/critical:" + pt(t, catalog.SectionsExecutiveSituationHPAMax, nil)},
		[]string{"ident:payments", "ident:api-hpa|text: |placeholder:(alvo não identificado)", "text:" + pt(t, catalog.PlaceholderUnavailable, nil), "text:" + pt(t, catalog.SectionsExecutiveReplicasObserved, catalog.Params{"Peak": "10", "Max": "10"}), "status/critical:" + pt(t, catalog.SectionsExecutiveSituationHPAMax, nil)})
}

func TestReplicaCeilingDeduplicatesTargets(t *testing.T) {
	s := executiveSnapshot(t, nil)
	s.HPAs = []snapshot.HPA{maxedHPA("a", "Deployment", "api", 2, 10, 10), maxedHPA("b", "Deployment", "api", 2, 10, 10)}
	rows := rowsOf(t, subsection(t, executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", false), 3))
	if len(rows) != 1 {
		t.Errorf("rows = %q, want one", rows)
	}
}

func TestReplicaCeilingAtCriticalHidesUncovered(t *testing.T) {
	s := executiveSnapshot(t, []snapshot.Workload{podsWorkload("payments", "Deployment", "api", 4), uncoveredDeployment("worker", floatPtr(2), true)})
	s.HPAs = []snapshot.HPA{maxedHPA("api", "Deployment", "api", 2, 10, 10)}
	blocks := executiveBlocks(t, s, catalog.PortugueseBrazil, "critical", false)

	assertRows(t, subsection(t, blocks, 3),
		[]string{"ident:payments", "ident:api", "text:Deployment", "text:" + pt(t, catalog.SectionsExecutiveReplicasObserved, catalog.Params{"Peak": "10", "Max": "10"}), "status/critical:" + pt(t, catalog.SectionsExecutiveSituationHPAMax, nil)})
}

func TestReplicaCeilingNotePlacement(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	title := catText(t, cat, catalog.SectionsHPAsNoteIncompleteTitle, nil)
	build := func(withRow, executiveProfile bool) []Block {
		s := executiveSnapshot(t, nil)
		s.Quality.Capabilities[3].State = discovery.StateUnavailable
		if withRow {
			s.HPAs = []snapshot.HPA{maxedHPA("api", "Deployment", "api", 2, 10, 10)}
		}
		return subsection(t, executiveBlocks(t, s, catalog.PortugueseBrazil, "attention", executiveProfile), 3)
	}
	assertCallout := func(blk Block) {
		t.Helper()
		c, ok := blk.(Callout)
		if !ok || runString(c.Title[0]) != "text:"+title {
			t.Errorf("block = %+v, want the callout %q", blk, title)
		}
	}

	for name, executiveProfile := range map[string]bool{"no row, executive": true, "no row, detailed": false} {
		sub := build(false, executiveProfile)
		if got := blockTypes(sub); !slices.Equal(got, []string{"heading", "paragraph", "callout"}) {
			t.Errorf("%s: blocks = %v", name, got)
			continue
		}
		assertCallout(sub[2])
	}

	sub := build(true, true)
	if got := blockTypes(sub); !slices.Equal(got, []string{"heading", "table", "callout"}) {
		t.Fatalf("rows, executive: blocks = %v", got)
	}
	assertCallout(sub[2])

	if got := blockTypes(build(true, false)); !slices.Equal(got, []string{"heading", "table"}) {
		t.Errorf("rows, detailed: blocks = %v", got)
	}
}

// poolStatusRows builds section 5 of a snapshot of nodes and findings and
// returns the rows of its pool table.
func poolStatusRows(t *testing.T, lang catalog.Language, severity string, nodes []snapshot.Node, findings ...snapshot.Finding) [][]string {
	t.Helper()
	s := poolSnapshot(t, nodes, findings...)
	return rowsOf(t, subsection(t, executiveBlocks(t, s, lang, severity, false), 4))
}

func plainNode(name, pool string) snapshot.Node {
	return testNode(name, pool, valueStat(10, 20), valueStat(10, 20))
}

func TestNodepoolStatusRows(t *testing.T) {
	rows := poolStatusRows(t, catalog.PortugueseBrazil, "attention",
		[]snapshot.Node{plainNode("worker-b", "stable"), plainNode("worker-a", "apps")},
		nodeFinding("NODE_MEMORY_USAGE_CRITICAL", analysis.SeverityCritical, "worker-a", number("peak_percent", 91)))

	want := [][]string{
		{"ident:apps", "status/critical:" + pt(t, catalog.Priority("critical"), nil), "text:" + pt(t, catalog.FindingObservation("node_usage", "memory"), nil)},
		{"ident:stable", "status/healthy:" + pt(t, catalog.SectionsExecutivePoolStable, nil), "text:" + pt(t, catalog.PlaceholderEmptyCell, nil)},
	}
	if len(rows) != 2 || !slices.Equal(rows[0], want[0]) || !slices.Equal(rows[1], want[1]) {
		t.Errorf("rows = %q, want %q", rows, want)
	}
}

func TestNodepoolStatusKeepsTheHighestSeverityOnly(t *testing.T) {
	rows := poolStatusRows(t, catalog.PortugueseBrazil, "attention",
		[]snapshot.Node{plainNode("pool-a-1", "pool-a")},
		snapshot.Finding{Code: "NODEPOOL_CPU_UNEVEN", Severity: analysis.SeverityAttention, ScopeType: analysis.ScopeNodepool, ScopeName: "pool-a", Confidence: "exact"},
		snapshot.Finding{Code: "NODEPOOL_MEMORY_UNEVEN", Severity: analysis.SeverityAttention, ScopeType: analysis.ScopeNodepool, ScopeName: "pool-a", Confidence: "exact"},
		nodeFinding("NODE_CPU_REQUEST_HIGH", analysis.SeverityCritical, "pool-a-1", number("percent", 91)))

	want := []string{"ident:pool-a", "status/critical:" + pt(t, catalog.Priority("critical"), nil), "text:" + pt(t, catalog.FindingObservation("node_request_high", "cpu"), nil)}
	if len(rows) != 1 || !slices.Equal(rows[0], want) {
		t.Errorf("rows = %q, want %q", rows, want)
	}
}

func TestNodepoolStatusUndeterminedSortsByItsWord(t *testing.T) {
	attention := func(pool string) snapshot.Finding {
		code := "NODEPOOL_CPU_UNEVEN"
		if pool == unknownPool {
			code = "NODEPOOL_UNKNOWN"
		}
		return snapshot.Finding{Code: code, Severity: analysis.SeverityAttention, ScopeType: analysis.ScopeNodepool, ScopeName: pool, Confidence: "exact"}
	}
	firsts := func(lang catalog.Language, pools ...string) []string {
		var nodes []snapshot.Node
		var findings []snapshot.Finding
		for _, pool := range pools {
			nodes = append(nodes, plainNode("node-"+pool, pool))
			findings = append(findings, attention(pool))
		}
		var out []string
		for _, row := range poolStatusRows(t, lang, "attention", nodes, findings...) {
			out = append(out, row[0])
		}
		return out
	}

	for lang, want := range map[catalog.Language][]string{
		catalog.PortugueseBrazil: {"placeholder:" + pt(t, catalog.PlaceholderUndetermined, nil), "ident:zeta"},
		catalog.English:          {"placeholder:" + catText(t, mustCatalog(t, catalog.English), catalog.PlaceholderUndetermined, nil), "ident:zeta"},
	} {
		if got := firsts(lang, "zeta", unknownPool); !slices.Equal(got, want) {
			t.Errorf("%s: order = %q, want %q", lang, got, want)
		}
	}
	// A capital letter sorts before the word in both languages, because the
	// comparison is by code point.
	if got, want := firsts(catalog.English, "zeta", unknownPool, "Alpha"), []string{"ident:Alpha", "placeholder:" + catText(t, mustCatalog(t, catalog.English), catalog.PlaceholderUndetermined, nil), "ident:zeta"}; !slices.Equal(got, want) {
		t.Errorf("order = %q, want %q", got, want)
	}
}

func TestNodepoolStatusNotFiltered(t *testing.T) {
	rows := poolStatusRows(t, catalog.PortugueseBrazil, "critical", []snapshot.Node{plainNode("n1", "stable")})
	if len(rows) != 1 || rows[0][1] != "status/healthy:"+pt(t, catalog.SectionsExecutivePoolStable, nil) {
		t.Errorf("rows = %q, want the stable pool", rows)
	}
}

func TestNodepoolStatusRefusesAFamilyWithoutObservation(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	poolObservation(b, snapshot.Finding{Code: "HPA_NEAR_MAX_REPLICAS"})
	if b.err == nil || b.err.Error() != "finding HPA_NEAR_MAX_REPLICAS has no observation" {
		t.Errorf("poolObservation() error = %v", b.err)
	}
}

// recommendation returns a recommendation by hand.
func recommendation(code, severity, scopeType string, scopes ...string) snapshot.Recommendation {
	return snapshot.Recommendation{Code: code, Severity: severity, ScopeType: scopeType, Scopes: scopes, Confidence: "exact"}
}

// planRows builds the action plan of recommendations by hand and returns
// its rows, or the builder error.
func planRows(t *testing.T, severity string, recommendations ...snapshot.Recommendation) ([][]string, []Block, error) {
	t.Helper()
	s := baseSnapshot(t)
	s.Recommendations = recommendations
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	blocks := actionPlan(b, s, minimumRank(raiseToAttention(severity)))
	if b.err != nil {
		return nil, blocks, b.err
	}
	if _, ok := blocks[1].(Table); !ok {
		return nil, blocks, nil
	}
	return rowsOf(t, blocks), blocks, nil
}

// action reads the action sentence of a family and resource.
func action(t *testing.T, family, resource string) string {
	t.Helper()
	return catText(t, mustCatalog(t, catalog.PortugueseBrazil), catalog.FindingAction(family, resource), nil)
}

func TestActionPlanRows(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	rows, _, err := planRows(t, "attention",
		recommendation("NAMESPACE_MEMORY_LIMIT_CRITICAL", "critical", "namespace", "payments"),
		recommendation("NAMESPACE_MEMORY_REQUEST_CRITICAL", "critical", "namespace", "payments"),
		recommendation("WORKLOAD_MEMORY_LIMIT_CRITICAL", "critical", "workload", "payments/payments-web"),
		recommendation("WORKLOAD_MEMORY_REQUEST_CRITICAL", "critical", "workload", "payments/payments-web"),
		recommendation("NAMESPACE_CPU_REQUEST_ATTENTION", "attention", "namespace", "payments"),
		recommendation("WORKLOAD_CPU_REQUEST_ATTENTION", "attention", "workload", "payments/payments-web"),
		recommendation("NODE_AUTOSCALING_NEAR_MAX", "attention", "cluster", "cluster"),
		recommendation("HPA_REPLICA_DIVERGENCE", "attention", "hpa", "payments/payments-web"))
	if err != nil {
		t.Fatalf("builder error = %v", err)
	}

	qualified := func(family, resource, qualifier string) string {
		return catText(t, cat, catalog.SectionsExecutiveActionQualified, catalog.Params{"Action": action(t, family, resource), "Qualifier": qualifier})
	}
	critical := "status/critical:" + catText(t, cat, catalog.Priority("critical"), nil)
	attention := "status/attention:" + catText(t, cat, catalog.Priority("attention"), nil)
	namespaces := catText(t, cat, catalog.ScopeQualifier("namespace"), nil)
	workloads := catText(t, cat, catalog.ScopeQualifier("workload"), nil)
	want := [][]string{
		{"text:1", critical, "text:" + qualified("resource_limit_usage", "memory", namespaces), "ident:payments"},
		{"text:2", critical, "text:" + qualified("resource_request_usage", "memory", namespaces), "ident:payments"},
		{"text:3", critical, "text:" + qualified("resource_limit_usage", "memory", workloads), "ident:payments-web"},
		{"text:4", critical, "text:" + qualified("resource_request_usage", "memory", workloads), "ident:payments-web"},
		{"text:5", attention, "text:" + action(t, "hpa_divergence", ""), "ident:payments-web"},
		{"text:6", attention, "text:" + qualified("resource_request_usage", "cpu", namespaces), "ident:payments"},
		{"text:7", attention, "text:" + qualified("resource_request_usage", "cpu", workloads), "ident:payments-web"},
	}
	if len(rows) != len(want) {
		t.Fatalf("rows = %q, want %d", rows, len(want))
	}
	for i := range want {
		if !slices.Equal(rows[i], want[i]) {
			t.Errorf("row %d = %q, want %q", i+1, rows[i], want[i])
		}
	}
}

// edgeScalingRecommendations are the nineteen recommendations of the
// edge-scaling scenario.
func edgeScalingRecommendations() []snapshot.Recommendation {
	return []snapshot.Recommendation{
		recommendation("NODE_CPU_USAGE_CRITICAL", "critical", "node", "aks-agentpool-12345678-vmss000000"),
		recommendation("WORKLOAD_MEMORY_LIMIT_CRITICAL", "critical", "workload", "payments/payments-web"),
		recommendation("WORKLOAD_MEMORY_REQUEST_CRITICAL", "critical", "workload", "payments/payments-web"),
		recommendation("HPA_MAX_REPLICAS_REACHED", "critical", "hpa", "payments/web"),
		recommendation("NODE_CPU_REQUEST_HIGH", "critical", "node", "pool-a-1"),
		recommendation("NAMESPACE_CPU_LIMIT_CRITICAL", "critical", "namespace", "payments"),
		recommendation("NAMESPACE_CPU_REQUEST_CRITICAL", "critical", "namespace", "payments"),
		recommendation("NAMESPACE_MEMORY_LIMIT_CRITICAL", "critical", "namespace", "payments"),
		recommendation("NAMESPACE_MEMORY_REQUEST_CRITICAL", "critical", "namespace", "payments"),
		recommendation("NODE_MEMORY_USAGE_ATTENTION", "attention", "node", "aks-agentpool-12345678-vmss000000"),
		recommendation("DEPLOYMENT_WITHOUT_HPA", "attention", "workload", "payments/api"),
		recommendation("WORKLOAD_CPU_REQUEST_ATTENTION", "attention", "workload", "payments/payments-web"),
		recommendation("NODEPOOL_CPU_UNEVEN", "attention", "nodepool", "pool-a"),
		recommendation("NODEPOOL_MEMORY_UNEVEN", "attention", "nodepool", "pool-a"),
		recommendation("NODE_LOW_UTILIZATION", "attention", "node", "pool-a-1"),
		recommendation("NODE_AUTOSCALING_NEAR_MAX", "attention", "cluster", "cluster"),
		recommendation("HPA_REPLICA_DIVERGENCE", "attention", "hpa", "payments/idle"),
		recommendation("NODE_OVERREQUESTED", "attention", "node", "pool-a-1"),
		recommendation("NODEPOOL_UNKNOWN", "attention", "nodepool", "unknown"),
	}
}

func TestActionPlanQualifiesOnlyIncludedCollisions(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	namespaces := catText(t, cat, catalog.ScopeQualifier("namespace"), nil)
	qualified := catText(t, cat, catalog.SectionsExecutiveActionQualified, catalog.Params{"Action": action(t, "resource_request_usage", "cpu"), "Qualifier": namespaces})

	rows, _, err := planRows(t, "attention", edgeScalingRecommendations()...)
	if err != nil {
		t.Fatalf("builder error = %v", err)
	}
	if len(rows) != 10 {
		t.Fatalf("rows at attention = %d, want 10", len(rows))
	}
	if rows[0][3] != "ident:web" {
		t.Errorf("row 1 scope = %q, want ident:web", rows[0][3])
	}
	if rows[2][2] != "text:"+qualified {
		t.Errorf("row 3 action at attention = %q, want %q", rows[2][2], qualified)
	}
	if rows[1][2] != "text:"+action(t, "resource_limit_usage", "cpu") {
		t.Errorf("row 2 action = %q, want no qualifier", rows[1][2])
	}

	rows, _, err = planRows(t, "critical", edgeScalingRecommendations()...)
	if err != nil {
		t.Fatalf("builder error = %v", err)
	}
	if len(rows) != 7 {
		t.Fatalf("rows at critical = %d, want 7", len(rows))
	}
	if rows[2][2] != "text:"+action(t, "resource_request_usage", "cpu") {
		t.Errorf("row 3 action at critical = %q, want no qualifier", rows[2][2])
	}
}

func TestActionPlanExcludesNodeAndPoolCodes(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	_, blocks, err := planRows(t, "attention",
		recommendation("NODE_MEMORY_USAGE_CRITICAL", "critical", "node", "n1"),
		recommendation("NODEPOOL_MEMORY_UNEVEN", "attention", "nodepool", "pool-a"),
		recommendation("NODE_AUTOSCALING_DISABLED_PRESSURE", "critical", "cluster", "cluster"))
	if err != nil {
		t.Fatalf("builder error = %v", err)
	}
	assertFallback(t, cat, blocks, catalog.SectionsExecutiveActionsEmpty)
}

func TestActionPlanScopeSummary(t *testing.T) {
	four := func(prefix string) []string {
		return []string{prefix + "1", prefix + "2", prefix + "3", prefix + "4"}
	}
	rows, _, err := planRows(t, "attention",
		recommendation("DEPLOYMENT_WITHOUT_HPA", "attention", "workload", four("payments/app-")...),
		recommendation("NAMESPACE_CPU_REQUEST_ATTENTION", "attention", "namespace", four("ns-")...),
		recommendation("HPA_MAX_REPLICAS_REACHED", "critical", "hpa", four("payments/hpa-")...))
	if err != nil {
		t.Fatalf("builder error = %v", err)
	}
	want := []string{"placeholder:" + pt(t, catalog.ScopeSummary("hpa"), catalog.Params{"Count": "4"}), "placeholder:" + pt(t, catalog.ScopeSummary("deployment"), catalog.Params{"Count": "4"}), "placeholder:" + pt(t, catalog.ScopeSummary("namespace"), catalog.Params{"Count": "4"})}
	if len(rows) != 3 {
		t.Fatalf("rows = %q, want 3", rows)
	}
	for i := range want {
		if rows[i][3] != want[i] {
			t.Errorf("row %d scope = %q, want %q", i+1, rows[i][3], want[i])
		}
	}
}

func TestActionPlanScopeNames(t *testing.T) {
	rows, _, err := planRows(t, "attention",
		recommendation("WORKLOAD_CPU_REQUEST_ATTENTION", "attention", "workload", "payments/api", "payments/worker"),
		recommendation("WORKLOAD_MEMORY_LIMIT_ATTENTION", "attention", "workload", "a/api", "b/api"))
	if err != nil {
		t.Fatalf("builder error = %v", err)
	}
	if len(rows) != 2 || rows[0][3] != "ident:api, worker" || rows[1][3] != "ident:a/api, b/api" {
		t.Errorf("scopes = %q, want a/api, b/api and api, worker", rows)
	}
}

func TestActionPlanRefusesAnUnnameableScope(t *testing.T) {
	_, _, err := planRows(t, "attention",
		recommendation("DEPLOYMENT_WITHOUT_HPA", "attention", "cluster", "a", "b", "c", "d"))
	if err == nil || !strings.Contains(err.Error(), `"cluster"`) {
		t.Errorf("builder error = %v, want one naming cluster", err)
	}
}

// availableCapabilities returns the nine capabilities, all available.
func availableCapabilities() []snapshot.Capability {
	caps := baseCapabilities()
	for i := range caps {
		caps[i].State = discovery.StateAvailable
		caps[i].Strategy = discovery.StrategyRawMetrics
		caps[i].Reasons = []string{}
	}
	return caps
}

func limitationRows(t *testing.T, s snapshot.Snapshot, severity string) [][]string {
	t.Helper()
	return rowsOf(t, subsection(t, executiveBlocks(t, s, catalog.PortugueseBrazil, severity, false), 6))
}

func TestMaterialLimitationsRows(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	s := baseSnapshot(t)
	s.Quality.Capabilities = availableCapabilities()
	s.Quality.Capabilities[8].State = discovery.StateUnavailable
	s.Quality.Capabilities[8].Reasons = []string{"NODE_AUTOSCALING_UNDETERMINED"}
	s.Quality.Warnings = []snapshot.Message{{Code: collect.WarningWorkloadHistoryCurrentAssociation, Params: map[string]float64{}}}

	want := [][]string{
		{"text:" + catText(t, cat, catalog.CapabilityName("node_autoscaling"), nil), "status/critical:" + pt(t, catalog.State("unavailable"), nil), "text:" + catText(t, cat, catalog.CapabilityReason("NODE_AUTOSCALING_UNDETERMINED"), nil)},
		{"text:" + pt(t, catalog.SectionsExecutiveInterpretation, nil), "status/attention:" + pt(t, catalog.State("partial"), nil), "text:" + catText(t, cat, catalog.Warning(collect.WarningWorkloadHistoryCurrentAssociation), nil)},
	}
	rows := limitationRows(t, s, "attention")
	if len(rows) != 2 || !slices.Equal(rows[0], want[0]) || !slices.Equal(rows[1], want[1]) {
		t.Errorf("rows = %q, want %q", rows, want)
	}
}

func TestMaterialLimitationsSkipsAWarningThatIsARemark(t *testing.T) {
	s := baseSnapshot(t)
	s.Quality.Capabilities = availableCapabilities()
	s.Quality.Warnings = []snapshot.Message{{Code: collect.WarningHPAAmbiguousSpec, Params: map[string]float64{}}}

	rows := limitationRows(t, s, "attention")
	if len(rows) != 1 || rows[0][1] != "status/attention:"+pt(t, catalog.State("partial"), nil) || strings.HasPrefix(rows[0][0], "text:"+pt(t, catalog.SectionsExecutiveInterpretation, nil)) {
		t.Errorf("rows = %q, want the one effective capability", rows)
	}
}

func TestMaterialLimitationsNotFiltered(t *testing.T) {
	s := baseSnapshot(t)
	s.Quality.Capabilities = availableCapabilities()
	s.Quality.Capabilities[6].State = discovery.StatePartial
	s.Quality.Capabilities[6].Reasons = []string{discovery.ReasonCodes[2]}

	rows := limitationRows(t, s, "critical")
	if len(rows) != 1 || rows[0][1] != "status/attention:"+pt(t, catalog.State("partial"), nil) {
		t.Errorf("rows = %q, want the partial capability", rows)
	}
}

func TestActionPlanRefusesAStatefulSetSummary(t *testing.T) {
	_, _, err := planRows(t, "attention",
		recommendation("STATEFULSET_WITHOUT_HPA_INFO", "attention", "workload", "a/1", "a/2", "a/3", "a/4"))
	want := `recommendation STATEFULSET_WITHOUT_HPA_INFO of scope type "workload" has no summary`
	if err == nil || err.Error() != want {
		t.Errorf("builder error = %v, want %q", err, want)
	}
}

func TestActionPlanRefusesAnUnqualifiableScopeType(t *testing.T) {
	_, _, err := planRows(t, "attention",
		recommendation("WORKLOAD_CPU_REQUEST_ATTENTION", "attention", "node", "a/x"),
		recommendation("NAMESPACE_CPU_REQUEST_ATTENTION", "attention", "node", "a"))
	want := `recommendation of scope type "node" has no qualifier`
	if err == nil || err.Error() != want {
		t.Errorf("builder error = %v, want %q", err, want)
	}
}
