//go:build template

package template

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

var pdfStream = regexp.MustCompile(`(?s)stream\r?\n(.*?)endstream`)

// artifactOpenings counts the marked-content openings of /Artifact in every
// stream of the PDF at path, inflating the ones that are deflated.
func artifactOpenings(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	n := 0
	for _, m := range pdfStream.FindAllSubmatch(data, -1) {
		body := m[1]
		if r, err := zlib.NewReader(bytes.NewReader(body)); err == nil {
			if inflated, err := io.ReadAll(r); err == nil {
				body = inflated
			}
		}
		n += bytes.Count(body, []byte("/Artifact BMC")) + bytes.Count(body, []byte("/Artifact <<"))
	}
	return n
}

// TestStatusGlyphIsAnArtifact checks that the mark of a status run is marked
// content of the PDF that a reader skips, and that a text run holds none.
func TestStatusGlyphIsAnArtifact(t *testing.T) {
	ws := workspace(t)
	build := func(name, run string) int {
		src := "#import \"capacity/runs.typ\": run-to-content\n" +
			"#set page(width: 100pt, height: 50pt)\n" +
			"#run-to-content(" + run + ")\n"
		if err := os.WriteFile(filepath.Join(ws, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
		pdf, stderr, err := compilePDF(t, ws, name, nil)
		assertCleanCompile(t, stderr, err)
		return artifactOpenings(t, pdf)
	}
	status := build("artifact-status.typ", `(kind: "status", status: "critical", text: none)`)
	text := build("artifact-text.typ", `(kind: "text", text: "x")`)
	if status != text+1 {
		t.Fatalf("artifact openings: status run %d, text run %d, want the status run to hold exactly one more", status, text)
	}
}
