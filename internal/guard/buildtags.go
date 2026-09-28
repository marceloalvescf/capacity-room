package guard

import (
	"bufio"
	"bytes"
	"fmt"
	"go/build/constraint"
	"sort"
	"strings"
)

// BuildTags lists every build tag this project uses to gate a file behind
// a machine-dependent or otherwise optional build. The repository import
// scan passes these to `go list` so a gated file's imports are not invisible
// to the layering rule, and the build-tag scan below rejects any tag not
// named here, so a new tag cannot go unnoticed by either.
var BuildTags = []string{"golden", "template", "toolchain"}

// buildConstraintLines returns the first "//go:build" line in source, if
// any, and every "// +build" line found alongside it. Per the language spec
// such lines must appear only among the blank lines and other line comments
// that precede the package clause, so scanning stops at the first line that
// is neither. The legacy "// +build" form is collected regardless of
// whether a "//go:build" line is also present: it is forbidden outright,
// not merely superseded.
func buildConstraintLines(source []byte) (goBuild string, plusBuild []string, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break
		}
		if goBuild == "" && constraint.IsGoBuild(line) {
			goBuild = line
		}
		if constraint.IsPlusBuild(line) {
			plusBuild = append(plusBuild, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return "", nil, err
	}
	return goBuild, plusBuild, nil
}

// collectTags walks a build constraint expression and records every tag
// name it mentions, regardless of how they combine with && , ||, or !: a
// tag named anywhere in the expression must still be declared.
func collectTags(expr constraint.Expr, tags map[string]bool) {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		tags[e.Tag] = true
	case *constraint.NotExpr:
		collectTags(e.X, tags)
	case *constraint.AndExpr:
		collectTags(e.X, tags)
		collectTags(e.Y, tags)
	case *constraint.OrExpr:
		collectTags(e.X, tags)
		collectTags(e.Y, tags)
	}
}

// UndeclaredBuildTags reads the build constraint lines of the file named
// filename and returns, sorted, one entry for every tag its //go:build line
// names that is not in declared, plus one entry for every legacy
// "// +build" line found, whatever tags it names and whether or not a
// //go:build line is also present: the legacy form is forbidden outright,
// not merely a second place to declare a tag. A file with neither kind of
// line returns nothing. A tag that happens to be a Go-recognized operating
// system or architecture (linux, darwin, arm64, and so on) is not
// special-cased: it is reported unless it is declared, the same as any
// other tag.
func UndeclaredBuildTags(filename string, source []byte, declared []string) ([]string, error) {
	goBuild, plusBuild, err := buildConstraintLines(source)
	if err != nil {
		return nil, err
	}

	var undeclared []string
	for range plusBuild {
		undeclared = append(undeclared, fmt.Sprintf("%s: legacy \"// +build\" line is forbidden; use //go:build", filename))
	}

	if goBuild != "" {
		expr, err := constraint.Parse(goBuild)
		if err != nil {
			return nil, err
		}

		declaredSet := make(map[string]bool, len(declared))
		for _, tag := range declared {
			declaredSet[tag] = true
		}

		tags := make(map[string]bool)
		collectTags(expr, tags)

		for tag := range tags {
			if declaredSet[tag] {
				continue
			}
			undeclared = append(undeclared, fmt.Sprintf("%s: build tag %q is not declared in guard.BuildTags", filename, tag))
		}
	}

	sort.Strings(undeclared)
	return undeclared, nil
}
