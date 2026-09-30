package document

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

func TestNotesOfRawFallback(t *testing.T) {
	s := baseSnapshot(t)
	s.Quality.Warnings = []snapshot.Message{
		{Code: collect.WarningWorkloadHistoryCurrentAssociation, Params: map[string]float64{}},
	}
	cat := mustCatalog(t, catalog.English)
	doc := mustBuild(t, s, cat, Options{})

	table := findTable(t, doc.Blocks, "interpretation-notes")
	if len(table.Rows) != 1 {
		t.Fatalf("len(Rows) = %d, want 1", len(table.Rows))
	}
	want, err := cat.Text(catalog.Warning(collect.WarningWorkloadHistoryCurrentAssociation), nil)
	if err != nil {
		t.Fatalf("Text(Warning): %v", err)
	}
	if got := runText(t, table.Rows[0][0][0]); got != want {
		t.Errorf("note text = %q, want %q", got, want)
	}
}

func TestNotesRenderAReasonCode(t *testing.T) {
	s := baseSnapshot(t)
	s.Quality.Warnings = []snapshot.Message{
		{Code: "METRICS_NOT_FOUND", Params: map[string]float64{}},
	}
	cat := mustCatalog(t, catalog.PortugueseBrazil)
	doc := mustBuild(t, s, cat, Options{})

	table := findTable(t, doc.Blocks, "interpretation-notes")
	want, err := cat.Text(catalog.CapabilityReason("METRICS_NOT_FOUND"), nil)
	if err != nil {
		t.Fatalf("Text(CapabilityReason): %v", err)
	}
	found := 0
	for _, row := range table.Rows {
		if runText(t, row[0][0]) == want {
			found++
		}
	}
	if found != 1 {
		t.Errorf("the sentence of METRICS_NOT_FOUND appears %d times among the notes, want 1", found)
	}
}

func TestNotesChooseThePlural(t *testing.T) {
	cat := mustCatalog(t, catalog.English)

	tests := []struct {
		count  float64
		key    catalog.Key
		params catalog.Params
	}{
		{1, catalog.Notice(collect.NoticeWorkloadsOmittedNoUsage, 1), nil},
		{3, catalog.Notice(collect.NoticeWorkloadsOmittedNoUsage, 3), catalog.Params{"Count": "3"}},
	}
	for _, tt := range tests {
		s := baseSnapshot(t)
		s.Quality.Notices = []snapshot.Message{
			{Code: collect.NoticeWorkloadsOmittedNoUsage, Params: map[string]float64{"count": tt.count}},
		}
		doc := mustBuild(t, s, cat, Options{})
		table := findTable(t, doc.Blocks, "interpretation-notes")

		want, err := cat.Text(tt.key, tt.params)
		if err != nil {
			t.Fatalf("Text(): %v", err)
		}
		if got := runText(t, table.Rows[0][0][0]); got != want {
			t.Errorf("count %v: note text = %q, want %q", tt.count, got, want)
		}
	}
}

func TestNotesAreLeftOutWhenEmpty(t *testing.T) {
	s := baseSnapshot(t)
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	for _, blk := range doc.Blocks {
		if table, ok := blk.(Table); ok && table.ID == "interpretation-notes" {
			t.Fatalf("document holds an interpretation-notes table, want none")
		}
	}
	last := doc.Blocks[len(doc.Blocks)-1]
	if _, ok := last.(Table); !ok {
		t.Fatalf("last block is %T, want Table", last)
	}
}

func TestNotesIgnoreTheMinimumSeverity(t *testing.T) {
	s := baseSnapshot(t)
	s.Quality.Warnings = []snapshot.Message{
		{Code: collect.WarningNamespaceUsageIncomplete, Params: map[string]float64{}},
	}
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{MinSeverity: "critical"})

	table := findTable(t, doc.Blocks, "interpretation-notes")
	if len(table.Rows) != 1 {
		t.Errorf("len(Rows) = %d, want 1", len(table.Rows))
	}
}
