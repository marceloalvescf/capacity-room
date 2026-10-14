//go:build template

package template

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/typst"
)

func negativeFixtureDir(ws string) string {
	return filepath.Join(ws, "tests", "graphite", "negative")
}

func TestNegativeFixturesFailWithExpectedMessage(t *testing.T) {
	ws := workspace(t)
	dir := negativeFixtureDir(ws)
	matches, err := filepath.Glob(filepath.Join(dir, "*.typ"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(matches) == 0 {
		t.Fatalf("no negative fixtures found in %s", dir)
	}

	for _, file := range matches {
		t.Run(filepath.Base(file), func(t *testing.T) {
			expectPath := strings.TrimSuffix(file, ".typ") + ".expect"
			raw, err := os.ReadFile(expectPath)
			if err != nil {
				t.Fatalf("reading %s: %v", expectPath, err)
			}
			// Only the trailing newline is trimmed: the fragment must
			// still match exactly, byte for byte, against the compiler's
			// stderr.
			fragment := strings.TrimSuffix(string(raw), "\n")

			rel, err := filepath.Rel(ws, file)
			if err != nil {
				t.Fatalf("computing relative path for %s: %v", file, err)
			}
			_, stderr, err := compilePDF(t, ws, rel, nil)

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

func TestEveryNegativeFixtureHasAnExpectation(t *testing.T) {
	ws := workspace(t)
	dir := negativeFixtureDir(ws)

	typFiles, err := filepath.Glob(filepath.Join(dir, "*.typ"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(typFiles) == 0 {
		t.Fatalf("no .typ fixtures found in %s", dir)
	}
	for _, f := range typFiles {
		expectPath := strings.TrimSuffix(f, ".typ") + ".expect"
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
		typPath := strings.TrimSuffix(f, ".expect") + ".typ"
		if _, err := os.Stat(typPath); err != nil {
			t.Errorf("%s has no matching .typ file", f)
		}
	}
}

func TestNegativeFixtureCount(t *testing.T) {
	ws := workspace(t)
	dir := negativeFixtureDir(ws)
	matches, err := filepath.Glob(filepath.Join(dir, "*.typ"))
	if err != nil {
		t.Fatalf("globbing %s: %v", dir, err)
	}
	if len(matches) != 12 {
		t.Fatalf("negative fixture count = %d, want 12", len(matches))
	}
}
