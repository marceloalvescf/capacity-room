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

// noSkipScanExcludedDirs mirrors SkipCalls' real-scan exclusion list: build
// output and vendored history under these names holds no test the project
// gates on.
var noSkipScanExcludedDirs = map[string]bool{
	"older":        true,
	".cache":       true,
	"web":          true,
	".git":         true,
	".superpowers": true,
	"node_modules": true,
}

// TestRepositoryHasNoSkippedTest walks every *_test.go file below the
// module root and parses it directly from disk with SkipCalls, rather than
// asking `go list` to enumerate files, so a test hidden behind a build tag
// is still read and checked.
func TestRepositoryHasNoSkippedTest(t *testing.T) {
	root := moduleRoot(t)

	var violations []string
	filesRead := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if noSkipScanExcludedDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
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

		found, err := SkipCalls(rel, source)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", rel, err)
		}
		filesRead++
		violations = append(violations, found...)
		return nil
	})
	if err != nil {
		t.Fatalf("walking repository: %v", err)
	}
	if filesRead == 0 {
		t.Fatalf("no test files were read; a scan that finds nothing protects nothing")
	}
	if len(violations) != 0 {
		t.Fatalf("skipped tests found:\n%s", strings.Join(violations, "\n"))
	}
}
