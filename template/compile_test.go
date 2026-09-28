//go:build template

package template

import (
	"path/filepath"
	"testing"
)

// compileFixturesCleanly compiles every *.typ file directly under
// workspace-relative dir and asserts each produces a clean, non-empty PDF.
func compileFixturesCleanly(t *testing.T, dir string) {
	t.Helper()
	ws := workspace(t)
	pattern := filepath.Join(ws, dir, "*.typ")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("globbing %s: %v", pattern, err)
	}
	if len(matches) == 0 {
		t.Fatalf("no fixtures found matching %s", pattern)
	}

	for _, file := range matches {
		rel, err := filepath.Rel(ws, file)
		if err != nil {
			t.Fatalf("computing relative path for %s: %v", file, err)
		}
		t.Run(filepath.Base(file), func(t *testing.T) {
			pdf, stderr, err := compilePDF(t, ws, rel, nil)
			assertCleanCompile(t, stderr, err)
			assertNonEmptyPDF(t, pdf)
		})
	}
}

func TestUnitFilesCompileCleanly(t *testing.T) {
	compileFixturesCleanly(t, filepath.Join("tests", "graphite", "unit"))
}

func TestExamplesCompileCleanly(t *testing.T) {
	compileFixturesCleanly(t, filepath.Join("graphite", "examples"))
}

func TestPositiveFixturesCompileCleanly(t *testing.T) {
	compileFixturesCleanly(t, filepath.Join("tests", "graphite", "positive"))
}
