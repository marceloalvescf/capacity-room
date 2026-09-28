package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	kcr "github.com/marceloalvescf/k8s-capacity-report"
)

// moduleRoot walks up from the current working directory to the first
// folder holding go.mod. Later guard tests in this package also need to
// locate real repository files, so the walk lives here rather than in a
// single test.
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

func TestNewestVersionHeadingNone(t *testing.T) {
	changelog := "## [Unreleased]\n"

	version, found, err := NewestVersionHeading(changelog)
	if err != nil {
		t.Fatalf("NewestVersionHeading returned error: %v", err)
	}
	if found {
		t.Fatalf("NewestVersionHeading found = true, want false")
	}
	if version != "" {
		t.Fatalf("NewestVersionHeading version = %q, want empty", version)
	}
}

func TestNewestVersionHeadingFirstWins(t *testing.T) {
	changelog := "## [Unreleased]\n## [0.2.0] - 2026-10-01\n## [0.1.0] - 2026-09-30\n"

	version, found, err := NewestVersionHeading(changelog)
	if err != nil {
		t.Fatalf("NewestVersionHeading returned error: %v", err)
	}
	if !found {
		t.Fatalf("NewestVersionHeading found = false, want true")
	}
	if version != "0.2.0" {
		t.Fatalf("NewestVersionHeading version = %q, want %q", version, "0.2.0")
	}
}

func TestNewestVersionHeadingMalformed(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"missing date", "## [0.2.0]"},
		{"short version", "## [0.2] - 2026-10-01"},
		{"single digit day", "## [0.2.0] - 2026-10-1"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Put the malformed heading on line 2 so the test also proves
			// line numbers are tracked, not hard-coded to 1.
			changelog := "## [Unreleased]\n" + tc.line + "\n"

			_, _, err := NewestVersionHeading(changelog)
			if err == nil {
				t.Fatalf("NewestVersionHeading(%q) returned no error, want error", changelog)
			}
			want := fmt.Sprintf("malformed changelog heading on line 2: %q", tc.line)
			if err.Error() != want {
				t.Fatalf("NewestVersionHeading(%q) error = %q, want %q", changelog, err.Error(), want)
			}
		})
	}
}

func TestNewestVersionHeadingIgnoresDeeperHeadings(t *testing.T) {
	changelog := "### [0.9.9] - 2026-01-01\n## [0.1.0] - 2026-09-30\n"

	version, found, err := NewestVersionHeading(changelog)
	if err != nil {
		t.Fatalf("NewestVersionHeading returned error: %v", err)
	}
	if !found {
		t.Fatalf("NewestVersionHeading found = false, want true")
	}
	if version != "0.1.0" {
		t.Fatalf("NewestVersionHeading version = %q, want %q", version, "0.1.0")
	}
}

func TestChangelogAgreesWithVersion(t *testing.T) {
	root := moduleRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatalf("reading CHANGELOG.md: %v", err)
	}

	version, found, err := NewestVersionHeading(string(raw))
	if err != nil {
		t.Fatalf("NewestVersionHeading returned error: %v", err)
	}

	if found {
		if version != kcr.Version() {
			t.Fatalf("CHANGELOG.md newest version = %q, want %q (kcr.Version())", version, kcr.Version())
		}
		return
	}

	if kcr.Version() != "0.0.0" {
		t.Fatalf("CHANGELOG.md has no version heading, but kcr.Version() = %q, want %q", kcr.Version(), "0.0.0")
	}
}
