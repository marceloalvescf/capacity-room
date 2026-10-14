package document

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
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

// unavailableNamespace returns a namespace row that answered nothing:
// every statistic unavailable, and its pod count unknown.
func unavailableNamespace(name string) snapshot.Namespace {
	unavailable := snapshot.Statistics{State: stateUnavailable}
	return snapshot.Namespace{
		Name:                 name,
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

// baseSnapshot returns "base": a snapshot that passes Check(), with every
// optional field of the metadata table already set, so a table test can
// clear exactly one of them, and one row for each namespace of its scope,
// sorted, every statistic unavailable and every pod count unknown.
// unknownAutoscaling is the autoscaling of a run that learned nothing
// about it.
func unknownAutoscaling() snapshot.Autoscaling {
	return snapshot.Autoscaling{Status: "unknown", Source: "none", Confidence: "unknown"}
}

func baseSnapshot(t *testing.T) snapshot.Snapshot {
	t.Helper()

	snap := snapshot.Snapshot{
		SchemaVersion: snapshot.SchemaVersion,
		Report: snapshot.Report{
			ID:               "run-001",
			GeneratorVersion: "capacityroom-9.9.9",
			EmittedAt:        "2026-07-15T14:30:00-03:00",
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
		Summary: snapshot.Summary{Namespaces: 2, Autoscaling: unknownAutoscaling()},
		Namespaces: []snapshot.Namespace{
			unavailableNamespace("checkout"),
			unavailableNamespace("payments"),
		},
		Workloads: []snapshot.Workload{},
		HPAs:      []snapshot.HPA{},
		Nodes:     []snapshot.Node{},
		Nodepools: []snapshot.NodePool{},
		Findings:  []snapshot.Finding{},
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

// frontBlock returns doc.Blocks[2], the opening page after the verdict, as
// a Front, failing the test if it is not one.
func frontBlock(t *testing.T, doc Document) Front {
	t.Helper()
	f, ok := doc.Blocks[2].(Front)
	if !ok {
		t.Fatalf("Blocks[2] is %T, want Front", doc.Blocks[2])
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
	for _, b := range doc.Blocks {
		if f, ok := b.(Front); ok {
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
		wantPeriod, wantProfile                                                   string
	}{
		{catalog.PortugueseBrazil, "Levantamento de capacidade", "Cluster", "Elaborado por", "Emitido em", "2026-07-15 14:30", "2026-07-14 14:30 a 2026-07-15 14:30", "Detalhado"},
		{catalog.English, "Capacity report", "Cluster", "Prepared by", "Issued on", "2026-07-15 14:30", "2026-07-14 14:30 to 2026-07-15 14:30", "Detailed"},
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
			if cover.Period != tt.wantPeriod {
				t.Errorf("Period = %q, want %q", cover.Period, tt.wantPeriod)
			}
			if cover.Profile != tt.wantProfile {
				t.Errorf("Profile = %q, want %q", cover.Profile, tt.wantProfile)
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

	wantTop := []string{
		"verdict", "pagebreak", "front", "pagebreak",
		// The executive section: heading, introduction, six subsections that
		// are a heading and a table or a paragraph, and the limitations table.
		"heading", "paragraph",
		"heading", "paragraph", "heading", "paragraph", "heading", "paragraph",
		"heading", "paragraph", "heading", "paragraph", "heading", "paragraph",
		"heading", "table",
		// The four sections of the evidence.
		"heading", "heading", "table", "heading", "table",
		"heading", "paragraph",
		"heading", "paragraph", "heading", "paragraph", "heading", "paragraph",
		"heading", "heading", "paragraph", "heading", "paragraph", "heading", "paragraph",
		"pagebreak",
		"heading", "table",
		"colophon",
	}
	if got := blockTypes(doc.Blocks); !slices.Equal(got, wantTop) {
		t.Fatalf("top-level block order = %v, want %v", got, wantTop)
	}

	wantFront := []string{"outline", "heading", "table", "heading", "table", "heading", "list"}
	front := frontBlock(t, doc)
	if got := blockTypes(front.Blocks); !slices.Equal(got, wantFront) {
		t.Fatalf("front block order = %v, want %v", got, wantFront)
	}
}

func TestBuildOrdersTheSections(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "snapshot", "direct-complete.json"))
	if err != nil {
		t.Fatalf("reading the golden snapshot: %v", err)
	}
	s, err := snapshot.Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	cat := mustCatalog(t, catalog.English)
	doc := mustBuild(t, s, cat, Options{})

	var got []string
	for _, blk := range doc.Blocks {
		if h, ok := blk.(Heading); ok && h.Level == 1 {
			if !h.Numbered {
				t.Errorf("level-1 heading %q is not numbered", runText(t, h.Runs[0]))
			}
			got = append(got, runText(t, h.Runs[0]))
		}
	}
	want := []string{
		"Executive summary", "Capacity by namespace", "Resources by workload", "HPA history",
		"Resource usage by node pool", "Technical quality appendix",
	}
	if !slices.Equal(got, want) {
		t.Errorf("level-1 headings = %v, want %v", got, want)
	}

	// The executive section follows the opening page, the four sections of
	// the evidence come before the weak page break, and the appendix comes
	// after the break.
	var executive, weakBreak, appendix int
	for i, blk := range doc.Blocks {
		switch x := blk.(type) {
		case Heading:
			switch runText(t, x.Runs[0]) {
			case "Executive summary":
				executive = i
			case "Technical quality appendix":
				appendix = i
			}
		case PageBreak:
			if x.Weak {
				weakBreak = i
			}
		}
	}
	if executive <= 0 || executive >= weakBreak || weakBreak+1 != appendix {
		t.Errorf("executive at %d, weak break at %d, appendix at %d", executive, weakBreak, appendix)
	}
}

func goldenSnapshot(t *testing.T, name string) snapshot.Snapshot {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "snapshot", name+".json"))
	if err != nil {
		t.Fatalf("reading the golden snapshot: %v", err)
	}
	s, err := snapshot.Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return s
}

func TestBuildExecutiveProfile(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	doc := mustBuild(t, goldenSnapshot(t, "direct-complete"), cat, Options{Profile: "executive"})

	var headings []string
	for _, blk := range doc.Blocks {
		if h, ok := blk.(Heading); ok && h.Level == 1 {
			headings = append(headings, runText(t, h.Runs[0]))
		}
	}
	want := []string{
		catText(t, cat, catalog.SectionsExecutiveHeading, nil),
		catText(t, cat, catalog.AppendixHeading, nil),
	}
	if !slices.Equal(headings, want) {
		t.Fatalf("level-1 headings = %v, want %v", headings, want)
	}

	if _, ok := doc.Blocks[0].(Verdict); !ok {
		t.Fatalf("first block is %T, want the verdict", doc.Blocks[0])
	}
	if _, ok := doc.Blocks[2].(Front); !ok {
		t.Fatalf("third block is %T, want the front", doc.Blocks[2])
	}
	for _, i := range []int{1, 3} {
		if pb, ok := doc.Blocks[i].(PageBreak); !ok || pb.Weak {
			t.Fatalf("block %d = %#v, want the hard page break", i, doc.Blocks[i])
		}
	}
	if h, ok := doc.Blocks[4].(Heading); !ok || runText(t, h.Runs[0]) != want[0] {
		t.Fatalf("fifth block = %#v, want the executive heading", doc.Blocks[4])
	}
	appendix := slices.IndexFunc(doc.Blocks, func(b Block) bool { h, ok := b.(Heading); return ok && runText(t, h.Runs[0]) == want[1] })
	if appendix < 0 {
		t.Fatal("no appendix heading")
	}
	if pb, ok := doc.Blocks[appendix-1].(PageBreak); ok {
		t.Fatalf("a %#v stands before the appendix heading that holds only its fallback sentence", pb)
	}
	if _, ok := doc.Blocks[appendix+1].(Paragraph); !ok {
		t.Fatalf("block after the appendix heading = %T, want the fallback Paragraph", doc.Blocks[appendix+1])
	}

	// A snapshot that carries warnings ends with the notes table.
	noted := mustBuild(t, goldenSnapshot(t, "partial-data"), cat, Options{Profile: "executive"})
	notes := catText(t, cat, catalog.NotesHeading, nil)
	n := len(noted.Blocks) - 1
	if h, ok := noted.Blocks[n-2].(Heading); !ok || runText(t, h.Runs[0]) != notes {
		t.Errorf("the block before the notes table = %#v, want the notes heading", noted.Blocks[n-2])
	}
	if table, ok := noted.Blocks[n-1].(Table); !ok || table.ID != "interpretation-notes" {
		t.Errorf("the last block = %#v, want the notes table", noted.Blocks[n-1])
	}
}

func TestBuildExecutiveRaisesTheMinimum(t *testing.T) {
	cat := mustCatalog(t, catalog.English)
	s := goldenSnapshot(t, "direct-complete")
	executive := mustBuild(t, s, cat, Options{Profile: "executive", MinSeverity: "all"})
	detailed := mustBuild(t, s, cat, Options{Profile: "detailed", MinSeverity: "all"})

	isWeak := func(b Block) bool { pb, ok := b.(PageBreak); return ok && pb.Weak }
	dw := slices.IndexFunc(detailed.Blocks, isWeak)
	if slices.IndexFunc(executive.Blocks, isWeak) >= 0 || dw < 0 {
		t.Fatalf("weak page breaks at %d (executive, want none) and %d", slices.IndexFunc(executive.Blocks, isWeak), dw)
	}
	ew := slices.IndexFunc(executive.Blocks, func(b Block) bool {
		h, ok := b.(Heading)
		return ok && h.Level == 1 && runText(t, h.Runs[0]) == catText(t, cat, catalog.AppendixHeading, nil)
	}) - 1
	for _, blk := range executive.Blocks[4:ew] {
		table, ok := blk.(Table)
		if !ok || table.ID != "priority-workloads" {
			continue
		}
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

	para, ok := executive.Blocks[ew+2].(Paragraph)
	if !ok {
		t.Fatalf("the executive appendix holds %T, want the fallback Paragraph", executive.Blocks[ew+2])
	}
	want, err := cat.Text(catalog.AppendixEmpty, nil)
	if err != nil {
		t.Fatalf("Text(AppendixEmpty): %v", err)
	}
	if got := runText(t, para.Runs[0]); got != want {
		t.Errorf("executive appendix = %q, want %q", got, want)
	}
	for _, blk := range executive.Blocks[ew:] {
		if table, ok := blk.(Table); ok && table.ID == "quality-appendix" {
			t.Errorf("the executive profile holds the appendix table with %d rows, want none", len(table.Rows))
		}
	}

	table := findTable(t, detailed.Blocks[dw:], "quality-appendix")
	if len(table.Rows) != 9 {
		t.Errorf("detailed appendix rows = %d, want 9", len(table.Rows))
	}
}

func TestBuildMetadataRows(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*snapshot.Snapshot)
		wantRows int
	}{
		{"all set", func(*snapshot.Snapshot) {}, 8},
		{"no organization", func(s *snapshot.Snapshot) { s.Report.Organization = "" }, 7},
		{"no classification", func(s *snapshot.Snapshot) { s.Report.Classification = "" }, 7},
		{"no excluded container", func(s *snapshot.Snapshot) { s.Scope.ExcludedContainers = []string{} }, 8},
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
			got := runText(t, table.Rows[5][1][0])
			if got != tt.want {
				t.Errorf("excluded containers value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildMetadataNamesAreIdentRuns(t *testing.T) {
	s := baseSnapshot(t)
	s.Scope.ClusterName = "prod-east"
	s.Scope.Namespaces = []string{"a", "b", "c"}
	s.Namespaces = []snapshot.Namespace{
		unavailableNamespace("a"),
		unavailableNamespace("b"),
		unavailableNamespace("c"),
	}
	s.Summary.Namespaces = 3
	s.Scope.ExcludedContainers = []string{"istio-proxy", "linkerd"}
	doc := mustBuild(t, s, mustCatalog(t, catalog.English), Options{})
	table := findTable(t, frontBlock(t, doc).Blocks, "report-metadata")

	want := map[int][]Run{
		0: {Ident("prod-east")},
		4: {Ident("a"), Text(", "), Ident("b"), Text(", "), Ident("c")},
		5: {Ident("istio-proxy"), Text(", "), Ident("linkerd")},
	}
	for row, runs := range want {
		if got := table.Rows[row][1]; !reflect.DeepEqual(got, runs) {
			t.Errorf("row %d value = %+v, want %+v", row, got, runs)
		}
	}
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
	if got := runText(t, table.Rows[6][1][0]); got != s.Report.Organization {
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

func TestBuildEndsWithTheColophon(t *testing.T) {
	tests := []struct {
		lang catalog.Language
		want string
	}{
		{catalog.English, "Generated by k8s-capacity-report from a snapshot of Prometheus-compatible metrics. Issued on 2026-07-15 14:30."},
		{catalog.PortugueseBrazil, "Gerado pelo k8s-capacity-report a partir de um snapshot de métricas compatíveis com o Prometheus. Emitido em 2026-07-15 14:30."},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			doc := mustBuild(t, baseSnapshot(t), mustCatalog(t, tt.lang), Options{})
			last := doc.Blocks[len(doc.Blocks)-1]
			if got, ok := last.(Colophon); !ok || got.Text != tt.want {
				t.Fatalf("last block = %#v, want Colophon{Text: %q}", last, tt.want)
			}
		})
	}
}
