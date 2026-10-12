//go:build template

package template

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// moduleRoot walks up from the current working directory to the first
// folder holding go.mod. This mirrors internal/guard's own helper (see
// internal/guard/changelog_test.go) and internal/typst's (see
// internal/typst/compile_real_test.go); it is duplicated here rather than
// shared because Go test helpers cannot be imported across packages.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// copyTree copies every entry under src into dst, preserving structure. An
// entry (file or directory) for which skip returns true is left out
// entirely: a skipped directory is neither created nor descended into.
func copyTree(t *testing.T, src, dst string, skip func(path string) bool) {
	t.Helper()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skip(path) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatalf("copying %s to %s: %v", src, dst, err)
	}
}

// workspace copies the template folder from disk into a fresh temp folder
// and returns it. It is the compile root. Copying is required rather than
// compiling in place because the source tree is mounted read-only when
// these tests run in the test image, and because some tests (recipes,
// package install) must write generated documents inside the compile root.
func workspace(t *testing.T) string {
	t.Helper()
	src := filepath.Join(moduleRoot(t), "template")
	dst := t.TempDir()
	copyTree(t, src, dst, func(path string) bool {
		return filepath.Ext(path) == ".go"
	})
	return dst
}

var (
	toolchainOnce   sync.Once
	cachedToolchain typst.Toolchain
	toolchainErr    error
)

// toolchain locates and probes typst once per test binary. It fails the
// calling test (never skips it) when either step fails, so a missing or
// wrong-version compiler shows up as a hard failure everywhere it matters.
func toolchain(t *testing.T) typst.Toolchain {
	t.Helper()
	toolchainOnce.Do(func() {
		tc, err := typst.Locate(os.Getenv, exec.LookPath)
		if err != nil {
			toolchainErr = err
			return
		}
		if err := tc.Probe(context.Background()); err != nil {
			toolchainErr = err
			return
		}
		cachedToolchain = tc
	})
	if toolchainErr != nil {
		t.Fatalf("typst toolchain unavailable: %v", toolchainErr)
	}
	return cachedToolchain
}

// compilePDF compiles source, a path relative to root (typically the value
// workspace returned), assuming the standard unpacked layout: fonts at
// FontPath(root). The PDF is written to a fresh temp file so repeated calls
// with the same t never collide.
func compilePDF(t *testing.T, root, source string, inputs map[string]string) (pdf string, stderr string, err error) {
	t.Helper()
	tc := toolchain(t)
	output := filepath.Join(t.TempDir(), "out.pdf")
	job := typst.Job{
		Root:     root,
		FontPath: FontPath(root),
		Source:   filepath.Join(root, source),
		Output:   output,
		Format:   typst.PDF,
		Inputs:   inputs,
	}
	stderr, err = tc.Compile(context.Background(), job)
	return output, stderr, err
}

// pdfText returns the text layer through pdftotext -layout. It fails the
// test (never skips it) when pdftotext is missing or exits non-zero.
func pdfText(t *testing.T, pdf string) string {
	t.Helper()
	out, err := exec.Command("pdftotext", "-layout", pdf, "-").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("pdftotext -layout %s -: %v, stderr: %s", pdf, exitErr, exitErr.Stderr)
		}
		t.Fatalf("pdftotext -layout %s -: %v", pdf, err)
	}
	return string(out)
}

// assertCleanCompile checks the two conditions a clean compile requires:
// no error, and no stderr output at all (a compiler warning is a failure
// project-wide, and Toolchain.Compile already turns one into an error, but
// this also catches stray diagnostics on an otherwise successful exit).
func assertCleanCompile(t *testing.T, stderr string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Compile() error = %v, stderr = %q", err, stderr)
	}
	if stderr != "" {
		t.Fatalf("Compile() stderr = %q, want empty", stderr)
	}
}

// assertNonEmptyPDF fails the test unless path names a non-empty file.
func assertNonEmptyPDF(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() == 0 {
		t.Fatalf("%s is empty", path)
	}
}

// pdfTextRaw reads the text layer in content order, cell by cell, so a
// value that wraps inside a cell stays whole.
func pdfTextRaw(t *testing.T, pdf string) string {
	t.Helper()
	out, err := exec.Command("pdftotext", "-raw", pdf, "-").Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			t.Fatalf("pdftotext -raw %s -: %v, stderr: %s", pdf, exitErr, exitErr.Stderr)
		}
		t.Fatalf("pdftotext -raw %s -: %v", pdf, err)
	}
	return string(out)
}
