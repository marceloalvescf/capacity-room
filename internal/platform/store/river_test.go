package store_test

import (
	"os"
	"strings"
	"testing"

	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
)

// stripTemplate removes the schema placeholder River leaves in its SQL.
func stripTemplate(sql string) string {
	return strings.ReplaceAll(sql, "/* TEMPLATE: schema */", "")
}

// gooseBlocks returns the text between each StatementBegin and its
// StatementEnd, in file order.
func gooseBlocks(t *testing.T, text string) []string {
	t.Helper()
	var blocks []string
	var current []string
	inside := false
	for _, line := range strings.Split(text, "\n") {
		switch strings.TrimSpace(line) {
		case "-- +goose StatementBegin":
			if inside {
				t.Fatal("StatementBegin inside a block")
			}
			inside = true
			current = nil
		case "-- +goose StatementEnd":
			if !inside {
				t.Fatal("StatementEnd outside a block")
			}
			inside = false
			blocks = append(blocks, strings.Join(current, "\n"))
		default:
			if inside {
				current = append(current, line)
			}
		}
	}
	if inside {
		t.Fatal("a block is not closed")
	}
	return blocks
}

func TestRiverMigrationMatchesTheLibrary(t *testing.T) {
	data, err := os.ReadFile("migrations/00003_river.sql")
	if err != nil {
		t.Fatal(err)
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(nil), nil)
	if err != nil {
		t.Fatal(err)
	}
	var versions []int
	var up, down []string
	for _, m := range migrator.AllVersions() {
		if m.Version == 1 {
			continue
		}
		versions = append(versions, m.Version)
		up = append(up, strings.TrimSpace(stripTemplate(m.SQLUp)))
		down = append(down, strings.TrimSpace(stripTemplate(m.SQLDown)))
	}
	if len(versions) != 7 || versions[0] != 2 || versions[6] != 8 {
		t.Fatalf("library versions = %v, want 2 to 8", versions)
	}

	blocks := gooseBlocks(t, string(data))
	if len(blocks) != 2*len(versions) {
		t.Fatalf("blocks = %d, want %d", len(blocks), 2*len(versions))
	}
	for i, version := range versions {
		if got := strings.TrimSpace(blocks[i]); got != up[i] {
			t.Errorf("Up block %d differs from version %d of the library", i, version)
		}
	}
	for i := range versions {
		j := len(versions) - 1 - i
		if got := strings.TrimSpace(blocks[len(versions)+i]); got != down[j] {
			t.Errorf("Down block %d differs from version %d of the library", i, versions[j])
		}
	}
}
