//go:build template

package template

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

func capacityNegativeDir(ws string) string {
	return filepath.Join(ws, "tests", "capacity", "negative")
}

const capacityEntry = "capacity/main.typ"

// compileCapacityData copies dataFile into the workspace as report.json and
// compiles capacity/main.typ against it, exactly as a real caller would:
// the compile root is ws, and `data` names a path relative to it.
func compileCapacityData(t *testing.T, ws, dataFile string) (pdf, stderr string, err error) {
	t.Helper()
	data, readErr := os.ReadFile(dataFile)
	if readErr != nil {
		t.Fatalf("reading %s: %v", dataFile, readErr)
	}
	if writeErr := os.WriteFile(filepath.Join(ws, "report.json"), data, 0o644); writeErr != nil {
		t.Fatalf("writing report.json: %v", writeErr)
	}
	return compilePDF(t, ws, capacityEntry, map[string]string{"data": "/report.json"})
}

func TestCapacityBaseFixtureCompilesCleanly(t *testing.T) {
	ws := workspace(t)
	base := filepath.Join(ws, "tests", "capacity", "fixtures", "base.pt-BR.json")
	pdf, stderr, err := compileCapacityData(t, ws, base)
	assertCleanCompile(t, stderr, err)
	assertNonEmptyPDF(t, pdf)
}

func TestCapacityNegativeFixturesFail(t *testing.T) {
	ws := workspace(t)
	dir := capacityNegativeDir(ws)
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(matches) == 0 {
		t.Fatalf("no negative fixtures found in %s", dir)
	}

	for _, file := range matches {
		t.Run(filepath.Base(file), func(t *testing.T) {
			expectPath := strings.TrimSuffix(file, ".json") + ".expect"
			raw, err := os.ReadFile(expectPath)
			if err != nil {
				t.Fatalf("reading %s: %v", expectPath, err)
			}
			fragment := strings.TrimSuffix(string(raw), "\n")

			_, stderr, err := compileCapacityData(t, ws, file)

			var compileErr *typst.CompileError
			if !errors.As(err, &compileErr) {
				t.Fatalf("Compile() error = %v, want *typst.CompileError", err)
			}
			if !strings.Contains(compileErr.Stderr, fragment) {
				t.Fatalf("stderr = %q, want it to contain %q", stderr, fragment)
			}
		})
	}
}

func TestCapacityEveryNegativeHasAnExpectation(t *testing.T) {
	ws := workspace(t)
	dir := capacityNegativeDir(ws)

	jsonFiles, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(jsonFiles) == 0 {
		t.Fatalf("no .json fixtures found in %s", dir)
	}
	for _, f := range jsonFiles {
		expectPath := strings.TrimSuffix(f, ".json") + ".expect"
		info, err := os.Stat(expectPath)
		if err != nil {
			t.Errorf("%s has no matching .expect file", f)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("%s is empty", expectPath)
		}
	}

	expectFiles, err := filepath.Glob(filepath.Join(dir, "*.expect"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	for _, f := range expectFiles {
		jsonPath := strings.TrimSuffix(f, ".expect") + ".json"
		if _, err := os.Stat(jsonPath); err != nil {
			t.Errorf("%s has no matching .json file", f)
		}
	}
}

func TestCapacityMissingInputFails(t *testing.T) {
	ws := workspace(t)
	_, stderr, err := compilePDF(t, ws, capacityEntry, nil)

	var compileErr *typst.CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("Compile() error = %v, want *typst.CompileError", err)
	}
	want := `capacity-report: missing input "data"`
	if !strings.Contains(compileErr.Stderr, want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr, want)
	}
}

var diagnosticNamePattern = regexp.MustCompile(`^\s*([a-z][a-z-]*)\s*:`)

// diagnosticNames extracts the dictionary keys between the
// `// diagnostics:start` and `// diagnostics:end` markers of validate.typ.
func diagnosticNames(t *testing.T, validateTyp string) []string {
	t.Helper()
	raw, err := os.ReadFile(validateTyp)
	if err != nil {
		t.Fatalf("reading %s: %v", validateTyp, err)
	}
	lines := strings.Split(string(raw), "\n")

	start, end := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "diagnostics:start") {
			start = i
		}
		if strings.Contains(line, "diagnostics:end") {
			end = i
			break
		}
	}
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("%s: could not find diagnostics:start/diagnostics:end markers", validateTyp)
	}

	var names []string
	for _, line := range lines[start+1 : end] {
		m := diagnosticNamePattern.FindStringSubmatch(line)
		if m != nil {
			names = append(names, m[1])
		}
	}
	if len(names) == 0 {
		t.Fatalf("%s: no diagnostic names found between the markers", validateTyp)
	}
	return names
}

func TestCapacityEveryDiagnosticIsCovered(t *testing.T) {
	ws := workspace(t)
	capDir := filepath.Join(ws, "capacity")
	names := diagnosticNames(t, filepath.Join(capDir, "validate.typ"))
	if len(names) != 19 {
		t.Errorf("diagnostic names = %v (%d), want 19", names, len(names))
	}

	dir := capacityNegativeDir(ws)
	nameSet := map[string]bool{}
	for _, name := range names {
		nameSet[name] = true
		if name == "missing-input" {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, name+".json")); err != nil {
			t.Errorf("diagnostic %q has no fixture %s.json", name, name)
		}
		if _, err := os.Stat(filepath.Join(dir, name+".expect")); err != nil {
			t.Errorf("diagnostic %q has no fixture %s.expect", name, name)
		}
	}

	// A fixture may be named "<short name>.<case>.json" when a diagnostic
	// needs more than one example; only the part before the first dot is
	// the short name that must be a key of validate.typ's dictionary.
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	for _, m := range matches {
		base := strings.TrimSuffix(filepath.Base(m), ".json")
		name := strings.SplitN(base, ".", 2)[0]
		if !nameSet[name] {
			t.Errorf("fixture %s has no matching name in validate.typ's diagnostics dictionary", base)
		}
	}

	err = filepath.WalkDir(capDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".typ" {
			return nil
		}
		if filepath.Base(path) == "validate.typ" {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(raw), "panic(") {
			t.Errorf("%s contains panic(; only validate.typ may call panic", path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", capDir, err)
	}
}

// TestCapacityNeverFailsWithoutItsPrefix catches a native crash whatever
// the fixture expected: even when a fixture's own .expect names a
// different diagnostic, the compiler's stderr must start (once compiler
// noise is set aside) with a diagnostic from this layer or from the
// generic one underneath it, never a bare Typst error message.
func TestCapacityNeverFailsWithoutItsPrefix(t *testing.T) {
	ws := workspace(t)
	dir := capacityNegativeDir(ws)
	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(matches) == 0 {
		t.Fatalf("no negative fixtures found in %s", dir)
	}

	for _, file := range matches {
		t.Run(filepath.Base(file), func(t *testing.T) {
			_, stderr, err := compileCapacityData(t, ws, file)
			var compileErr *typst.CompileError
			if !errors.As(err, &compileErr) {
				t.Fatalf("Compile() error = %v, want *typst.CompileError", err)
			}
			if !strings.Contains(compileErr.Stderr, "capacity-report: ") &&
				!strings.Contains(compileErr.Stderr, "graphite-report: ") {
				t.Fatalf("stderr = %q, want it to contain \"capacity-report: \" or \"graphite-report: \"", stderr)
			}
		})
	}
}

// TestCapacityFrontBlockIsSetSmaller proves `front` really does set its
// content at a smaller size than the same content outside it. Comparing
// `text.size` directly does not work here: the `set text(size: 8pt)`
// inside `front`'s own `block(...)` body is out of scope again by the
// time control returns to the top level of this test document, so a
// `context` placed after the call never sees it. Measuring the rendered
// height of the very same paragraph, once wrapped in `front` and once
// not, sidesteps that: smaller text at the same width wraps into a
// shorter block. `measure` is given an explicit width instead of relying
// on its default (unbounded) one, because `front`'s own `block(width:
// 100%)` has no page to resolve "100%" against when measured outside a
// page context, which otherwise measures a wildly wrong height.
func TestCapacityFrontBlockIsSetSmaller(t *testing.T) {
	ws := workspace(t)

	source := `#import "blocks.typ": build-block

#let para = (type: "paragraph", runs: ((kind: "text", text: "The quick brown fox jumps over the lazy dog, repeated to force wrapping across at least two lines of text."),))
#let front-entry = (type: "front", blocks: (para,))

#context {
  let h-front = measure(build-block(front-entry), width: 400pt).height
  let h-plain = measure(build-block(para), width: 400pt).height
  assert(
    h-front < h-plain,
    message: "front text is not smaller: " + repr(h-front) + " vs " + repr(h-plain),
  )
}
`
	path := filepath.Join(ws, "capacity", "front_smaller_check.typ")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
	pdf, stderr, err := compilePDF(t, ws, filepath.Join("capacity", "front_smaller_check.typ"), nil)
	assertCleanCompile(t, stderr, err)
	assertNonEmptyPDF(t, pdf)
}

// unnumberedHeadingFixture is a minimal, self-contained document (every
// top-level key, one outline, one numbered heading, one unnumbered one)
// used only to prove B2's rendering rule end to end.
const unnumberedHeadingFixture = `{
  "schema": "1",
  "language": "pt-BR",
  "theme": {
    "lang": "pt",
    "region": "BR",
    "date_format": "[day padding:zero]/[month padding:zero]/[year]"
  },
  "labels": {
    "author": "Responsável",
    "date": "Emissão",
    "classification": "Classificação",
    "outline": "Sumário",
    "note": "Nota",
    "info": "Informação",
    "warning": "Atenção"
  },
  "cover": {
    "title": "LEVANTAMENTO DE CAPACIDADE",
    "cluster_label": "Cluster:",
    "cluster": "production-east",
    "author_label": "ELABORADO POR:",
    "author": "Time SRE",
    "issued_label": "EMITIDO EM:",
    "issued": "15/07/2026 14:30"
  },
  "footer": {
    "classification": "Uso interno",
    "page_label": "Página"
  },
  "blocks": [
    { "type": "outline", "title": "Sumário", "depth": 1 },
    { "type": "heading", "level": 1, "numbered": true, "runs": [ { "kind": "text", "text": "Alpha" } ] },
    { "type": "heading", "level": 1, "numbered": false, "runs": [ { "kind": "text", "text": "Beta" } ] }
  ]
}
`

func TestCapacityUnnumberedHeadingIsLeftOutOfTheOutline(t *testing.T) {
	ws := workspace(t)
	dataFile := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(dataFile, []byte(unnumberedHeadingFixture), 0o644); err != nil {
		t.Fatalf("writing %s: %v", dataFile, err)
	}
	pdf, stderr, err := compileCapacityData(t, ws, dataFile)
	assertCleanCompile(t, stderr, err)
	assertNonEmptyPDF(t, pdf)

	text := pdfText(t, pdf)
	if got := strings.Count(text, "Alpha"); got != 2 {
		t.Errorf("page text has %d occurrences of %q, want 2 (outline entry + heading)", got, "Alpha")
	}
	if got := strings.Count(text, "Beta"); got != 1 {
		t.Errorf("page text has %d occurrences of %q, want 1 (heading only, left out of the outline)", got, "Beta")
	}
}

// capacityLayout is one row of the brief's table of layouts.
type capacityLayout struct {
	id      string
	columns int
	size    string
	insetY  string
}

var capacityLayouts = []capacityLayout{
	{"report-metadata", 2, "8.5pt", "2pt"},
	{"status-legend", 2, "8.5pt", "2pt"},
	{"namespace-capacity", 7, "7.1pt", "3.5pt"},
	{"workload-resources", 10, "6.2pt", "3.5pt"},
	{"hpa-observed", 7, "7pt", "3.5pt"},
	{"workloads-without-hpa", 4, "7.4pt", "3.5pt"},
	{"nodepool-summary", 7, "7.1pt", "3.5pt"},
	{"nodepool-profile", 7, "6.8pt", "3.5pt"},
	{"node-detail", 5, "7.1pt", "3.5pt"},
	{"priority-workloads", 4, "7.2pt", "3.5pt"},
	{"namespaces-at-risk", 6, "7.2pt", "3.5pt"},
	{"replica-ceiling", 5, "7.2pt", "3.5pt"},
	{"nodepool-status", 3, "7.3pt", "3.5pt"},
	{"action-plan", 4, "7.2pt", "3.5pt"},
	{"material-limitations", 3, "7.5pt", "3.5pt"},
	{"quality-appendix", 5, "7pt", "3.5pt"},
	{"interpretation-notes", 1, "7.5pt", "3.5pt"},
}

func TestCapacityTableLayoutsMatchThePlan(t *testing.T) {
	ws := workspace(t)

	var b strings.Builder
	b.WriteString("#import \"tables.typ\": layouts\n\n")
	b.WriteString("#assert.eq(layouts.keys().len(), ")
	b.WriteString(strconv.Itoa(len(capacityLayouts)))
	b.WriteString(")\n")
	for _, l := range capacityLayouts {
		b.WriteString("#assert(\"" + l.id + "\" in layouts.keys(), message: \"missing layout " + l.id + "\")\n")
		b.WriteString("#assert.eq(layouts.at(\"" + l.id + "\").columns.len(), " + strconv.Itoa(l.columns) + ")\n")
		b.WriteString("#assert.eq(layouts.at(\"" + l.id + "\").size, " + l.size + ")\n")
		b.WriteString("#assert.eq(layouts.at(\"" + l.id + "\").inset-y, " + l.insetY + ")\n")
	}
	b.WriteString("#\"ok\"\n")

	source := filepath.Join(ws, "capacity", "tables_layout_check.typ")
	if err := os.WriteFile(source, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("writing %s: %v", source, err)
	}

	rel, err := filepath.Rel(ws, source)
	if err != nil {
		t.Fatalf("computing relative path for %s: %v", source, err)
	}
	pdf, stderr, err := compilePDF(t, ws, rel, nil)
	assertCleanCompile(t, stderr, err)
	assertNonEmptyPDF(t, pdf)
}
