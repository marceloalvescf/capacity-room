// Package golden renders chosen pages of chosen Typst documents to PNG and
// compares them, by SHA-256, against the golden pages kept in
// template/tests/golden/. It imports internal/typst and the standard
// library only; it must never import the template package, so that the
// rendering logic stays independent of how the template is unpacked.
package golden

import "fmt"

// PPI is the resolution every golden page is rendered at and compared
// under. It is not configurable: a different PPI produces a different
// PNG, so the whole suite is pinned to one value.
const PPI = 144

// Snapshot names one page set rendered from a Typst source, for visual
// regression testing.
type Snapshot struct {
	Name   string // file stem, for example "complete"
	Source string // path relative to the template folder
	// Data is the path, relative to the compile root, of a JSON data file
	// to pass as the "data" input — for example
	// "tests/capacity/fixtures/detailed.pt-BR.json". It is empty for a
	// snapshot of the generic graphite layer, which reads no data file.
	// Render (never a caller) turns it into the "/"-prefixed input value
	// data.typ expects; a Snapshot must not set both Data and an Inputs
	// entry named "data".
	Data   string
	Pages  []int
	Inputs map[string]string
	// Omits says which pages of the document Pages leaves out and why the
	// covered pages are still enough for what Protects describes. Empty
	// means Pages already covers every page of the document.
	Omits    string
	Protects string // the one behaviour this snapshot alone protects
}

// Files returns the PNG names a snapshot produces, one per page and in the
// same order as Pages, for example "complete-1.png".
func (s Snapshot) Files() []string {
	files := make([]string, len(s.Pages))
	for i, page := range s.Pages {
		files[i] = fmt.Sprintf("%s-%d.png", s.Name, page)
	}
	return files
}

// Snapshots is the single, sorted-by-nothing-but-history list of pages the
// golden suite renders and checks. Adding a snapshot is editing this slice
// and nothing else. Each entry's Protects names the one behaviour that no
// other snapshot in the list already covers, translated from the comments
// of older/graphite-report/tests/harness/snapshots.py, the original list
// this one replaces.
var Snapshots = []Snapshot{
	{
		Name:   "complete",
		Source: "graphite/examples/complete.typ",
		Pages:  []int{1, 2},
		Protects: "the full front matter together with a summary, a table, " +
			"an information callout, and a caption on the same document",
	},
	{
		Name:   "minimal",
		Source: "graphite/examples/minimal.typ",
		Pages:  []int{1},
		Protects: "the sparsest possible front matter, with only a title: " +
			"an unset field must not take up space on the page",
	},
	{
		Name:   "theme",
		Source: "graphite/examples/custom-theme.typ",
		Pages:  []int{1},
		Protects: "theme overrides (10.5pt body, 2.8cm margin, three lumas) " +
			"actually reaching the page; the only snapshot that proves a " +
			"documented theme key has an effect",
	},
	{
		Name:   "running",
		Source: "graphite/examples/custom-running-elements.typ",
		Pages:  []int{1, 2},
		Protects: "running elements: page 1 pins the absence of header, " +
			"footer, and page number, and page 2 pins their presence; " +
			"neither page alone proves the contract",
	},
	{
		Name:   "table",
		Source: "tests/graphite/positive/multipage-table.typ",
		Pages:  []int{1, 2},
		Omits: "pages 3 and 4: the document's 120 rows run to 4 pages, " +
			"but pages 3 and 4 would only show the same header repeating " +
			"again as more rows overflow, which pages 1 and 2 already prove " +
			"it repeats correctly",
		Protects: "the semantic table header repeating on page 2 after a " +
			"page break, which is native Typst behaviour the theme must " +
			"not break",
	},
	{
		Name:   "starter",
		Source: "tests/graphite/positive/starter.typ",
		Pages:  []int{1},
		Protects: "what a new user's entry point looks like; replaces the " +
			"original template snapshot, which needed an installed package",
	},
	{
		Name:   "callout-break",
		Source: "tests/graphite/positive/callout-page-break.typ",
		Pages:  []int{1},
		Omits: "page 2: the callout moves there whole rather than " +
			"splitting, but once relocated it is an ordinary callout " +
			"render already pinned by the complete snapshot; what this " +
			"snapshot alone protects — the callout not splitting at the " +
			"bottom of page 1 — is visible on page 1 alone",
		Protects: "the default breakable: false on a callout pushed to the " +
			"page edge: without it the block splits across two pages, and " +
			"no other snapshot shows this because the callout fits where " +
			"it sits everywhere else",
	},
	{
		Name:   "locale",
		Source: "tests/graphite/positive/locale-english.typ",
		Pages:  []int{1},
		Protects: "a report entirely in English: labels, language, and " +
			"date format acting together; the only snapshot proving " +
			"translation reaches the page",
	},
	{
		Name:   "components",
		Source: "tests/graphite/unit/components-valid.typ",
		Pages:  []int{1},
		Protects: "the rendered output of all three callouts and the " +
			"keep-together block; without a golden page, a clean compile " +
			"alone would certify vanished content as healthy",
	},
	{
		Name:   "identifiers",
		Source: "tests/graphite/positive/identifier-wrapping.typ",
		Pages:  []int{1},
		Protects: "the hyphen that Portuguese hyphenation repeats when " +
			"breaking a machine identifier, and an unbreakable string " +
			"overflowing its cell into the neighbour; the only snapshot " +
			"where the suite can see either, since Typst compiles both clean",
	},
	{
		Name:   "capacity-pt",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/detailed.pt-BR.json",
		Pages:  []int{1, 2, 3, 4, 5},
		Protects: "the cover, the opening matter set small, the dense " +
			"tables, and the executive-summary tables of the capacity " +
			"report, in Portuguese — every page of the document",
	},
	{
		Name:   "capacity-en",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/detailed.en.json",
		Pages:  []int{1, 2, 3, 4, 5},
		Protects: "the same pages as capacity-pt in English: labels, date " +
			"order, decimal point, and hyphenation, across every page of " +
			"the document, including the executive-summary tables",
	},
	{
		Name:   "capacity-minimal",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/minimal.pt-BR.json",
		Pages:  []int{1, 2},
		Protects: "a cover with no author and a footer with no " +
			"classification, in the capacity report",
	},
	{
		Name:   "capacity-identifiers",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/identifiers.pt-BR.json",
		Pages:  []int{1},
		Protects: "identifier wrapping in the capacity report's own table, " +
			"and silent overflow of a name with no break point",
	},
	{
		Name:   "capacity-parity",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/parity.pt-BR.json",
		Pages:  []int{1, 2, 3, 4},
		Protects: "the report layer reproduces the original generated " +
			"document page for page",
	},
}
