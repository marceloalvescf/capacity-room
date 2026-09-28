package guard

import (
	"fmt"
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
