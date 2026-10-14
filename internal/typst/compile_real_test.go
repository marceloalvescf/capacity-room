//go:build template

package typst

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// moduleRoot walks up from the current working directory to the first
// folder holding go.mod. See internal/guard/changelog_test.go for the same
// pattern; it is duplicated here rather than imported because
// internal/typst must not import any other package of this module.
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

// realFontPath returns the font directory checked into the template
// (template/graphite/assets/fonts). These tests exercise the real typst
// binary, so they need real fonts rather than the empty directories the
// fake-binary tests use.
func realFontPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(moduleRoot(t), "template", "graphite", "assets", "fonts")
}

// realToolchain locates the real typst binary the same way production code
// does, via CAPACITYROOM_TYPST or PATH.
func realToolchain(t *testing.T) Toolchain {
	t.Helper()
	tc, err := Locate(os.Getenv, exec.LookPath)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	return tc
}

func TestRealCompileWritesPDF(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "doc.typ")
	if err := os.WriteFile(source, []byte("Hello, world!\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	output := filepath.Join(root, "out.pdf")
	job := Job{Root: root, FontPath: realFontPath(t), Source: source, Output: output, Format: PDF}

	stderr, err := realToolchain(t).Compile(context.Background(), job)
	if err != nil {
		t.Fatalf("Compile() error = %v, stderr = %q", err, stderr)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-")) {
		t.Fatalf("output does not start with %%PDF-: %q", raw[:min(len(raw), 16)])
	}
}

func TestRealCompileInFolderWithSpace(t *testing.T) {
	root := filepath.Join(t.TempDir(), "my work folder")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	source := filepath.Join(root, "doc.typ")
	if err := os.WriteFile(source, []byte("Hello, world!\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	output := filepath.Join(root, "out.pdf")
	job := Job{Root: root, FontPath: realFontPath(t), Source: source, Output: output, Format: PDF}

	stderr, err := realToolchain(t).Compile(context.Background(), job)
	if err != nil {
		t.Fatalf("Compile() error = %v, stderr = %q", err, stderr)
	}

	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte("%PDF-")) {
		t.Fatalf("output does not start with %%PDF-: %q", raw[:min(len(raw), 16)])
	}
}

func TestRealCompileReadsInput(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "doc.typ")
	if err := os.WriteFile(source, []byte(`#sys.inputs.at("name")`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	output := filepath.Join(root, "out.pdf")
	job := Job{
		Root:     root,
		FontPath: realFontPath(t),
		Source:   source,
		Output:   output,
		Format:   PDF,
		Inputs:   map[string]string{"name": "valor"},
	}

	stderr, err := realToolchain(t).Compile(context.Background(), job)
	if err != nil {
		t.Fatalf("Compile() error = %v, stderr = %q", err, stderr)
	}

	out, err := exec.Command("pdftotext", output, "-").Output()
	if err != nil {
		t.Fatalf("pdftotext: %v", err)
	}
	if !strings.Contains(string(out), "valor") {
		t.Fatalf("pdftotext output = %q, want it to contain %q", out, "valor")
	}
}

func TestRealCompileReportsError(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "doc.typ")
	if err := os.WriteFile(source, []byte(`#panic("stop here")`+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	output := filepath.Join(root, "out.pdf")
	job := Job{Root: root, FontPath: realFontPath(t), Source: source, Output: output, Format: PDF}

	_, err := realToolchain(t).Compile(context.Background(), job)
	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("Compile() error = %v, want *CompileError", err)
	}
	if !strings.Contains(compileErr.Stderr, "stop here") {
		t.Fatalf("CompileError.Stderr = %q, want it to contain %q", compileErr.Stderr, "stop here")
	}
}

func TestRealCompileReportsWarning(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "doc.typ")
	content := "#set text(font: \"No Such Font\")\nHello\n"
	if err := os.WriteFile(source, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	output := filepath.Join(root, "out.pdf")
	job := Job{Root: root, FontPath: realFontPath(t), Source: source, Output: output, Format: PDF}

	_, err := realToolchain(t).Compile(context.Background(), job)
	if !errors.Is(err, ErrWarning) {
		t.Fatalf("Compile() error = %v, want errors.Is(err, ErrWarning)", err)
	}
}

func TestRealCompileIsDeterministic(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "doc.typ")
	if err := os.WriteFile(source, []byte("Hello, world!\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	output := filepath.Join(root, "out.pdf")
	job := Job{Root: root, FontPath: realFontPath(t), Source: source, Output: output, Format: PDF}
	tc := realToolchain(t)

	if _, err := tc.Compile(context.Background(), job); err != nil {
		t.Fatalf("Compile() error (first run) = %v", err)
	}
	first, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading first output: %v", err)
	}
	sumFirst := sha256.Sum256(first)

	if _, err := tc.Compile(context.Background(), job); err != nil {
		t.Fatalf("Compile() error (second run) = %v", err)
	}
	second, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading second output: %v", err)
	}
	sumSecond := sha256.Sum256(second)

	if sumFirst != sumSecond {
		t.Fatalf("outputs differ: sha256 %x != %x", sumFirst, sumSecond)
	}
}
