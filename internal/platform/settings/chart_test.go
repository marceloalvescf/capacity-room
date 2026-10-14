package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"testing"
)

// chartOwnedKeys are the keys the chart writes itself and refuses under
// platform.
var chartOwnedKeys = []string{"server_secret_path", "credential_dir", "ca_bundle_dir"}

func TestChartSchemaNamesEverySetting(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/helm/capacity-room/values.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Platform struct {
				Properties map[string]struct {
					Type string `json:"type"`
				} `json:"properties"`
			} `json:"platform"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	got := append([]string(nil), chartOwnedKeys...)
	for key := range schema.Properties.Platform.Properties {
		got = append(got, key)
	}
	want := codeKeys()
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schema and chart-owned keys %v, want the settings keys %v", got, want)
	}
	rt := reflect.TypeOf(tomlSettings{})
	for i := 0; i < rt.NumField(); i++ {
		key := rt.Field(i).Tag.Get("toml")
		entry, ok := schema.Properties.Platform.Properties[key]
		if !ok {
			continue
		}
		want := map[reflect.Kind]string{reflect.Int64: "integer", reflect.Slice: "array"}[rt.Field(i).Type.Kind()]
		if entry.Type != want {
			t.Errorf("schema type of %s is %q, want %q", key, entry.Type, want)
		}
	}
}

// TestChartSchemaBoundsEqualTheProgram writes each integer key of the schema
// at its bounds and just outside them, and requires Load to accept the first
// two and refuse the others.
func TestChartSchemaBoundsEqualTheProgram(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/helm/capacity-room/values.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Platform struct {
				Properties map[string]struct {
					Type    string `json:"type"`
					Minimum *int64 `json:"minimum"`
					Maximum *int64 `json:"maximum"`
				} `json:"properties"`
			} `json:"platform"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	// companions keep the rules between keys satisfied while a key is probed.
	companions := func(key string, value int64) string {
		switch {
		case key == "session_idle_seconds":
			return "session_ttl_seconds = 2592000\n"
		case key == "admission_profile_limit":
			return "admission_limit = 1000\n"
		case key == "session_ttl_seconds" && value == 60:
			return "session_idle_seconds = 60\n"
		case key == "admission_limit" && value == 1:
			return "admission_profile_limit = 1\n"
		}
		return ""
	}
	load := func(key string, value int64) error {
		_, err := loadText(t, secretLine+companions(key, value)+fmt.Sprintf("%s = %d\n", key, value), goodDSN)
		return err
	}
	probed := 0
	for key, entry := range schema.Properties.Platform.Properties {
		if entry.Type != "integer" {
			continue
		}
		probed++
		if entry.Minimum == nil || entry.Maximum == nil {
			t.Errorf("%s: the schema lacks a bound", key)
			continue
		}
		for _, v := range []int64{*entry.Minimum, *entry.Maximum} {
			if err := load(key, v); err != nil {
				t.Errorf("%s = %d refused: %v", key, v, err)
			}
		}
		for _, v := range []int64{*entry.Minimum - 1, *entry.Maximum + 1} {
			if err := load(key, v); err == nil {
				t.Errorf("%s = %d accepted", key, v)
			}
		}
	}
	if probed != 12 {
		t.Errorf("probed %d integer keys, want 12", probed)
	}
}
