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
var BuildTags = []string{"toolchain"}

// buildConstraintLine returns the first "//go:build" line in source, if
// any. Per the language spec such a line must appear only among the
// blank lines and other line comments that precede the package clause, so
// scanning stops at the first line that is neither.
func buildConstraintLine(source []byte) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(source))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "//") {
			break
		}
		if constraint.IsGoBuild(line) {
			return line, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
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

// UndeclaredBuildTags reads the //go:build line of the file named filename,
// if any, and returns one entry for every tag it names that is not in
// declared, sorted. A file with no such line returns nothing. A tag that
// happens to be a Go-recognized operating system or architecture (linux,
// darwin, arm64, and so on) is not special-cased: it is reported unless it
// is declared, the same as any other tag.
func UndeclaredBuildTags(filename string, source []byte, declared []string) ([]string, error) {
	line, err := buildConstraintLine(source)
	if err != nil {
		return nil, err
	}
	if line == "" {
		return nil, nil
	}

	expr, err := constraint.Parse(line)
	if err != nil {
		return nil, err
	}

	declaredSet := make(map[string]bool, len(declared))
	for _, tag := range declared {
		declaredSet[tag] = true
	}

	tags := make(map[string]bool)
	collectTags(expr, tags)

	var undeclared []string
	for tag := range tags {
		if declaredSet[tag] {
			continue
		}
		undeclared = append(undeclared, fmt.Sprintf("%s: build tag %q is not declared in guard.BuildTags", filename, tag))
	}
	sort.Strings(undeclared)
	return undeclared, nil
}
