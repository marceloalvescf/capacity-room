package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"testing"
	"testing/fstest"
)

// releasedMigrations holds the SHA-256 of each migration released with 1.0.0.
// A released file is never edited; a schema change adds a new file, which is
// not in this map and passes.
var releasedMigrations = map[string]string{
	"00001_accounts.sql": "5e4c0d8deb3679e80be0b77d7150e2253938fe0fcb48bd7565cd4bd1e3b9c915",
	"00002_profiles.sql": "35e7c850fbf22883b797143c1d54d3b39d644713559970e3500edc25e7d349e6",
	"00003_river.sql":    "fe866eb8d2b53813385e75d3c32cd8f8e3429afe792068ea468a486c2a1b1abc",
	"00004_reports.sql":  "d8fe0296b01dc90df1b2c33ca46a9f58692904d9db9ac7a98b9ad140b683f59b",
}

// checkReleased reports each released file of the folder that is missing or
// whose bytes differ from the pinned hash.
func checkReleased(folder fs.FS) []error {
	var problems []error
	for name, want := range releasedMigrations {
		data, err := fs.ReadFile(folder, "migrations/"+name)
		if err != nil {
			problems = append(problems, fmt.Errorf("released migration %s is missing: %w", name, err))
			continue
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != want {
			problems = append(problems, fmt.Errorf("released migration %s changed: sha256 %s, want %s", name, got, want))
		}
	}
	return problems
}

func TestReleasedMigrationsAreUnchanged(t *testing.T) {
	t.Run("embedded files match", func(t *testing.T) {
		for _, err := range checkReleased(migrations) {
			t.Error(err)
		}
	})

	copyOf := func(t *testing.T) fstest.MapFS {
		t.Helper()
		folder := fstest.MapFS{}
		for name := range releasedMigrations {
			data, err := fs.ReadFile(migrations, "migrations/"+name)
			if err != nil {
				t.Fatal(err)
			}
			folder["migrations/"+name] = &fstest.MapFile{Data: data}
		}
		return folder
	}

	t.Run("a changed byte is refused", func(t *testing.T) {
		folder := copyOf(t)
		file := folder["migrations/00004_reports.sql"]
		file.Data = append([]byte(nil), file.Data...)
		file.Data[0] ^= 1
		problems := checkReleased(folder)
		if len(problems) != 1 {
			t.Fatalf("problems = %v, want one", problems)
		}
	})

	t.Run("a missing file is refused", func(t *testing.T) {
		folder := copyOf(t)
		delete(folder, "migrations/00002_profiles.sql")
		problems := checkReleased(folder)
		if len(problems) != 1 {
			t.Fatalf("problems = %v, want one", problems)
		}
	})

	t.Run("a new file passes", func(t *testing.T) {
		folder := copyOf(t)
		folder["migrations/00005_next.sql"] = &fstest.MapFile{Data: []byte("select 1;")}
		if problems := checkReleased(folder); len(problems) != 0 {
			t.Fatalf("problems = %v, want none", problems)
		}
	})
}
