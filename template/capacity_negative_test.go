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

	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	for _, m := range matches {
		base := strings.TrimSuffix(filepath.Base(m), ".json")
		if !nameSet[base] {
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
