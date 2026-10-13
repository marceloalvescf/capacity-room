package document

import (
	"slices"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
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
	last := doc.Blocks[len(doc.Blocks)-2]
	if _, ok := last.(Table); !ok {
		t.Fatalf("block before the colophon is %T, want Table", last)
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

func TestEffectiveCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		warning  string
		capIndex int
		counts   []string
	}{
		{"ambiguous spec turns hpa_history partial", collect.WarningHPAAmbiguousSpec, 3, []string{" 5", " 3", " 1"}},
		{"ambiguous target turns workloads_without_hpa partial", collect.WarningHPAAmbiguousTarget, 4, []string{" 5", " 3", " 1"}},
		{"incomplete node evidence turns node_utilization partial", collect.WarningNodeCapacityEvidenceIncomplete, 7, []string{" 6", " 2", " 1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := baseSnapshot(t)
			s.Quality.Capabilities[tt.capIndex].State = discovery.StateAvailable
			s.Quality.Capabilities[tt.capIndex].Reasons = []string{}
			s.Quality.Warnings = []snapshot.Message{{Code: tt.warning, Params: map[string]float64{}}}

			got := effectiveCapabilities(s)
			if got[tt.capIndex].State != discovery.StatePartial {
				t.Errorf("state = %q, want partial", got[tt.capIndex].State)
			}
			if want := []string{tt.warning}; !slices.Equal(got[tt.capIndex].Reasons, want) {
				t.Errorf("reasons = %v, want %v", got[tt.capIndex].Reasons, want)
			}
			if s.Quality.Capabilities[tt.capIndex].State != discovery.StateAvailable {
				t.Errorf("the snapshot's own capability changed to %q", s.Quality.Capabilities[tt.capIndex].State)
			}

			cat := mustCatalog(t, catalog.English)
			doc := mustBuild(t, s, cat, Options{})
			list := frontBlock(t, doc).Blocks[6].(List)
			for i, want := range tt.counts {
				if got := runText(t, list.Items[i][1]); got != want {
					t.Errorf("front count %d = %q, want %q", i, got, want)
				}
			}
			table := findTable(t, doc.Blocks, "quality-appendix")
			wantRemark, err := cat.Text(catalog.Warning(tt.warning), nil)
			if err != nil {
				t.Fatalf("Text(Warning): %v", err)
			}
			if got := runText(t, table.Rows[tt.capIndex][3][0]); got != wantRemark {
				t.Errorf("remark = %q, want %q", got, wantRemark)
			}
			assertStatus(t, table.Rows[tt.capIndex][1][0], "attention", "Partial")
		})
	}
}

func TestEffectiveCapabilitiesLeaveOtherStatesAlone(t *testing.T) {
	s := baseSnapshot(t)
	s.Quality.Capabilities[3].State = discovery.StateUnavailable
	s.Quality.Capabilities[3].Reasons = []string{discovery.ReasonCodes[0]}
	s.Quality.Warnings = []snapshot.Message{{Code: collect.WarningHPAAmbiguousSpec, Params: map[string]float64{}}}

	got := effectiveCapabilities(s)
	if got[3].State != discovery.StateUnavailable || !slices.Equal(got[3].Reasons, []string{discovery.ReasonCodes[0]}) {
		t.Errorf("hpa_history = %+v, want it left unavailable with its own reason", got[3])
	}
}

func TestAppendixStatusDrawsUnavailableAsUnavailable(t *testing.T) {
	for state, want := range map[string]string{
		discovery.StateAvailable:   "healthy",
		discovery.StatePartial:     "attention",
		discovery.StateUnavailable: "unavailable",
	} {
		if got := appendixStatus(state); got != want {
			t.Errorf("appendixStatus(%q) = %q, want %q", state, got, want)
		}
	}
}

func TestCriticalMinimumKeepsOnlyUnavailableCapabilities(t *testing.T) {
	for state, want := range map[string]bool{
		discovery.StateAvailable:   false,
		discovery.StatePartial:     false,
		discovery.StateUnavailable: true,
	} {
		if got := passesSeverity(state, "critical"); got != want {
			t.Errorf("passesSeverity(%q, critical) = %v, want %v", state, got, want)
		}
	}
}
