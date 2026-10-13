//go:build template

package template

import (
	"bytes"
	"compress/zlib"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// pdfStreamStart finds the end of a stream's dictionary and the keyword
// that its bytes follow, pdfObjectStart the opening of an object, and
// pdfLength the length of a stream in its dictionary.
var (
	pdfStreamStart = regexp.MustCompile(`>>\s*stream\r?\n`)
	pdfObjectStart = regexp.MustCompile(`\d+ \d+ obj\b`)
	pdfLength      = regexp.MustCompile(`/Length (\d+)(\s+\d+\s+R)?`)
)

// pdfBodies returns the bytes of the PDF at path outside its streams, and
// the bytes of every stream, a FlateDecode stream inflated. Object streams
// are among them, so a structure element stored there is read too. A
// stream that fails to inflate fails the test.
func pdfBodies(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var bodies [][]byte
	var outside []byte
	pos := 0
	for {
		loc := pdfStreamStart.FindIndex(data[pos:])
		if loc == nil {
			break
		}
		start, open := pos+loc[0], pos+loc[1]
		objects := pdfObjectStart.FindAllIndex(data[:start], -1)
		if len(objects) == 0 {
			t.Fatalf("stream at byte %d has no object", start)
		}
		dict := data[objects[len(objects)-1][0] : start+2]
		length := pdfLength.FindSubmatch(dict)
		if length == nil || length[2] != nil {
			t.Fatalf("stream at byte %d has no direct /Length: %q", start, dict)
		}
		n, err := strconv.Atoi(string(length[1]))
		if err != nil || open+n > len(data) {
			t.Fatalf("stream at byte %d has a bad /Length %q", start, length[1])
		}
		body := data[open : open+n]
		if bytes.Contains(dict, []byte("/FlateDecode")) {
			r, err := zlib.NewReader(bytes.NewReader(body))
			if err != nil {
				t.Fatalf("stream at byte %d: %v", start, err)
			}
			inflated, err := io.ReadAll(r)
			if err != nil {
				t.Fatalf("stream at byte %d: %v", start, err)
			}
			body = inflated
		}
		bodies = append(bodies, body)
		outside = append(outside, data[pos:start]...)
		pos = open + n
	}
	return append(bodies, append(outside, data[pos:]...))
}

// artifactOpenings counts the marked-content openings of /Artifact in
// bodies.
func artifactOpenings(bodies [][]byte) int {
	n := 0
	for _, body := range bodies {
		n += bytes.Count(body, []byte("/Artifact BMC")) + bytes.Count(body, []byte("/Artifact <<"))
	}
	return n
}

// holds reports whether any of bodies matches pattern.
func holds(bodies [][]byte, pattern *regexp.Regexp) bool {
	for _, body := range bodies {
		if pattern.Match(body) {
			return true
		}
	}
	return false
}

// buildRun compiles one run alone and returns the bodies of its PDF.
func buildRun(t *testing.T, ws, name, run string) [][]byte {
	t.Helper()
	src := "#import \"capacity/runs.typ\": run-to-content\n" +
		"#set page(width: 100pt, height: 50pt)\n" +
		"#run-to-content(" + run + ")\n"
	if err := os.WriteFile(filepath.Join(ws, name), []byte(src), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
	pdf, stderr, err := compilePDF(t, ws, name, nil)
	assertCleanCompile(t, stderr, err)
	return pdfBodies(t, pdf)
}

// TestStatusGlyphIsAnArtifact checks that the mark of a status run with no
// alternate text is marked content of the PDF that a reader skips, and that
// a text run holds none.
func TestStatusGlyphIsAnArtifact(t *testing.T) {
	ws := workspace(t)
	status := artifactOpenings(buildRun(t, ws, "artifact-status.typ", `(kind: "status", status: "critical", text: none, alt: none)`))
	text := artifactOpenings(buildRun(t, ws, "artifact-text.typ", `(kind: "text", text: "x")`))
	if status != text+1 {
		t.Fatalf("artifact openings: status run %d, text run %d, want the status run to hold exactly one more", status, text)
	}
}

// TestStatusGlyphCarriesItsWord checks that the mark of a status run beside
// a value carries the run's alt as the alternate text of the PDF, and is no
// artifact.
func TestStatusGlyphCarriesItsWord(t *testing.T) {
	ws := workspace(t)
	value := buildRun(t, ws, "alt-status.typ", `(kind: "status", status: "critical", text: "24000%", alt: "Critical")`)
	text := buildRun(t, ws, "alt-text.typ", `(kind: "text", text: "24000%")`)
	if !holds(value, regexp.MustCompile(`/Alt\s*\(Critical\)`)) {
		t.Fatalf("the PDF of a status run with alt holds no /Alt (Critical)")
	}
	if holds(text, regexp.MustCompile(`/Alt\b`)) {
		t.Fatalf("the PDF of a text run holds an /Alt")
	}
	if got, want := artifactOpenings(value), artifactOpenings(text); got != want {
		t.Fatalf("artifact openings: status run with alt %d, text run %d, want them equal", got, want)
	}
}
