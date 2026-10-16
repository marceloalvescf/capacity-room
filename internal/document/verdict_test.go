package document

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

func englishVerdict(t *testing.T, s snapshot.Snapshot, severity string) Verdict {
	t.Helper()
	b := &builder{cat: mustCatalog(t, catalog.English)}
	v := buildVerdict(b, s, severity)
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
	return v
}

func strPtr(s string) *string { return &s }

func intPtr(n int) *int { return &n }

func TestVerdictOfEdgeScalingTotalsAndQuality(t *testing.T) {
	v := englishVerdict(t, goldenSnapshot(t, "edge-scaling"), "all")

	wantTotals := []VerdictCount{
		{"critical", "Critical", "10"},
		{"attention", "Attention", "11"},
		{"healthy", "Healthy", "7"},
	}
	if !reflect.DeepEqual(v.Totals.Items, wantTotals) || v.Totals.None != nil {
		t.Errorf("Totals = %+v, want items %+v and no sentence", v.Totals, wantTotals)
	}

	wantQuality := &VerdictQuality{
		Label: "Data quality",
		Count: "2",
		Items: []string{
			"Ambiguous HPA configuration or target values were omitted.",
			"The capacity evidence of the nodes is incomplete.",
		},
	}
	if !reflect.DeepEqual(v.Quality, wantQuality) {
		t.Errorf("Quality = %+v, want %+v", v.Quality, wantQuality)
	}
}

// scalingWithBounds is the golden snapshot of edge-scaling with an enabled
// autoscaling that holds the bounds of a cluster autoscaler: three nodes of a
// maximum of three, a minimum of two, and one node added.
func scalingWithBounds(t *testing.T) snapshot.Snapshot {
	t.Helper()
	s := goldenSnapshot(t, "edge-scaling")
	a := &s.Summary.Autoscaling
	a.Status = "enabled"
	a.CurrentNodes, a.MinNodes, a.MaxNodes, a.ScaleEvents = floatPtr(3), floatPtr(2), floatPtr(3), floatPtr(1)
	return s
}

func TestVerdictOfEdgeScalingGrowth(t *testing.T) {
	g := englishVerdict(t, goldenSnapshot(t, "edge-scaling"), "all").Growth

	want := VerdictGrowth{
		Title:   "Cluster autoscaling",
		State:   strPtr("Disabled"),
		Finding: &VerdictFinding{"critical", "Critical", "Node autoscaling is disabled."},
		Warning: strPtr("The capacity evidence of the nodes is incomplete."),
	}
	if !reflect.DeepEqual(g, want) {
		t.Errorf("Growth = %+v, want %+v", g, want)
	}
}

func TestVerdictOfEdgeScalingPressure(t *testing.T) {
	p := englishVerdict(t, goldenSnapshot(t, "edge-scaling"), "all").Pressure

	want := []VerdictPressureRow{
		{
			Status: "critical", Label: "Critical", Scope: "Nodes",
			Sentence: "Observed CPU usage on the node.",
			Name:     []Run{Ident("aks-agentpool-12345678-vmss000000")},
			Peak:     &VerdictFact{"Peak", "95%"},
			Counts:   []VerdictCount{{"critical", "Critical", "2"}, {"attention", "Attention", "1"}},
		},
		{
			Status: "critical", Label: "Critical", Scope: "Namespaces",
			Sentence: "CPU usage against the limit.",
			Name:     []Run{Ident("payments")},
			Peak:     &VerdictFact{"Peak", "330%"},
			Counts:   []VerdictCount{{"critical", "Critical", "1"}},
		},
		{
			Status: "critical", Label: "Critical", Scope: "Workloads",
			Sentence: "Memory usage against the limit.",
			Name:     []Run{Ident("payments/payments-web")},
			Peak:     &VerdictFact{"Peak", "12000%"},
			Counts:   []VerdictCount{{"critical", "Critical", "1"}, {"attention", "Attention", "1"}},
		},
		{
			Status: "critical", Label: "Critical", Scope: "HPAs",
			Sentence: "The HPA reached its observed maximum of replicas.",
			Name:     []Run{Ident("payments/web")},
			Counts:   []VerdictCount{{"critical", "Critical", "1"}, {"attention", "Attention", "1"}},
		},
		{
			Status: "attention", Label: "Attention", Scope: "Node pools",
			Sentence: "Peak CPU pressure varied among the nodes of the pool.",
			Name:     []Run{Ident("pool-a")},
			Counts:   []VerdictCount{{"attention", "Attention", "2"}},
		},
	}
	if p.Title != "Pressure by scope" || p.Empty != nil {
		t.Errorf("Pressure title = %q, empty = %v", p.Title, p.Empty)
	}
	if !reflect.DeepEqual(p.Rows, want) {
		t.Errorf("Pressure rows = %+v, want %+v", p.Rows, want)
	}
}

func TestVerdictNamesAnUnknownPoolAsAPlaceholder(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	var kept []snapshot.Finding
	for _, f := range s.Findings {
		if f.ScopeType != "nodepool" || f.ScopeName == "unknown" {
			kept = append(kept, f)
		}
	}
	s.Findings = kept

	rows := englishVerdict(t, s, "all").Pressure.Rows
	last := rows[len(rows)-1]
	if last.Scope != "Node pools" || !reflect.DeepEqual(last.Name, []Run{Placeholder("Undetermined")}) {
		t.Errorf("last row = %+v, want the node pools with the placeholder Undetermined", last)
	}
	if want := []VerdictCount{{"attention", "Attention", "1"}}; !reflect.DeepEqual(last.Counts, want) {
		t.Errorf("counts = %+v, want %+v", last.Counts, want)
	}
}

func TestVerdictOfEdgeScalingPlan(t *testing.T) {
	plan := englishVerdict(t, goldenSnapshot(t, "edge-scaling"), "all").Plan

	if plan.Title != "Prioritized action plan" || plan.Empty != nil {
		t.Errorf("Plan title = %q, empty = %v", plan.Title, plan.Empty)
	}
	wantHeaders := []string{"#", "Priority", "Action", "Scope"}
	if plan.Table == nil || plan.Table.ID != "verdict-plan" || !reflect.DeepEqual(plan.Table.Headers, wantHeaders) {
		t.Fatalf("Plan table = %+v, want id verdict-plan with headers %v", plan.Table, wantHeaders)
	}
	if len(plan.Table.Rows) != 5 {
		t.Fatalf("Plan rows = %d, want 5", len(plan.Table.Rows))
	}
	wantFirst := [][]Run{
		{Text("1")},
		{StatusText("critical", "Critical")},
		{Text("Review the configured maximum and the capacity of the workload.")},
		{Text("HPA "), Ident("payments/web")},
	}
	if !reflect.DeepEqual(plan.Table.Rows[0], wantFirst) {
		t.Errorf("first row = %+v, want %+v", plan.Table.Rows[0], wantFirst)
	}
	wantMore := "5 of 10 actions. The full plan is in the executive summary."
	if plan.More == nil || *plan.More != wantMore {
		t.Errorf("More = %v, want %q", plan.More, wantMore)
	}
}

func TestVerdictLegendFollowsTheOrderOfTheLegend(t *testing.T) {
	legend := englishVerdict(t, goldenSnapshot(t, "edge-scaling"), "all").Legend
	want := []VerdictLegendItem{
		{"healthy", "Healthy"},
		{"attention", "Attention"},
		{"critical", "Critical"},
		{"unavailable", "Data unavailable"},
		{"undefined", "No base for the ratio"},
	}
	if !reflect.DeepEqual(legend, want) {
		t.Errorf("Legend = %+v, want %+v", legend, want)
	}
}

func TestVerdictOfDirectComplete(t *testing.T) {
	v := englishVerdict(t, goldenSnapshot(t, "direct-complete"), "all")

	wantTotals := []VerdictCount{
		{"critical", "Critical", "4"},
		{"attention", "Attention", "3"},
		{"healthy", "Healthy", "6"},
	}
	if !reflect.DeepEqual(v.Totals.Items, wantTotals) {
		t.Errorf("Totals = %+v, want %+v", v.Totals.Items, wantTotals)
	}
	if v.Quality != nil {
		t.Errorf("Quality = %+v, want none", v.Quality)
	}
	if v.Growth.Gauge != nil {
		t.Errorf("Gauge = %+v, want none: the snapshot holds no maximum", v.Growth.Gauge)
	}
	wantMore := "5 of 7 actions. The full plan is in the executive summary."
	if v.Plan.More == nil || *v.Plan.More != wantMore {
		t.Errorf("More = %v, want %q", v.Plan.More, wantMore)
	}
}

func TestVerdictWithoutAutoscaling(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	s.Summary.Autoscaling.Status = "disabled"
	g := englishVerdict(t, s, "all").Growth

	if g.State == nil || *g.State != "Disabled" {
		t.Errorf("State = %v, want Disabled", g.State)
	}
	if g.Current != nil || g.Max != nil || g.Gauge != nil || g.Caption != nil || len(g.Facts) != 0 {
		t.Errorf("Growth = %+v, want no nodes, gauge, caption, or facts", g)
	}
}

func TestVerdictEnabledWithoutAMaximumStatesTheKnownCounts(t *testing.T) {
	s := scalingWithBounds(t)
	a := &s.Summary.Autoscaling
	a.CurrentNodes, a.MinNodes, a.MaxNodes = floatPtr(1), nil, nil
	a.ScaleEvents, a.UnschedulablePods = floatPtr(1), floatPtr(1)
	g := englishVerdict(t, s, "all").Growth

	if g.State == nil || *g.State != "Enabled" {
		t.Errorf("State = %v, want Enabled", g.State)
	}
	if g.Current != nil || g.Max != nil || g.Caption != nil || g.Gauge != nil {
		t.Errorf("Growth = %+v, want no pair, caption, or gauge", g)
	}
	want := []VerdictFact{{"Current nodes", "1"}, {"Unschedulable pods", "1"}, {"Nodes added", "1"}}
	if !reflect.DeepEqual(g.Facts, want) {
		t.Errorf("Facts = %+v, want %+v", g.Facts, want)
	}
}

func TestVerdictEnabledWithAMaximumDrawsThePairWithoutAMinimum(t *testing.T) {
	s := scalingWithBounds(t)
	a := &s.Summary.Autoscaling
	a.MinNodes, a.UnschedulablePods, a.ScaleEvents = nil, floatPtr(2), floatPtr(1)
	g := englishVerdict(t, s, "all").Growth

	if g.Current == nil || *g.Current != "3" || g.Max == nil || *g.Max != "3" {
		t.Errorf("Current = %v, Max = %v, want 3 and 3", g.Current, g.Max)
	}
	if g.Caption == nil || *g.Caption != "Nodes · current / maximum" || g.Gauge == nil {
		t.Errorf("Caption = %v, Gauge = %+v, want the caption and a gauge", g.Caption, g.Gauge)
	}
	if g.State != nil {
		t.Errorf("State = %v, want none", *g.State)
	}
	want := []VerdictFact{{"Unschedulable pods", "2"}, {"Nodes added", "1"}}
	if !reflect.DeepEqual(g.Facts, want) {
		t.Errorf("Facts = %+v, want %+v", g.Facts, want)
	}
}

func storedSnapshot(t *testing.T) snapshot.Snapshot {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "snapshot", "testdata", "stored-2.3.0-direct-complete.json"))
	if err != nil {
		t.Fatalf("reading the stored snapshot: %v", err)
	}
	s, err := snapshot.Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return s
}

func TestVerdictOfAReportStoredBy230KeepsItsPair(t *testing.T) {
	g := englishVerdict(t, storedSnapshot(t), "all").Growth

	if g.Current == nil || *g.Current != "3" || g.Max == nil || *g.Max != "3" || g.Gauge == nil {
		t.Errorf("Current = %v, Max = %v, Gauge = %+v, want 3, 3, and a gauge", g.Current, g.Max, g.Gauge)
	}
	want := []VerdictFact{{"Minimum nodes", "1"}, {"Unschedulable pods", "1"}, {"Nodes added", "1"}}
	if !reflect.DeepEqual(g.Facts, want) {
		t.Errorf("Facts = %+v, want %+v", g.Facts, want)
	}
}

func TestVerdictDisabledFromRawMetricsStatesTheWordAlone(t *testing.T) {
	g := englishVerdict(t, goldenSnapshot(t, "edge-scaling"), "all").Growth

	if g.State == nil || *g.State != "Disabled" || g.Current != nil || len(g.Facts) != 0 {
		t.Errorf("Growth = %+v, want the word Disabled and no facts", g)
	}
}

func TestVerdictWithoutACurrentCountStatesTheKnownFacts(t *testing.T) {
	s := scalingWithBounds(t)
	s.Summary.Autoscaling.CurrentNodes = nil
	s.Summary.Autoscaling.UnschedulablePods = floatPtr(2)
	g := englishVerdict(t, s, "all").Growth

	if g.State == nil || *g.State != "Enabled" || g.Current != nil || g.Max != nil || g.Gauge != nil {
		t.Errorf("Growth = %+v, want the word Enabled and no pair", g)
	}
	want := []VerdictFact{{"Unschedulable pods", "2"}, {"Nodes added", "1"}}
	if !reflect.DeepEqual(g.Facts, want) {
		t.Errorf("Facts = %+v, want %+v", g.Facts, want)
	}
}

func TestVerdictDrawsTheMinimumOnlyBetweenNoneAndTheMaximum(t *testing.T) {
	for _, tc := range []struct {
		name    string
		minimum float64
		want    *int
	}{
		{"above the maximum", 5, nil},
		{"negative", -1, nil},
		{"equal to the maximum", 3, nil},
		{"zero", 0, nil},
		{"inside", 2, intPtr(2)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := scalingWithBounds(t)
			s.Summary.Autoscaling.MinNodes = floatPtr(tc.minimum)
			g := englishVerdict(t, s, "all").Growth.Gauge
			if g == nil {
				t.Fatal("Gauge = nil, want a gauge")
			}
			if !reflect.DeepEqual(g.Min, tc.want) {
				t.Errorf("Min = %v, want %v", g.Min, tc.want)
			}
		})
	}
}

func TestVerdictRoundsTheGaugeToWholeNodes(t *testing.T) {
	s := scalingWithBounds(t)
	s.Summary.Autoscaling.CurrentNodes = floatPtr(7.4)
	s.Summary.Autoscaling.MaxNodes = floatPtr(5.6)
	g := englishVerdict(t, s, "all").Growth

	if want := (&VerdictGauge{Current: 6, Max: 6, Min: intPtr(2), Hazard: true}); !reflect.DeepEqual(g.Gauge, want) {
		t.Errorf("Gauge = %+v, want %+v", g.Gauge, want)
	}
}

func TestVerdictWithNoFindings(t *testing.T) {
	s := goldenSnapshot(t, "edge-scaling")
	s.Findings = []snapshot.Finding{}
	s.Recommendations = []snapshot.Recommendation{}
	s.Summary.Critical, s.Summary.Attention, s.Summary.Healthy = 0, 0, 0
	v := englishVerdict(t, s, "all")

	if len(v.Totals.Items) != 0 || v.Totals.None == nil || *v.Totals.None != "This report holds no findings." {
		t.Errorf("Totals = %+v, want the sentence of no findings", v.Totals)
	}
	if len(v.Pressure.Rows) != 0 || v.Pressure.Empty == nil || *v.Pressure.Empty != "Nothing needs attention." {
		t.Errorf("Pressure = %+v, want the empty sentence", v.Pressure)
	}
	if v.Plan.Table != nil || v.Plan.More != nil || v.Plan.Empty == nil || *v.Plan.Empty != "No prioritized action was identified." {
		t.Errorf("Plan = %+v, want the empty sentence and no table", v.Plan)
	}
	if v.Growth.Finding != nil {
		t.Errorf("Growth finding = %+v, want none", v.Growth.Finding)
	}
}

func TestVerdictOfTheExecutiveProfileRaisesTheMinimum(t *testing.T) {
	doc := mustBuild(t, goldenSnapshot(t, "edge-scaling"), mustCatalog(t, catalog.English), Options{Profile: "executive", MinSeverity: "all"})
	v, ok := doc.Blocks[0].(Verdict)
	if !ok {
		t.Fatalf("first block is %T, want the verdict", doc.Blocks[0])
	}

	wantTotals := []VerdictCount{
		{"critical", "Critical", "10"},
		{"attention", "Attention", "11"},
		{"healthy", "Healthy", "7"},
	}
	if !reflect.DeepEqual(v.Totals.Items, wantTotals) {
		t.Errorf("Totals = %+v, want %+v", v.Totals.Items, wantTotals)
	}
	for _, row := range v.Plan.Table.Rows {
		if status := row[1][0].status; status != "critical" && status != "attention" {
			t.Errorf("plan row %q has status %q, want critical or attention", runText(t, row[0][0]), status)
		}
	}
	for _, row := range v.Pressure.Rows {
		if row.Status == "healthy" {
			t.Errorf("pressure row %q is healthy", row.Scope)
		}
	}
}

func TestVerdictOfAttentionMinimumLeavesOutAHealthyClusterFinding(t *testing.T) {
	s := scalingWithBounds(t)
	for i := range s.Findings {
		if s.Findings[i].ScopeType == "cluster" {
			s.Findings[i].Severity = "healthy"
		}
	}
	if g := englishVerdict(t, s, "attention").Growth; g.Finding != nil || g.Gauge == nil || g.Gauge.Hazard {
		t.Errorf("Growth = %+v, want no finding and a gauge without hazard", g)
	}
	if g := englishVerdict(t, s, "all").Growth; g.Finding == nil || g.Finding.Status != "healthy" || g.Gauge.Hazard {
		t.Errorf("Growth = %+v, want the healthy finding and a gauge without hazard", g)
	}
}

func TestAReportStoredBy230BuildsItsDocumentAndViewWithThePair(t *testing.T) {
	s := storedSnapshot(t)
	for _, lang := range []catalog.Language{catalog.English, catalog.PortugueseBrazil} {
		cat := mustCatalog(t, lang)
		doc := mustBuild(t, s, cat, Options{})
		v, ok := doc.Blocks[0].(Verdict)
		if !ok {
			t.Fatalf("%v: first block = %T, want the verdict", lang, doc.Blocks[0])
		}
		g := v.Growth
		if g.Current == nil || *g.Current != "3" || g.Max == nil || *g.Max != "3" || g.Gauge == nil {
			t.Errorf("%v: Current = %v, Max = %v, Gauge = %+v, want 3, 3, and a gauge", lang, g.Current, g.Max, g.Gauge)
		}
		view, err := BuildView(s, cat)
		if err != nil {
			t.Fatalf("%v: BuildView() error = %v", lang, err)
		}
		if view.Verdict.Gauge == nil {
			t.Errorf("%v: view gauge = nil, want a gauge", lang)
		}
	}
}
