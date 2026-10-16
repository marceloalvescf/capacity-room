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

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/document"
	"github.com/marceloalvescf/capacity-room/internal/render"
	"github.com/marceloalvescf/capacity-room/internal/typst"
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

func TestCompileDrawsAMinimumOutsideTheGauge(t *testing.T) {
	tc, err := typst.Locate(os.Getenv, exec.LookPath)
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if err := tc.Probe(context.Background()); err != nil {
		t.Fatalf("Probe: %v", err)
	}
	for _, minimum := range []float64{5, -1} {
		snap := loadSnapshot(t, "edge-scaling")
		snap.Summary.Autoscaling.MinNodes = &minimum
		data, err := render.Document(snap, catalog.English, document.Options{})
		if err != nil {
			t.Fatalf("Document with minimum %v: %v", minimum, err)
		}
		pdf, err := render.Compile(context.Background(), tc, t.TempDir(), data)
		if err != nil {
			t.Fatalf("Compile with minimum %v: %v", minimum, err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
			t.Fatalf("minimum %v: the bytes do not start with %%PDF-", minimum)
		}
	}
}
