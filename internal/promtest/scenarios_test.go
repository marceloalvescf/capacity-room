package promtest

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestScenariosHoldEight checks that Scenarios holds eight entries, six
// under the folder prometheus and two under prometheus-own.
func TestScenariosHoldEight(t *testing.T) {
	if len(Scenarios) != 8 {
		t.Fatalf("len(Scenarios) = %d, want 8", len(Scenarios))
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
	if prometheus != 6 {
		t.Errorf("%d scenarios hold the folder %q, want 6: %v", prometheus, "prometheus", names)
	}
	if own != 2 {
		t.Errorf("%d scenarios hold the folder %q, want 2: %v", own, "prometheus-own", names)
	}
}

// TestPathFollowsTheFolder checks that Path places edge-values under
// testdata/prometheus-own, its own folder, rather than testdata/prometheus.
func TestPathFollowsTheFolder(t *testing.T) {
	path, err := Path("edge-values")
	if err != nil {
		t.Fatalf("Path(%q): %v", "edge-values", err)
	}
	want := filepath.Join("testdata", "prometheus-own")
	if !strings.Contains(path, want) {
		t.Fatalf("Path(%q) = %q, want it to hold %q", "edge-values", path, want)
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

// TestOriginalScenariosAreUntouched checks every one of the six files of
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

	if len(checksums) != 6 {
		t.Fatalf("%s names %d checksums, want 6: %v", readmePath, len(checksums), checksums)
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
