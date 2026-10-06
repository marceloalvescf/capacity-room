//go:build template

package render_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/render"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

func TestCompileReturnsAPDF(t *testing.T) {
	tc, err := typst.Locate(os.Getenv, exec.LookPath)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if err := tc.Probe(context.Background()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	data, err := render.Document(loadSnapshot(t, "direct-complete"), catalog.English, document.Options{})
	if err != nil {
		t.Fatalf("Document: %v", err)
	}

	tempDir := t.TempDir()
	pdf, err := render.Compile(context.Background(), tc, tempDir, data)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("the bytes start %q, want %%PDF-", pdf[:min(8, len(pdf))])
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("the temporary folder holds %d entries after Compile, want none", len(entries))
	}
}

func TestCompileRefusesBytesThatAreNotAPDF(t *testing.T) {
	script := "#!/bin/sh\n" +
		"last=\"\"\n" +
		"for a in \"$@\"; do last=\"$a\"; done\n" +
		"printf hello > \"$last\"\n"
	binary := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tempDir := t.TempDir()
	_, err := render.Compile(context.Background(), typst.Toolchain{Binary: binary}, tempDir, []byte("{}"))
	if !errors.Is(err, render.ErrNotPDF) {
		t.Fatalf("err = %v, want render.ErrNotPDF", err)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("the temporary folder holds %d entries after the refusal, want none", len(entries))
	}
}
