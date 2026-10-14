package settings

import (
	"encoding/json"
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
