// Package guard holds repository-consistency checks that run as tests,
// keeping documents and generated artifacts honest against each other.
package guard

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// versionHeadingPattern matches a well-formed Keep a Changelog release
// heading: "## [MAJOR.MINOR.PATCH] - YYYY-MM-DD". It only constrains digit
// count per group, not their range, so a heading that matches still needs
// its date checked with time.Parse before it is accepted.
var versionHeadingPattern = regexp.MustCompile(`^## \[(\d+\.\d+\.\d+)\] - (\d{4}-\d{2}-\d{2})$`)

// setextUnderlinePattern matches a line that consists solely of a run of
// "=" or of "-" characters (with up to three leading spaces and only
// trailing white space allowed). Markdown reads such a line, together with
// the non-blank line right above it, as a setext heading — a second way to
// write a level-one ("=") or level-two ("-") heading that an ATX-only scan
// for a leading "##" would never see.
var setextUnderlinePattern = regexp.MustCompile(`^ {0,3}(=+|-+)[ \t]*$`)

// fenceOpenPattern matches the line that opens a fenced code block: a run
// of three or more backticks or tildes, after up to three leading spaces.
var fenceOpenPattern = regexp.MustCompile("^ {0,3}(`{3,}|~{3,})")

// fenceClosePattern reads a candidate closing line the same way fenceOpenPattern
// reads an opening one, so its character and length can be compared
// against the fence it might close.
var fenceClosePattern = regexp.MustCompile("^ {0,3}(`+|~+)[ \t]*$")

// openingFence reports the fence character and length of line, and whether
// it opens a fenced code block at all.
func openingFence(line string) (char byte, length int, opens bool) {
	match := fenceOpenPattern.FindStringSubmatch(line)
	if match == nil {
		return 0, 0, false
	}
	run := match[1]
	return run[0], len(run), true
}

// closesFence reports whether line closes a fence opened with char and
// length: the same character, run at least as long, with nothing but
// white space around it.
func closesFence(line string, char byte, length int) bool {
	match := fenceClosePattern.FindStringSubmatch(line)
	if match == nil {
		return false
	}
	run := match[1]
	return run[0] == char && len(run) >= length
}

// isLevelTwoHeading reports whether line is a level-two Markdown heading:
// after removing leading and trailing white space it starts with "##" and
// its third character is not "#". This is deliberately loose about what
// follows "##" so that a heading with odd interior spacing (two spaces, a
// tab) is still caught by this check, rather than silently falling through
// as ordinary prose.
func isLevelTwoHeading(line string) bool {
	trimmed := strings.TrimSpace(line)
	return strings.HasPrefix(trimmed, "##") && !strings.HasPrefix(trimmed, "###")
}

// compareVersions orders two dotted-triple version strings by comparing
// their major, minor, and patch numbers in turn. It returns a negative
// number when a is lower than b, zero when they are equal, and a positive
// number when a is higher. Both arguments must already have the shape
// versionHeadingPattern captures, a run of digits per part, so each part
// always parses.
func compareVersions(a, b string) int {
	aParts := strings.SplitN(a, ".", 3)
	bParts := strings.SplitN(b, ".", 3)
	for i := 0; i < 3; i++ {
		an, _ := strconv.Atoi(aParts[i])
		bn, _ := strconv.Atoi(bParts[i])
		if an != bn {
			return an - bn
		}
	}
	return 0
}

// NewestVersionHeading returns the version named by the first well-formed
// release heading in changelog, scanning from the top. Keep a Changelog
// lists releases newest first, so the first match is the newest version.
// "## [Unreleased]" is allowed once, and only before every version heading;
// a second one, or one that stands after a version heading, is reported.
// Every level-two heading of the changelog is checked, in order, in both of
// Markdown's forms — "##" (ATX) and a non-blank line followed by a line of
// dashes (setext) — all the way to the end of the file, rather than
// stopping at the first one accepted: one that is not exactly
// "## [Unreleased]" and does not match the strict version form is reported
// as an error immediately, so a malformed heading cannot hide behind an
// older, well-formed one further down. Every version heading is also
// compared with the one above it: one that is not lower is reported,
// whether it repeats that version or names a newer one out of place. A line
// inside a fenced code block is never read as a heading of any of these
// forms, so a worked example of the changelog's own format can sit inside
// one without confusing the scan.
func NewestVersionHeading(changelog string) (version string, found bool, err error) {
	scanner := bufio.NewScanner(strings.NewReader(changelog))
	lineNum := 0
	var prevLine string
	havePrev := false
	var prevVersion string
	havePrevVersion := false
	haveUnreleased := false
	inFence := false
	var fenceChar byte
	var fenceLen int

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		if inFence {
			if closesFence(line, fenceChar, fenceLen) {
				inFence = false
			}
			havePrev = false
			continue
		}
		if char, length, opens := openingFence(line); opens {
			inFence = true
			fenceChar = char
			fenceLen = length
			havePrev = false
			continue
		}

		if havePrev && strings.TrimSpace(prevLine) != "" && setextUnderlinePattern.MatchString(line) {
			return "", false, fmt.Errorf("malformed changelog heading on line %d: %q", lineNum, line)
		}
		prevLine = line
		havePrev = true

		if match := versionHeadingPattern.FindStringSubmatch(line); match != nil {
			// The pattern only checks digit count, not range, so a
			// shape like "2026-99-99" matches syntactically; parsing
			// catches an impossible calendar date and treats it the same
			// as any other malformed heading.
			if _, parseErr := time.Parse("2006-01-02", match[2]); parseErr != nil {
				return "", false, fmt.Errorf("malformed changelog heading on line %d: %q", lineNum, line)
			}
			current := match[1]
			if havePrevVersion && compareVersions(current, prevVersion) >= 0 {
				return "", false, fmt.Errorf("changelog heading out of order on line %d: %q", lineNum, line)
			}
			prevVersion = current
			havePrevVersion = true
			if !found {
				version = current
				found = true
			}
			continue
		}

		if line == "## [Unreleased]" {
			if haveUnreleased {
				return "", false, fmt.Errorf("changelog holds a second heading of the unreleased changes on line %d: %q", lineNum, line)
			}
			if havePrevVersion {
				return "", false, fmt.Errorf("the heading of the unreleased changes stands after a version heading on line %d: %q", lineNum, line)
			}
			haveUnreleased = true
			continue
		}

		if isLevelTwoHeading(line) {
			return "", false, fmt.Errorf("malformed changelog heading on line %d: %q", lineNum, line)
		}
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return "", false, scanErr
	}
	return version, found, nil
}
