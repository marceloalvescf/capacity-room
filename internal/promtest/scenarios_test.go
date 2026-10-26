package promtest

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// TestScenariosHoldSeven checks that Scenarios holds seven entries, five
// under the folder prometheus and two under prometheus-own.
func TestScenariosHoldSeven(t *testing.T) {
	if len(Scenarios) != 7 {
		t.Fatalf("len(Scenarios) = %d, want 7", len(Scenarios))
	}

	var names []string
	prometheus, own := 0, 0
	for _, s := range Scenarios {
		names = append(names, s.Name)
		switch s.Folder {
		case "prometheus":
			prometheus++
		case "prometheus-own":
			own++
		default:
			t.Fatalf("scenario %q has folder %q, want %q or %q", s.Name, s.Folder, "prometheus", "prometheus-own")
		}
	}
	if prometheus != 5 {
		t.Errorf("%d scenarios hold the folder %q, want 5: %v", prometheus, "prometheus", names)
	}
	if own != 2 {
		t.Errorf("%d scenarios hold the folder %q, want 2: %v", own, "prometheus-own", names)
	}
}

// TestPathFollowsTheFolder checks that Path places edge-values under
// testdata/prometheus-own, its own folder, rather than testdata/prometheus.
func TestPathFollowsTheFolder(t *testing.T) {
	path, err := scenarioPath("edge-values")
	if err != nil {
		t.Fatalf("scenarioPath(%q): %v", "edge-values", err)
	}
	want := filepath.Join("testdata", "prometheus-own")
	if !strings.Contains(path, want) {
		t.Fatalf("scenarioPath(%q) = %q, want it to hold %q", "edge-values", path, want)
	}
}

// checksumLine matches one line of the SHA-256 block of
// testdata/prometheus/README.md: a 64 character hex digest, whitespace, and
// the file name it names.
var checksumLine = regexp.MustCompile(`^([0-9a-f]{64})\s+(\S+)$`)

// readmeChecksums reads the fenced code block under the "## SHA-256" heading
// of path and returns the checksum it holds for every file name.
func readmeChecksums(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("opening %s: %v", path, err)
	}
	defer func() { _ = file.Close() }()

	checksums := make(map[string]string)
	inBlock := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "```") {
			inBlock = !inBlock
			continue
		}
		if !inBlock {
			continue
		}
		m := checksumLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		checksums[m[2]] = m[1]
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if len(checksums) == 0 {
		t.Fatalf("%s: no checksum line found", path)
	}
	return checksums
}

// sha256File returns the lower-case hex SHA-256 of the file at path.
func sha256File(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// TestOriginalScenariosAreUntouched checks every one of the five files of
// testdata/prometheus against the checksum testdata/prometheus/README.md
// names for it, so that a change to a byte of a recorded answer is caught
// without holding a checksum here as a literal of its own.
func TestOriginalScenariosAreUntouched(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	dir := filepath.Join(root, "testdata", "prometheus")
	readmePath := filepath.Join(dir, "README.md")
	checksums := readmeChecksums(t, readmePath)

	if len(checksums) != 5 {
		t.Fatalf("%s names %d checksums, want 5: %v", readmePath, len(checksums), checksums)
	}

	for name, want := range checksums {
		got := sha256File(t, filepath.Join(dir, name))
		if got != want {
			t.Errorf("%s: sha256 = %s, want %s (from %s)", name, got, want, readmePath)
		}
	}
}

// TestOwnScenariosMatchTheirChecksums checks both files of
// testdata/prometheus-own against the checksum its README names for each.
func TestOwnScenariosMatchTheirChecksums(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	dir := filepath.Join(root, "testdata", "prometheus-own")
	readmePath := filepath.Join(dir, "README.md")
	checksums := readmeChecksums(t, readmePath)

	if len(checksums) != 2 {
		t.Fatalf("%s names %d checksums, want 2: %v", readmePath, len(checksums), checksums)
	}

	for name, want := range checksums {
		got := sha256File(t, filepath.Join(dir, name))
		if got != want {
			t.Errorf("%s: sha256 = %s, want %s (from %s)", name, got, want, readmePath)
		}
	}
}

// TestPathOfARangeScenarioReadsItsFixture checks that a range scenario
// resolves to the recorded answers of the scenario it borrows, and that no
// scenario of the original's list is a range.
func TestPathOfARangeScenarioReadsItsFixture(t *testing.T) {
	path, err := scenarioPath("direct-complete-range")
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if want := filepath.Join("testdata", "prometheus", "direct-complete.json"); !strings.HasSuffix(path, want) {
		t.Fatalf("Path = %q, want it to end with %q", path, want)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load: %v", err)
	}
}

func mustPath(t *testing.T, name string) string {
	t.Helper()
	path, err := scenarioPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func mustAddedPath(t *testing.T, name string) string {
	t.Helper()
	path, err := AddedPath(name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestEveryScenarioHasItsAddedAnswers(t *testing.T) {
	for _, s := range slices.Concat(Scenarios, RangeScenarios) {
		if _, err := LoadScenario(s.Name); err != nil {
			t.Errorf("LoadScenario(%q): %v", s.Name, err)
		}
	}
}

func TestAddedAnswersComeFirst(t *testing.T) {
	recorded, err := Load(mustPath(t, "direct-complete"))
	if err != nil {
		t.Fatal(err)
	}
	added, err := Load(mustAddedPath(t, "direct-complete"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := LoadScenario("direct-complete")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Requests) != len(added.Requests)+len(recorded.Requests) {
		t.Fatalf("requests = %d, want %d", len(got.Requests), len(added.Requests)+len(recorded.Requests))
	}
	if !reflect.DeepEqual(got.Requests[len(added.Requests):], recorded.Requests) {
		t.Fatal("the recorded requests do not follow the added ones unchanged")
	}
}

// TestAddedAnswersMatchTheirChecksums checks the seven files of
// testdata/prometheus-added against the checksum its README names for each,
// and refuses a file of the folder that the README does not name.
func TestAddedAnswersMatchTheirChecksums(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatalf("moduleRoot: %v", err)
	}
	dir := filepath.Join(root, "testdata", "prometheus-added")
	readmePath := filepath.Join(dir, "README.md")
	checksums := readmeChecksums(t, readmePath)

	if len(checksums) != 7 {
		t.Fatalf("%s names %d checksums, want 7: %v", readmePath, len(checksums), checksums)
	}

	for name, want := range checksums {
		got := sha256File(t, filepath.Join(dir, name))
		if got != want {
			t.Errorf("%s: sha256 = %s, want %s (from %s)", name, got, want, readmePath)
		}
	}

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if _, ok := checksums[filepath.Base(file)]; !ok {
			t.Errorf("%s is not named by %s", filepath.Base(file), readmePath)
		}
	}
}
