// Package document builds report.json, the one JSON data file
// template/capacity reads: a snapshot and a catalog go in, and a Document
// ready for Encode comes out. Go decides what is said; the template
// decides how it looks. This package holds no sentence of its own: every
// word it writes comes from the catalog, given a key, or from the
// snapshot, given a name or a number already computed by an earlier
// layer.
package document

import (
	"bytes"
	"encoding/json"
)

// Schema is the schema version this package writes, and the only one
// template/capacity/validate.typ accepts.
const Schema = "1"

// Document is the whole data file. The cover and the footer are not
// blocks: they are top-level parts of the file, because the generic
// layer, template/graphite, receives them as arguments of its own.
type Document struct {
	Schema   string  `json:"schema"`
	Language string  `json:"language"`
	Theme    Theme   `json:"theme"`
	Labels   Labels  `json:"labels"`
	Cover    Cover   `json:"cover"`
	Footer   Footer  `json:"footer"`
	Blocks   []Block `json:"blocks"`
}

// Theme carries the language and region codes that drive hyphenation.
type Theme struct {
	Lang   string `json:"lang"`
	Region string `json:"region"`
}

// Labels holds the three callout labels the generic layer prints beside a
// callout's mark. This part of the project builds no callout block, but
// template/graphite still requires the three labels at the top level.
type Labels struct {
	Note    string `json:"note"`
	Info    string `json:"info"`
	Warning string `json:"warning"`
}

// Cover is the top-level title block of the first page.
type Cover struct {
	Title        string  `json:"title"`
	ClusterLabel string  `json:"cluster_label"`
	Cluster      string  `json:"cluster"`
	AuthorLabel  string  `json:"author_label"`
	Author       *string `json:"author"` // null when the author is empty
	IssuedLabel  string  `json:"issued_label"`
	Issued       string  `json:"issued"`
}

// Footer is the top-level running footer, shown from the second page on.
type Footer struct {
	Classification *string `json:"classification"` // null when it is empty
	PageLabel      string  `json:"page_label"`
}

// Options narrows what Build reads from the snapshot: an empty field keeps
// the value the snapshot already stores.
type Options struct {
	Profile     string
	MinSeverity string
}

// Block is one entry of "blocks", or of a "front" block's own "blocks".
// Each concrete kind below writes exactly the keys
// template/capacity/validate.typ accepts for it, key "type" first.
type Block interface{ blockType() string }

// Front holds the opening matter, set in smaller type by the template: the
// outline, the report metadata, the status legend, and the data-quality
// counts.
type Front struct{ Blocks []Block }

func (Front) blockType() string { return "front" }

// MarshalJSON writes the keys "type" and "blocks", in that order.
func (f Front) MarshalJSON() ([]byte, error) {
	return marshalNoEscape(struct {
		Type   string  `json:"type"`
		Blocks []Block `json:"blocks"`
	}{"front", f.Blocks})
}

// Outline is the table of contents.
type Outline struct {
	Title string
	Depth int
}

func (Outline) blockType() string { return "outline" }

// MarshalJSON writes the keys "type", "title", and "depth", in that order.
func (o Outline) MarshalJSON() ([]byte, error) {
	return marshalNoEscape(struct {
		Type  string `json:"type"`
		Title string `json:"title"`
		Depth int    `json:"depth"`
	}{"outline", o.Title, o.Depth})
}

// Heading is a section title, numbered or not.
type Heading struct {
	Level    int
	Numbered bool
	Runs     []Run
}

func (Heading) blockType() string { return "heading" }

// MarshalJSON writes the keys "type", "level", "numbered", and "runs", in
// that order.
func (h Heading) MarshalJSON() ([]byte, error) {
	return marshalNoEscape(struct {
		Type     string `json:"type"`
		Level    int    `json:"level"`
		Numbered bool   `json:"numbered"`
		Runs     []Run  `json:"runs"`
	}{"heading", h.Level, h.Numbered, nonNilRuns(h.Runs)})
}

// Paragraph is one paragraph of prose.
type Paragraph struct{ Runs []Run }

func (Paragraph) blockType() string { return "paragraph" }

// MarshalJSON writes the keys "type" and "runs", in that order.
func (p Paragraph) MarshalJSON() ([]byte, error) {
	return marshalNoEscape(struct {
		Type string `json:"type"`
		Runs []Run  `json:"runs"`
	}{"paragraph", nonNilRuns(p.Runs)})
}

// List is a bullet list, each item a sequence of runs.
type List struct{ Items [][]Run }

func (List) blockType() string { return "list" }

// MarshalJSON writes the keys "type" and "items", in that order.
func (l List) MarshalJSON() ([]byte, error) {
	items := make([][]Run, len(l.Items))
	for i, item := range l.Items {
		items[i] = nonNilRuns(item)
	}
	return marshalNoEscape(struct {
		Type  string  `json:"type"`
		Items [][]Run `json:"items"`
	}{"list", items})
}

// Table is one table, named by the identifier that
// template/capacity/tables.typ lays out.
type Table struct {
	ID      string
	Headers []string
	Rows    [][][]Run // rows, cells, runs
}

func (Table) blockType() string { return "table" }

// MarshalJSON writes the keys "type", "id", "headers", and "rows", in that
// order.
func (t Table) MarshalJSON() ([]byte, error) {
	rows := make([][][]Run, len(t.Rows))
	for i, row := range t.Rows {
		cells := make([][]Run, len(row))
		for j, cell := range row {
			cells[j] = nonNilRuns(cell)
		}
		rows[i] = cells
	}
	return marshalNoEscape(struct {
		Type    string    `json:"type"`
		ID      string    `json:"id"`
		Headers []string  `json:"headers"`
		Rows    [][][]Run `json:"rows"`
	}{"table", t.ID, t.Headers, rows})
}

// Callout is a note beside the flow of the text.
type Callout struct {
	Kind  string // note, info, warning
	Title []Run  // nil for no title
	Runs  []Run
}

func (Callout) blockType() string { return "callout" }

// MarshalJSON writes the keys "type", "kind", "title", and "runs", in that
// order; the title is null when there is none.
func (c Callout) MarshalJSON() ([]byte, error) {
	var title []Run
	if c.Title != nil {
		title = nonNilRuns(c.Title)
	}
	return marshalNoEscape(struct {
		Type  string `json:"type"`
		Kind  string `json:"kind"`
		Title []Run  `json:"title"`
		Runs  []Run  `json:"runs"`
	}{"callout", c.Kind, title, nonNilRuns(c.Runs)})
}

// PageBreak forces a new page, or, when Weak is set, forces one only if a
// page break has not already happened since the last content.
type PageBreak struct{ Weak bool }

func (PageBreak) blockType() string { return "pagebreak" }

// MarshalJSON writes the keys "type" and "weak", in that order.
func (p PageBreak) MarshalJSON() ([]byte, error) {
	return marshalNoEscape(struct {
		Type string `json:"type"`
		Weak bool   `json:"weak"`
	}{"pagebreak", p.Weak})
}

// nonNilRuns returns runs, or an empty, non-nil slice in its place, so that
// Encode never writes "null" for an empty table cell or list item.
func nonNilRuns(runs []Run) []Run {
	if runs == nil {
		return []Run{}
	}
	return runs
}

// Run is one run of a cell, a paragraph, a heading, or a list item.
type Run struct {
	kind   string
	status string
	text   *string
}

// Text returns a plain run: prose in the document language.
func Text(text string) Run { return Run{kind: "text", text: &text} }

// Strong returns a bold run, for a label inside a sentence.
func Strong(text string) Run { return Run{kind: "strong", text: &text} }

// Ident returns a real name run: a namespace, a workload, set without
// hyphenation.
func Ident(text string) Run { return Run{kind: "ident", text: &text} }

// Placeholder returns a run that stands for a value the language names,
// such as an undetermined pool.
func Placeholder(text string) Run { return Run{kind: "placeholder", text: &text} }

// Status returns a status mark alone: its text is null.
func Status(status string) Run { return Run{kind: "status", status: status} }

// StatusText returns a status mark together with its label.
func StatusText(status, text string) Run { return Run{kind: "status", status: status, text: &text} }

// MarshalJSON writes "kind" first, then "status" when this run carries one,
// then "text", which is null for a status run built by Status.
func (r Run) MarshalJSON() ([]byte, error) {
	if r.kind == "status" {
		return marshalNoEscape(struct {
			Kind   string  `json:"kind"`
			Status string  `json:"status"`
			Text   *string `json:"text"`
		}{r.kind, r.status, r.text})
	}
	return marshalNoEscape(struct {
		Kind string  `json:"kind"`
		Text *string `json:"text"`
	}{r.kind, r.text})
}

// marshalNoEscape encodes v without escaping "<", ">", or "&" for HTML. A
// block or a run needs its own MarshalJSON, since each fixes its leading
// key and carries unexported fields, but encoding/json only ever compacts
// the bytes a MarshalJSON method returns; it never re-escapes or unescapes
// them. So a nested MarshalJSON that built its bytes with the library's
// own default json.Marshal would freeze in "<a>" as "<a>" before
// Encode's own encoder, further down this file, ever saw it. Using this
// same helper at every level keeps the choice, and the indentation, in one
// place: json.Encoder.Encode reindents its whole output only after every
// nested MarshalJSON has already run, so the bytes returned here need not
// be indented themselves.
func marshalNoEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// Encode writes d with two spaces of indentation and one line break at the
// end, without escaping "<", ">", or "&" for HTML. The same document
// always gives the same bytes.
func Encode(d Document) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
