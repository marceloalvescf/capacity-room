package api

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// schemaWalk checks a decoded JSON value against the schemas of the contract.
type schemaWalk struct {
	schemas map[string]map[string]any
	errs    []string
}

func (w *schemaWalk) fail(path, format string, args ...any) {
	w.errs = append(w.errs, path+": "+fmt.Sprintf(format, args...))
}

func (w *schemaWalk) resolve(schema map[string]any) map[string]any {
	ref, ok := schema["$ref"].(string)
	if !ok {
		return schema
	}
	name, found := strings.CutPrefix(ref, "#/components/schemas/")
	if !found || w.schemas[name] == nil {
		w.fail(ref, "reference does not name a schema")
		return map[string]any{}
	}
	return w.schemas[name]
}

// walk follows the references, honours nullable, and checks each JSON type;
// a schema without a type accepts anything.
func (w *schemaWalk) walk(path string, schema map[string]any, value any) {
	schema = w.resolve(schema)
	kind, _ := schema["type"].(string)
	if value == nil {
		if kind != "" && schema["nullable"] != true {
			w.fail(path, "null where the schema is %s and not nullable", kind)
		}
		return
	}
	switch kind {
	case "":
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			w.fail(path, "%T where the schema is an object", value)
			return
		}
		properties, _ := schema["properties"].(map[string]any)
		for key, v := range object {
			property, named := properties[key].(map[string]any)
			if !named {
				w.fail(path, "key %q is not named by the schema", key)
				continue
			}
			w.walk(path+"."+key, property, v)
		}
		for key := range properties {
			if _, present := object[key]; !present {
				w.fail(path, "key %q is named by the schema and absent", key)
			}
		}
		required, _ := schema["required"].([]any)
		for _, key := range required {
			if _, present := object[key.(string)]; !present {
				w.fail(path, "required key %q is absent", key)
			}
		}
	case "array":
		list, ok := value.([]any)
		if !ok {
			w.fail(path, "%T where the schema is an array", value)
			return
		}
		items, _ := schema["items"].(map[string]any)
		for i, v := range list {
			w.walk(fmt.Sprintf("%s[%d]", path, i), items, v)
		}
	case "string":
		if _, ok := value.(string); !ok {
			w.fail(path, "%T where the schema is a string", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			w.fail(path, "%T where the schema is a boolean", value)
		}
	case "number":
		if _, ok := value.(float64); !ok {
			w.fail(path, "%T where the schema is a number", value)
		}
	case "integer":
		if n, ok := value.(float64); !ok || n != math.Trunc(n) {
			w.fail(path, "%v where the schema is an integer", value)
		}
	default:
		w.fail(path, "the walk does not know the type %q", kind)
	}
	if allowed, ok := schema["enum"].([]any); ok && !slices.Contains(allowed, value) {
		w.fail(path, "%v is not one of %v", value, allowed)
	}
}

func loadSchemas(t *testing.T) map[string]map[string]any {
	t.Helper()
	raw, err := os.ReadFile(contractPath(t))
	if err != nil {
		t.Fatalf("reading the contract: %v", err)
	}
	var doc struct {
		Components struct {
			Schemas map[string]map[string]any `yaml:"schemas"`
		} `yaml:"components"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the contract: %v", err)
	}
	return doc.Components.Schemas
}

func TestViewMatchesTheContract(t *testing.T) {
	walk := &schemaWalk{schemas: loadSchemas(t)}
	if walk.schemas["ReportView"] == nil {
		t.Fatal("the contract has no ReportView")
	}
	dir := filepath.Join(filepath.Dir(filepath.Dir(contractPath(t))), "testdata", "snapshot")
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) < 8 {
		t.Fatalf("golden snapshots: %v, %v", files, err)
	}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		snap, err := snapshot.Decode(raw)
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		for _, lang := range catalog.Languages() {
			name := filepath.Base(file) + " " + string(lang)
			cat, err := catalog.Load(lang)
			if err != nil {
				t.Fatal(err)
			}
			view, err := document.BuildView(snap, cat)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			body, err := document.EncodeView(view)
			if err != nil {
				t.Fatal(err)
			}
			var decoded any
			if err := json.Unmarshal(body, &decoded); err != nil {
				t.Fatal(err)
			}
			walk.errs = nil
			walk.walk("view", map[string]any{"$ref": "#/components/schemas/ReportView"}, decoded)
			if len(walk.errs) > 0 {
				t.Errorf("%s: %d differences, the first: %s", name, len(walk.errs), strings.Join(walk.errs[:min(5, len(walk.errs))], "; "))
			}
		}
	}
}
