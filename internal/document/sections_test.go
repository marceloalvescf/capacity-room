package document

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// valueStat returns a Statistics of state value, holding average and peak.
func valueStat(average, peak float64) snapshot.Statistics {
	return snapshot.Statistics{State: stateValue, Average: &average, Peak: &peak}
}

// floatPtr returns a pointer to value, for a pod count.
func floatPtr(value float64) *float64 { return &value }

// unavailableWorkload returns a workload row that answered nothing: every
// statistic unavailable, and its pod count unknown.
func unavailableWorkload(namespace, kind, name string) snapshot.Workload {
	unavailable := snapshot.Statistics{State: stateUnavailable}
	return snapshot.Workload{
		Namespace:            namespace,
		Kind:                 kind,
		Name:                 name,
		ObservedUsage:        true,
		CPUUsage:             unavailable,
		MemoryUsage:          unavailable,
		CPURequests:          unavailable,
		MemoryRequests:       unavailable,
		CPULimits:            unavailable,
		MemoryLimits:         unavailable,
		CPURequestPercent:    unavailable,
		MemoryRequestPercent: unavailable,
		CPULimitPercent:      unavailable,
		MemoryLimitPercent:   unavailable,
	}
}

// directCompleteNamespace returns "payments", the namespace row of the
// scenario TestFindingsOfDirectComplete of internal/analysis pins: usage
// 0.3/0.4 CPU and 110/120 B, requests 0.5, limits 1, a request ratio of
// 60/80 and a limit ratio of 30/40 for CPU, and of 22000/24000 and
// 11000/12000 for memory.
func directCompleteNamespace() snapshot.Namespace {
	return snapshot.Namespace{
		Name:                 "payments",
		CPUUsage:             valueStat(0.3, 0.4),
		MemoryUsage:          valueStat(110, 120),
		CPURequests:          valueStat(0.5, 0.5),
		MemoryRequests:       valueStat(0.5, 0.5),
		CPULimits:            valueStat(1, 1),
		MemoryLimits:         valueStat(1, 1),
		CPURequestPercent:    valueStat(60, 80),
		MemoryRequestPercent: valueStat(22000, 24000),
		CPULimitPercent:      valueStat(30, 40),
		MemoryLimitPercent:   valueStat(11000, 12000),
	}
}

// assertStatus fails the test unless run is a status run of status holding
// text.
func assertStatus(t *testing.T, run Run, status, text string) {
	t.Helper()
	if run.kind != "status" {
		t.Fatalf("run kind = %q, want status", run.kind)
	}
	if run.status != status {
		t.Errorf("status = %q, want %q", run.status, status)
	}
	if got := runText(t, run); got != text {
		t.Errorf("status text = %q, want %q", got, text)
	}
}

// namespacesShown returns the ident of every level-2 heading among blocks,
// in order: the namespaces a section actually printed.
func namespacesShown(t *testing.T, blocks []Block) []string {
	t.Helper()
	var names []string
	for _, blk := range blocks {
		h, ok := blk.(Heading)
		if !ok || h.Level != 2 {
			continue
		}
		if len(h.Runs) != 2 {
			t.Fatalf("namespace heading has %d runs, want 2", len(h.Runs))
		}
		names = append(names, runText(t, h.Runs[1]))
	}
	return names
}

func TestIdentRun(t *testing.T) {
	data, err := json.Marshal(Ident("api"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"kind":"ident","text":"api"}`
	if string(data) != want {
		t.Errorf("Marshal(Ident(\"api\")) = %s, want %s", data, want)
	}
}

func TestSectionsOfDirectCompleteInPortuguese(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	blocks := buildNamespacesSection(b, []snapshot.Namespace{directCompleteNamespace()}, nil, config.Default().Thresholds, "all")
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}

	table, ok := blocks[2].(Table)
	if !ok {
		t.Fatalf("blocks[2] is %T, want Table", blocks[2])
	}
	if len(table.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(table.Rows))
	}

	cpu := table.Rows[0]
	wantCPU := []string{"CPU", "300 mCPU", "400 mCPU", "500 mCPU", "1 cores"}
	for i, want := range wantCPU {
		if got := runText(t, cpu[i][0]); got != want {
			t.Errorf("cpu cell %d = %q, want %q", i, got, want)
		}
	}
	assertStatus(t, cpu[5][0], "attention", "80%")
	assertStatus(t, cpu[6][0], "healthy", "40%")

	mem := table.Rows[1]
	wantMem := []string{"Memória", "110 B", "120 B", "0,5 B", "1 B"}
	for i, want := range wantMem {
		if got := runText(t, mem[i][0]); got != want {
			t.Errorf("memory cell %d = %q, want %q", i, got, want)
		}
	}
	assertStatus(t, mem[5][0], "critical", "24000%")
	assertStatus(t, mem[6][0], "critical", "12000%")
}

func TestSectionsOfDirectCompleteInEnglish(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	blocks := buildNamespacesSection(b, []snapshot.Namespace{directCompleteNamespace()}, nil, config.Default().Thresholds, "all")
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}

	table := blocks[2].(Table)
	wantHeaders := []string{
		"Resource", "Average usage", "Peak", "Request", "Limit", "% Peak/Request", "% Peak/Limit",
	}
	if !slices.Equal(table.Headers, wantHeaders) {
		t.Errorf("Headers = %v, want %v", table.Headers, wantHeaders)
	}

	if got := runText(t, table.Rows[1][0][0]); got != "Memory" {
		t.Errorf("memory row label = %q, want %q", got, "Memory")
	}
	if got := runText(t, table.Rows[1][3][0]); got != "0.5 B" {
		t.Errorf("memory request = %q, want %q", got, "0.5 B")
	}
}

func TestWorkloadRow(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	workload := snapshot.Workload{
		Namespace:            "payments",
		Kind:                 "Deployment",
		Name:                 "payments-web",
		Pods:                 floatPtr(1),
		CPUUsage:             valueStat(0.3, 0.4),
		MemoryUsage:          valueStat(110, 120),
		CPURequests:          valueStat(0.5, 0.5),
		MemoryRequests:       valueStat(0.5, 0.5),
		CPULimits:            valueStat(1, 1),
		MemoryLimits:         valueStat(1, 1),
		CPURequestPercent:    valueStat(60, 80),
		MemoryRequestPercent: valueStat(22000, 24000),
		CPULimitPercent:      valueStat(30, 40),
		MemoryLimitPercent:   valueStat(11000, 12000),
	}

	row := workloadRow(b, workload, config.Default().Thresholds)
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
	if len(row) != 10 {
		t.Fatalf("len(row) = %d, want 10", len(row))
	}
	if row[0][0].kind != "ident" {
		t.Errorf("name run kind = %q, want ident", row[0][0].kind)
	}
	if got := runText(t, row[0][0]); got != "payments-web" {
		t.Errorf("name = %q, want %q", got, "payments-web")
	}
	if got := runText(t, row[1][0]); got != "1" {
		t.Errorf("pods = %q, want %q", got, "1")
	}
}

// scenarioSnapshot returns a full, valid snapshot: one namespace,
// "payments", with one workload, "payments-web", and one warning — enough
// to walk every block sections 1, 2, and the appendix's interpretation
// notes can produce.
func scenarioSnapshot(t *testing.T) snapshot.Snapshot {
	t.Helper()
	s := snapshot.Snapshot{
		SchemaVersion: snapshot.SchemaVersion,
		Report: snapshot.Report{
			ID:               "run-001",
			GeneratorVersion: "kcr-9.9.9",
			EmittedAt:        "2026-07-15T14:30:00-03:00",
			Period:           "24h",
			PeriodStart:      "2026-07-14T14:30:00-03:00",
			PeriodEnd:        "2026-07-15T14:30:00-03:00",
			Timezone:         "America/Sao_Paulo",
			Profile:          "detailed",
			MinSeverity:      "all",
			Thresholds: snapshot.Thresholds{
				WarningPercent:           70,
				CriticalPercent:          85,
				LowUtilizationPercent:    30,
				AutoscalerNearMaxPercent: 80,
				NodepoolSpreadPoints:     30,
				BurstyGapPercent:         100,
				HPAReplicaDivergence:     1,
			},
			Settings: snapshot.Settings{
				EndpointType:    "prometheus",
				TLSVerify:       true,
				TimeoutSeconds:  42,
				NodeAutoscaling: "auto",
				NodepoolLabel:   "eks.amazonaws.com/nodegroup",
			},
		},
		Scope: snapshot.Scope{
			ClusterName:        "prod",
			ClusterLabel:       "cluster",
			FilterMode:         "required",
			MatcherEnabled:     true,
			Confidence:         discovery.ConfidenceExact,
			DiscoveredValues:   []string{"prod"},
			Namespaces:         []string{"payments"},
			ExcludedContainers: []string{},
		},
		Quality: snapshot.Quality{
			Capabilities:          baseCapabilities(),
			Warnings:              []snapshot.Message{{Code: collect.WarningWorkloadHistoryCurrentAssociation, Params: map[string]float64{}}},
			Notices:               []snapshot.Message{},
			CollectionStartedAt:   "2026-07-15T14:25:00-03:00",
			CollectionCompletedAt: "2026-07-15T14:29:00-03:00",
		},
		Summary:    snapshot.Summary{Namespaces: 1, Workloads: 1, Autoscaling: unknownAutoscaling()},
		Namespaces: []snapshot.Namespace{directCompleteNamespace()},
		Workloads: []snapshot.Workload{{
			Namespace:            "payments",
			Kind:                 "Deployment",
			Name:                 "payments-web",
			ObservedUsage:        true,
			HPACoverage:          "unknown",
			Pods:                 floatPtr(1),
			CPUUsage:             valueStat(0.3, 0.4),
			MemoryUsage:          valueStat(110, 120),
			CPURequests:          valueStat(0.5, 0.5),
			MemoryRequests:       valueStat(0.5, 0.5),
			CPULimits:            valueStat(1, 1),
			MemoryLimits:         valueStat(1, 1),
			CPURequestPercent:    valueStat(60, 80),
			MemoryRequestPercent: valueStat(22000, 24000),
			CPULimitPercent:      valueStat(30, 40),
			MemoryLimitPercent:   valueStat(11000, 12000),
		}},
		HPAs:      []snapshot.HPA{},
		Nodes:     []snapshot.Node{},
		Nodepools: []snapshot.NodePool{},
		Findings:  []snapshot.Finding{},
	}
	if err := s.Check(); err != nil {
		t.Fatalf("scenarioSnapshot: Check() error = %v", err)
	}
	return s
}

func TestBlocksFollowTheOrderOfTheOriginal(t *testing.T) {
	s := scenarioSnapshot(t)
	doc := mustBuild(t, s, mustCatalog(t, catalog.PortugueseBrazil), Options{})

	wantTypes := []string{
		"front", "pagebreak",
		"heading", "heading", "table",
		"heading", "heading", "table",
		"pagebreak",
		"heading", "table",
		"heading", "table",
	}
	if got := blockTypes(doc.Blocks); !slices.Equal(got, wantTypes) {
		t.Fatalf("block types = %v, want %v", got, wantTypes)
	}

	wantHeading := func(i, level int, numbered bool) {
		t.Helper()
		h, ok := doc.Blocks[i].(Heading)
		if !ok {
			t.Fatalf("Blocks[%d] is %T, want Heading", i, doc.Blocks[i])
		}
		if h.Level != level || h.Numbered != numbered {
			t.Errorf("Blocks[%d] = level %d numbered %v, want level %d numbered %v", i, h.Level, h.Numbered, level, numbered)
		}
	}
	wantHeading(2, 1, true)   // Capacidade por Namespace
	wantHeading(3, 2, false)  // Namespace: payments
	wantHeading(5, 1, true)   // Recursos por workload
	wantHeading(6, 2, false)  // Namespace: payments
	wantHeading(9, 1, true)   // Apêndice técnico de qualidade
	wantHeading(11, 2, false) // Avisos de interpretação

	if pb, ok := doc.Blocks[1].(PageBreak); !ok || pb.Weak {
		t.Errorf("Blocks[1] = %+v, want a non-weak pagebreak", doc.Blocks[1])
	}
	if pb, ok := doc.Blocks[8].(PageBreak); !ok || !pb.Weak {
		t.Errorf("Blocks[8] = %+v, want a weak pagebreak", doc.Blocks[8])
	}

	wantTable := func(i int, id string) {
		t.Helper()
		table, ok := doc.Blocks[i].(Table)
		if !ok || table.ID != id {
			t.Errorf("Blocks[%d] = %+v, want table %q", i, doc.Blocks[i], id)
		}
	}
	wantTable(4, "namespace-capacity")
	wantTable(7, "workload-resources")
	wantTable(10, "quality-appendix")
	wantTable(12, "interpretation-notes")
}

func TestHeadingOfANamespaceHasTwoRuns(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	heading := namespaceHeading(b, "payments")
	if len(heading.Runs) != 2 {
		t.Fatalf("len(Runs) = %d, want 2", len(heading.Runs))
	}
	if got := runText(t, heading.Runs[0]); got != "Namespace: " {
		t.Errorf("Runs[0] = %q, want %q", got, "Namespace: ")
	}
	if heading.Runs[1].kind != "ident" {
		t.Errorf("Runs[1] kind = %q, want ident", heading.Runs[1].kind)
	}
	if got := runText(t, heading.Runs[1]); got != "payments" {
		t.Errorf("Runs[1] text = %q, want %q", got, "payments")
	}
}

func TestRatioCellStates(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	thresholds := config.Default().Thresholds

	tests := []struct {
		name       string
		stat       snapshot.Statistics
		wantStatus string
		wantText   string
	}{
		{"healthy", valueStat(0, 69.9), "healthy", "69,9%"},
		{"attention", valueStat(0, 70), "attention", "70%"},
		{"critical", valueStat(0, 85), "critical", "85%"},
		{"no_base", snapshot.Statistics{State: stateNoBase}, "undefined", "Não definido"},
		{"unavailable", snapshot.Statistics{State: stateUnavailable}, "unavailable", "Indisponível"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := ratioRun(b, tt.stat, thresholds)
			assertStatus(t, run, tt.wantStatus, tt.wantText)
		})
	}
}

func TestRatioCellUsesTheThresholdsOfTheSnapshot(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	thresholds := config.Thresholds{WarningPercent: 50, CriticalPercent: 60}
	run := ratioRun(b, valueStat(0, 55), thresholds)
	assertStatus(t, run, "attention", "55%")
}

func TestQuantityWithoutValue(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	unavailable := snapshot.Statistics{State: stateUnavailable}
	want := b.text(catalog.PlaceholderUnavailable, nil)

	if got := runText(t, quantityRun(b, unavailable, unavailable.Average, b.cat.CPU)); got != want {
		t.Errorf("quantityRun(usage) = %q, want %q", got, want)
	}
	if got := runText(t, podsRun(b, nil)); got != want {
		t.Errorf("podsRun(nil) = %q, want %q", got, want)
	}
}

func TestNamespaceThatAnsweredNothingIsPrinted(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	row := unavailableNamespace("payments")

	blocks := buildNamespacesSection(b, []snapshot.Namespace{row}, nil, config.Default().Thresholds, "all")
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
	table, ok := blocks[2].(Table)
	if !ok {
		t.Fatalf("blocks[2] is %T, want Table", blocks[2])
	}
	if len(table.Rows) != 2 {
		t.Fatalf("len(Rows) = %d, want 2", len(table.Rows))
	}

	want := b.text(catalog.PlaceholderUnavailable, nil)
	for r, cells := range table.Rows {
		for c := 1; c < len(cells); c++ {
			if got := runText(t, cells[c][0]); got != want {
				t.Errorf("row %d cell %d = %q, want %q", r, c, got, want)
			}
		}
	}
}

func TestMinimumSeverityFiltersRows(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	rows := []snapshot.Namespace{unavailableNamespace("a"), unavailableNamespace("b")}
	findings := []snapshot.Finding{
		{Code: "X", Severity: analysis.SeverityCritical, ScopeType: analysis.ScopeNamespace, ScopeName: "a"},
	}

	tests := []struct {
		severity string
		want     []string
	}{
		{"all", []string{"a", "b"}},
		{"attention", []string{"a"}},
		{"critical", []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			blocks := buildNamespacesSection(b, rows, findings, config.Default().Thresholds, tt.severity)
			if got := namespacesShown(t, blocks); !slices.Equal(got, tt.want) {
				t.Errorf("shown = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMinimumSeverityJudgesByKind(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	rows := []snapshot.Workload{
		unavailableWorkload("payments", "Deployment", "api"),
		unavailableWorkload("payments", "StatefulSet", "api"),
	}
	findings := []snapshot.Finding{
		{Code: "X", Severity: analysis.SeverityCritical, ScopeType: analysis.ScopeWorkload, ScopeName: "payments/api", ScopeKind: "Deployment"},
	}

	blocks := buildWorkloadsSection(b, rows, findings, config.Default().Thresholds, "critical")
	table, ok := blocks[2].(Table)
	if !ok {
		t.Fatalf("blocks[2] is %T, want Table", blocks[2])
	}
	if len(table.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(table.Rows))
	}
	if got := runText(t, table.Rows[0][0][0]); got != "api" {
		t.Errorf("workload name = %q, want %q", got, "api")
	}
}

func TestWorkloadSectionSkipsANamespaceWithoutRows(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	rows := []snapshot.Workload{
		unavailableWorkload("a", "Deployment", "svc"),
		unavailableWorkload("b", "Deployment", "svc"),
	}
	findings := []snapshot.Finding{
		{Code: "X", Severity: analysis.SeverityCritical, ScopeType: analysis.ScopeWorkload, ScopeName: "a/svc", ScopeKind: "Deployment"},
	}

	blocks := buildWorkloadsSection(b, rows, findings, config.Default().Thresholds, "critical")
	want := []string{"a"}
	if got := namespacesShown(t, blocks); !slices.Equal(got, want) {
		t.Errorf("namespaces shown = %v, want %v", got, want)
	}
}

func TestEmptySections(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	sections := map[string][]Block{
		"namespaces": buildNamespacesSection(b, nil, nil, config.Default().Thresholds, "all"),
		"workloads":  buildWorkloadsSection(b, nil, nil, config.Default().Thresholds, "all"),
	}
	for name, blocks := range sections {
		t.Run(name, func(t *testing.T) {
			if len(blocks) != 2 {
				t.Fatalf("len(blocks) = %d, want 2", len(blocks))
			}
			if _, ok := blocks[0].(Heading); !ok {
				t.Errorf("blocks[0] is %T, want Heading", blocks[0])
			}
			if _, ok := blocks[1].(Paragraph); !ok {
				t.Errorf("blocks[1] is %T, want Paragraph", blocks[1])
			}
		})
	}
}

// parityTexts maps each text of template/tests/capacity/fixtures/parity.pt-BR.json
// that the catalog produces to its key. The fixture is a transcription of
// the original report and keeps the original's words; the document writes
// the words of the catalog. The parity comparison reads the current text of
// each key so that it compares numbers, statuses, identifiers, and shape,
// and not the words, which may change.
var parityTexts = map[string]catalog.Key{
	"Capacidade por Namespace":       catalog.SectionsNamespacesHeading,
	"Recurso":                        catalog.SectionsNamespacesHeaderResource,
	"Uso médio":                      catalog.SectionsNamespacesHeaderAverage,
	"Pico":                           catalog.SectionsNamespacesHeaderPeak,
	"Solicitação":                    catalog.SectionsNamespacesHeaderRequest,
	"Limite":                         catalog.SectionsNamespacesHeaderLimit,
	"% Pico/Solicitação":             catalog.SectionsNamespacesHeaderPeakRequest,
	"% Pico/Limite":                  catalog.SectionsNamespacesHeaderPeakLimit,
	"CPU":                            catalog.SectionsNamespacesRowCPU,
	"Memória":                        catalog.SectionsNamespacesRowMemory,
	"Recursos por carga de trabalho": catalog.SectionsWorkloadsHeading,
	"Carga de trabalho":              catalog.SectionsWorkloadsHeaderWorkload,
	"Pods":                           catalog.SectionsWorkloadsHeaderPods,
	"CPU média":                      catalog.SectionsWorkloadsHeaderCPUAverage,
	"CPU pico":                       catalog.SectionsWorkloadsHeaderCPUPeak,
	"% CPU/Solicitação":              catalog.SectionsWorkloadsHeaderCPURequest,
	"% CPU/Limite":                   catalog.SectionsWorkloadsHeaderCPULimit,
	"Memória média":                  catalog.SectionsWorkloadsHeaderMemoryAverage,
	"Memória pico":                   catalog.SectionsWorkloadsHeaderMemoryPeak,
	"% Memória/Solicitação":          catalog.SectionsWorkloadsHeaderMemoryRequest,
	"% Memória/Limite":               catalog.SectionsWorkloadsHeaderMemoryLimit,
	"Indisponível":                   catalog.PlaceholderUnavailable,
	"Não definido":                   catalog.PlaceholderNoBase,
}

// parityLabelText is the one run of the fixture that is a catalog text
// followed by a space: the namespace label before the identifier.
const parityLabelText = "Namespace: "

// withCatalogTexts returns a copy of v, a decoded JSON value, in which every
// header and every run that is not an identifier and whose text parityTexts
// knows carries the current catalog text of its key. Every other value is
// copied as it is.
func withCatalogTexts(t *testing.T, cat *catalog.Catalog, v any) any {
	t.Helper()
	replace := func(s string) string {
		if s == parityLabelText {
			text, err := cat.Text(catalog.SectionsNamespaceLabel, nil)
			if err != nil {
				t.Fatalf("Text(%q): %v", catalog.SectionsNamespaceLabel, err)
			}
			return text + " "
		}
		key, ok := parityTexts[s]
		if !ok {
			return s
		}
		text, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%q): %v", key, err)
		}
		return text
	}

	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = val
		}
		if headers, ok := x["headers"].([]any); ok {
			replaced := make([]any, len(headers))
			for i, h := range headers {
				if s, ok := h.(string); ok {
					replaced[i] = replace(s)
				} else {
					replaced[i] = h
				}
			}
			out["headers"] = replaced
		}
		if kind, ok := x["kind"].(string); ok && kind != "ident" {
			if s, ok := x["text"].(string); ok {
				out["text"] = replace(s)
			}
		}
		for k, val := range out {
			if k == "headers" || k == "text" {
				continue
			}
			out[k] = withCatalogTexts(t, cat, val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = withCatalogTexts(t, cat, val)
		}
		return out
	default:
		return v
	}
}

// TestDocumentEqualsParityForItsSections proves that the namespace and
// workload sections built straight from a snapshot holding the numbers of
// template/tests/capacity/fixtures/parity.pt-BR.json equal its blocks 2 to
// 7 in every number, status, identifier, and block, with the words of the
// fixture read through the catalog.
func TestDocumentEqualsParityForItsSections(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.PortugueseBrazil)}
	thresholds := config.Thresholds{WarningPercent: 70, CriticalPercent: 85}
	noBase := snapshot.Statistics{State: stateUnavailable}

	namespace := snapshot.Namespace{
		Name:                 "payments",
		CPUUsage:             valueStat(3.5, 6.2),
		MemoryUsage:          valueStat(12, 18),
		CPURequests:          noBase,
		MemoryRequests:       noBase,
		CPULimits:            noBase,
		MemoryLimits:         noBase,
		CPURequestPercent:    valueStat(0, 91),
		MemoryRequestPercent: valueStat(0, 79),
		CPULimitPercent:      noBase,
		MemoryLimitPercent:   noBase,
	}

	// The fixture names "worker" before "api", which is not the order the
	// collectors would produce (sorted by name). The rows go straight to
	// the section builder, in the fixture's own order, without passing
	// through Snapshot.Check, which would otherwise refuse them as
	// unsorted.
	workloads := []snapshot.Workload{
		{
			Namespace: "payments", Kind: "Deployment", Name: "worker",
			ObservedUsage:        true,
			Pods:                 floatPtr(2),
			CPUUsage:             valueStat(0.5, 0.8),
			MemoryUsage:          valueStat(1, 1.5),
			CPURequestPercent:    valueStat(0, 50),
			MemoryRequestPercent: valueStat(0, 55),
			CPULimitPercent:      noBase,
			MemoryLimitPercent:   noBase,
		},
		{
			Namespace: "payments", Kind: "Deployment", Name: "api",
			ObservedUsage:        true,
			Pods:                 floatPtr(4),
			CPUUsage:             valueStat(3, 5.4),
			MemoryUsage:          valueStat(11, 17),
			CPURequestPercent:    valueStat(0, 97),
			MemoryRequestPercent: valueStat(0, 95),
			CPULimitPercent:      noBase,
			MemoryLimitPercent:   noBase,
		},
	}

	var blocks []Block
	blocks = append(blocks, buildNamespacesSection(b, []snapshot.Namespace{namespace}, nil, thresholds, "all")...)
	blocks = append(blocks, buildWorkloadsSection(b, workloads, nil, thresholds, "all")...)
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}

	got, err := marshalNoEscape(blocks)
	if err != nil {
		t.Fatalf("marshalNoEscape() error = %v", err)
	}

	fixture, err := os.ReadFile(filepath.Join("..", "..", "template", "tests", "capacity", "fixtures", "parity.pt-BR.json"))
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var doc struct {
		Blocks []json.RawMessage `json:"blocks"`
	}
	if err := json.Unmarshal(fixture, &doc); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}
	if len(doc.Blocks) < 8 {
		t.Fatalf("fixture has %d blocks, want at least 8", len(doc.Blocks))
	}
	want, err := json.Marshal(doc.Blocks[2:8])
	if err != nil {
		t.Fatalf("marshal want: %v", err)
	}

	var gotNorm, wantNorm any
	if err := json.Unmarshal(got, &gotNorm); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if err := json.Unmarshal(want, &wantNorm); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	wantNorm = withCatalogTexts(t, b.cat, wantNorm)
	if !reflect.DeepEqual(gotNorm, wantNorm) {
		gotPretty, _ := json.MarshalIndent(gotNorm, "", "  ")
		wantPretty, _ := json.MarshalIndent(wantNorm, "", "  ")
		t.Fatalf("blocks differ:\ngot:\n%s\nwant:\n%s", gotPretty, wantPretty)
	}
}

// TestParitySubstitutionChangesTheFixture proves that withCatalogTexts
// rewrites a text the mapping knows and leaves alone a number, an
// identifier, and a text it does not know, so that the parity comparison
// cannot pass by mapping nothing.
func TestParitySubstitutionChangesTheFixture(t *testing.T) {
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	in := map[string]any{
		"type":    "table",
		"id":      "namespace-capacity",
		"headers": []any{"Recurso", "Solicitação", "% Pico/Limite"},
		"rows": []any{
			[]any{
				[]any{map[string]any{"kind": "ident", "text": "Solicitação"}},
				[]any{map[string]any{"kind": "text", "text": "3,5 cores"}},
				[]any{map[string]any{"kind": "status", "status": "unavailable", "text": "Indisponível"}},
			},
		},
	}

	got := withCatalogTexts(t, cat, in).(map[string]any)

	want := func(key catalog.Key) string {
		text, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%q): %v", key, err)
		}
		return text
	}
	headers := got["headers"].([]any)
	if headers[0] != want(catalog.SectionsNamespacesHeaderResource) ||
		headers[1] != want(catalog.SectionsNamespacesHeaderRequest) ||
		headers[2] != want(catalog.SectionsNamespacesHeaderPeakLimit) {
		t.Errorf("headers = %v, want the catalog texts of resource, request, and peak/limit", headers)
	}
	row := got["rows"].([]any)[0].([]any)
	ident := row[0].([]any)[0].(map[string]any)
	if ident["text"] != "Solicitação" {
		t.Errorf("ident run = %q, want it left alone", ident["text"])
	}
	number := row[1].([]any)[0].(map[string]any)
	if number["text"] != "3,5 cores" {
		t.Errorf("number run = %q, want it left alone", number["text"])
	}
	status := row[2].([]any)[0].(map[string]any)
	if status["text"] != want(catalog.PlaceholderUnavailable) {
		t.Errorf("status run = %q, want the catalog text of the placeholder", status["text"])
	}

	// The input is not changed in place.
	if in["headers"].([]any)[1] != "Solicitação" {
		t.Error("withCatalogTexts changed its input")
	}
}

func TestWorkloadsSectionSkipsCoverageOnlyRows(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	observed := unavailableWorkload("payments", "Deployment", "api")
	coverageOnly := unavailableWorkload("payments", "Deployment", "worker")
	coverageOnly.ObservedUsage = false
	coverageOnly.HPACoverage = "uncovered"

	blocks := buildWorkloadsSection(b, []snapshot.Workload{observed, coverageOnly}, nil, config.Default().Thresholds, "all")
	table, ok := blocks[2].(Table)
	if !ok {
		t.Fatalf("blocks[2] is %T, want Table", blocks[2])
	}
	if len(table.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(table.Rows))
	}
	if got := runText(t, table.Rows[0][0][0]); got != "api" {
		t.Errorf("workload name = %q, want %q", got, "api")
	}
}

func TestWorkloadsSectionIsEmptyWithOnlyCoverageRows(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	coverageOnly := unavailableWorkload("payments", "Deployment", "worker")
	coverageOnly.ObservedUsage = false

	blocks := buildWorkloadsSection(b, []snapshot.Workload{coverageOnly}, nil, config.Default().Thresholds, "all")
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if _, ok := blocks[1].(Paragraph); !ok {
		t.Errorf("blocks[1] is %T, want Paragraph", blocks[1])
	}
}
