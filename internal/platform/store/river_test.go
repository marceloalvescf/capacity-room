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

// gooseFile is a goose migration read as River's blocks: the text between
// each StatementBegin and its StatementEnd, in file order, and the number of
// blocks that come before the Up and the Down annotations.
type gooseFile struct {
	blocks           []string
	upAt, downAt     int
	upSeen, downSeen int
}

// readGoose splits text into its blocks. Outside a block it accepts only the
// Up and Down annotations, once each, blank lines, and comments, so no
// statement runs that the comparison with the library does not see.
func readGoose(t *testing.T, text string) gooseFile {
	t.Helper()
	var f gooseFile
	var current []string
	inside := false
	for n, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch trimmed {
		case "-- +goose StatementBegin":
			if inside {
				t.Fatalf("line %d: StatementBegin inside a block", n+1)
			}
			inside = true
			current = nil
			continue
		case "-- +goose StatementEnd":
			if !inside {
				t.Fatalf("line %d: StatementEnd outside a block", n+1)
			}
			inside = false
			f.blocks = append(f.blocks, strings.Join(current, "\n"))
			continue
		}
		if inside {
			current = append(current, line)
			continue
		}
		switch {
		case trimmed == "-- +goose Up":
			f.upAt, f.upSeen = len(f.blocks), f.upSeen+1
		case trimmed == "-- +goose Down":
			f.downAt, f.downSeen = len(f.blocks), f.downSeen+1
		case trimmed == "":
		case strings.HasPrefix(trimmed, "-- +goose"):
			t.Fatalf("line %d: annotation %q outside a block", n+1, trimmed)
		case strings.HasPrefix(trimmed, "--"):
		default:
			t.Fatalf("line %d: %q outside a block", n+1, trimmed)
		}
	}
	if inside {
		t.Fatal("a block is not closed")
	}
	return f
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

	file := readGoose(t, string(data))
	blocks := file.blocks
	if len(blocks) != 2*len(versions) {
		t.Fatalf("blocks = %d, want %d", len(blocks), 2*len(versions))
	}
	if file.upSeen != 1 || file.upAt != 0 {
		t.Fatalf("Up annotation %d times, after %d blocks; want once, before the first", file.upSeen, file.upAt)
	}
	if file.downSeen != 1 || file.downAt != len(versions) {
		t.Fatalf("Down annotation %d times, after %d blocks; want once, after %d", file.downSeen, file.downAt, len(versions))
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
