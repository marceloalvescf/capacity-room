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

	"github.com/marceloalvescf/capacity-room/internal/analysis"
	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/document"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// schemaWalk checks a decoded JSON value against the schemas of the contract.
type schemaWalk struct {
	schemas map[string]map[string]any
	errs    []string
}

func (w *schemaWalk) fail(path, format string, args ...any) {
	w.errs = append(w.errs, path+": "+fmt.Sprintf(format, args...))
}

func (w *schemaWalk) resolve(path string, schema map[string]any) map[string]any {
	if entries, ok := schema["allOf"].([]any); ok {
		if len(entries) != 1 {
			w.fail(path, "allOf holds %d entries, want exactly one reference", len(entries))
			return map[string]any{}
		}
		for key := range schema {
			if key != "allOf" && key != "description" && key != "nullable" {
				w.fail(path, "allOf has the sibling key %q, want only description and nullable", key)
				return map[string]any{}
			}
		}
		entry, _ := entries[0].(map[string]any)
		if _, ok := entry["$ref"].(string); !ok {
			w.fail(path, "the allOf entry is not a map holding a $ref")
			return map[string]any{}
		}
		schema = entry
	}
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

// matches reports whether value satisfies schema.
func (w *schemaWalk) matches(schema map[string]any, value any) bool {
	sub := &schemaWalk{schemas: w.schemas}
	sub.walk("alternative", schema, value)
	return len(sub.errs) == 0
}

// walk follows the references, honours nullable, enum, and oneOf, and checks
// each JSON type; a schema without a type or alternatives accepts anything.
func (w *schemaWalk) walk(path string, schema map[string]any, value any) {
	schema = w.resolve(path, schema)
	if alternatives, ok := schema["oneOf"].([]any); ok {
		n := 0
		for _, alternative := range alternatives {
			if w.matches(alternative.(map[string]any), value) {
				n++
			}
		}
		if n != 1 {
			w.fail(path, "%v matches %d of the %d alternatives, want exactly one", value, n, len(alternatives))
		}
		return
	}
	kind, _ := schema["type"].(string)
	if value == nil {
		if kind != "" && schema["nullable"] != true {
			w.fail(path, "null where the schema is %s and not nullable", kind)
		}
		if allowed, ok := schema["enum"].([]any); ok && !slices.Contains(allowed, nil) {
			w.fail(path, "null is not one of %v", allowed)
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
			decoded := decodeView(t, view)
			walk.errs = nil
			walk.walk("view", map[string]any{"$ref": "#/components/schemas/ReportView"}, decoded)
			if len(walk.errs) > 0 {
				t.Errorf("%s: %d differences, the first: %s", name, len(walk.errs), strings.Join(walk.errs[:min(5, len(walk.errs))], "; "))
			}
		}
	}
}

func decodeView(t *testing.T, view document.View) any {
	t.Helper()
	body, err := document.EncodeView(view)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// check walks value against the schema and returns the differences.
func (w *schemaWalk) check(schema map[string]any, value any) []string {
	w.errs = nil
	w.walk("v", schema, value)
	return w.errs
}

func TestWalkerKnowsOneOfAndNullInAnEnum(t *testing.T) {
	w := &schemaWalk{schemas: map[string]map[string]any{}}
	raw := map[string]any{"oneOf": []any{
		map[string]any{"type": "number"}, map[string]any{"type": "boolean"}, map[string]any{"type": "string"},
	}}
	for name, v := range map[string]any{"number": 1.5, "boolean": true, "string": "x"} {
		if errs := w.check(raw, v); len(errs) != 0 {
			t.Errorf("oneOf refuses a %s: %v", name, errs)
		}
	}
	for name, v := range map[string]any{"object": map[string]any{"a": 1.0}, "array": []any{1.0}, "null": nil} {
		if errs := w.check(raw, v); len(errs) == 0 {
			t.Errorf("oneOf accepts a %s", name)
		}
	}
	both := map[string]any{"oneOf": []any{map[string]any{"type": "number"}, map[string]any{"type": "integer"}}}
	if errs := w.check(both, 2.0); len(errs) == 0 {
		t.Error("oneOf accepts a value that two alternatives match")
	}

	mark := map[string]any{"type": "string", "nullable": true, "enum": []any{"healthy", nil}}
	for name, v := range map[string]any{"a member": "healthy", "null": nil} {
		if errs := w.check(mark, v); len(errs) != 0 {
			t.Errorf("the enum refuses %s: %v", name, errs)
		}
	}
	if errs := w.check(mark, "bogus"); len(errs) == 0 {
		t.Error("the enum accepts a text outside it")
	}
	closed := map[string]any{"type": "string", "nullable": true, "enum": []any{"healthy"}}
	if errs := w.check(closed, nil); len(errs) == 0 {
		t.Error("an enum without null accepts null")
	}
}

func TestWalkerResolvesAnAllOfOfOneReference(t *testing.T) {
	w := &schemaWalk{schemas: map[string]map[string]any{
		"Code": {"type": "string", "enum": []any{"a"}},
	}}
	one := map[string]any{
		"allOf":       []any{map[string]any{"$ref": "#/components/schemas/Code"}},
		"description": "A described code.",
	}
	if errs := w.check(one, "a"); len(errs) != 0 {
		t.Errorf("an allOf of one reference refuses a member: %v", errs)
	}
	if errs := w.check(one, "b"); len(errs) == 0 {
		t.Error("an allOf of one reference accepts a text outside the enum")
	}
	two := map[string]any{"allOf": []any{
		map[string]any{"$ref": "#/components/schemas/Code"}, map[string]any{"type": "string"},
	}}
	errs := w.check(two, "a")
	if len(errs) != 1 || !strings.Contains(errs[0], "v: allOf") {
		t.Errorf("an allOf of two entries is not refused with the property named: %v", errs)
	}
	malformed := map[string]map[string]any{
		"an inline schema": {"allOf": []any{map[string]any{"type": "string"}}},
		"a non-map entry":  {"allOf": []any{"#/components/schemas/Code"}},
		"an unknown sibling": {
			"allOf":    []any{map[string]any{"$ref": "#/components/schemas/Code"}},
			"maxItems": 3,
		},
	}
	for name, schema := range malformed {
		if errs := w.check(schema, "zzz"); len(errs) != 1 || !strings.Contains(errs[0], "v: ") || !strings.Contains(errs[0], "allOf") {
			t.Errorf("%s is not refused with the property named: %v", name, errs)
		}
	}
}

func TestViewContractRefusesAMutatedView(t *testing.T) {
	walk := &schemaWalk{schemas: loadSchemas(t)}
	root := map[string]any{"$ref": "#/components/schemas/ReportView"}
	snap := goldenView(t, "direct-complete.json")
	if errs := walk.check(root, decodeView(t, snap)); len(errs) != 0 {
		t.Fatalf("the unmutated view differs: %v", errs)
	}
	mutations := map[string]func(view map[string]any){
		"an evidence raw that is an object": func(view map[string]any) {
			view["findings"].([]any)[0].(map[string]any)["evidence"].([]any)[0].(map[string]any)["raw"] = map[string]any{"n": 1.0}
		},
		"a status outside the enum": func(view map[string]any) {
			view["namespaces"].([]any)[0].(map[string]any)["cpu_request_percent"].(map[string]any)["peak"].(map[string]any)["status"] = "bogus"
		},
		"a situation status outside the enum": func(view map[string]any) {
			view["hpas"].([]any)[0].(map[string]any)["situation_status"] = "bogus"
		},
		"a reason status outside the enum": func(view map[string]any) {
			view["nodes"].([]any)[0].(map[string]any)["reason_status"] = "bogus"
		},
	}
	for name, mutate := range mutations {
		decoded := decodeView(t, snap)
		mutate(decoded.(map[string]any))
		if errs := walk.check(root, decoded); len(errs) == 0 {
			t.Errorf("%s: the walk accepts it", name)
		}
	}
}

func goldenView(t *testing.T, file string) document.View {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(filepath.Dir(contractPath(t))), "testdata", "snapshot", file))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := snapshot.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Load(catalog.English)
	if err != nil {
		t.Fatal(err)
	}
	view, err := document.BuildView(snap, cat)
	if err != nil {
		t.Fatal(err)
	}
	return view
}

// TestViewEnumsEqualTheRegistries compares each closed set of the view's
// contract with the Go list that holds the same values: as sets, except that
// the capability names keep the order of discovery.Names and the evidence
// names are sorted. A value that is not a text, or that appears twice, fails.
func TestViewEnumsEqualTheRegistries(t *testing.T) {
	scopeTypes := map[string]bool{}
	evidenceNames := map[string]bool{}
	for _, definition := range analysis.Registry {
		scopeTypes[definition.ScopeType] = true
		for _, spec := range definition.Evidence {
			evidenceNames[spec.Name] = true
		}
	}
	keys := func(set map[string]bool) []string {
		list := make([]string, 0, len(set))
		for key := range set {
			list = append(list, key)
		}
		slices.Sort(list)
		return list
	}
	type enum struct {
		want    []string
		ordered bool
	}
	enums := map[string]enum{
		"ViewSeverity":          {want: analysis.Severities},
		"ViewHazard":            {want: document.GaugeHazards},
		"ViewScopeType":         {want: keys(scopeTypes)},
		"ViewConfidence":        {want: []string{discovery.ConfidenceExact, discovery.ConfidenceInferred, discovery.ConfidenceUnknown}},
		"ViewHPACoverage":       {want: snapshot.Coverages},
		"ViewAutoscalingStatus": {want: snapshot.AutoscalingStatuses},
		"ViewAutoscalingSource": {want: snapshot.AutoscalingSources},
		"ViewStrategy":          {want: discovery.Strategies},
		"ViewCapabilityName":    {want: discovery.Names, ordered: true},
		"ViewCapabilityState":   {want: discovery.States},
		"ViewEvidenceName":      {want: keys(evidenceNames), ordered: true},
	}
	schemas := loadSchemas(t)
	for name, kind := range enums {
		schema := schemas[name]
		if schema == nil {
			t.Errorf("%s: the contract has no such schema", name)
			continue
		}
		var got []string
		listed, _ := schema["enum"].([]any)
		for _, value := range listed {
			if value == nil && schema["nullable"] == true {
				continue
			}
			text, ok := value.(string)
			if !ok {
				t.Errorf("%s: the contract holds %v, which is not a text", name, value)
				continue
			}
			if slices.Contains(got, text) {
				t.Errorf("%s: the contract holds %q twice", name, text)
				continue
			}
			got = append(got, text)
		}
		if kind.ordered {
			if !slices.Equal(got, kind.want) {
				t.Errorf("%s: the contract holds %v, the registry %v, in this order", name, got, kind.want)
			}
			continue
		}
		for _, value := range kind.want {
			if !slices.Contains(got, value) {
				t.Errorf("%s: the contract lacks %q, which the registry holds", name, value)
			}
		}
		for _, value := range got {
			if !slices.Contains(kind.want, value) {
				t.Errorf("%s: the contract holds %q, which the registry lacks", name, value)
			}
		}
	}
}

// TestViewFieldsTakeTheirEnums pins the reference of every field that carries
// one of the view's closed sets, so that a field cannot fall back to a bare
// text without a test failing.
func TestViewFieldsTakeTheirEnums(t *testing.T) {
	refs := map[string]map[string]string{
		"ViewFinding":        {"severity": "ViewSeverity", "scope_type": "ViewScopeType", "confidence": "ViewConfidence"},
		"ViewEvidence":       {"name": "ViewEvidenceName"},
		"ViewRecommendation": {"severity": "ViewSeverity", "scope_type": "ViewScopeType", "confidence": "ViewConfidence"},
		"ViewPressure":       {"scope_type": "ViewScopeType", "severity": "ViewSeverity"},
		"ViewGauge":          {"hazard": "ViewHazard"},
		"ViewNamespace":      {"status": "ViewSeverity"},
		"ViewWorkload":       {"status": "ViewSeverity", "hpa_coverage": "ViewHPACoverage"},
		"ViewNode":           {"status": "ViewSeverity", "pool_confidence": "ViewConfidence"},
		"ViewNodepool":       {"confidence": "ViewConfidence", "status": "ViewSeverity"},
		"ViewHPA":            {"status": "ViewSeverity"},
		"ViewTotal":          {"severity": "ViewSeverity"},
		"ViewAutoscaling":    {"status": "ViewAutoscalingStatus", "source": "ViewAutoscalingSource", "confidence": "ViewConfidence"},
		"ViewCapability":     {"name": "ViewCapabilityName", "state": "ViewCapabilityState", "strategy": "ViewStrategy"},
		"ViewReport":         {"profile": "ReportProfile"},
	}
	schemas := loadSchemas(t)
	for schemaName, fields := range refs {
		properties, _ := schemas[schemaName]["properties"].(map[string]any)
		if properties == nil {
			t.Errorf("%s: the contract has no such schema, or it has no properties", schemaName)
			continue
		}
		for field, target := range fields {
			property, _ := properties[field].(map[string]any)
			got, _ := property["$ref"].(string)
			if entries, ok := property["allOf"].([]any); ok && len(entries) == 1 {
				entry, _ := entries[0].(map[string]any)
				got, _ = entry["$ref"].(string)
			}
			if want := "#/components/schemas/" + target; got != want {
				t.Errorf("%s.%s: the reference is %q, want %q", schemaName, field, got, want)
			}
		}
	}
}
