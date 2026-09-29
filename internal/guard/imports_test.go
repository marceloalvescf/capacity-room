package guard

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

const testModule = "example.com/widget"

func TestImportViolationsLowerRankAllowed(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli":   {testModule + "/internal/typst"},
		testModule + "/internal/typst": {},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsHigherRankRefused(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/typst": {testModule + "/internal/cli"},
		testModule + "/internal/cli":   {},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/typst (rank 0) imports " + testModule + "/internal/cli (rank 100)",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsSameLayerAllowed(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/typst/a": {testModule + "/internal/typst/b"},
		testModule + "/internal/typst/b": {},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsUnclassified(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/mystery": {},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{testModule + "/internal/mystery: not classified in guard.Layers"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsLongestLayerWins(t *testing.T) {
	layers := []Layer{
		{Path: "internal/platform", Rank: 50},
		{Path: "internal/platform/api", Rank: 60},
	}
	imports := map[string][]string{
		testModule + "/internal/platform/api":   {testModule + "/internal/platform/store"},
		testModule + "/internal/platform/store": {},
	}

	got := ImportViolations(testModule, layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsIgnoresOutsideModule(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli": {"fmt", "github.com/spf13/cobra"},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsRefusesImportOfCmd(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/typst":  {testModule + "/cmd/tool/helper"},
		testModule + "/cmd/tool/helper": {},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/typst imports " + testModule + "/cmd/tool/helper: packages under cmd/ are not importable",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsAllowsCmdToImportCmd(t *testing.T) {
	imports := map[string][]string{
		testModule + "/cmd/tool":        {testModule + "/cmd/tool/helper"},
		testModule + "/cmd/tool/helper": {},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsAllowsCmdToImportAnyLayer(t *testing.T) {
	imports := map[string][]string{
		testModule + "/cmd/tool": {testModule + "/internal/cli", testModule + "/internal/typst"},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsRefusesRootImportingModule(t *testing.T) {
	imports := map[string][]string{
		testModule: {testModule + "/internal/cli"},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + " imports " + testModule + "/internal/cli: the root package imports nothing of this module",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsAllowsImportOfRoot(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli": {testModule},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsEdgeFromUnclassifiedSource(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/mystery": {testModule + "/internal/mystery2"},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/mystery imports " + testModule + "/internal/mystery2: " + testModule + "/internal/mystery2 is not classified in guard.Layers",
		testModule + "/internal/mystery: not classified in guard.Layers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsUnclassifiedSourceImportingCmd(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/mystery": {testModule + "/cmd/tool"},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/mystery imports " + testModule + "/cmd/tool: packages under cmd/ are not importable",
		testModule + "/internal/mystery: not classified in guard.Layers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsUnclassifiedSourceImportingClassified(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/mystery": {testModule + "/internal/cli"},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/mystery: not classified in guard.Layers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsReportsEdgeToUnclassified(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli": {testModule + "/internal/mystery"},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/cli imports " + testModule + "/internal/mystery: " + testModule + "/internal/mystery is not classified in guard.Layers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsReportsEdgeFromCmd(t *testing.T) {
	imports := map[string][]string{
		testModule + "/cmd/tool": {testModule + "/internal/mystery"},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/cmd/tool imports " + testModule + "/internal/mystery: " + testModule + "/internal/mystery is not classified in guard.Layers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsReportsSourceAndEdge(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli":     {testModule + "/internal/mystery"},
		testModule + "/internal/mystery": {},
	}

	got := ImportViolations(testModule, Layers, imports)
	want := []string{
		testModule + "/internal/cli imports " + testModule + "/internal/mystery: " + testModule + "/internal/mystery is not classified in guard.Layers",
		testModule + "/internal/mystery: not classified in guard.Layers",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ImportViolations = %v, want %v", got, want)
	}
}

func TestImportViolationsEdgeIgnoresStandardLibrary(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli": {"fmt", "github.com/spf13/cobra"},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsEdgeIgnoresRoot(t *testing.T) {
	imports := map[string][]string{
		testModule + "/internal/cli": {testModule},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestImportViolationsEdgeAllowsCmdToClassified(t *testing.T) {
	imports := map[string][]string{
		testModule + "/cmd/tool": {testModule + "/internal/cli"},
	}

	got := ImportViolations(testModule, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}

func TestLayersHoldEveryPackageOfThisPart(t *testing.T) {
	want := map[string]int{
		"internal/guard":      0,
		"internal/typst":      0,
		"internal/redact":     0,
		"internal/promtest":   0,
		"template":            0,
		"template/golden":     10,
		"internal/config":     20,
		"internal/prom":       30,
		"internal/discovery":  40,
		"internal/resolve":    50,
		"internal/collect":    55,
		"internal/analysis":   60,
		"internal/snapshot":   70,
		"internal/catalog":    80,
		"internal/document":   90,
		"internal/goldentest": 95,
		"internal/cli":        100,
	}

	got := make(map[string]int, len(Layers))
	for _, layer := range Layers {
		got[layer.Path] = layer.Rank
	}

	for path, rank := range want {
		gotRank, ok := got[path]
		if !ok {
			t.Fatalf("Layers is missing %q", path)
		}
		if gotRank != rank {
			t.Fatalf("Layers[%q].Rank = %d, want %d", path, gotRank, rank)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("Layers has %d entries, want %d", len(got), len(want))
	}
}

func TestLayersHoldThePackagesOfThisPart(t *testing.T) {
	want := map[string]int{
		"internal/resolve":  50,
		"internal/collect":  55,
		"internal/analysis": 60,
	}

	got := make(map[string]int, len(Layers))
	for _, layer := range Layers {
		got[layer.Path] = layer.Rank
	}

	for path, rank := range want {
		gotRank, ok := got[path]
		if !ok {
			t.Fatalf("Layers is missing %q", path)
		}
		if gotRank != rank {
			t.Fatalf("Layers[%q].Rank = %d, want %d", path, gotRank, rank)
		}
	}
}

// goListPackage mirrors the subset of `go list -json` output this guard
// relies on. TestImports and XTestImports are intentionally absent: an
// import made only by test files is outside the rule.
type goListPackage struct {
	ImportPath string
	Imports    []string
}

// scanImports runs `go list -json -tags=<guard.BuildTags> ./...` from root
// and decodes the stream of JSON objects it produces (one per package, not
// a JSON array). Passing every declared build tag makes files that are
// otherwise gated behind one (e.g. //go:build toolchain) visible to the
// scan, since go list only reports the imports of files it would compile.
func scanImports(t *testing.T, root string) map[string][]string {
	t.Helper()
	cmd := exec.Command("go", "list", "-json", "-tags="+strings.Join(BuildTags, ","), "./...")
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go list -json ./...: %v\n%s", err, stderr.String())
	}

	imports := make(map[string][]string)
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decoding go list output: %v", err)
		}
		imports[pkg.ImportPath] = pkg.Imports
	}
	return imports
}

func TestRepositoryImportsPointOneWay(t *testing.T) {
	root := moduleRoot(t)

	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("reading go.mod: %v", err)
	}
	module := modfile.ModulePath(raw)
	if module == "" {
		t.Fatalf("could not read module path from go.mod")
	}

	imports := scanImports(t, root)
	if len(imports) == 0 {
		t.Fatalf("go list returned no packages; a scan that finds nothing protects nothing")
	}

	got := ImportViolations(module, Layers, imports)
	if len(got) != 0 {
		t.Fatalf("ImportViolations = %v, want empty", got)
	}
}
