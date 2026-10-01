package document

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"testing"
)

// sampleDocument returns a Document with every top-level field filled and a
// handful of blocks, so Encode has something realistic to work on without
// going through Build.
func sampleDocument() Document {
	author := "capacity-bot"
	classification := "internal"
	return Document{
		Schema:   Schema,
		Language: "en",
		Theme:    Theme{Lang: "en", Region: "US"},
		Labels:   Labels{Note: "Note", Info: "Information", Warning: "Warning"},
		Cover: Cover{
			Title:        "CAPACITY REPORT",
			ClusterLabel: "Cluster:",
			Cluster:      "prod",
			AuthorLabel:  "PREPARED BY:",
			Author:       &author,
			IssuedLabel:  "ISSUED ON:",
			Issued:       "07/15/2026 14:30",
		},
		Footer: Footer{Classification: &classification, PageLabel: "Page"},
		Blocks: []Block{
			Front{Blocks: []Block{
				Outline{Title: "Contents", Depth: 2},
			}},
			PageBreak{Weak: false},
			PageBreak{Weak: true},
			Heading{Level: 1, Numbered: true, Runs: []Run{Text("Technical quality appendix")}},
			Paragraph{Runs: []Run{Text("No technical evidence is available for the current filters.")}},
		},
	}
}

func TestEncodeIsStable(t *testing.T) {
	doc := sampleDocument()

	first, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	second, err := Encode(doc)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("Encode() is not stable:\n%s\nvs\n%s", first, second)
	}
	if !strings.HasSuffix(string(first), "\n") {
		t.Fatalf("Encode() does not end in a line break: %q", first)
	}
	if !strings.Contains(string(first), "\n  \"schema\"") {
		t.Fatalf("Encode() does not indent with two spaces: %s", first)
	}
}

// assertKeys unmarshals data into a generic map and compares its keys,
// sorted, with want, also sorted.
func assertKeys(t *testing.T, data []byte, want []string) {
	t.Helper()

	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	got := make([]string, 0, len(m))
	for k := range m {
		got = append(got, k)
	}
	sort.Strings(got)
	wantSorted := append([]string(nil), want...)
	sort.Strings(wantSorted)
	if !slices.Equal(got, wantSorted) {
		t.Errorf("keys = %v, want %v (encoded: %s)", got, wantSorted, data)
	}
}

// TestEncodeWritesExactKeys states, kind by kind, the exact set of keys
// template/capacity/validate.typ accepts for one block of each kind and
// one run of each kind this part of the project declares, and checks that
// json.Marshal (which dispatches to each type's own MarshalJSON, the same
// way Encode's encoder does) writes exactly that set, no more and no
// fewer.
func TestEncodeWritesExactKeys(t *testing.T) {
	blockCases := []struct {
		name  string
		block Block
		keys  []string
	}{
		{"front", Front{Blocks: []Block{PageBreak{Weak: true}}}, []string{"type", "blocks"}},
		{"outline", Outline{Title: "Contents", Depth: 2}, []string{"type", "title", "depth"}},
		{"heading", Heading{Level: 2, Numbered: false, Runs: []Run{Text("x")}}, []string{"type", "level", "numbered", "runs"}},
		{"paragraph", Paragraph{Runs: []Run{Text("x")}}, []string{"type", "runs"}},
		{"list", List{Items: [][]Run{{Text("x")}}}, []string{"type", "items"}},
		{
			"table",
			Table{ID: "report-metadata", Headers: []string{"a", "b"}, Rows: [][][]Run{{{Text("x")}, {Text("y")}}}},
			[]string{"type", "id", "headers", "rows"},
		},
		{"pagebreak", PageBreak{Weak: true}, []string{"type", "weak"}},
	}
	for _, tc := range blockCases {
		t.Run("block/"+tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.block)
			if err != nil {
				t.Fatalf("Marshal(%s): %v", tc.name, err)
			}
			assertKeys(t, data, tc.keys)
		})
	}

	runCases := []struct {
		name string
		run  Run
		keys []string
	}{
		{"text", Text("x"), []string{"kind", "text"}},
		{"strong", Strong("x"), []string{"kind", "text"}},
		{"status", Status("healthy"), []string{"kind", "status", "text"}},
		{"status-text", StatusText("healthy", "x"), []string{"kind", "status", "text"}},
	}
	for _, tc := range runCases {
		t.Run("run/"+tc.name, func(t *testing.T) {
			data, err := json.Marshal(tc.run)
			if err != nil {
				t.Fatalf("Marshal(%s): %v", tc.name, err)
			}
			assertKeys(t, data, tc.keys)
		})
	}
}

// TestEncodeNeverWritesNullLists proves that an empty table cell is
// written as "[]", never "null": encoding/json writes "null" for a nil
// slice by default, which template/capacity/validate.typ would refuse
// with a wrong-type diagnostic, since it expects an array.
func TestEncodeNeverWritesNullLists(t *testing.T) {
	table := Table{
		ID:      "report-metadata",
		Headers: []string{"a", "b"},
		Rows:    [][][]Run{{nil, {Text("x")}}},
	}
	data, err := json.Marshal(table)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(data), "null") {
		t.Fatalf("encoded table contains \"null\": %s", data)
	}
	if !strings.Contains(string(data), "[]") {
		t.Fatalf("encoded table has no \"[]\" for the empty cell: %s", data)
	}
}

func TestCalloutMarshals(t *testing.T) {
	tests := []struct {
		name    string
		callout Callout
		want    string
	}{
		{
			"with a title",
			Callout{Kind: "note", Title: []Run{Text("Title")}, Runs: []Run{Text("Body")}},
			`{"type":"callout","kind":"note","title":[{"kind":"text","text":"Title"}],"runs":[{"kind":"text","text":"Body"}]}`,
		},
		{
			"without a title",
			Callout{Kind: "note", Runs: []Run{Text("Body")}},
			`{"type":"callout","kind":"note","title":null,"runs":[{"kind":"text","text":"Body"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.callout)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(data) != tt.want {
				t.Errorf("Marshal() = %s, want %s", data, tt.want)
			}
		})
	}
}

func TestPlaceholderRun(t *testing.T) {
	data, err := json.Marshal(Placeholder("Undetermined"))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `{"kind":"placeholder","text":"Undetermined"}`
	if string(data) != want {
		t.Errorf("Marshal() = %s, want %s", data, want)
	}
}
