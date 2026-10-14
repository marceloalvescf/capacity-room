package guard

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestOldNameLinesFindsEveryForm(t *testing.T) {
	lines := []string{
		"run k8s-capacity-report --check",              // 1: the command
		"export KCR_DATABASE_DSN=x",                    // 2: a variable
		"VITE_KCR_VERSION",                             // 3: a variable inside a longer one
		`getItem("kcr.theme")`,                         // 4: a storage key
		"mount /etc/kcr",                               // 5: a path
		"POSTGRES_USER: kcr",                           // 6: a word alone
		"name: kcr-e2e",                                // 7: a prefix
		"package kcr",                                  // 8: a package
		"capacity-room and CAPACITYROOM_X",             // 9: the new names
		"backcrawl, skcr2, kcrypt",                     // 10: kcr inside a word
		"see 2026-09-27-k8s-capacity-report-design.md", // 11: the design keeps its file name
	}
	got := OldNameLines("sample.txt", []byte(strings.Join(lines, "\n")))
	want := []int{1, 2, 3, 4, 5, 6, 7, 8}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("OldNameLines = %v, want %v", got, want)
	}
}

func TestOldNameLinesSkipsBinary(t *testing.T) {
	if got := OldNameLines("font.ttf", []byte("\x00\x01kcr_x")); got != nil {
		t.Fatalf("OldNameLines = %v, want nil", got)
	}
}

func TestOldNameLinesSkipsTheChangelogOf1x(t *testing.T) {
	text := strings.Join([]string{
		"# Changelog",                           // 1
		"## [Unreleased]",                       // 2
		"- KCR_ in unreleased",                  // 3: found
		"## [2.0.0] - 2026-10-14",               // 4
		"- The command was k8s-capacity-report", // 5: found
		"## [1.6.0] - 2026-10-14",               // 6
		"- KCR_TOKEN",                           // 7: history
		"## [0.1.0] - 2026-10-02",               // 8
		"- kcr",                                 // 9: history
	}, "\n")
	got := OldNameLines("CHANGELOG.md", []byte(text))
	if want := []int{3, 5}; !reflect.DeepEqual(got, want) {
		t.Fatalf("OldNameLines = %v, want %v", got, want)
	}
}

func TestRepositoryHoldsNoOldName(t *testing.T) {
	root := moduleRoot(t)
	for _, rel := range repositoryGoFiles(t, root) {
		if oldNameHistory(rel) {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		for _, line := range OldNameLines(rel, content) {
			t.Errorf("%s:%d holds a name of 1.x", rel, line)
		}
	}
}
