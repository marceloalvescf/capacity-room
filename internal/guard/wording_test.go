package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

const portugueseSource = `package sample

func f() {
	x := "Sem observação"
	_ = x
}
`

func TestReaderTextFindsPortuguese(t *testing.T) {
	got, err := ReaderText("sample.go", []byte(portugueseSource), false)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	want := []string{`sample.go:4: "Sem observação"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReaderText = %v, want %v", got, want)
	}
}

const sentenceSource = `package sample

func f() {
	x := "No remark"
	_ = x
}
`

func TestReaderTextFindsSentenceWhenStrict(t *testing.T) {
	strictGot, err := ReaderText("sample.go", []byte(sentenceSource), true)
	if err != nil {
		t.Fatalf("ReaderText(strict) returned error: %v", err)
	}
	want := []string{`sample.go:4: "No remark"`}
	if !reflect.DeepEqual(strictGot, want) {
		t.Fatalf("ReaderText(strict) = %v, want %v", strictGot, want)
	}

	notStrictGot, err := ReaderText("sample.go", []byte(sentenceSource), false)
	if err != nil {
		t.Fatalf("ReaderText(not strict) returned error: %v", err)
	}
	if len(notStrictGot) != 0 {
		t.Fatalf("ReaderText(not strict) = %v, want empty", notStrictGot)
	}
}

const capitalisedSource = `package sample

func f() {
	x := "Page"
	_ = x
}
`

func TestReaderTextFindsCapitalisedWordWhenStrict(t *testing.T) {
	got, err := ReaderText("sample.go", []byte(capitalisedSource), true)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	want := []string{`sample.go:4: "Page"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReaderText = %v, want %v", got, want)
	}
}

func TestReaderTextIgnoresIdentifiers(t *testing.T) {
	cases := []string{
		"report-metadata",
		"text",
		"pt-BR",
		"healthy",
		"schema_version",
		"2006-01-02T15:04:05Z07:00",
		"02/01/2006 15:04",
		", ",
		" ",
		"%s: %d",
		"—",
		"●",
		"100%% of %s",
	}

	for _, value := range cases {
		t.Run(value, func(t *testing.T) {
			source := "package sample\n\nfunc f() {\n\tx := " + strconv.Quote(value) + "\n\t_ = x\n}\n"
			got, err := ReaderText("sample.go", []byte(source), true)
			if err != nil {
				t.Fatalf("ReaderText returned error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("ReaderText(%q) = %v, want empty", value, got)
			}
		})
	}
}

func TestReaderTextIgnoresDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
	}{
		{
			name: "errors.New",
			source: `package sample

import "errors"

func f() error {
	return errors.New("Report profile is wrong")
}
`,
		},
		{
			name: "fmt.Errorf",
			source: `package sample

import "fmt"

func f(v string) error {
	return fmt.Errorf("Unknown value %q", v)
}
`,
		},
		{
			name: "t.Errorf",
			source: `package sample

import "testing"

func f(t *testing.T) {
	t.Errorf("Got it")
}
`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ReaderText("sample.go", []byte(c.source), true)
			if err != nil {
				t.Fatalf("ReaderText returned error: %v", err)
			}
			if len(got) != 0 {
				t.Fatalf("ReaderText(%s) = %v, want empty", c.name, got)
			}
		})
	}
}

const tagsAndImportsSource = "package sample\n" +
	"\n" +
	"import \"encoding/json\"\n" +
	"\n" +
	"type Cluster struct {\n" +
	"\tName string `json:\"cluster name\"`\n" +
	"}\n" +
	"\n" +
	"var _ = json.Marshal\n"

func TestReaderTextIgnoresTagsAndImports(t *testing.T) {
	got, err := ReaderText("sample.go", []byte(tagsAndImportsSource), true)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReaderText = %v, want empty", got)
	}
}

const paramNamesSource = `package sample

import "github.com/marceloalvescf/k8s-capacity-report/internal/catalog"

func f() catalog.Params {
	start := "x"
	end := "y"
	return catalog.Params{"Start": start, "End": end}
}
`

func TestReaderTextIgnoresParameterNames(t *testing.T) {
	got, err := ReaderText("sample.go", []byte(paramNamesSource), true)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReaderText = %v, want empty", got)
	}
}

const paramValueSource = `package sample

import "github.com/marceloalvescf/k8s-capacity-report/internal/catalog"

func f() catalog.Params {
	return catalog.Params{"Start": "No remark"}
}
`

func TestReaderTextReadsParameterValues(t *testing.T) {
	got, err := ReaderText("sample.go", []byte(paramValueSource), true)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	want := []string{`sample.go:6: "No remark"`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReaderText = %v, want %v", got, want)
	}
}

func TestReaderTextReadsOtherMaps(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "map[string]string",
			source: `package sample

func f() {
	x := "y"
	_ = map[string]string{"Start": x}
}
`,
			want: `sample.go:5: "Start"`,
		},
		{
			name: "other.Params",
			source: `package sample

func f() {
	x := "y"
	_ = other.Params{"Start": x}
}
`,
			want: `sample.go:5: "Start"`,
		},
		{
			name: "slice element",
			source: `package sample

func f() {
	_ = []string{"Start"}
}
`,
			want: `sample.go:4: "Start"`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ReaderText("sample.go", []byte(c.source), true)
			if err != nil {
				t.Fatalf("ReaderText returned error: %v", err)
			}
			want := []string{c.want}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("ReaderText(%s) = %v, want %v", c.name, got, want)
			}
		})
	}
}

const paramNamesInCatalogSource = `package catalog

func f() Params {
	x := "y"
	return Params{"Start": x}
}
`

func TestReaderTextIgnoresParameterNamesInsideCatalog(t *testing.T) {
	got, err := ReaderText("params.go", []byte(paramNamesInCatalogSource), true)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("ReaderText = %v, want empty", got)
	}
}

func TestReaderTextHonoursTheComment(t *testing.T) {
	cases := []struct {
		name string
		line string
		want int
	}{
		{
			name: "valid reason",
			line: `	x := "America/Sao_Paulo" // wording: not reader text, a zone name`,
			want: 0,
		},
		{
			name: "empty reason",
			line: `	x := "America/Sao_Paulo" // wording: not reader text,`,
			want: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := "package sample\n\nfunc f() {\n" + c.line + "\n\t_ = x\n}\n"
			got, err := ReaderText("sample.go", []byte(source), true)
			if err != nil {
				t.Fatalf("ReaderText returned error: %v", err)
			}
			if len(got) != c.want {
				t.Fatalf("ReaderText(%s) = %v, want %d entries", c.name, got, c.want)
			}
		})
	}

	const commentAboveSource = `package sample

func f() {
	// wording: not reader text, a zone name
	x := "America/Sao_Paulo"
	_ = x
}
`
	t.Run("comment on the line above", func(t *testing.T) {
		got, err := ReaderText("sample.go", []byte(commentAboveSource), true)
		if err != nil {
			t.Fatalf("ReaderText returned error: %v", err)
		}
		if len(got) != 1 {
			t.Fatalf("ReaderText = %v, want 1 entry", got)
		}
	})
}

const threeLiteralsSource = `package sample

func f() {
	a := "pt-BR"
	b := "First sentence here"
	c := "Second sentence here"
	_, _, _ = a, b, c
}
`

func TestReaderTextReportsEveryLiteral(t *testing.T) {
	got, err := ReaderText("sample.go", []byte(threeLiteralsSource), true)
	if err != nil {
		t.Fatalf("ReaderText returned error: %v", err)
	}
	want := []string{
		`sample.go:5: "First sentence here"`,
		`sample.go:6: "Second sentence here"`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ReaderText = %v, want %v", got, want)
	}
}

func TestReaderTextRejectsBrokenSource(t *testing.T) {
	_, err := ReaderText("broken.go", []byte("packag x"), true)
	if err == nil {
		t.Fatalf("ReaderText returned no error, want error")
	}
}

// TestRepositoryHoldsNoReaderTextOutsideTheCatalog reads every Go file that
// git tracks under internal/ and cmd/, test files left out, and asks
// ReaderText whether it holds a sentence for a reader. internal/catalog and
// internal/guard are left out: the catalog is where such text belongs, and
// this package's own tests hold Portuguese and English text as fixtures.
// internal/document is read with strict, since it is closest to composing a
// report and so most exposed to acquiring wording of its own; every other
// package is read under the plain rule that holds everywhere.
func TestRepositoryHoldsNoReaderTextOutsideTheCatalog(t *testing.T) {
	root := moduleRoot(t)

	cmd := exec.Command("git", "ls-files", "--", "internal", "cmd")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git ls-files: %v", err)
	}

	var violations []string
	filesScanned := 0
	for _, rel := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if rel == "" {
			continue
		}
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if strings.HasPrefix(rel, "internal/catalog/") || strings.HasPrefix(rel, "internal/guard/") {
			continue
		}

		source, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}

		strict := strings.HasPrefix(rel, "internal/document/")
		found, err := ReaderText(rel, source, strict)
		if err != nil {
			t.Fatalf("parsing %s: %v", rel, err)
		}
		filesScanned++
		violations = append(violations, found...)
	}
	if filesScanned == 0 {
		t.Fatalf("no files were scanned for reader text; a scan that finds nothing protects nothing")
	}
	if len(violations) != 0 {
		t.Fatalf("reader text found outside the catalog:\n%s", strings.Join(violations, "\n"))
	}
}
