// Package guard holds repository-consistency checks that run as tests,
// keeping documents and generated artifacts honest against each other.
package guard

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// versionHeadingPattern matches a well-formed Keep a Changelog release
// heading: "## [MAJOR.MINOR.PATCH] - YYYY-MM-DD". It only constrains digit
// count per group, not their range, so a heading that matches still needs
// its date checked with time.Parse before it is accepted.
var versionHeadingPattern = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})$`)

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
			// The pattern only checks digit count, not range, so a
			// shape like "2026-99-99" matches syntactically; parsing
			// catches an impossible calendar date and treats it the same
			// as any other malformed heading.
			if _, err := time.Parse("2006-01-02", match[2]); err != nil {
				return "", false, fmt.Errorf("malformed changelog heading on line %d: %q", lineNum, line)
			}
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
