package snapshot

import (
	"bytes"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

// numberValue is a Value that holds a number.
func numberValue(v float64) Value { return Value{Kind: "number", Number: v} }

func TestValueMarshalsAsItsKind(t *testing.T) {
	cases := []struct {
		name  string
		value Value
		want  string
	}{
		{"number", numberValue(4), `4`},
		{"fraction", numberValue(4.5), `4.5`},
		{"boolean", Value{Kind: "boolean", Boolean: true}, `true`},
		{"false", Value{Kind: "boolean", Boolean: false}, `false`},
		{"text", Value{Kind: "text", Text: "cpu,memory"}, `"cpu,memory"`},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.value)
		if err != nil {
			t.Fatalf("%s: Marshal() error = %v", c.name, err)
		}
		if string(got) != c.want {
			t.Errorf("%s: Marshal() = %s, want %s", c.name, got, c.want)
		}
	}
}

func TestValueUnmarshalsEachKind(t *testing.T) {
	good := []struct {
		in   string
		want Value
	}{
		{`4`, numberValue(4)},
		{`4.5`, numberValue(4.5)},
		{`true`, Value{Kind: "boolean", Boolean: true}},
		{`"cpu"`, Value{Kind: "text", Text: "cpu"}},
	}
	for _, c := range good {
		var got Value
		if err := json.Unmarshal([]byte(c.in), &got); err != nil {
			t.Errorf("Unmarshal(%s) error = %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("Unmarshal(%s) = %+v, want %+v", c.in, got, c.want)
		}
	}
	for _, in := range []string{`null`, `{}`, `[]`} {
		var got Value
		if err := got.UnmarshalJSON([]byte(in)); err == nil {
			t.Errorf("UnmarshalJSON(%s) = nil, want an error", in)
		}
	}
}

// goldenEvidenceSnapshot reads a golden snapshot and lets change set its
// first evidence of the first finding in the generic tree.
func goldenEvidenceSnapshot(t *testing.T, change func(evidence map[string]any)) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "snapshot", "direct-complete.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatal(err)
	}
	finding := tree["findings"].([]any)[0].(map[string]any)
	change(finding["evidence"].([]any)[0].(map[string]any))
	out, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestDecodeRefusesAnEvidenceOfTheWrongKind(t *testing.T) {
	data := goldenEvidenceSnapshot(t, func(e map[string]any) { e["value"] = true })
	_, err := Decode(data)
	if err == nil {
		t.Fatal("Decode() = nil, want an error")
	}
	for _, part := range []string{"findings[0]", "NAMESPACE_MEMORY_LIMIT_CRITICAL", "average_percent", "boolean", "number"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("Decode() = %q, want it to name %q", err.Error(), part)
		}
	}
}

func TestDecodeRefusesAnEvidenceObject(t *testing.T) {
	data := goldenEvidenceSnapshot(t, func(e map[string]any) { e["value"] = map[string]any{} })
	_, err := Decode(data)
	want := "snapshot holds a value of the wrong kind at findings[0].evidence[0].value"
	if err == nil || err.Error() != want {
		t.Fatalf("Decode() = %v, want %q", err, want)
	}
}

func TestDecodeRefusesAnEvidenceListAndNull(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{"a list", []any{}, "snapshot holds a value of the wrong kind at findings[0].evidence[0].value"},
		{"null", nil, "snapshot holds null at findings[0].evidence[0].value"},
	}
	for _, tt := range tests {
		data := goldenEvidenceSnapshot(t, func(e map[string]any) { e["value"] = tt.value })
		_, err := Decode(data)
		if err == nil || err.Error() != tt.want {
			t.Errorf("%s: Decode() = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestCheckValueRefusesWhatItCannotWrite(t *testing.T) {
	tests := []struct {
		name  string
		value Value
		ok    bool
	}{
		{"a number", numberValue(4), true},
		{"a boolean", Value{Kind: "boolean", Boolean: false}, true},
		{"a text", Value{Kind: "text", Text: "cpu"}, true},
		{"not a number", numberValue(math.NaN()), false},
		{"infinite", numberValue(math.Inf(-1)), false},
		{"an empty text", Value{Kind: "text"}, false},
		{"no kind", Value{}, false},
		{"another kind", Value{Kind: "list"}, false},
	}
	for _, tt := range tests {
		if err := checkValue(tt.value); (err == nil) != tt.ok {
			t.Errorf("%s: checkValue() = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestGoldenSnapshotsAreUnchanged(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "testdata", "snapshot", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := len(promtest.Scenarios) + len(promtest.RangeScenarios)
	if len(files) != want {
		t.Fatalf("found %d golden snapshots, want %d", len(files), want)
	}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		snap, err := Decode(data)
		if err != nil {
			t.Errorf("%s: Decode() error = %v", file, err)
			continue
		}
		out, err := Encode(snap)
		if err != nil {
			t.Errorf("%s: Encode() error = %v", file, err)
			continue
		}
		if !bytes.Equal(out, data) {
			t.Errorf("%s: Encode(Decode(file)) differs from the file", file)
		}
	}
}

func TestValueKeepsTheCharactersJSONEscapesForHTML(t *testing.T) {
	value := Value{Kind: "text", Text: "a<b&c>"}
	data, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if got, want := string(data), `"a<b&c>"`; got != want {
		t.Errorf("MarshalJSON = %s, want %s", got, want)
	}
	var back Value
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back != value {
		t.Errorf("round trip = %+v, want %+v", back, value)
	}
}
