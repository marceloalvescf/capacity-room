package guard

import (
	"bytes"
	"path"
	"regexp"
	"strings"
)

// oldNamePattern matches a name the project carried before 2.0.0: the
// command k8s-capacity-report, and kcr as a word or a prefix in any case,
// with "_" counted as a separator so that VITE_KCR_VERSION is found.
var oldNamePattern = regexp.MustCompile(`(?i)k8s-capacity-report|(^|[^a-z0-9])kcr([^a-z0-9]|$)`)

// OldNameHistory lists the paths that keep the old names as a record. A
// path ending in "/" covers its folder. The design keeps its file name and
// is not history, so it is named apart in oldNameHistory.
var OldNameHistory = []string{
	"docs/superpowers/plans/",
	"docs/superpowers/specs/",
	"docs/decisions/",
	"docs/reference/",
	"testdata/python/",
	"testdata/prometheus/",
	"internal/guard/oldname.go",
	"internal/guard/oldname_test.go",
}

// livingDesign is the design of the project. It keeps its dated file name,
// so a citation of that name is not an old name, and the file itself is
// read like any living document.
const livingDesign = "docs/superpowers/specs/2026-09-27-k8s-capacity-report-design.md"

func oldNameHistory(rel string) bool {
	if rel == livingDesign {
		return false
	}
	for _, p := range OldNameHistory {
		if rel == p || (strings.HasSuffix(p, "/") && strings.HasPrefix(rel, p)) {
			return true
		}
	}
	return false
}

// OldNameLines returns the 1-based numbers of the lines of content that hold
// an old name. A binary file, one holding a NUL byte, has none. In
// CHANGELOG.md the lines from the first heading of a version below 2.0.0 to
// the end are history and are not read.
func OldNameLines(rel string, content []byte) []int {
	if bytes.IndexByte(content, 0) >= 0 {
		return nil
	}
	var found []int
	for i, line := range strings.Split(string(content), "\n") {
		if rel == "CHANGELOG.md" {
			if m := versionHeadingPattern.FindStringSubmatch(line); m != nil && compareVersions(m[1], "2.0.0") < 0 {
				break
			}
		}
		if oldNamePattern.MatchString(strings.ReplaceAll(line, path.Base(livingDesign), "")) {
			found = append(found, i+1)
		}
	}
	return found
}
