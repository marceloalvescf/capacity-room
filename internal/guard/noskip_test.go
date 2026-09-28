package guard

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const skipSource = `package sample

import "testing"
func TestSomething(t *testing.T) {
	t.Skip("x")
}
`

func TestSkipCallsFindsSkip(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(skipSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	want := []string{"sample_test.go:5: Skip is forbidden; a skipped gate is not a checked gate"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("SkipCalls = %v, want %v", got, want)
	}
}

const skipfSkipNowSource = `package sample

import "testing"
func TestSomething(t *testing.T) {
	t.Skipf("reason %d", 1)
	t.SkipNow()
}
`

func TestSkipCallsFindsSkipfAndSkipNow(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(skipfSkipNowSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	want := []string{
		"sample_test.go:5: Skipf is forbidden; a skipped gate is not a checked gate",
		"sample_test.go:6: SkipNow is forbidden; a skipped gate is not a checked gate",
	}
	if len(got) != len(want) {
		t.Fatalf("SkipCalls = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("SkipCalls = %v, want %v", got, want)
		}
	}
}

const methodValueSkipSource = `package sample

import "testing"

func TestSomething(t *testing.T) {
	skip := t.Skip
	skip("x")
}
`

func TestSkipCallsFindsMethodValue(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(methodValueSkipSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	want := []string{"sample_test.go:6: Skip is forbidden; a skipped gate is not a checked gate"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipCalls = %v, want %v", got, want)
	}
}

const anyReceiverSkipSource = `package sample

import "testing"
func TestSomething(t *testing.T, s *suite, tb testing.TB) {
	s.T().SkipNow()
	tb.Skipf("x")
}
`

func TestSkipCallsFindsAnyReceiver(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(anyReceiverSkipSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	want := []string{
		"sample_test.go:5: SkipNow is forbidden; a skipped gate is not a checked gate",
		"sample_test.go:6: Skipf is forbidden; a skipped gate is not a checked gate",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SkipCalls = %v, want %v", got, want)
	}
}

func TestSkipCallsReportsACallOnce(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(skipSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("SkipCalls = %v, want exactly one entry", got)
	}
}

const stringsAndCommentsSource = `package sample

// t.Skip() should never run, by design.
var note = "call t.Skip() to bail out"
`

func TestSkipCallsIgnoresStringsAndComments(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(stringsAndCommentsSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SkipCalls = %v, want empty", got)
	}
}

const skippedSelectorSource = `package sample

import "testing"
func TestSomething(t *testing.T) {
	if t.Skipped() {
		return
	}
}
`

func TestSkipCallsIgnoresOtherSelectors(t *testing.T) {
	got, err := SkipCalls("sample_test.go", []byte(skippedSelectorSource))
	if err != nil {
		t.Fatalf("SkipCalls returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("SkipCalls = %v, want empty", got)
	}
}

const testFileNoImportsSource = `package sample

func helper() {}
`

func TestScannedForSkipsTestFile(t *testing.T) {
	got, err := ScannedForSkips("a_test.go", []byte(testFileNoImportsSource))
	if err != nil {
		t.Fatalf("ScannedForSkips returned error: %v", err)
	}
	if !got {
		t.Fatalf("ScannedForSkips = false, want true")
	}
}

const importsTestingSource = `package sample

import "testing"

func SkipIfFlaky(t *testing.T) {
	t.Skip("flaky in CI")
}
`

func TestScannedForSkipsHelperImportingTesting(t *testing.T) {
	got, err := ScannedForSkips("helper.go", []byte(importsTestingSource))
	if err != nil {
		t.Fatalf("ScannedForSkips returned error: %v", err)
	}
	if !got {
		t.Fatalf("ScannedForSkips = false, want true")
	}
}

const importsTestingAliasedSource = `package sample

import tt "testing"

func SkipIfFlaky(t *tt.T) {
	t.Skip("flaky in CI")
}
`

func TestScannedForSkipsHelperWithAlias(t *testing.T) {
	got, err := ScannedForSkips("helper.go", []byte(importsTestingAliasedSource))
	if err != nil {
		t.Fatalf("ScannedForSkips returned error: %v", err)
	}
	if !got {
		t.Fatalf("ScannedForSkips = false, want true")
	}
}

const plainFileSource = `package sample

import "fmt"

type options struct {
	Skip bool
}

func describe(opts options) string {
	return fmt.Sprintf("skip=%v", opts.Skip)
}
`

func TestScannedForSkipsPlainFile(t *testing.T) {
	got, err := ScannedForSkips("model.go", []byte(plainFileSource))
	if err != nil {
		t.Fatalf("ScannedForSkips returned error: %v", err)
	}
	if got {
		t.Fatalf("ScannedForSkips = true, want false")
	}
}

func TestScannedForSkipsUnparsable(t *testing.T) {
	_, err := ScannedForSkips("bad.go", []byte("packag x"))
	if err == nil {
		t.Fatalf("ScannedForSkips returned no error, want error")
	}
}

// scanExcludedDirs lists folders that hold no source this project gates on:
// build output, vendored history, and scratch. The skip scan and the
// build-tag scan both walk the same tree, so they share this list and the
// walkGoFiles helper below.
var scanExcludedDirs = map[string]bool{
	"older":        true,
	".cache":       true,
	"web":          true,
	".git":         true,
	".superpowers": true,
	"node_modules": true,
}

// walkGoFiles walks every .go file below root, skipping scanExcludedDirs,
// and calls visit with each file's path relative to root and its contents.
// It returns the number of files visited, since a repository test whose
// walk silently visits nothing would protect nothing while still passing.
func walkGoFiles(t *testing.T, root string, visit func(rel string, source []byte) error) int {
	t.Helper()

	visited := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if scanExcludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if err := visit(rel, source); err != nil {
			return err
		}
		visited++
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	return visited
}

// TestRepositoryHasNoSkippedTest walks every .go file below the module
// root and parses it directly from disk, rather than asking `go list` to
// enumerate files, so a file hidden behind a build tag is still read. Each
// file is asked ScannedForSkips first, so a non-test helper that imports
// testing (and so can call t.Skip on a *testing.T handed to it) is checked
// too, not just files named *_test.go.
func TestRepositoryHasNoSkippedTest(t *testing.T) {
	root := moduleRoot(t)

	var violations []string
	filesScanned := 0
	walkGoFiles(t, root, func(rel string, source []byte) error {
		scan, err := ScannedForSkips(rel, source)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", rel, err)
		}
		if !scan {
			return nil
		}

		found, err := SkipCalls(rel, source)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", rel, err)
		}
		filesScanned++
		violations = append(violations, found...)
		return nil
	})
	if filesScanned == 0 {
		t.Fatalf("no files were scanned for skips; a scan that finds nothing protects nothing")
	}
	if len(violations) != 0 {
		t.Fatalf("skipped tests found:\n%s", strings.Join(violations, "\n"))
	}
}
