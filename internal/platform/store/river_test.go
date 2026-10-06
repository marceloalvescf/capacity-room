package store_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// gooseAnnotations are the only annotations the migrations use.
var gooseAnnotations = map[string]bool{
	"-- +goose Up":             true,
	"-- +goose Down":           true,
	"-- +goose StatementBegin": true,
	"-- +goose StatementEnd":   true,
}

// checkAnnotations refuses every comment line that holds "+goose" and is not
// exactly one of the four annotations, since goose reads such a line as an
// annotation whatever its case or spacing, and refuses an Up that is repeated
// or follows a Down, and a Down that is repeated or comes before the Up.
func checkAnnotations(text string) error {
	ups, downs := 0, 0
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(strings.TrimSpace(line), "--") || !strings.Contains(strings.ToLower(line), "+goose") {
			continue
		}
		if !gooseAnnotations[line] {
			return fmt.Errorf("line %d: %q is not an annotation the migrations use", n+1, line)
		}
		switch line {
		case "-- +goose Up":
			if ups > 0 || downs > 0 {
				return fmt.Errorf("line %d: Up annotation repeated or after Down", n+1)
			}
			ups++
		case "-- +goose Down":
			if ups == 0 || downs > 0 {
				return fmt.Errorf("line %d: Down annotation before Up or repeated", n+1)
			}
			downs++
		}
	}
	return nil
}

// readGoose splits text into its blocks after checkAnnotations accepts it.
// Outside a block it accepts only the annotations, blank lines, and comments,
// so no statement runs that the comparison with the library does not see.
func readGoose(text string) (gooseFile, error) {
	var f gooseFile
	if err := checkAnnotations(text); err != nil {
		return f, err
	}
	var current []string
	inside := false
	for n, line := range strings.Split(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		trimmed := strings.TrimSpace(line)
		switch line {
		case "-- +goose StatementBegin":
			if inside {
				return f, fmt.Errorf("line %d: StatementBegin inside a block", n+1)
			}
			inside = true
			current = nil
			continue
		case "-- +goose StatementEnd":
			if !inside {
				return f, fmt.Errorf("line %d: StatementEnd outside a block", n+1)
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
		case line == "-- +goose Up":
			f.upAt, f.upSeen = len(f.blocks), f.upSeen+1
		case line == "-- +goose Down":
			f.downAt, f.downSeen = len(f.blocks), f.downSeen+1
		case trimmed == "", strings.HasPrefix(trimmed, "--"):
		default:
			return f, fmt.Errorf("line %d: %q outside a block", n+1, trimmed)
		}
	}
	if inside {
		return f, errors.New("a block is not closed")
	}
	return f, nil
}

func TestReadGooseRefusesHiddenAnnotations(t *testing.T) {
	for _, line := range []string{
		"--+goose NO TRANSACTION",
		"--  +GOOSE NO TRANSACTION",
		"-- +goose envsub on",
	} {
		text := "-- +goose Up\n" + line + "\n-- +goose StatementBegin\nSELECT 1;\n-- +goose StatementEnd\n"
		_, err := readGoose(text)
		if err == nil || !strings.Contains(err.Error(), "line 2") {
			t.Errorf("%q: error = %v, want a refusal naming line 2", line, err)
		}
	}
}

func TestReadGooseRefusesAStatementOutsideABlock(t *testing.T) {
	text := "-- +goose Up\n-- +goose StatementBegin\nSELECT 1;\n-- +goose StatementEnd\nSELECT 1;\n" +
		"-- +goose StatementBegin\nSELECT 2;\n-- +goose StatementEnd\n"
	if _, err := readGoose(text); err == nil || !strings.Contains(err.Error(), "line 5") {
		t.Errorf("error = %v, want a refusal naming line 5", err)
	}
}

func TestReadGooseRefusesAMisplacedDown(t *testing.T) {
	block := "-- +goose StatementBegin\nSELECT 1;\n-- +goose StatementEnd\n"
	for name, text := range map[string]string{
		"before Up": "-- +goose Down\n" + block + "-- +goose Up\n" + block,
		"twice":     "-- +goose Up\n" + block + "-- +goose Down\n" + block + "-- +goose Down\n" + block,
	} {
		if _, err := readGoose(text); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestEveryMigrationHasOnlyKnownAnnotations(t *testing.T) {
	files, err := filepath.Glob("migrations/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("migrations = %v, %v", files, err)
	}
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := checkAnnotations(string(data)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if got := strings.Count(string(data), "-- +goose Up\n"); got != 1 {
			t.Errorf("%s: Up annotation %d times, want once", name, got)
		}
		if got := strings.Count(string(data), "-- +goose Down\n"); got != 1 {
			t.Errorf("%s: Down annotation %d times, want once", name, got)
		}
	}
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

	file, err := readGoose(string(data))
	if err != nil {
		t.Fatal(err)
	}
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
