package document

import (
	"slices"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

func boolPtr(value bool) *bool { return &value }

// testNode returns a node of pool with the usage statistics given, an
// exact pool, ready and schedulable, and nothing else known.
func testNode(name, pool string, cpu, memory snapshot.Statistics) snapshot.Node {
	return snapshot.Node{
		Name: name, Pool: pool, PoolConfidence: "exact",
		CPUUsagePercent: cpu, MemoryUsagePercent: memory,
		Ready: boolPtr(true), Schedulable: boolPtr(true),
	}
}

// nodeFinding returns a finding of a node with its evidence.
func nodeFinding(code, severity, node string, evidence ...snapshot.Evidence) snapshot.Finding {
	return snapshot.Finding{Code: code, Severity: severity, ScopeType: analysis.ScopeNode, ScopeName: node, Evidence: evidence, Confidence: "exact"}
}

func number(name string, value float64) snapshot.Evidence {
	return snapshot.Evidence{Name: name, Value: snapshot.Value{Kind: "number", Number: value}}
}

// poolSnapshot returns the base snapshot holding nodes, one pool for each
// pool name among them, in order of first appearance, and findings.
func poolSnapshot(t *testing.T, nodes []snapshot.Node, findings ...snapshot.Finding) snapshot.Snapshot {
	t.Helper()
	s := baseSnapshot(t)
	s.Nodes = nodes
	s.Findings = findings
	var pools []snapshot.NodePool
	for _, node := range nodes {
		i := slices.IndexFunc(pools, func(p snapshot.NodePool) bool { return p.Name == node.Pool })
		if i < 0 {
			pools = append(pools, snapshot.NodePool{Name: node.Pool, Confidence: node.PoolConfidence})
			i = len(pools) - 1
		}
		pools[i].Nodes = append(pools[i].Nodes, node.Name)
	}
	s.Nodepools = pools
	return s
}

// poolBlocks builds section 4 of s in lang with the default thresholds.
func poolBlocks(t *testing.T, s snapshot.Snapshot, lang catalog.Language, severity string) []Block {
	t.Helper()
	b := &builder{cat: mustCatalog(t, lang)}
	blocks := buildNodepoolsSection(b, s, config.Default().Thresholds, severity)
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
	return blocks
}

func TestPoolSectionHeadings(t *testing.T) {
	s := poolSnapshot(t, []snapshot.Node{testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))})
	blocks := poolBlocks(t, s, catalog.English, "all")

	want := []string{"heading", "heading", "table", "heading", "table", "heading", "heading", "table"}
	if got := blockTypes(blocks); !slices.Equal(got, want) {
		t.Fatalf("block order = %v, want %v", got, want)
	}
	levels := []struct {
		index    int
		level    int
		numbered bool
		text     string
	}{
		{0, 1, true, "Resource usage by node pool"},
		{1, 2, true, "Summary by node pool"},
		{3, 2, true, "Aggregate usage profile by node pool"},
		{5, 2, true, "Detail by node"},
		{6, 3, false, "Node pool: "},
	}
	for _, l := range levels {
		h := blocks[l.index].(Heading)
		if h.Level != l.level || h.Numbered != l.numbered || runText(t, h.Runs[0]) != l.text {
			t.Errorf("blocks[%d] = level %d numbered %v %q, want level %d numbered %v %q",
				l.index, h.Level, h.Numbered, runText(t, h.Runs[0]), l.level, l.numbered, l.text)
		}
	}
	if h := blocks[6].(Heading); len(h.Runs) != 2 || h.Runs[1].kind != "ident" || runText(t, h.Runs[1]) != "apps" {
		t.Errorf("pool heading runs = %+v, want the pool as an ident", h.Runs)
	}
}

func TestPoolFilterReadsPoolFindingsOnly(t *testing.T) {
	s := poolSnapshot(t,
		[]snapshot.Node{testNode("n1", "apps", valueStat(90, 95), valueStat(10, 20))},
		nodeFinding("NODE_CPU_USAGE_CRITICAL", analysis.SeverityCritical, "n1", number("average_percent", 90), number("peak_percent", 95)),
	)
	blocks := poolBlocks(t, s, catalog.English, "attention")

	want := []string{"heading", "heading", "paragraph", "heading", "paragraph", "heading", "heading", "table"}
	if got := blockTypes(blocks); !slices.Equal(got, want) {
		t.Fatalf("block order = %v, want %v", got, want)
	}
	detail := blocks[7].(Table)
	if detail.ID != "node-detail" || runText(t, detail.Rows[0][0][0]) != "n1" {
		t.Errorf("detail = %q first row %q, want node-detail with n1", detail.ID, runText(t, detail.Rows[0][0][0]))
	}
}

func TestPoolFilterKeepsAPoolWithAPoolFinding(t *testing.T) {
	s := poolSnapshot(t,
		[]snapshot.Node{testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))},
		snapshot.Finding{Code: "NODEPOOL_UNKNOWN", Severity: analysis.SeverityAttention, ScopeType: analysis.ScopeNodepool, ScopeName: "apps", Evidence: []snapshot.Evidence{number("affected_nodes", 1)}, Confidence: "exact"},
	)
	blocks := poolBlocks(t, s, catalog.English, "attention")
	if got := findTable(t, blocks, "nodepool-summary"); len(got.Rows) != 1 {
		t.Errorf("summary rows = %d, want 1", len(got.Rows))
	}
	if got := findTable(t, blocks, "nodepool-profile"); len(got.Rows) != 1 {
		t.Errorf("profile rows = %d, want 1", len(got.Rows))
	}
	for _, blk := range blocks {
		if table, ok := blk.(Table); ok && table.ID == "node-detail" {
			t.Errorf("the detail shows a node with no node finding at attention")
		}
	}
}

func TestPoolSummaryMeans(t *testing.T) {
	s := poolSnapshot(t, []snapshot.Node{
		testNode("n1", "apps", valueStat(20, 20), valueStat(50, 90)),
		testNode("n2", "apps", valueStat(57.5, 60), valueStat(70, 75)),
	})
	table := findTable(t, poolBlocks(t, s, catalog.PortugueseBrazil, "all"), "nodepool-summary")
	row := table.Rows[0]

	if len(table.Headers) != 7 || len(row) != 7 {
		t.Fatalf("headers = %d, cells = %d, want 7 and 7", len(table.Headers), len(row))
	}
	if row[0][0].kind != "ident" || runText(t, row[0][0]) != "apps" {
		t.Errorf("pool cell = %+v, want the ident apps", row[0][0])
	}
	if got := runText(t, row[1][0]); got != "2" {
		t.Errorf("nodes = %q, want 2", got)
	}
	assertStatus(t, row[2][0], "healthy", "38,8%")
	assertStatus(t, row[3][0], "healthy", "60%")
	assertStatus(t, row[4][0], "healthy", "60%")
	assertStatus(t, row[5][0], "critical", "90%")
	if got := runText(t, row[6][0]); got != "Exata" {
		t.Errorf("confidence = %q, want Exata", got)
	}
}

func TestPoolSummaryUnknownNumbers(t *testing.T) {
	unknown := snapshot.Statistics{State: stateUnavailable}
	s := poolSnapshot(t, []snapshot.Node{testNode("n1", "apps", unknown, unknown)})
	row := findTable(t, poolBlocks(t, s, catalog.English, "all"), "nodepool-summary").Rows[0]
	for i := 2; i <= 5; i++ {
		assertStatus(t, row[i][0], "unavailable", "Unavailable")
	}
}

func TestPoolSummaryUnknownPool(t *testing.T) {
	n := testNode("n1", "unknown", valueStat(10, 20), valueStat(10, 20))
	n.PoolConfidence = "unknown"
	s := poolSnapshot(t, []snapshot.Node{n})
	blocks := poolBlocks(t, s, catalog.PortugueseBrazil, "all")

	summary := findTable(t, blocks, "nodepool-summary").Rows[0]
	if summary[0][0].kind != "placeholder" || runText(t, summary[0][0]) != "Indeterminado" {
		t.Errorf("summary pool = %+v, want the placeholder Indeterminado", summary[0][0])
	}
	if got := runText(t, summary[6][0]); got != "Desconhecida" {
		t.Errorf("confidence = %q, want Desconhecida", got)
	}
	profile := findTable(t, blocks, "nodepool-profile").Rows[0]
	if profile[0][0].kind != "placeholder" {
		t.Errorf("profile pool kind = %q, want placeholder", profile[0][0].kind)
	}
	heading := blocks[6].(Heading)
	if heading.Runs[1].kind != "placeholder" || runText(t, heading.Runs[1]) != "Indeterminado" {
		t.Errorf("detail heading pool = %+v, want the placeholder", heading.Runs[1])
	}
}

func TestPoolProfileMeans(t *testing.T) {
	n := testNode("n1", "apps", valueStat(55, 78), valueStat(64, 80))
	n.CPUAllocatable, n.MemoryAllocatable = floatPtr(7.5), floatPtr(30)
	n.CPURequestPercent, n.MemoryRequestPercent = floatPtr(60), floatPtr(70)
	s := poolSnapshot(t, []snapshot.Node{n})
	row := findTable(t, poolBlocks(t, s, catalog.PortugueseBrazil, "all"), "nodepool-profile").Rows[0]

	want := []string{"apps", "7,5\u00a0cores", "4,12\u00a0cores", "4,5\u00a0cores", "30\u00a0B", "19,2\u00a0B", "21\u00a0B"}
	for i, text := range want {
		if got := runText(t, row[i][0]); got != text {
			t.Errorf("cell %d = %q, want %q", i, got, text)
		}
	}
}

func TestPoolProfileSubsetMeans(t *testing.T) {
	unknown := snapshot.Statistics{State: stateUnavailable}
	n := testNode("n1", "apps", unknown, unknown)
	n.CPUAllocatable, n.MemoryAllocatable = floatPtr(4), floatPtr(2048)
	s := poolSnapshot(t, []snapshot.Node{n})
	row := findTable(t, poolBlocks(t, s, catalog.English, "all"), "nodepool-profile").Rows[0]

	want := []string{"apps", "4\u00a0cores", "Unavailable", "Unavailable", "2\u00a0KiB", "Unavailable", "Unavailable"}
	for i, text := range want {
		if got := runText(t, row[i][0]); got != text {
			t.Errorf("cell %d = %q, want %q", i, got, text)
		}
	}
}

func TestPoolProfileDivisorIsTheSubset(t *testing.T) {
	a := testNode("a", "apps", valueStat(50, 50), valueStat(50, 50))
	a.CPUAllocatable = floatPtr(4)
	b := testNode("b", "apps", valueStat(50, 50), valueStat(50, 50))
	c := testNode("c", "apps", snapshot.Statistics{State: stateUnavailable}, valueStat(50, 50))
	c.CPUAllocatable = floatPtr(8)
	s := poolSnapshot(t, []snapshot.Node{a, b, c})
	row := findTable(t, poolBlocks(t, s, catalog.English, "all"), "nodepool-profile").Rows[0]

	// Allocatable: mean of 4 and 8. Used: only a has both numbers, so 4 * 50 / 100.
	if got := runText(t, row[1][0]); got != "6\u00a0cores" {
		t.Errorf("allocatable = %q, want 6 cores", got)
	}
	if got := runText(t, row[2][0]); got != "2\u00a0cores" {
		t.Errorf("used = %q, want 2 cores", got)
	}
}

func TestPoolSectionEmptyParagraphs(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	blocks := poolBlocks(t, poolSnapshot(t, nil), catalog.English, "all")

	want := []string{"heading", "heading", "paragraph", "heading", "paragraph", "heading", "paragraph"}
	if got := blockTypes(blocks); !slices.Equal(got, want) {
		t.Fatalf("block order = %v, want %v", got, want)
	}
	for index, key := range map[int]catalog.Key{2: catalog.SectionsNodepoolsEmptySummary, 4: catalog.SectionsNodepoolsEmptyProfile, 6: catalog.SectionsNodepoolsEmptyDetail} {
		text, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%q): %v", key, err)
		}
		if got := runText(t, blocks[index].(Paragraph).Runs[0]); got != text {
			t.Errorf("blocks[%d] = %q, want %q", index, got, text)
		}
	}
}

func TestNodeDetailEmpty(t *testing.T) {
	s := poolSnapshot(t, []snapshot.Node{testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))})
	blocks := poolBlocks(t, s, catalog.English, "critical")
	last := blocks[len(blocks)-1]
	if _, ok := last.(Paragraph); !ok {
		t.Fatalf("last block is %T, want the empty paragraph", last)
	}
	if h, ok := blocks[len(blocks)-2].(Heading); !ok || h.Level != 2 {
		t.Errorf("the empty paragraph does not stand under the detail heading: %T", blocks[len(blocks)-2])
	}
}

// detailRow builds the detail table of s in lang and returns the row of
// its first node.
func detailRow(t *testing.T, s snapshot.Snapshot, lang catalog.Language) [][]Run {
	t.Helper()
	return findTable(t, poolBlocks(t, s, lang, "all"), "node-detail").Rows[0]
}

func TestNodeDetailCells(t *testing.T) {
	unknown := snapshot.Statistics{State: stateUnavailable}
	s := poolSnapshot(t, []snapshot.Node{testNode("worker-a", "apps", valueStat(55, 78), unknown)})
	row := detailRow(t, s, catalog.PortugueseBrazil)

	if len(row) != 5 {
		t.Fatalf("cells = %d, want 5", len(row))
	}
	if row[0][0].kind != "ident" || runText(t, row[0][0]) != "worker-a" {
		t.Errorf("name = %+v", row[0][0])
	}
	if len(row[1]) != 3 {
		t.Fatalf("cpu cell has %d runs, want 3", len(row[1]))
	}
	assertStatus(t, row[1][0], "healthy", "55%")
	if row[1][1].kind != "text" || runText(t, row[1][1]) != " / " {
		t.Errorf("separator = %+v, want the text \" / \"", row[1][1])
	}
	assertStatus(t, row[1][2], "attention", "78%")
	assertStatus(t, row[2][0], "unavailable", "Indisponível")
	assertStatus(t, row[2][2], "unavailable", "Indisponível")
}

func TestNodeDetailNoFinding(t *testing.T) {
	s := poolSnapshot(t, []snapshot.Node{testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))})
	row := detailRow(t, s, catalog.PortugueseBrazil)
	if got := runText(t, row[3][0]); got != "Sem achado" || row[3][0].kind != "text" {
		t.Errorf("reason = %+v %q, want the text Sem achado", row[3][0], got)
	}
}

func TestNodeDetailReason(t *testing.T) {
	s := poolSnapshot(t,
		[]snapshot.Node{testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20)), testNode("n2", "apps", valueStat(10, 20), valueStat(10, 20))},
		nodeFinding("NODE_CPU_REQUEST_HIGH", analysis.SeverityCritical, "n1", number("percent", 91)),
		nodeFinding("NODE_CPU_USAGE_ATTENTION", analysis.SeverityAttention, "n1", number("average_percent", 70), number("peak_percent", 78)),
		nodeFinding("NODE_MEMORY_REQUEST_HIGH", analysis.SeverityCritical, "n1", number("percent", 88.5)),
		nodeFinding("NODE_MEMORY_REQUEST_HIGH", analysis.SeverityCritical, "n1", number("percent", 88.5)),
		nodeFinding("NODE_LOW_UTILIZATION", analysis.SeverityAttention, "n2", number("maximum_average_percent", 10)),
	)
	table := findTable(t, poolBlocks(t, s, catalog.English, "all"), "node-detail")

	want := "CPU: 91% of the allocatable requested; Memory: 88.5% of the allocatable requested"
	assertStatus(t, table.Rows[0][3][0], "critical", want)
	assertStatus(t, table.Rows[1][3][0], "attention", "The node showed low average utilization")
}

func TestNodeDetailState(t *testing.T) {
	yes, no := boolPtr(true), boolPtr(false)
	tests := []struct {
		name               string
		ready, schedulable *bool
		findings           []snapshot.Finding
		want               string
		kind, status       string
	}{
		{"unknown", nil, nil, nil, "Unavailable; Unavailable", "status", "unavailable"},
		{"not ready", no, yes, []snapshot.Finding{nodeFinding("NODE_NOT_READY", analysis.SeverityAttention, "n1")}, "Not ready; Schedulable", "status", "attention"},
		{"cordoned", yes, no, []snapshot.Finding{nodeFinding("NODE_UNSCHEDULABLE", analysis.SeverityAttention, "n1")}, "Ready; Unschedulable", "status", "attention"},
		{"fine", yes, yes, nil, "Ready; Schedulable", "text", ""},
		{"stored without the finding", no, yes, nil, "Not ready; Schedulable", "text", ""},
		{"another node's finding", yes, yes, []snapshot.Finding{nodeFinding("NODE_NOT_READY", analysis.SeverityAttention, "n2")}, "Ready; Schedulable", "text", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := testNode("n1", "apps", valueStat(10, 20), valueStat(10, 20))
			n.Ready, n.Schedulable = tt.ready, tt.schedulable
			row := detailRow(t, poolSnapshot(t, []snapshot.Node{n}, tt.findings...), catalog.English)
			run := row[4][0]
			if got := runText(t, run); got != tt.want || run.kind != tt.kind || run.status != tt.status {
				t.Errorf("state = %q (%s %q), want %q (%s %q)", got, run.kind, run.status, tt.want, tt.kind, tt.status)
			}
		})
	}
}

func TestNodeDetailStateMarksOnlyWhatIsTrue(t *testing.T) {
	marks := map[string]Run{}
	for _, block := range poolBlocks(t, goldenSnapshot(t, "edge-scaling"), catalog.PortugueseBrazil, "all") {
		if table, ok := block.(Table); ok && table.ID == "node-detail" {
			for _, row := range table.Rows {
				marks[runText(t, row[0][0])] = row[4][0]
			}
		}
	}
	assertStatus(t, marks["pool-a-2"], "attention", "Não pronto; Não agendável")
	assertStatus(t, marks["orphan"], "unavailable", "Indisponível; Indisponível")
	if r := marks["pool-a-1"]; r.kind != "text" {
		t.Errorf("a ready and schedulable node = %s, want a plain text cell", r.kind)
	}
}

// TestValueStatusRunsCarryTheLegendWord checks that a status run whose text
// is a value, a reason, or a situation carries the legend's word for its
// status as its alternate text, in both languages.
func TestValueStatusRunsCarryTheLegendWord(t *testing.T) {
	thresholds := config.Default().Thresholds
	low := 12.0
	no := boolPtr(false)
	findings := []snapshot.Finding{
		nodeFinding("NODE_LOW_UTILIZATION", analysis.SeverityAttention, "n1", number("maximum_average_percent", 10)),
	}
	maxReached := snapshot.HPA{
		SpecMin: floatPtr(1), SpecMax: floatPtr(3),
		Current: snapshot.Statistics{State: stateValue, Peak: floatPtr(3)},
	}
	runs := []struct {
		name string
		run  func(b *builder) Run
		en   string
		pt   string
	}{
		{"ratio critical", func(b *builder) Run { return ratioRun(b, valueStat(0, 240), thresholds) }, "Critical", "Crítico"},
		{"ratio no base", func(b *builder) Run { return ratioRun(b, snapshot.Statistics{State: stateNoBase}, thresholds) }, "No base for the ratio", "Sem base de cálculo"},
		{"percent healthy", func(b *builder) Run { return percentRun(b, &low, thresholds) }, "Healthy", "Saudável"},
		{"percent unavailable", func(b *builder) Run { return percentRun(b, nil, thresholds) }, "Data unavailable", "Dado indisponível"},
		{"node reason", func(b *builder) Run { return nodeReason(b, "n1", findings) }, "Attention", "Atenção"},
		{"node state", func(b *builder) Run {
			return nodeState(b, snapshot.Node{Name: "n1", Ready: no, Schedulable: no}, []snapshot.Finding{nodeFinding("NODE_NOT_READY", analysis.SeverityAttention, "n1")})
		}, "Attention", "Atenção"},
		{"hpa situation", func(b *builder) Run { return hpaSituation(b, maxReached) }, "Critical", "Crítico"},
		{"no hpa", func(b *builder) Run {
			return uncoveredTable(b, []snapshot.Workload{{Kind: deploymentKind, Name: "web"}}).Rows[0][3][0]
		}, "Attention", "Atenção"},
	}
	for _, tc := range runs {
		for _, lang := range []struct {
			lang catalog.Language
			want string
		}{{catalog.English, tc.en}, {catalog.PortugueseBrazil, tc.pt}} {
			t.Run(tc.name+"/"+string(lang.lang), func(t *testing.T) {
				b := &builder{cat: mustCatalog(t, lang.lang)}
				run := tc.run(b)
				if b.err != nil {
					t.Fatalf("builder error = %v", b.err)
				}
				if run.alt == nil || *run.alt != lang.want {
					t.Errorf("alt = %v, want %q", run.alt, lang.want)
				}
			})
		}
	}
}

// TestCapabilityStateRunsCarryTheLegendWord checks that the state mark of the
// appendix capability table and of the executive material-limitations table
// carries the legend's word for its status as its alternate text, in both
// languages, including the row of a warning no capability explains.
func TestCapabilityStateRunsCarryTheLegendWord(t *testing.T) {
	capability := func(name, state string) snapshot.Capability {
		return snapshot.Capability{
			Name: name, State: state, Strategy: discovery.StrategyRawMetrics,
			Evidence: []string{}, Reasons: []string{},
		}
	}
	var s snapshot.Snapshot
	s.Quality.Capabilities = []snapshot.Capability{
		capability("hpa_history", discovery.StateAvailable),
		capability("workloads_without_hpa", discovery.StatePartial),
		capability("node_utilization", discovery.StateUnavailable),
	}
	s.Quality.Warnings = []snapshot.Message{{Code: collect.WarningNodeIdentityUnavailable, Params: map[string]float64{}}}
	for _, lang := range []struct {
		lang                            catalog.Language
		healthy, attention, unavailable string
	}{
		{catalog.English, "Healthy", "Attention", "Data unavailable"},
		{catalog.PortugueseBrazil, "Saudável", "Atenção", "Dado indisponível"},
	} {
		t.Run(string(lang.lang), func(t *testing.T) {
			b := &builder{cat: mustCatalog(t, lang.lang)}
			appendix := appendixRows(b, s.Quality.Capabilities, "all")
			limits := materialLimitations(b, s)[1].(Table).Rows
			if b.err != nil {
				t.Fatalf("builder error = %v", b.err)
			}
			for _, tc := range []struct {
				name string
				run  Run
				want string
			}{
				{"appendix available", appendix[0][1][0], lang.healthy},
				{"appendix partial", appendix[1][1][0], lang.attention},
				{"appendix unavailable", appendix[2][1][0], lang.unavailable},
				{"limitations partial", limits[0][1][0], lang.attention},
				{"limitations unavailable", limits[1][1][0], lang.unavailable},
				{"limitations warning", limits[len(limits)-1][1][0], lang.attention},
			} {
				if tc.run.alt == nil || *tc.run.alt != tc.want {
					t.Errorf("%s: alt = %v, want %q", tc.name, tc.run.alt, tc.want)
				}
			}
		})
	}
}
