package document

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

func mustCatalog(t *testing.T, lang catalog.Language) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Load(lang)
	if err != nil {
		t.Fatalf("Load(%q): %v", lang, err)
	}
	return cat
}

func mustBuild(t *testing.T, s snapshot.Snapshot, c *catalog.Catalog, opts Options) Document {
	t.Helper()
	doc, err := Build(s, c, opts)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return doc
}

// baseCapabilities returns the nine capabilities in the order of
// discovery.Names: six available, two partial, one unavailable with two
// reasons.
func baseCapabilities() []snapshot.Capability {
	caps := make([]snapshot.Capability, len(discovery.Names))
	for i, name := range discovery.Names {
		caps[i] = snapshot.Capability{
			Name:     name,
			State:    discovery.StateAvailable,
			Strategy: discovery.StrategyRawMetrics,
			Evidence: []string{"metric_" + name},
			Reasons:  []string{},
		}
	}
	caps[6].State = discovery.StatePartial
	caps[6].Reasons = []string{discovery.ReasonCodes[2]}
	caps[7].State = discovery.StatePartial
	caps[7].Reasons = []string{discovery.ReasonCodes[3]}
	caps[8].State = discovery.StateUnavailable
	caps[8].Strategy = discovery.StrategyNone
	caps[8].Reasons = []string{discovery.ReasonCodes[0], discovery.ReasonCodes[1]}
	return caps
}

// baseSnapshot returns "base": a snapshot that passes Check(), with every
// optional field of the metadata table already set, so a table test can
// clear exactly one of them.
func baseSnapshot(t *testing.T) snapshot.Snapshot {
	t.Helper()

	snap := snapshot.Snapshot{
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
			Organization:     "Example Org",
			Classification:   "internal",
			Author:           "capacity-bot",
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
			DiscoveredValues:   []string{"prod", "staging"},
			Namespaces:         []string{"checkout", "payments"},
			ExcludedContainers: []string{"POD", "istio-proxy"},
		},
		Quality: snapshot.Quality{
			Capabilities:          baseCapabilities(),
			CollectionStartedAt:   "2026-07-15T14:25:00-03:00",
			CollectionCompletedAt: "2026-07-15T14:29:00-03:00",
		},
	}
	if err := snap.Check(); err != nil {
		t.Fatalf("baseSnapshot: Check() error = %v", err)
	}
	return snap
}

// runText returns the text a run holds, failing the test if it is null.
func runText(t *testing.T, r Run) string {
	t.Helper()
	if r.text == nil {
		t.Fatalf("run %+v has null text", r)
	}
	return *r.text
}

// blockTypes returns the kind of every block, in order.
func blockTypes(blocks []Block) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.blockType()
	}
	return out
}

// frontBlock returns doc.Blocks[0] as a Front, failing the test if it is
// not one.
func frontBlock(t *testing.T, doc Document) Front {
	t.Helper()
	f, ok := doc.Blocks[0].(Front)
	if !ok {
		t.Fatalf("Blocks[0] is %T, want Front", doc.Blocks[0])
	}
	return f
}

// findTable returns the table of id among blocks, failing the test if
// there is none.
func findTable(t *testing.T, blocks []Block, id string) Table {
	t.Helper()
	for _, b := range blocks {
		if table, ok := b.(Table); ok && table.ID == id {
			return table
		}
	}
	t.Fatalf("no table %q found among %d blocks", id, len(blocks))
	return Table{}
}

// allTableRowCounts maps every table id of doc, at the top level and
// inside front, to its row count.
func allTableRowCounts(doc Document) map[string]int {
	out := make(map[string]int)
	collect := func(blocks []Block) {
		for _, b := range blocks {
			if table, ok := b.(Table); ok {
				out[table.ID] = len(table.Rows)
			}
		}
	}
	collect(doc.Blocks)
	if len(doc.Blocks) > 0 {
		if f, ok := doc.Blocks[0].(Front); ok {
			collect(f.Blocks)
		}
	}
	return out
}

func TestBuildTopLevel(t *testing.T) {
	tests := []struct {
		lang                            catalog.Language
		wantLang, wantRegion            string
		wantNote, wantInfo, wantWarning string
		wantPageLabel                   string
	}{
		{catalog.PortugueseBrazil, "pt", "BR", "Nota", "Informação", "Atenção", "Página"},
		{catalog.English, "en", "US", "Note", "Information", "Warning", "Page"},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			s := baseSnapshot(t)
			doc := mustBuild(t, s, mustCatalog(t, tt.lang), Options{})

			if doc.Schema != Schema {
				t.Errorf("Schema = %q, want %q", doc.Schema, Schema)
			}
			if doc.Language != string(tt.lang) {
				t.Errorf("Language = %q, want %q", doc.Language, tt.lang)
			}
			if doc.Theme.Lang != tt.wantLang || doc.Theme.Region != tt.wantRegion {
				t.Errorf("Theme = %+v, want (%q, %q)", doc.Theme, tt.wantLang, tt.wantRegion)
			}
			if doc.Labels.Note != tt.wantNote || doc.Labels.Info != tt.wantInfo || doc.Labels.Warning != tt.wantWarning {
				t.Errorf("Labels = %+v, want (%q, %q, %q)", doc.Labels, tt.wantNote, tt.wantInfo, tt.wantWarning)
			}
			if doc.Footer.PageLabel != tt.wantPageLabel {
				t.Errorf("Footer.PageLabel = %q, want %q", doc.Footer.PageLabel, tt.wantPageLabel)
			}
		})
	}
}

func TestBuildCover(t *testing.T) {
	tests := []struct {
		lang                                                                      catalog.Language
		wantTitle, wantClusterLabel, wantAuthorLabel, wantIssuedLabel, wantIssued string
	}{
		{catalog.PortugueseBrazil, "LEVANTAMENTO DE CAPACIDADE", "Cluster:", "ELABORADO POR:", "EMITIDO EM:", "15/07/2026 14:30"},
		{catalog.English, "CAPACITY REPORT", "Cluster:", "PREPARED BY:", "ISSUED ON:", "07/15/2026 14:30"},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			s := baseSnapshot(t)
			doc := mustBuild(t, s, mustCatalog(t, tt.lang), Options{})
			cover := doc.Cover

			if cover.Title != tt.wantTitle {
				t.Errorf("Title = %q, want %q", cover.Title, tt.wantTitle)
			}
			if cover.ClusterLabel != tt.wantClusterLabel {
				t.Errorf("ClusterLabel = %q, want %q", cover.ClusterLabel, tt.wantClusterLabel)
			}
			if cover.Cluster != "prod" {
				t.Errorf("Cluster = %q, want %q", cover.Cluster, "prod")
			}
			if cover.AuthorLabel != tt.wantAuthorLabel {
				t.Errorf("AuthorLabel = %q, want %q", cover.AuthorLabel, tt.wantAuthorLabel)
			}
			if cover.Author == nil || *cover.Author != "capacity-bot" {
				t.Errorf("Author = %v, want %q", cover.Author, "capacity-bot")
			}
			if cover.IssuedLabel != tt.wantIssuedLabel {
				t.Errorf("IssuedLabel = %q, want %q", cover.IssuedLabel, tt.wantIssuedLabel)
			}
			if cover.Issued != tt.wantIssued {
				t.Errorf("Issued = %q, want %q", cover.Issued, tt.wantIssued)
			}
		})
	}
}

func TestBuildCoverWithoutAuthor(t *testing.T) {
	s := baseSnapshot(t)
	s.Report.Author = ""
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	if doc.Cover.Author != nil {
		t.Fatalf("Cover.Author = %q, want nil", *doc.Cover.Author)
	}
	data, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.Contains(string(data), `"author": null`) {
		t.Fatalf("encoded document has no \"author\": null: %s", data)
	}
}

func TestBuildFooterWithoutClassification(t *testing.T) {
	s := baseSnapshot(t)
	s.Report.Classification = ""
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	if doc.Footer.Classification != nil {
		t.Fatalf("Footer.Classification = %q, want nil", *doc.Footer.Classification)
	}
	data, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.Contains(string(data), `"classification": null`) {
		t.Fatalf("encoded document has no \"classification\": null: %s", data)
	}
}

func TestBuildBlockOrder(t *testing.T) {
	s := baseSnapshot(t)
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	wantTop := []string{"front", "pagebreak", "pagebreak", "heading", "table"}
	if got := blockTypes(doc.Blocks); !slices.Equal(got, wantTop) {
		t.Fatalf("top-level block order = %v, want %v", got, wantTop)
	}

	wantFront := []string{"outline", "heading", "table", "heading", "table", "heading", "list"}
	front := frontBlock(t, doc)
	if got := blockTypes(front.Blocks); !slices.Equal(got, wantFront) {
		t.Fatalf("front block order = %v, want %v", got, wantFront)
	}
}

func TestBuildMetadataRows(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*snapshot.Snapshot)
		wantRows int
	}{
		{"all set", func(*snapshot.Snapshot) {}, 6},
		{"no organization", func(s *snapshot.Snapshot) { s.Report.Organization = "" }, 5},
		{"no classification", func(s *snapshot.Snapshot) { s.Report.Classification = "" }, 5},
		{"no excluded container", func(s *snapshot.Snapshot) { s.Scope.ExcludedContainers = []string{} }, 6},
	}
	cat := mustCatalog(t, catalog.English)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := baseSnapshot(t)
			tt.mutate(&s)
			doc := mustBuild(t, s, cat, Options{})
			table := findTable(t, frontBlock(t, doc).Blocks, "report-metadata")
			if len(table.Rows) != tt.wantRows {
				t.Fatalf("len(Rows) = %d, want %d", len(table.Rows), tt.wantRows)
			}
		})
	}
}

func TestBuildMetadataNoExcludedContainersFallback(t *testing.T) {
	tests := []struct {
		lang catalog.Language
		want string
	}{
		{catalog.PortugueseBrazil, "Nenhum"},
		{catalog.English, "None"},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			s := baseSnapshot(t)
			s.Scope.ExcludedContainers = []string{}
			doc := mustBuild(t, s, mustCatalog(t, tt.lang), Options{})
			table := findTable(t, frontBlock(t, doc).Blocks, "report-metadata")
			got := runText(t, table.Rows[3][1][0])
			if got != tt.want {
				t.Errorf("excluded containers value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMetadataJoinsNames(t *testing.T) {
	s := baseSnapshot(t)
	s.Scope.Namespaces = []string{"a", "b", "c"}
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})
	table := findTable(t, frontBlock(t, doc).Blocks, "report-metadata")
	got := runText(t, table.Rows[2][1][0])
	if got != "a, b, c" {
		t.Errorf("Namespaces value = %q, want %q", got, "a, b, c")
	}
}

func TestBuildLegendRows(t *testing.T) {
	wantStatuses := []string{"healthy", "attention", "critical", "unavailable", "undefined"}
	for _, lang := range catalog.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			s := baseSnapshot(t)
			doc := mustBuild(t, s, mustCatalog(t, lang), Options{})
			table := findTable(t, frontBlock(t, doc).Blocks, "status-legend")

			if len(table.Rows) != 5 {
				t.Fatalf("len(Rows) = %d, want 5", len(table.Rows))
			}
			for i, row := range table.Rows {
				cell := row[0]
				if len(cell) != 1 {
					t.Fatalf("row %d first cell has %d runs, want 1", i, len(cell))
				}
				run := cell[0]
				if run.kind != "status" {
					t.Errorf("row %d run kind = %q, want status", i, run.kind)
				}
				if run.status != wantStatuses[i] {
					t.Errorf("row %d status = %q, want %q", i, run.status, wantStatuses[i])
				}
				if run.text != nil {
					t.Errorf("row %d text = %q, want null", i, *run.text)
				}
			}
		})
	}
}

func TestBuildQualityCounts(t *testing.T) {
	s := baseSnapshot(t)
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})
	list, ok := frontBlock(t, doc).Blocks[6].(List)
	if !ok {
		t.Fatalf("front.Blocks[6] is %T, want List", frontBlock(t, doc).Blocks[6])
	}

	wantCounts := []string{" 6", " 2", " 1"}
	for i, item := range list.Items {
		if len(item) != 2 {
			t.Fatalf("item %d has %d runs, want 2", i, len(item))
		}
		if item[0].kind != "strong" {
			t.Errorf("item %d first run kind = %q, want strong", i, item[0].kind)
		}
		if got := runText(t, item[1]); got != wantCounts[i] {
			t.Errorf("item %d count = %q, want %q", i, got, wantCounts[i])
		}
	}
}

func TestBuildQualityCountsIgnoreSeverity(t *testing.T) {
	s := baseSnapshot(t)
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{MinSeverity: "critical"})
	list := frontBlock(t, doc).Blocks[6].(List)

	wantCounts := []string{" 6", " 2", " 1"}
	for i, item := range list.Items {
		if got := runText(t, item[1]); got != wantCounts[i] {
			t.Errorf("item %d count = %q, want %q", i, got, wantCounts[i])
		}
	}
}

func TestBuildAppendixRows(t *testing.T) {
	s := baseSnapshot(t)
	cat := mustCatalog(t, catalog.English)
	doc := mustBuild(t, s, cat, Options{})
	table := findTable(t, doc.Blocks, "quality-appendix")

	if len(table.Rows) != 9 {
		t.Fatalf("len(Rows) = %d, want 9", len(table.Rows))
	}
	for i, row := range table.Rows {
		if len(row) != 5 {
			t.Fatalf("row %d has %d cells, want 5", i, len(row))
		}
		wantName, err := cat.Text(catalog.CapabilityName(discovery.Names[i]), nil)
		if err != nil {
			t.Fatalf("Text(CapabilityName(%q)): %v", discovery.Names[i], err)
		}
		if got := runText(t, row[0][0]); got != wantName {
			t.Errorf("row %d capability = %q, want %q", i, got, wantName)
		}
	}
}

func TestBuildAppendixCoverage(t *testing.T) {
	tests := []struct {
		state, status, wantLabel string
	}{
		{discovery.StateAvailable, "healthy", "Available"},
		{discovery.StatePartial, "attention", "Partial"},
		{discovery.StateUnavailable, "critical", "Unavailable"},
	}
	cat := mustCatalog(t, catalog.English)
	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			s := baseSnapshot(t)
			s.Quality.Capabilities[0].State = tt.state
			switch tt.state {
			case discovery.StatePartial, discovery.StateUnavailable:
				s.Quality.Capabilities[0].Reasons = []string{discovery.ReasonCodes[0]}
			}
			if tt.state == discovery.StateUnavailable {
				s.Quality.Capabilities[0].Strategy = discovery.StrategyNone
			}
			doc := mustBuild(t, s, cat, Options{MinSeverity: "all"})
			table := findTable(t, doc.Blocks, "quality-appendix")

			cell := table.Rows[0][1]
			if len(cell) != 1 {
				t.Fatalf("coverage cell has %d runs, want 1", len(cell))
			}
			run := cell[0]
			if run.kind != "status" || run.status != tt.status {
				t.Errorf("coverage run = %+v, want status %q", run, tt.status)
			}
			if got := runText(t, run); got != tt.wantLabel {
				t.Errorf("coverage label = %q, want %q", got, tt.wantLabel)
			}
		})
	}
}

func TestBuildAppendixRemark(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	reasonSentence := func(t *testing.T, code string) string {
		t.Helper()
		text, err := cat.Text(catalog.CapabilityReason(code), nil)
		if err != nil {
			t.Fatalf("Text(CapabilityReason(%q)): %v", code, err)
		}
		return text
	}

	tests := []struct {
		name    string
		reasons []string
		want    func(t *testing.T) string
	}{
		{"no reason", nil, func(*testing.T) string { return "No remark" }},
		{
			"one reason",
			[]string{discovery.ReasonCodes[0]},
			func(t *testing.T) string { return reasonSentence(t, discovery.ReasonCodes[0]) },
		},
		{
			"two reasons",
			[]string{discovery.ReasonCodes[0], discovery.ReasonCodes[1]},
			func(t *testing.T) string {
				return reasonSentence(t, discovery.ReasonCodes[0]) + " " + reasonSentence(t, discovery.ReasonCodes[1])
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := baseSnapshot(t)
			if tt.reasons == nil {
				// Check refuses an unavailable capability with no reason,
				// so the fallback "No remark" text is reached from an
				// available one instead: available is the one state that
				// may legitimately carry no reason.
				s.Quality.Capabilities[8].State = discovery.StateAvailable
				s.Quality.Capabilities[8].Strategy = discovery.StrategyRawMetrics
				s.Quality.Capabilities[8].Reasons = []string{}
			} else {
				s.Quality.Capabilities[8].State = discovery.StateUnavailable
				s.Quality.Capabilities[8].Strategy = discovery.StrategyNone
				s.Quality.Capabilities[8].Reasons = tt.reasons
			}
			doc := mustBuild(t, s, cat, Options{})
			table := findTable(t, doc.Blocks, "quality-appendix")

			got := runText(t, table.Rows[8][3][0])
			want := tt.want(t)
			if got != want {
				t.Errorf("remark = %q, want %q", got, want)
			}
		})
	}
}

func TestBuildAppendixEvidence(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	for idx, name := range discovery.Names {
		for _, state := range discovery.States {
			t.Run(name+"/"+state, func(t *testing.T) {
				s := baseSnapshot(t)
				// Every field Check cross-checks against state is
				// normalised together, regardless of what the base
				// fixture happened to hold at this index: a state, a
				// strategy, and a set of reasons that agree with each
				// other are what every one of the 27 combinations this
				// test exercises must produce.
				s.Quality.Capabilities[idx].State = state
				switch state {
				case discovery.StateAvailable:
					s.Quality.Capabilities[idx].Strategy = discovery.StrategyRawMetrics
					s.Quality.Capabilities[idx].Reasons = []string{}
				case discovery.StatePartial:
					s.Quality.Capabilities[idx].Strategy = discovery.StrategyRawMetrics
					s.Quality.Capabilities[idx].Reasons = []string{discovery.ReasonCodes[0]}
				case discovery.StateUnavailable:
					s.Quality.Capabilities[idx].Strategy = discovery.StrategyNone
					s.Quality.Capabilities[idx].Reasons = []string{discovery.ReasonCodes[0]}
				}
				doc := mustBuild(t, s, cat, Options{})
				table := findTable(t, doc.Blocks, "quality-appendix")

				want, err := cat.Text(catalog.CapabilityEvidence(name, state), nil)
				if err != nil {
					t.Fatalf("Text(CapabilityEvidence(%q, %q)): %v", name, state, err)
				}
				if got := runText(t, table.Rows[idx][4][0]); got != want {
					t.Errorf("evidence = %q, want %q", got, want)
				}
			})
		}
	}
}

func TestBuildAppendixSeverity(t *testing.T) {
	tests := []struct {
		severity string
		wantRows int
	}{
		{"all", 9},
		{"attention", 3},
		{"critical", 1},
	}
	cat := mustCatalog(t, catalog.English)
	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			s := baseSnapshot(t)
			doc := mustBuild(t, s, cat, Options{MinSeverity: tt.severity})
			table := findTable(t, doc.Blocks, "quality-appendix")
			if len(table.Rows) != tt.wantRows {
				t.Errorf("len(Rows) = %d, want %d", len(table.Rows), tt.wantRows)
			}
		})
	}
}

func TestBuildAppendixEmpty(t *testing.T) {
	s := baseSnapshot(t)
	for i := range s.Quality.Capabilities {
		s.Quality.Capabilities[i].State = discovery.StateAvailable
		s.Quality.Capabilities[i].Strategy = discovery.StrategyRawMetrics
		s.Quality.Capabilities[i].Reasons = []string{}
	}
	cat := mustCatalog(t, catalog.English)
	doc := mustBuild(t, s, cat, Options{MinSeverity: "attention"})

	if len(doc.Blocks) != 5 {
		t.Fatalf("len(Blocks) = %d, want 5", len(doc.Blocks))
	}
	para, ok := doc.Blocks[4].(Paragraph)
	if !ok {
		t.Fatalf("Blocks[4] is %T, want Paragraph", doc.Blocks[4])
	}
	want, err := cat.Text(catalog.AppendixEmpty, nil)
	if err != nil {
		t.Fatalf("Text(AppendixEmpty): %v", err)
	}
	if got := runText(t, para.Runs[0]); got != want {
		t.Errorf("paragraph text = %q, want %q", got, want)
	}
	for _, b := range doc.Blocks {
		if _, ok := b.(Table); ok {
			t.Fatalf("a table block follows the empty appendix, want none")
		}
	}
}

func TestBuildOptionsDefaultToTheSnapshot(t *testing.T) {
	s := baseSnapshot(t)
	s.Report.MinSeverity = "attention"
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})
	table := findTable(t, doc.Blocks, "quality-appendix")
	if len(table.Rows) != 3 {
		t.Errorf("len(Rows) = %d, want 3", len(table.Rows))
	}
}

func TestBuildOptionsOverrideTheSnapshot(t *testing.T) {
	s := baseSnapshot(t)
	s.Report.MinSeverity = "attention"
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{MinSeverity: "all"})
	table := findTable(t, doc.Blocks, "quality-appendix")
	if len(table.Rows) != 9 {
		t.Errorf("len(Rows) = %d, want 9", len(table.Rows))
	}
}

func TestBuildRefuses(t *testing.T) {
	cat := mustCatalog(t, catalog.English)

	t.Run("profile executive", func(t *testing.T) {
		_, err := Build(baseSnapshot(t), cat, Options{Profile: "executive"})
		if err == nil || !strings.Contains(err.Error(), `"executive"`) {
			t.Fatalf("Build() error = %v, want a message naming \"executive\"", err)
		}
	})

	t.Run("profile x", func(t *testing.T) {
		_, err := Build(baseSnapshot(t), cat, Options{Profile: "x"})
		if err == nil || !strings.Contains(err.Error(), `"x"`) {
			t.Fatalf("Build() error = %v, want a message naming \"x\"", err)
		}
	})

	t.Run("severity x", func(t *testing.T) {
		_, err := Build(baseSnapshot(t), cat, Options{MinSeverity: "x"})
		if err == nil || !strings.Contains(err.Error(), `"x"`) {
			t.Fatalf("Build() error = %v, want a message naming \"x\"", err)
		}
	})

	t.Run("a snapshot with eight capabilities", func(t *testing.T) {
		s := baseSnapshot(t)
		s.Quality.Capabilities = s.Quality.Capabilities[:8]
		if _, err := Build(s, cat, Options{}); err == nil {
			t.Fatal("Build() error = nil, want one")
		}
	})
}

func TestBuildPrintsNamesAsTheyAre(t *testing.T) {
	s := baseSnapshot(t)
	s.Scope.ClusterName = `#prod *x* $y$ <a> @b [c]`
	s.Report.Organization = `ACME & Filhos "Ltda."`
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	if doc.Cover.Cluster != s.Scope.ClusterName {
		t.Errorf("Cover.Cluster = %q, want %q", doc.Cover.Cluster, s.Scope.ClusterName)
	}

	table := findTable(t, frontBlock(t, doc).Blocks, "report-metadata")
	if got := runText(t, table.Rows[4][1][0]); got != s.Report.Organization {
		t.Errorf("Organization value = %q, want %q", got, s.Report.Organization)
	}

	data, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	rendered := string(data)

	// ClusterName holds no quote and no backslash, so it must appear
	// verbatim, character for character: HTML escaping would otherwise
	// turn its angle brackets into a six-character escape sequence.
	if !strings.Contains(rendered, s.Scope.ClusterName) {
		t.Errorf("encoded document does not contain the cluster name verbatim: %s", rendered)
	}

	// Organization holds a double quote, which valid JSON always escapes
	// with a backslash regardless of the HTML-escaping setting; only its
	// "&", ahead of that quote, is at risk of HTML escaping.
	if !strings.Contains(rendered, "ACME & Filhos") {
		t.Errorf("encoded document does not contain the organization's unescaped prefix: %s", rendered)
	}

	// None of the six-character escape sequences HTML escaping would use
	// for "<", ">", or "&" may appear anywhere in the document.
	escapes := []string{"\\u003c", "\\u003e", "\\u0026"}
	for _, escape := range escapes {
		if strings.Contains(rendered, escape) {
			t.Errorf("encoded document HTML-escapes a character with %q: %s", escape, rendered)
		}
	}
}

func TestBuildNeedsNoZoneData(t *testing.T) {
	t.Setenv("ZONEINFO", filepath.Join(t.TempDir(), "does-not-exist"))

	s := baseSnapshot(t)
	s.Report.EmittedAt = "2026-07-15T14:30:00-03:00"
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	if !strings.Contains(doc.Cover.Issued, "14:30") {
		t.Errorf("Cover.Issued = %q, want it to contain 14:30", doc.Cover.Issued)
	}
}

func TestSameSnapshotGivesSameBytes(t *testing.T) {
	s := baseSnapshot(t)
	for _, lang := range catalog.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			cat := mustCatalog(t, lang)
			first := mustBuild(t, s, cat, Options{})
			second := mustBuild(t, s, cat, Options{})

			firstBytes, err := Encode(first)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			secondBytes, err := Encode(second)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}
			if string(firstBytes) != string(secondBytes) {
				t.Fatalf("Encode() is not stable across two Build() calls")
			}
		})
	}
}

func TestLanguagesDifferOnlyInWording(t *testing.T) {
	s := baseSnapshot(t)
	pt := mustBuild(t, s, mustCatalog(t, catalog.PortugueseBrazil), Options{})
	en := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})

	if got, want := blockTypes(pt.Blocks), blockTypes(en.Blocks); !slices.Equal(got, want) {
		t.Fatalf("top-level block kinds differ: %v vs %v", got, want)
	}
	if got, want := blockTypes(frontBlock(t, pt).Blocks), blockTypes(frontBlock(t, en).Blocks); !slices.Equal(got, want) {
		t.Fatalf("front block kinds differ: %v vs %v", got, want)
	}

	ptTables := allTableRowCounts(pt)
	enTables := allTableRowCounts(en)
	if len(ptTables) != len(enTables) {
		t.Fatalf("table count differs: %d vs %d", len(ptTables), len(enTables))
	}
	for id, count := range ptTables {
		if enTables[id] != count {
			t.Errorf("table %q row count differs: %d vs %d", id, count, enTables[id])
		}
	}
}
