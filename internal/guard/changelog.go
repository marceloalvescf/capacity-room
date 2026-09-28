// Package guard holds repository-consistency checks that run as tests,
// keeping documents and generated artifacts honest against each other.
package guard

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

// versionHeadingPattern matches a well-formed Keep a Changelog release
// heading: "## [MAJOR.MINOR.PATCH] - YYYY-MM-DD".
var versionHeadingPattern = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - \d{4}-\d{2}-\d{2}$`)

// versionHeadingPrefix matches any line that looks like it is attempting a
// version heading (a top-level heading opening on a digit), whether or not
// it is well-formed.
var versionHeadingPrefix = regexp.MustCompile(`^## \[\d`)

// NewestVersionHeading returns the version named by the first well-formed
// release heading in changelog, scanning from the top. Keep a Changelog
// lists releases newest first, so the first match is the newest version.
// "## [Unreleased]" is allowed and ignored. A line that opens like a
// version heading but does not match the full form is reported as an
// error immediately, rather than skipped, so a malformed heading cannot
// hide behind an older, well-formed one further down.
func NewestVersionHeading(changelog string) (version string, found bool, err error) {
	scanner := bufio.NewScanner(strings.NewReader(changelog))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if match := versionHeadingPattern.FindStringSubmatch(line); match != nil {
			return match[1], true, nil
		}

		if line == "## [Unreleased]" {
			continue
		}

		if versionHeadingPrefix.MatchString(line) {
			return "", false, fmt.Errorf("malformed changelog heading on line %d: %q", lineNum, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", false, err
	}
	return "", false, nil
}
