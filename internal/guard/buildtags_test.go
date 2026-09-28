package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

const noBuildLineSource = `package sample

func helper() {}
`

func TestUndeclaredBuildTagsNone(t *testing.T) {
	got, err := UndeclaredBuildTags("plain.go", []byte(noBuildLineSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want empty", got)
	}
}

const toolchainBuildLineSource = `//go:build toolchain

package sample

func helper() {}
`

func TestUndeclaredBuildTagsDeclared(t *testing.T) {
	got, err := UndeclaredBuildTags("tagged.go", []byte(toolchainBuildLineSource), []string{"toolchain"})
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want empty", got)
	}
}

const integrationBuildLineSource = `//go:build integration

package sample

func helper() {}
`

func TestUndeclaredBuildTagsUnknown(t *testing.T) {
	got, err := UndeclaredBuildTags("tagged.go", []byte(integrationBuildLineSource), []string{"toolchain"})
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`tagged.go: build tag "integration" is not declared in guard.BuildTags`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const expressionBuildLineSource = `//go:build toolchain && !slow

package sample

func helper() {}
`

func TestUndeclaredBuildTagsExpression(t *testing.T) {
	got, err := UndeclaredBuildTags("tagged.go", []byte(expressionBuildLineSource), []string{"toolchain"})
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`tagged.go: build tag "slow" is not declared in guard.BuildTags`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const malformedBuildLineSource = `//go:build &&

package sample

func helper() {}
`

func TestUndeclaredBuildTagsMalformed(t *testing.T) {
	_, err := UndeclaredBuildTags("bad.go", []byte(malformedBuildLineSource), BuildTags)
	if err == nil {
		t.Fatalf("UndeclaredBuildTags returned no error, want error")
	}
}

const legacyOnlyBuildLineSource = `// +build hidden

package sample

func helper() {}
`

func TestUndeclaredBuildTagsLegacyOnly(t *testing.T) {
	got, err := UndeclaredBuildTags("legacy.go", []byte(legacyOnlyBuildLineSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`legacy.go: legacy "// +build" line is forbidden; use //go:build`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const legacyAlongsideGoBuildSource = `//go:build template
// +build template

package sample

func helper() {}
`

func TestUndeclaredBuildTagsLegacyAlongsideGoBuild(t *testing.T) {
	got, err := UndeclaredBuildTags("both.go", []byte(legacyAlongsideGoBuildSource), []string{"template"})
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`both.go: legacy "// +build" line is forbidden; use //go:build`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const twoLegacyBuildLinesSource = `// +build a
// +build b

package sample

func helper() {}
`

func TestUndeclaredBuildTagsTwoLegacyLines(t *testing.T) {
	got, err := UndeclaredBuildTags("two.go", []byte(twoLegacyBuildLinesSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("UndeclaredBuildTags = %v, want 2 entries", got)
	}
	want := `two.go: legacy "// +build" line is forbidden; use //go:build`
	if got[0] != want || got[1] != want {
		t.Fatalf("UndeclaredBuildTags = %v, want both entries %q", got, want)
	}
}

const legacyLookalikesAfterPackageSource = `package sample

// see +build docs
func helper() {
	s := "// +build x"
	_ = s
}
`

func TestUndeclaredBuildTagsIgnoresLegacyLookalikes(t *testing.T) {
	got, err := UndeclaredBuildTags("lookalike.go", []byte(legacyLookalikesAfterPackageSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want empty", got)
	}
}

// The remaining cases cover F-24: a block comment ahead of a build
// constraint line. The hand-written scanner this package used to have
// stopped at the first line that was neither blank nor "//"-prefixed, so a
// block comment hid every constraint line behind it from the guard, even
// though the toolchain honours it (go/build's leading-comment reader treats
// a //go:build line as controlling wherever it appears in the header,
// block comments included). Verified against the real toolchain with
// `go list -f '{{.GoFiles}} {{.IgnoredGoFiles}}'`, with and without
// `-tags=hidden`, on scratch packages built from each row.

const blockCommentThenGoBuildSource = `/* licence */
//go:build hidden

package sample

func helper() {}
`

func TestUndeclaredBuildTagsBlockCommentThenGoBuild(t *testing.T) {
	got, err := UndeclaredBuildTags("blockthengo.go", []byte(blockCommentThenGoBuildSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`blockthengo.go: build tag "hidden" is not declared in guard.BuildTags`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const multiLineBlockCommentThenGoBuildSource = `/*
multi
line
licence
*/

//go:build hidden

package sample

func helper() {}
`

func TestUndeclaredBuildTagsMultiLineBlockCommentThenGoBuild(t *testing.T) {
	got, err := UndeclaredBuildTags("multiblock.go", []byte(multiLineBlockCommentThenGoBuildSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`multiblock.go: build tag "hidden" is not declared in guard.BuildTags`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const blockCommentThenLegacyBuildSource = `/* licence */
// +build hidden

package sample

func helper() {}
`

func TestUndeclaredBuildTagsBlockCommentThenLegacyBuild(t *testing.T) {
	got, err := UndeclaredBuildTags("blockthenlegacy.go", []byte(blockCommentThenLegacyBuildSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	want := []string{`blockthenlegacy.go: legacy "// +build" line is forbidden; use //go:build`}
	if len(got) != 1 || got[0] != want[0] {
		t.Fatalf("UndeclaredBuildTags = %v, want %v", got, want)
	}
}

const copyrightBlockCommentThenGoBuildDeclaredSource = `// Copyright line

/* block */

//go:build template

package sample

func helper() {}
`

func TestUndeclaredBuildTagsCopyrightBlockCommentThenGoBuildDeclared(t *testing.T) {
	got, err := UndeclaredBuildTags("copyrightblock.go", []byte(copyrightBlockCommentThenGoBuildDeclaredSource), []string{"template"})
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want empty", got)
	}
}

// The constraint text sits INSIDE the block comment itself, so it is not a
// "//"-prefixed line at all. Go does not read a constraint that appears
// inside a block comment, confirmed with `go list`: the file built with
// and without -tags=hidden.
const constraintInsideBlockCommentSource = `/* //go:build hidden */

package sample

func helper() {}
`

func TestUndeclaredBuildTagsConstraintInsideBlockComment(t *testing.T) {
	got, err := UndeclaredBuildTags("insideblock.go", []byte(constraintInsideBlockCommentSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want empty", got)
	}
}

// A comment after the package clause is not part of the header, whatever it
// says. Confirmed with `go list`: the file built with and without
// -tags=hidden.
const goBuildAfterPackageClauseSource = `package sample

//go:build hidden

func helper() {}
`

func TestUndeclaredBuildTagsGoBuildAfterPackageClause(t *testing.T) {
	got, err := UndeclaredBuildTags("afterpackage.go", []byte(goBuildAfterPackageClauseSource), BuildTags)
	if err != nil {
		t.Fatalf("UndeclaredBuildTags returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want empty", got)
	}
}

const notGoSource = `this is not go`

func TestUndeclaredBuildTagsNotGo(t *testing.T) {
	got, err := UndeclaredBuildTags("notgo.go", []byte(notGoSource), BuildTags)
	if err == nil {
		t.Fatalf("UndeclaredBuildTags returned no error, want error")
	}
	if len(got) != 0 {
		t.Fatalf("UndeclaredBuildTags = %v, want no entries alongside the error", got)
	}
}

// TestRepositoryDeclaresEveryBuildTag walks every .go file below the module
// root, the same walk TestRepositoryHasNoSkippedTest uses, and fails if any
// file names a build tag that guard.BuildTags does not declare.
func TestRepositoryDeclaresEveryBuildTag(t *testing.T) {
	root := moduleRoot(t)

	var violations []string
	visited := walkGoFiles(t, root, func(rel string, source []byte) error {
		found, err := UndeclaredBuildTags(rel, source, BuildTags)
		if err != nil {
			return fmt.Errorf("parsing %s: %w", rel, err)
		}
		violations = append(violations, found...)
		return nil
	})
	if visited == 0 {
		t.Fatalf("no files were read; a scan that finds nothing protects nothing")
	}
	if len(violations) != 0 {
		t.Fatalf("undeclared build tags found:\n%s", strings.Join(violations, "\n"))
	}
}

// TestMakefileBuildTagsMatchDeclaredTags checks that the Makefile's own
// BUILD_TAGS variable, which golangci-lint's second pass uses to see every
// gated file, names exactly the tags guard.BuildTags declares. Otherwise a
// tag could be added to one and forgotten in the other, and either the
// linter or this repository-import scan would silently stop covering a
// gated file.
func TestMakefileBuildTagsMatchDeclaredTags(t *testing.T) {
	root := moduleRoot(t)
	path := filepath.Join(root, "Makefile")

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}

	const prefix = "BUILD_TAGS := "
	var value string
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			value = strings.TrimPrefix(line, prefix)
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("%s: no line starting with %q", path, "BUILD_TAGS :=")
	}

	got := strings.Split(value, ",")
	sort.Strings(got)
	want := append([]string(nil), BuildTags...)
	sort.Strings(want)

	if len(got) != len(want) {
		t.Fatalf("Makefile BUILD_TAGS = %v, want %v (guard.BuildTags)", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Makefile BUILD_TAGS = %v, want %v (guard.BuildTags)", got, want)
		}
	}
}
