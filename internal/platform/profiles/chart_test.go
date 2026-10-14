package profiles

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"testing"
)

type schemaEntry struct {
	Type       string                 `json:"type"`
	Properties map[string]schemaEntry `json:"properties"`
	Items      *schemaEntry           `json:"items"`
}

// tomlKeys returns the toml keys of t's fields and the schema type each
// field's Go type stands for.
func tomlKeys(t reflect.Type) map[string]string {
	types := map[string]string{}
	for i := 0; i < t.NumField(); i++ {
		key := t.Field(i).Tag.Get("toml")
		if key == "" || key == "-" {
			continue
		}
		ft := t.Field(i).Type
		if ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		types[key] = map[reflect.Kind]string{
			reflect.String: "string", reflect.Bool: "boolean", reflect.Float64: "number",
			reflect.Slice: "array", reflect.Struct: "object",
		}[ft.Kind()]
	}
	return types
}

func compareKeys(t *testing.T, where string, schema map[string]schemaEntry, code map[string]string) {
	t.Helper()
	var got, want []string
	for k := range schema {
		got = append(got, k)
	}
	for k := range code {
		want = append(want, k)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s: schema keys %v, want the import's %v", where, got, want)
	}
	for k, typ := range code {
		if schema[k].Type != typ {
			t.Errorf("%s: schema type of %s is %q, want %q", where, k, schema[k].Type, typ)
		}
	}
}

func TestChartSchemaNamesEveryImportKey(t *testing.T) {
	data, err := os.ReadFile("../../../deploy/helm/capacity-room/values.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties struct {
			Profiles schemaEntry `json:"profiles"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatal(err)
	}
	entry := schema.Properties.Profiles.Items
	if entry == nil {
		t.Fatal("the schema has no profiles items")
	}
	compareKeys(t, "profiles", entry.Properties, tomlKeys(reflect.TypeOf(ImportEntry{})))
	compareKeys(t, "thresholds", entry.Properties["thresholds"].Properties, tomlKeys(reflect.TypeOf(ImportThresholds{})))
}
