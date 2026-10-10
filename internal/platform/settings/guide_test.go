package settings

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

const settingsMarker = "<!-- table: settings -->"

// guideKeys returns the first backticked text of the first cell of each data
// row of the table that follows the settings marker.
func guideKeys(guide string) ([]string, error) {
	_, after, found := strings.Cut(guide, settingsMarker+"\n")
	if !found {
		return nil, fmt.Errorf("the settings marker is missing from the guide")
	}
	var keys []string
	for i, line := range strings.Split(after, "\n") {
		if !strings.HasPrefix(line, "|") {
			break
		}
		if i < 2 {
			continue
		}
		cell := strings.Split(line, "|")[1]
		parts := strings.Split(cell, "`")
		if len(parts) < 3 {
			return nil, fmt.Errorf("row %d of the settings table has no key in backticks", i)
		}
		keys = append(keys, parts[1])
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("the settings table holds no row")
	}
	return keys, nil
}

func codeKeys() []string {
	var keys []string
	rt := reflect.TypeOf(tomlSettings{})
	for i := 0; i < rt.NumField(); i++ {
		keys = append(keys, rt.Field(i).Tag.Get("toml"))
	}
	return keys
}

// settingsDrift names the keys the guide lacks and the keys it adds.
func settingsDrift(guide string) (string, error) {
	rows, err := guideKeys(guide)
	if err != nil {
		return "", err
	}
	inGuide := map[string]bool{}
	for _, k := range rows {
		inGuide[k] = true
	}
	inCode := map[string]bool{}
	for _, k := range codeKeys() {
		inCode[k] = true
	}
	var missing, extra []string
	for k := range inCode {
		if !inGuide[k] {
			missing = append(missing, k)
		}
	}
	for k := range inGuide {
		if !inCode[k] {
			extra = append(extra, k)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)
	if len(missing) == 0 && len(extra) == 0 {
		return "", nil
	}
	return fmt.Sprintf("missing from the guide: %v; extra in the guide: %v", missing, extra), nil
}

func TestOperationsGuideNamesEverySetting(t *testing.T) {
	raw, err := os.ReadFile("../../../docs/operations.md")
	if err != nil {
		t.Fatalf("reading the guide: %v", err)
	}
	guide := string(raw)

	drift, err := settingsDrift(guide)
	if err != nil {
		t.Fatal(err)
	}
	if drift != "" {
		t.Fatalf("the guide and the settings differ: %s", drift)
	}

	t.Run("a removed row is seen", func(t *testing.T) {
		lines := strings.Split(guide, "\n")
		var kept []string
		for _, l := range lines {
			if !strings.HasPrefix(l, "| `worker_slots` |") {
				kept = append(kept, l)
			}
		}
		mutated := strings.Join(kept, "\n")
		drift, err := settingsDrift(mutated)
		if err != nil || !strings.Contains(drift, "worker_slots") {
			t.Fatalf("drift = %q, err = %v; want worker_slots reported missing", drift, err)
		}
	})
	t.Run("an extra row is seen", func(t *testing.T) {
		mutated := strings.Replace(guide, "| `worker_slots` |", "| `invented_key` | 1 | 1 | x |\n| `worker_slots` |", 1)
		drift, err := settingsDrift(mutated)
		if err != nil || !strings.Contains(drift, "invented_key") {
			t.Fatalf("drift = %q, err = %v; want invented_key reported extra", drift, err)
		}
	})
	t.Run("a removed marker is seen", func(t *testing.T) {
		mutated := strings.Replace(guide, settingsMarker, "", 1)
		if _, err := settingsDrift(mutated); err == nil {
			t.Fatal("a guide without the marker was accepted")
		}
	})
}
