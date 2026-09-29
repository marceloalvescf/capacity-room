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

func TestNewestVersionHeadingRejectsImpossibleDate(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"month out of range", "## [1.0.0] - 2026-99-99"},
		{"day out of range", "## [1.0.0] - 2026-02-30"},
		{"zero month", "## [1.0.0] - 2026-00-10"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Put the impossible-date heading on line 2 so the test also
			// proves line numbers are tracked, not hard-coded to 1.
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

func TestNewestVersionHeadingRejectsOddSpacing(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"two spaces", "##  [1.0.0] - 2026-09-28"},
		{"tab", "##\t[1.0.0] - 2026-09-28"},
		{"leading space", " ## [1.0.0] - 2026-09-28"},
		{"trailing space", "## [1.0.0] - 2026-09-28 "},
		{"unreleased no brackets", "## Unreleased"},
		{"unreleased lowercase", "## [unreleased]"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Put the malformed heading above a well-formed, older heading,
			// so the test also proves the older heading is not reported as
			// the newest version instead of raising an error.
			changelog := tc.line + "\n## [0.9.0] - 2026-01-01\n"

			_, _, err := NewestVersionHeading(changelog)
			if err == nil {
				t.Fatalf("NewestVersionHeading(%q) returned no error, want error", changelog)
			}
			want := fmt.Sprintf("malformed changelog heading on line 1: %q", tc.line)
			if err.Error() != want {
				t.Fatalf("NewestVersionHeading(%q) error = %q, want %q", changelog, err.Error(), want)
			}
		})
	}
}

func TestNewestVersionHeadingIgnoresNonHeadingLines(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"top heading", "# Changelog"},
		{"level three", "### Added"},
		{"level four", "#### note"},
		{"prose with heading text mid-line", "See section ## [1.0.0] below."},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changelog := tc.line + "\n## [0.9.0] - 2026-01-01\n"

			version, found, err := NewestVersionHeading(changelog)
			if err != nil {
				t.Fatalf("NewestVersionHeading(%q) returned error: %v", changelog, err)
			}
			if !found || version != "0.9.0" {
				t.Fatalf("NewestVersionHeading(%q) = (%q, %v), want (\"0.9.0\", true)", changelog, version, found)
			}
		})
	}
}

func TestNewestVersionHeadingRejectsSetextUnderline(t *testing.T) {
	cases := []struct {
		name      string
		changelog string
		wantLine  int
		wantText  string
	}{
		{
			name:      "dash underline hides a newer version below",
			changelog: "[1.1.0] - 2026-10-01\n--------\n\n## [1.0.0] - 2026-09-28\n",
			wantLine:  2,
			wantText:  "--------",
		},
		{
			name:      "equals underline under a bare title",
			changelog: "Changelog\n=========\n## [1.0.0] - 2026-09-28\n",
			wantLine:  2,
			wantText:  "=========",
		},
		{
			name:      "a single dash underline",
			changelog: "1.1.0\n-\n",
			wantLine:  2,
			wantText:  "-",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := NewestVersionHeading(tc.changelog)
			if err == nil {
				t.Fatalf("NewestVersionHeading(%q) returned no error, want error", tc.changelog)
			}
			want := fmt.Sprintf("malformed changelog heading on line %d: %q", tc.wantLine, tc.wantText)
			if err.Error() != want {
				t.Fatalf("NewestVersionHeading(%q) error = %q, want %q", tc.changelog, err.Error(), want)
			}
		})
	}
}

// TestNewestVersionHeadingIgnoresDashLinesThatAreNotUnderlines proves the
// setext check does not over-reject: a thematic break, a list, a table
// separator row, and a dash-led line of prose all look superficially like a
// run of "-" or "=" characters but are not a setext underline, and must not
// stop the well-formed heading below them from being found.
func TestNewestVersionHeadingIgnoresDashLinesThatAreNotUnderlines(t *testing.T) {
	cases := []struct {
		name      string
		changelog string
	}{
		{"thematic break with blank lines around it", "\n---\n\n## [0.9.0] - 2026-01-01\n"},
		{"thematic break as the first line of the file", "---\n## [0.9.0] - 2026-01-01\n"},
		{"a list", "- item one\n- item two\n## [0.9.0] - 2026-01-01\n"},
		{"a table separator row", "| a | b |\n|---|---|\n## [0.9.0] - 2026-01-01\n"},
		{"a dash-led line of prose", "some text\n--- text\n## [0.9.0] - 2026-01-01\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			version, found, err := NewestVersionHeading(tc.changelog)
			if err != nil {
				t.Fatalf("NewestVersionHeading(%q) returned error: %v", tc.changelog, err)
			}
			if !found || version != "0.9.0" {
				t.Fatalf("NewestVersionHeading(%q) = (%q, %v), want (\"0.9.0\", true)", tc.changelog, version, found)
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

func TestChangelogReportsMalformedOlderHeading(t *testing.T) {
	changelog := "## [1.1.0] - 2026-10-01\n## 1.0.0 (2026-01-01)\n"

	_, _, err := NewestVersionHeading(changelog)
	if err == nil {
		t.Fatalf("NewestVersionHeading(%q) returned no error, want error", changelog)
	}
	want := `malformed changelog heading on line 2: "## 1.0.0 (2026-01-01)"`
	if err.Error() != want {
		t.Fatalf("NewestVersionHeading(%q) error = %q, want %q", changelog, err.Error(), want)
	}
}

func TestChangelogReportsHeadingsOutOfOrder(t *testing.T) {
	changelog := "## [1.0.0] - 2026-01-01\n## [1.1.0] - 2026-02-01\n"

	_, _, err := NewestVersionHeading(changelog)
	if err == nil {
		t.Fatalf("NewestVersionHeading(%q) returned no error, want error", changelog)
	}
	want := `changelog heading out of order on line 2: "## [1.1.0] - 2026-02-01"`
	if err.Error() != want {
		t.Fatalf("NewestVersionHeading(%q) error = %q, want %q", changelog, err.Error(), want)
	}
}

func TestChangelogReportsRepeatedVersion(t *testing.T) {
	changelog := "## [1.0.0] - 2026-01-01\n## [1.0.0] - 2026-01-01\n"

	_, _, err := NewestVersionHeading(changelog)
	if err == nil {
		t.Fatalf("NewestVersionHeading(%q) returned no error, want error", changelog)
	}
	want := `changelog heading out of order on line 2: "## [1.0.0] - 2026-01-01"`
	if err.Error() != want {
		t.Fatalf("NewestVersionHeading(%q) error = %q, want %q", changelog, err.Error(), want)
	}
}

func TestChangelogAcceptsManyGoodHeadings(t *testing.T) {
	changelog := "## [Unreleased]\n## [1.1.0] - 2026-03-01\n## [1.0.1] - 2026-02-01\n## [1.0.0] - 2026-01-01\n"

	version, found, err := NewestVersionHeading(changelog)
	if err != nil {
		t.Fatalf("NewestVersionHeading(%q) returned error: %v", changelog, err)
	}
	if !found || version != "1.1.0" {
		t.Fatalf("NewestVersionHeading(%q) = (%q, %v), want (\"1.1.0\", true)", changelog, version, found)
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
