package document

import (
	"slices"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// hpaStat returns the current statistics of an HPA: average, minimum, and
// peak known.
func hpaStat(average, minimum, peak float64) snapshot.Statistics {
	return snapshot.Statistics{State: stateValue, Average: &average, Minimum: &minimum, Peak: &peak, Current: &peak}
}

// hpaRow returns an HPA of payments with the given name, specs, and
// current statistics.
func hpaRow(name string, specMin, specMax *float64, current snapshot.Statistics) snapshot.HPA {
	return snapshot.HPA{
		Namespace: "payments", Name: name, TargetKind: "Deployment", TargetName: name,
		SpecMin: specMin, SpecMax: specMax, Current: current,
		Desired: snapshot.Statistics{State: stateUnavailable},
	}
}

// hpaSnapshot returns the base snapshot with payments as its one namespace.
func hpaSnapshot(t *testing.T) snapshot.Snapshot {
	t.Helper()
	s := baseSnapshot(t)
	s.Scope.Namespaces = []string{"payments"}
	return s
}

// hpaBlocks builds section 3 of s in lang and fails on a builder error.
func hpaBlocks(t *testing.T, s snapshot.Snapshot, lang catalog.Language) []Block {
	t.Helper()
	b := &builder{cat: mustCatalog(t, lang)}
	blocks := buildHPAsSection(b, s)
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
	return blocks
}

func TestHPASectionNote(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	text := func(key catalog.Key) string {
		s, err := cat.Text(key, nil)
		if err != nil {
			t.Fatalf("Text(%q): %v", key, err)
		}
		return s
	}

	tests := []struct {
		name         string
		hpa, without string
		warnings     []string
		wantTitle    catalog.Key
		wantBody     catalog.Key
	}{
		{"unavailable", discovery.StateUnavailable, discovery.StateAvailable, nil, catalog.SectionsHPAsNoteIncompleteTitle, catalog.SectionsHPAsNoteIncomplete},
		{"unavailable beside partial", discovery.StatePartial, discovery.StateUnavailable, nil, catalog.SectionsHPAsNoteIncompleteTitle, catalog.SectionsHPAsNoteIncomplete},
		{"partial", discovery.StateAvailable, discovery.StatePartial, nil, catalog.SectionsHPAsNotePartialTitle, catalog.SectionsHPAsNotePartial},
		{"available", discovery.StateAvailable, discovery.StateAvailable, nil, "", ""},
		{"ambiguous spec", discovery.StateAvailable, discovery.StateAvailable, []string{collect.WarningHPAAmbiguousSpec}, catalog.SectionsHPAsNotePartialTitle, catalog.SectionsHPAsNotePartial},
		{"ambiguous target", discovery.StateAvailable, discovery.StateAvailable, []string{collect.WarningHPAAmbiguousTarget}, catalog.SectionsHPAsNotePartialTitle, catalog.SectionsHPAsNotePartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := hpaSnapshot(t)
			s.Quality.Capabilities[3].State = tt.hpa
			s.Quality.Capabilities[4].State = tt.without
			for _, code := range tt.warnings {
				s.Quality.Warnings = append(s.Quality.Warnings, snapshot.Message{Code: code, Params: map[string]float64{}})
			}
			blocks := hpaBlocks(t, s, catalog.English)

			callout, isCallout := blocks[2].(Callout)
			if tt.wantBody == "" {
				if isCallout {
					t.Fatalf("blocks[2] is a callout, want none")
				}
				return
			}
			if !isCallout {
				t.Fatalf("blocks[2] is %T, want Callout", blocks[2])
			}
			if callout.Kind != "warning" {
				t.Errorf("Kind = %q, want warning", callout.Kind)
			}
			if len(callout.Title) != 1 || runText(t, callout.Title[0]) != text(tt.wantTitle) {
				t.Errorf("Title = %+v, want %q", callout.Title, text(tt.wantTitle))
			}
			if len(callout.Runs) != 1 || runText(t, callout.Runs[0]) != text(tt.wantBody) {
				t.Errorf("Runs = %+v, want %q", callout.Runs, text(tt.wantBody))
			}
		})
	}
}

// hpaNames returns the first column of every row of the hpa-observed
// table among blocks, or nil when there is none.
func hpaNames(t *testing.T, blocks []Block) []string {
	t.Helper()
	for _, blk := range blocks {
		if table, ok := blk.(Table); ok && table.ID == "hpa-observed" {
			var names []string
			for _, row := range table.Rows {
				names = append(names, runText(t, row[0][0]))
			}
			return names
		}
	}
	return nil
}

func TestHPASectionHidesScaledToZero(t *testing.T) {
	s := hpaSnapshot(t)
	unknownPeak := snapshot.Statistics{State: stateUnavailable}
	averageOnly := 3.0
	noPeak := snapshot.Statistics{State: stateValue, Average: &averageOnly}
	s.HPAs = []snapshot.HPA{
		hpaRow("a-zero", floatPtr(0), floatPtr(5), hpaStat(0, 0, 0)),
		hpaRow("b-unknown", floatPtr(1), floatPtr(5), unknownPeak),
		hpaRow("c-no-peak", floatPtr(1), floatPtr(5), noPeak),
		hpaRow("d-busy", floatPtr(1), floatPtr(5), hpaStat(2, 1, 3)),
	}

	got := hpaNames(t, hpaBlocks(t, s, catalog.English))
	want := []string{"b-unknown", "c-no-peak", "d-busy"}
	if !slices.Equal(got, want) {
		t.Errorf("HPAs shown = %v, want %v", got, want)
	}
}

func TestHPASectionSortsByName(t *testing.T) {
	s := hpaSnapshot(t)
	s.HPAs = []snapshot.HPA{
		hpaRow("zeta", floatPtr(1), floatPtr(5), hpaStat(2, 1, 3)),
		hpaRow("alpha", floatPtr(1), floatPtr(5), hpaStat(2, 1, 3)),
	}
	got := hpaNames(t, hpaBlocks(t, s, catalog.English))
	if want := []string{"alpha", "zeta"}; !slices.Equal(got, want) {
		t.Errorf("HPAs shown = %v, want %v", got, want)
	}
}

func TestHPASectionSituation(t *testing.T) {
	b := &builder{cat: mustCatalog(t, catalog.English)}
	noPeak := snapshot.Statistics{State: stateUnavailable}

	tests := []struct {
		name       string
		hpa        snapshot.HPA
		wantStatus string
		wantText   string
	}{
		{"maximum reached", hpaRow("h", floatPtr(2), floatPtr(10), hpaStat(7, 2, 10)), "critical", "Maximum reached"},
		{"maximum reached needs no minimum", hpaRow("h", nil, floatPtr(10), hpaStat(7, 2, 12)), "critical", "Maximum reached"},
		{"zero maximum and zero peak", hpaRow("h", nil, floatPtr(0), hpaStat(0, 0, 0)), "critical", "Maximum reached"},
		{"pinned", hpaRow("h", floatPtr(4), floatPtr(4), hpaStat(4, 4, 3)), "healthy", "Pinned"},
		{"pinned at its peak", hpaRow("h", floatPtr(4), floatPtr(4), hpaStat(4, 4, 4)), "healthy", "Pinned"},
		{"below the maximum", hpaRow("h", floatPtr(2), floatPtr(10), hpaStat(7, 2, 9)), "healthy", "Below the maximum"},
		{"unknown peak", hpaRow("h", floatPtr(2), floatPtr(10), noPeak), "unavailable", "Unavailable"},
		{"unknown maximum", hpaRow("h", floatPtr(2), nil, hpaStat(7, 2, 9)), "unavailable", "Unavailable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertStatus(t, hpaSituation(b, tt.hpa), tt.wantStatus, tt.wantText)
		})
	}
	if b.err != nil {
		t.Fatalf("builder error = %v", b.err)
	}
}

func TestHPASectionCells(t *testing.T) {
	s := hpaSnapshot(t)
	s.HPAs = []snapshot.HPA{hpaRow("api", floatPtr(2), floatPtr(10), hpaStat(7.25, 2.5, 10))}
	blocks := hpaBlocks(t, s, catalog.PortugueseBrazil)
	table := findTable(t, blocks, "hpa-observed")

	row := table.Rows[0]
	if len(table.Headers) != 7 || len(row) != 7 {
		t.Fatalf("headers = %d, cells = %d, want 7 and 7", len(table.Headers), len(row))
	}
	if row[0][0].kind != "ident" {
		t.Errorf("name run kind = %q, want ident", row[0][0].kind)
	}
	want := []string{"api", "2", "10", "7,2", "2", "10"}
	for i, text := range want {
		if got := runText(t, row[i][0]); got != text {
			t.Errorf("cell %d = %q, want %q", i, got, text)
		}
	}
	assertStatus(t, row[6][0], "critical", "Máximo atingido")
}

func TestHPASectionUnknownCells(t *testing.T) {
	s := hpaSnapshot(t)
	s.HPAs = []snapshot.HPA{hpaRow("api", nil, nil, snapshot.Statistics{State: stateUnavailable})}
	table := findTable(t, hpaBlocks(t, s, catalog.English), "hpa-observed")
	for i := 1; i <= 5; i++ {
		if got := runText(t, table.Rows[0][i][0]); got != "Unavailable" {
			t.Errorf("cell %d = %q, want Unavailable", i, got)
		}
	}
}

func TestHPASectionEmptyNamespace(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	tests := []struct {
		name    string
		partial bool
		want    catalog.Key
	}{
		{"no note", false, catalog.SectionsHPAsEmptyNamespace},
		{"with a note", true, catalog.SectionsHPAsEmptyNamespacePartial},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := hpaSnapshot(t)
			if tt.partial {
				s.Quality.Capabilities[3].State = discovery.StatePartial
			}
			blocks := hpaBlocks(t, s, catalog.English)

			last, ok := blocks[len(blocks)-1].(Paragraph)
			if !ok {
				t.Fatalf("last block is %T, want Paragraph", blocks[len(blocks)-1])
			}
			want, err := cat.Text(tt.want, nil)
			if err != nil {
				t.Fatalf("Text(%q): %v", tt.want, err)
			}
			if got := runText(t, last.Runs[0]); got != want {
				t.Errorf("paragraph = %q, want %q", got, want)
			}
			heading := blocks[len(blocks)-2].(Heading)
			if heading.Level != 2 || heading.Numbered {
				t.Errorf("namespace heading = %+v, want level 2 unnumbered", heading)
			}
		})
	}
}

func uncoveredRow(kind, name string, pods *float64) snapshot.Workload {
	w := unavailableWorkload("payments", kind, name)
	w.Pods = pods
	w.HPACoverage = snapshot.CoverageUncovered
	return w
}

func TestHPASectionOneTableOnly(t *testing.T) {
	s := hpaSnapshot(t)
	s.Workloads = []snapshot.Workload{uncoveredRow("Deployment", "worker", floatPtr(2))}
	blocks := hpaBlocks(t, s, catalog.English)

	// The section heading, its paragraph, the namespace heading, then one
	// level-3 heading and one table.
	want := []string{"heading", "paragraph", "heading", "heading", "table"}
	if got := blockTypes(blocks); !slices.Equal(got, want) {
		t.Fatalf("block order = %v, want %v", got, want)
	}
	if h := blocks[3].(Heading); h.Level != 3 || h.Numbered {
		t.Errorf("table heading = %+v, want level 3 unnumbered", h)
	}
	if id := blocks[4].(Table).ID; id != "workloads-without-hpa" {
		t.Errorf("table = %q, want workloads-without-hpa", id)
	}
}

func TestHPASectionBothTables(t *testing.T) {
	s := hpaSnapshot(t)
	s.HPAs = []snapshot.HPA{hpaRow("api", floatPtr(2), floatPtr(10), hpaStat(7, 2, 9))}
	s.Workloads = []snapshot.Workload{uncoveredRow("Deployment", "worker", floatPtr(2))}
	want := []string{"heading", "paragraph", "heading", "heading", "table", "heading", "table"}
	if got := blockTypes(hpaBlocks(t, s, catalog.English)); !slices.Equal(got, want) {
		t.Fatalf("block order = %v, want %v", got, want)
	}
}

func TestHPASectionSkipsRowsThatAreNotUncovered(t *testing.T) {
	s := hpaSnapshot(t)
	covered := unavailableWorkload("payments", "Deployment", "api")
	covered.HPACoverage = snapshot.CoverageCovered
	elsewhere := uncoveredRow("Deployment", "other", nil)
	elsewhere.Namespace = "checkout"
	s.Workloads = []snapshot.Workload{covered, elsewhere}

	blocks := hpaBlocks(t, s, catalog.English)
	for _, blk := range blocks {
		if table, ok := blk.(Table); ok {
			t.Errorf("unexpected table %q", table.ID)
		}
	}
}

func TestUncoveredRowCells(t *testing.T) {
	s := hpaSnapshot(t)
	s.Workloads = []snapshot.Workload{
		uncoveredRow("StatefulSet", "db", nil),
		uncoveredRow("Deployment", "worker", floatPtr(2)),
		uncoveredRow("Deployment", "api", floatPtr(0)),
	}
	table := findTable(t, hpaBlocks(t, s, catalog.PortugueseBrazil), "workloads-without-hpa")
	if len(table.Headers) != 4 {
		t.Fatalf("len(Headers) = %d, want 4", len(table.Headers))
	}
	if len(table.Rows) != 3 {
		t.Fatalf("len(Rows) = %d, want 3", len(table.Rows))
	}

	// Sorted by kind, then name.
	order := []string{"api", "worker", "db"}
	for i, name := range order {
		if got := runText(t, table.Rows[i][1][0]); got != name {
			t.Errorf("row %d name = %q, want %q", i, got, name)
		}
		if table.Rows[i][1][0].kind != "ident" {
			t.Errorf("row %d name kind = %q, want ident", i, table.Rows[i][1][0].kind)
		}
	}
	if got := runText(t, table.Rows[1][0][0]); got != "Deployment" {
		t.Errorf("kind = %q, want Deployment", got)
	}

	pods := []string{"0", "2", "Indisponível"}
	for i, want := range pods {
		if got := runText(t, table.Rows[i][2][0]); got != want {
			t.Errorf("row %d pods = %q, want %q", i, got, want)
		}
	}
	assertStatus(t, table.Rows[1][3][0], "attention", "Sem HPA")
	assertStatus(t, table.Rows[2][3][0], "unavailable", "Estratégia de escala específica")
}

func TestHPASectionHeadingsAndIntro(t *testing.T) {
	s := hpaSnapshot(t)
	blocks := hpaBlocks(t, s, catalog.PortugueseBrazil)

	h := blocks[0].(Heading)
	if h.Level != 1 || !h.Numbered || runText(t, h.Runs[0]) != "Histórico de HPA" {
		t.Errorf("heading = %+v", h)
	}
	want := "Comportamento dos Horizontal Pod Autoscalers no período de 24 horas. A situação indica se o máximo configurado foi observado."
	if got := runText(t, blocks[1].(Paragraph).Runs[0]); got != want {
		t.Errorf("intro = %q, want %q", got, want)
	}
}

func TestHPASectionWalksTheScopeNamespaces(t *testing.T) {
	s := baseSnapshot(t) // checkout and payments
	other := hpaRow("api", floatPtr(1), floatPtr(5), hpaStat(2, 1, 3))
	other.Namespace = "elsewhere"
	s.HPAs = []snapshot.HPA{other}
	blocks := hpaBlocks(t, s, catalog.English)

	if got, want := namespacesShown(t, blocks), []string{"checkout", "payments"}; !slices.Equal(got, want) {
		t.Errorf("namespaces shown = %v, want %v", got, want)
	}
	if names := hpaNames(t, blocks); names != nil {
		t.Errorf("HPAs shown = %v, want none", names)
	}
}
