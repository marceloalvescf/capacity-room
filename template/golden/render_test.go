package golden

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// moduleRoot walks up from the current working directory to the first
// folder holding go.mod. Duplicated from internal/guard/changelog_test.go
// and template/harness_test.go rather than imported, because this package
// must not import the template package (see package doc) and Go test
// helpers cannot be shared across packages anyway.
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

func TestEverySnapshotStatesWhatItProtects(t *testing.T) {
	seen := make(map[string]string)
	for _, s := range Snapshots {
		if strings.TrimSpace(s.Protects) == "" {
			t.Errorf("%s: Protects is empty", s.Name)
			continue
		}
		if other, ok := seen[s.Protects]; ok {
			t.Errorf("%s and %s: Protects text is identical: %q", s.Name, other, s.Protects)
			continue
		}
		seen[s.Protects] = s.Name
	}
}

// TestSnapshotPagesAreAscendingAndPositive checks Pages' own shape,
// independent of any real document: every page number is positive, and
// the list is strictly ascending (so also free of repeats). A snapshot
// whose Pages is out of order or repeats a page would silently render and
// compare the same PNG twice while skipping another.
func TestSnapshotPagesAreAscendingAndPositive(t *testing.T) {
	for _, s := range Snapshots {
		if len(s.Pages) == 0 {
			t.Errorf("%s: Pages is empty", s.Name)
			continue
		}
		for i, page := range s.Pages {
			if page < 1 {
				t.Errorf("%s: Pages[%d] = %d, want a positive page number", s.Name, i, page)
			}
			if i > 0 && s.Pages[i-1] >= page {
				t.Errorf("%s: Pages = %v is not strictly ascending at index %d", s.Name, s.Pages, i)
			}
		}
	}
}

func TestSnapshotNamesAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for _, s := range Snapshots {
		if seen[s.Name] {
			t.Errorf("Name %q appears more than once in Snapshots", s.Name)
		}
		seen[s.Name] = true
	}
}

func TestSnapshotSourcesExist(t *testing.T) {
	templateDir := filepath.Join(moduleRoot(t), "template")
	for _, s := range Snapshots {
		path := filepath.Join(templateDir, s.Source)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("%s: source %s: %v", s.Name, s.Source, err)
			continue
		}
		if info.IsDir() {
			t.Errorf("%s: source %s is a directory, want a file", s.Name, s.Source)
		}

		if s.Data == "" {
			continue
		}
		dataPath := filepath.Join(templateDir, s.Data)
		dataInfo, err := os.Stat(dataPath)
		if err != nil {
			t.Errorf("%s: data %s: %v", s.Name, s.Data, err)
			continue
		}
		if dataInfo.IsDir() {
			t.Errorf("%s: data %s is a directory, want a file", s.Name, s.Data)
		}
	}
}

func TestJobSetsDataInput(t *testing.T) {
	s := Snapshot{
		Name:   "capacity-pt",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/detailed.pt-BR.json",
		Pages:  []int{1, 2},
	}
	j, err := job("/root", s, "/dir")
	if err != nil {
		t.Fatalf("job() error = %v", err)
	}
	want := "/tests/capacity/fixtures/detailed.pt-BR.json"
	if got := j.Inputs["data"]; got != want {
		t.Errorf(`Inputs["data"] = %q, want %q`, got, want)
	}
}

func TestJobLeavesInputsAloneWithoutData(t *testing.T) {
	s := Snapshot{
		Name:   "theme",
		Source: "graphite/examples/custom-theme.typ",
		Pages:  []int{1},
		Inputs: map[string]string{"foo": "bar"},
	}
	j, err := job("/root", s, "/dir")
	if err != nil {
		t.Fatalf("job() error = %v", err)
	}
	if _, exists := j.Inputs["data"]; exists {
		t.Error(`Inputs has "data" set, want it absent when Data is empty`)
	}
	if got := j.Inputs["foo"]; got != "bar" {
		t.Errorf(`Inputs["foo"] = %q, want %q`, got, "bar")
	}
}

func TestJobRejectsDataAndDataInputTogether(t *testing.T) {
	s := Snapshot{
		Name:   "capacity-pt",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/detailed.pt-BR.json",
		Pages:  []int{1},
		Inputs: map[string]string{"data": "/report.json"},
	}
	if _, err := job("/root", s, "/dir"); err == nil {
		t.Fatal("job() error = nil, want an error naming the Data/Inputs[\"data\"] conflict")
	}
}

func TestJobBuildsExpectedFields(t *testing.T) {
	s := Snapshot{
		Name:   "capacity-pt",
		Source: "capacity/main.typ",
		Data:   "tests/capacity/fixtures/detailed.pt-BR.json",
		Pages:  []int{1, 3},
	}
	j, err := job("/root", s, "/dir")
	if err != nil {
		t.Fatalf("job() error = %v", err)
	}
	if j.Root != "/root" {
		t.Errorf("Root = %q, want %q", j.Root, "/root")
	}
	if want := filepath.Join("/root", "graphite", "assets", "fonts"); j.FontPath != want {
		t.Errorf("FontPath = %q, want %q", j.FontPath, want)
	}
	if want := filepath.Join("/root", "capacity", "main.typ"); j.Source != want {
		t.Errorf("Source = %q, want %q", j.Source, want)
	}
	if want := filepath.Join("/dir", "capacity-pt-{p}.png"); j.Output != want {
		t.Errorf("Output = %q, want %q", j.Output, want)
	}
	if j.Pages != "1,3" {
		t.Errorf("Pages = %q, want %q", j.Pages, "1,3")
	}
	if j.PPI != PPI {
		t.Errorf("PPI = %d, want %d", j.PPI, PPI)
	}
}

func TestFilesNamesPages(t *testing.T) {
	s := Snapshot{Name: "x", Pages: []int{1, 3}}
	got := s.Files()
	want := []string{"x-1.png", "x-3.png"}
	if len(got) != len(want) {
		t.Fatalf("Files() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Files() = %v, want %v", got, want)
		}
	}
}

func TestSumsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	sums := map[string]string{
		"b-1.png": strings.Repeat("b", 64),
		"a-1.png": strings.Repeat("a", 64),
	}

	if err := WriteSums(path, sums); err != nil {
		t.Fatalf("WriteSums() error = %v", err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if !sort.StringsAreSorted(namesFromLines(t, lines)) {
		t.Fatalf("SHA256SUMS is not sorted by name: %v", lines)
	}

	got, err := ReadSums(path)
	if err != nil {
		t.Fatalf("ReadSums() error = %v", err)
	}
	if len(got) != len(sums) {
		t.Fatalf("ReadSums() = %v, want %v", got, sums)
	}
	for name, digest := range sums {
		if got[name] != digest {
			t.Errorf("ReadSums()[%q] = %q, want %q", name, got[name], digest)
		}
	}
}

// namesFromLines extracts the file name (the part after the two-space
// separator) from each SHA256SUMS line, so the round-trip test can check
// the file is sorted by name rather than by digest.
func namesFromLines(t *testing.T, lines []string) []string {
	t.Helper()
	names := make([]string, len(lines))
	for i, line := range lines {
		idx := strings.Index(line, "  ")
		if idx < 0 {
			t.Fatalf("malformed SHA256SUMS line: %q", line)
		}
		names[i] = line[idx+2:]
	}
	return names
}

func TestReadSumsRejectsMalformedLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	if err := os.WriteFile(path, []byte("not-a-valid-line\n"), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	_, err := ReadSums(path)
	if err == nil {
		t.Fatal("ReadSums() error = nil, want an error naming the line number")
	}
	if !strings.Contains(err.Error(), "1") {
		t.Fatalf("ReadSums() error = %q, want it to name the line number", err)
	}
}

func TestReadSumsRejectsDuplicate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SHA256SUMS")
	digest := strings.Repeat("a", 64)
	content := digest + "  dup.png\n" + digest + "  dup.png\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}

	_, err := ReadSums(path)
	if err == nil {
		t.Fatal("ReadSums() error = nil, want an error for the duplicate name")
	}
}

// greyPNG writes a w x h all-grey RGBA PNG to path, optionally with a
// single red pixel at (0, 0). It always encodes through image.RGBA so the
// test exercises the same pixel format Typst emits, per Typst writing RGBA
// even for a page with no colour at all.
func greyPNG(t *testing.T, path string, w, h int, redCorner bool) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	grey := color.RGBA{R: 128, G: 128, B: 128, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, grey)
		}
	}
	if redCorner {
		img.Set(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encoding %s: %v", path, err)
	}
}

func TestHasColourFalseForGrey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "grey.png")
	greyPNG(t, path, 4, 4, false)

	got, err := HasColour(path)
	if err != nil {
		t.Fatalf("HasColour() error = %v", err)
	}
	if got {
		t.Fatal("HasColour() = true, want false for an all-grey image")
	}
}

func TestHasColourTrueForOneRedPixel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "red-corner.png")
	greyPNG(t, path, 4, 4, true)

	got, err := HasColour(path)
	if err != nil {
		t.Fatalf("HasColour() error = %v", err)
	}
	if !got {
		t.Fatal("HasColour() = false, want true when one pixel is red")
	}
}

func TestWriteDifferenceMarksChangedPixels(t *testing.T) {
	dir := t.TempDir()
	goldenPath := filepath.Join(dir, "golden.png")
	actualPath := filepath.Join(dir, "actual.png")
	outPath := filepath.Join(dir, "diff.png")

	greyPNG(t, goldenPath, 3, 3, false)
	greyPNG(t, actualPath, 3, 3, true) // one pixel apart, at (0, 0)

	if err := WriteDifference(goldenPath, actualPath, outPath); err != nil {
		t.Fatalf("WriteDifference() error = %v", err)
	}

	if _, err := os.Stat(outPath); err != nil {
		t.Fatalf("stat %s: %v", outPath, err)
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("opening %s: %v", outPath, err)
	}
	defer func() { _ = f.Close() }()
	diff, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decoding %s: %v", outPath, err)
	}

	bounds := diff.Bounds()
	marked := 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := diff.At(x, y).RGBA()
			if r == 0 && g == 0 && b == 0 {
				marked++
			}
		}
	}
	if marked != 1 {
		t.Fatalf("difference image has %d marked (black) pixels, want 1", marked)
	}
}

func TestWriteDifferenceRejectsDifferentSizes(t *testing.T) {
	dir := t.TempDir()
	goldenPath := filepath.Join(dir, "golden.png")
	actualPath := filepath.Join(dir, "actual.png")
	outPath := filepath.Join(dir, "diff.png")

	greyPNG(t, goldenPath, 2, 2, false)
	greyPNG(t, actualPath, 3, 3, false)

	err := WriteDifference(goldenPath, actualPath, outPath)
	if err == nil {
		t.Fatal("WriteDifference() error = nil, want an error naming both sizes")
	}
	if !strings.Contains(err.Error(), "2x2") || !strings.Contains(err.Error(), "3x3") {
		t.Fatalf("WriteDifference() error = %q, want it to name both sizes (2x2 and 3x3)", err)
	}
}
