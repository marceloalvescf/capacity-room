package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// validSnapshot returns a Snapshot that passes Check(), so each table test
// below can start from a known-good baseline and change exactly one field.
func validSnapshot(t *testing.T) Snapshot {
	t.Helper()
	return mustBuild(t, baseInput())
}

func TestEncodeIsStable(t *testing.T) {
	snap := validSnapshot(t)

	first, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	second, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if string(first) != string(second) {
		t.Fatalf("Encode() is not stable:\n%s\nvs\n%s", first, second)
	}
	if !strings.HasSuffix(string(first), "\n") {
		t.Fatalf("Encode() does not end in a line break: %q", first)
	}
	if strings.HasSuffix(string(first), "\n\n") {
		t.Fatalf("Encode() ends in more than one line break: %q", first)
	}
}

func TestEncodeKeepsNamesAsTheyAre(t *testing.T) {
	in := baseInput()
	in.Scope.ClusterName = "a<b>&c"
	in.Scope.DiscoveredValues = []string{"a<b>&c"}
	snap := mustBuild(t, in)

	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if !strings.Contains(string(data), "a<b>&c") {
		t.Fatalf("Encode() escaped the cluster name: %s", data)
	}
}

func TestDecodeReadsWhatEncodeWrote(t *testing.T) {
	tests := []struct {
		name string
		snap Snapshot
	}{
		{"no recommendation", validSnapshot(t)},
		{"two recommendations", clusterSnapshot(t)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := Encode(tt.snap)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			got, err := Decode(data)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.snap) {
				t.Fatalf("Decode(Encode(s)) = %+v, want %+v", got, tt.snap)
			}
		})
	}
}

// mutateJSON decodes good into a generic tree, applies mutate, and
// re-encodes it, so a test can corrupt one key without hand-writing JSON.
func mutateJSON(t *testing.T, good []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	mutate(tree)
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}
	return data
}

// TestDecodeOrder exercises the order in which Decode judges a file: the
// schema version, read loosely, before anything about the rest of the
// document's shape.
func TestDecodeOrder(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	tests := []struct {
		name     string
		data     []byte
		want     string
		isPrefix bool
	}{
		{"an empty object", []byte(`{}`), "snapshot has no schema_version", false},
		{"a JSON array", []byte(`[]`), "snapshot is not a JSON object", false},
		{"a JSON string", []byte(`"text"`), "snapshot is not a JSON object", false},
		{"a JSON number", []byte(`3`), "snapshot is not a JSON object", false},
		{"schema_version null", []byte(`{"schema_version": null}`), "snapshot has no schema_version", false},
		{"schema_version a number", []byte(`{"schema_version": 1.0}`), "snapshot has no schema_version", false},
		{"schema_version another version", []byte(`{"schema_version": "3.0"}`), `snapshot has schema_version "3.0"; this version reads "2.0"`, false},
		{
			"another version, a missing key, and an unknown key at once",
			[]byte(`{"schema_version": "3.0", "report": {}, "extra": 1}`),
			`snapshot has schema_version "3.0"; this version reads "2.0"`,
			false,
		},
		{
			"a valid snapshot of another version",
			mutateJSON(t, good, func(tree map[string]any) { tree["schema_version"] = "3.0" }),
			`snapshot has schema_version "3.0"; this version reads "2.0"`,
			false,
		},
		{
			"a valid snapshot with an unknown key",
			mutateJSON(t, good, func(tree map[string]any) { tree["bogus"] = true }),
			"snapshot holds an unknown key: ",
			true,
		},
		{
			"a valid snapshot followed by another document",
			append(append([]byte{}, good...), []byte("{}")...),
			"snapshot holds data after the document",
			false,
		},
		{
			"a valid snapshot of another version, followed by another document",
			append(mutateJSON(t, good, func(tree map[string]any) { tree["schema_version"] = "3.0" }), []byte("{}")...),
			"snapshot holds data after the document",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.data)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", tt.want)
			}
			if tt.isPrefix {
				if !strings.HasPrefix(err.Error(), tt.want) {
					t.Fatalf("Decode() = %q, want prefix %q", err.Error(), tt.want)
				}
				return
			}
			if err.Error() != tt.want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestDecodeRefuses(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	tests := []struct {
		name    string
		data    []byte
		want    string
		isExact bool
	}{
		{
			name:    "cut in the middle",
			data:    good[:len(good)-40],
			want:    "snapshot is not valid JSON: ",
			isExact: false,
		},
		{
			name: "data after the document",
			data: append(append([]byte{}, good...), []byte("{}")...),
			want: "snapshot holds data after the document",
		},
		{
			name: "unknown key",
			data: mutateJSON(t, good, func(tree map[string]any) {
				tree["bogus"] = true
			}),
			want:    "snapshot holds an unknown key: ",
			isExact: false,
		},
		{
			name: "schema_version missing",
			data: mutateJSON(t, good, func(tree map[string]any) {
				delete(tree, "schema_version")
			}),
			want: "snapshot has no schema_version",
		},
		{
			name: "another schema_version",
			data: mutateJSON(t, good, func(tree map[string]any) {
				tree["schema_version"] = "3.0"
			}),
			want: `snapshot has schema_version "3.0"; this version reads "2.0"`,
		},
		{
			name: "an integrity failure",
			data: mutateJSON(t, good, func(tree map[string]any) {
				tree["report"].(map[string]any)["id"] = ""
			}),
			want: "report.id is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decode(tt.data)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", tt.want)
			}
			if tt.isExact {
				if err.Error() != tt.want {
					t.Fatalf("Decode() = %q, want %q", err.Error(), tt.want)
				}
				return
			}
			if !strings.HasPrefix(err.Error(), tt.want) {
				t.Fatalf("Decode() = %q, want prefix %q", err.Error(), tt.want)
			}
		})
	}
}

// capitalizeFirst returns s with its first rune upper-cased. Every path of
// the contract is written in lower snake case, so this is exactly the
// change of letter case a file writing a duplicate, forged key beside a
// real one would carry.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// parentAndKey navigates tree (the generic decoding of a document) by path
// (dot-separated, with an optional "[N]" on a segment that names an array)
// and returns the object that holds the value at path and the plain name
// of that value's own key, so a test can add a forged sibling key beside
// it without caring whether path names a leaf or an object.
func parentAndKey(t *testing.T, tree map[string]any, path string) (map[string]any, string) {
	t.Helper()
	segments := strings.Split(path, ".")
	var cur any = tree

	for i, seg := range segments {
		name, idx, hasIdx := splitIndex(seg)
		m, ok := cur.(map[string]any)
		if !ok {
			t.Fatalf("parentAndKey(%q): segment %d (%q) is not an object", path, i, seg)
		}
		if i == len(segments)-1 {
			return m, name
		}
		next, ok := m[name]
		if !ok {
			t.Fatalf("parentAndKey(%q): missing key %q", path, name)
		}
		if hasIdx {
			arr, ok := next.([]any)
			if !ok || idx >= len(arr) {
				t.Fatalf("parentAndKey(%q): segment %d (%q) is not a long enough array", path, i, seg)
			}
			cur = arr[idx]
		} else {
			cur = next
		}
	}
	return nil, ""
}

// TestDecodeRefusesAnotherLetterCase builds, for each of the 80 paths of
// the contract, a valid snapshot with one more key beside the real one: the
// same name, its first letter capitalised, holding a forged value. The
// walk of the map compares a key letter for letter, so every one of these
// is refused as an unknown key, never silently merged into the real one by
// a case-insensitive struct decode.
func TestDecodeRefusesAnotherLetterCase(t *testing.T) {
	if len(contractPaths) != 80 {
		t.Fatalf("contractPaths has %d entries, want 80", len(contractPaths))
	}

	snap := fullSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for _, path := range contractPaths {
		t.Run(path, func(t *testing.T) {
			var tree map[string]any
			if err := json.Unmarshal(good, &tree); err != nil {
				t.Fatalf("Unmarshal(good): %v", err)
			}

			parent, key := parentAndKey(t, tree, path)
			forgedName := capitalizeFirst(key)
			parent[forgedName] = "a forged value"

			data, err := json.Marshal(tree)
			if err != nil {
				t.Fatalf("Marshal(tree): %v", err)
			}

			segments := strings.Split(path, ".")
			segments[len(segments)-1] = forgedName
			want := fmt.Sprintf("snapshot holds an unknown key: %s", strings.Join(segments, "."))

			_, err = Decode(data)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// TestDecodeRefusesOnlyAnotherLetterCase renames report.classification to
// report.Classification, in place of adding a sibling: the key the contract
// declares is now entirely missing, and the only key present at that name
// carries the wrong letter case. The unknown key is still the fault
// reported, never the missing one: the walk finds it first.
func TestDecodeRefusesOnlyAnotherLetterCase(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	report := tree["report"].(map[string]any)
	report["Classification"] = report["classification"]
	delete(report, "classification")

	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}

	_, err = Decode(data)
	want := "snapshot holds an unknown key: report.Classification"
	if err == nil {
		t.Fatalf("Decode() = nil, want error %q", want)
	}
	if err.Error() != want {
		t.Fatalf("Decode() = %q, want %q", err.Error(), want)
	}
}

// TestDecodeRefusesAWrongKind mutates one key of a valid snapshot to a
// value of a JSON kind the contract does not expect there, and checks the
// walk reports it by path, never as an unsupported field or a missing one.
func TestDecodeRefusesAWrongKind(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	tests := []struct {
		name   string
		path   string
		mutate func(tree map[string]any)
	}{
		{"text for tls_verify", "report.settings.tls_verify", func(tree map[string]any) {
			tree["report"].(map[string]any)["settings"].(map[string]any)["tls_verify"] = "true"
		}},
		{"number for cluster_name", "scope.cluster_name", func(tree map[string]any) {
			tree["scope"].(map[string]any)["cluster_name"] = 1.0
		}},
		{"object for namespaces", "scope.namespaces", func(tree map[string]any) {
			tree["scope"].(map[string]any)["namespaces"] = map[string]any{}
		}},
		{"list for report", "report", func(tree map[string]any) {
			tree["report"] = []any{}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tree map[string]any
			if err := json.Unmarshal(good, &tree); err != nil {
				t.Fatalf("Unmarshal(good): %v", err)
			}
			tt.mutate(tree)

			data, err := json.Marshal(tree)
			if err != nil {
				t.Fatalf("Marshal(tree): %v", err)
			}

			_, err = Decode(data)
			want := fmt.Sprintf("snapshot holds a value of the wrong kind at %s", tt.path)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestCheckRefuses(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		message string
	}{
		{"report.id empty", func(s *Snapshot) { s.Report.ID = "" }, "report.id is empty"},
		{"report.generator_version empty", func(s *Snapshot) { s.Report.GeneratorVersion = "" }, "report.generator_version is empty"},
		{"report.emitted_at unparsable", func(s *Snapshot) { s.Report.EmittedAt = "not-a-time" },
			`report.emitted_at is not an RFC 3339 time: "not-a-time"`},
		{"report.period_end after report.emitted_at", func(s *Snapshot) {
			emitted, _ := Time(s.Report.EmittedAt)
			s.Report.PeriodEnd = formatTime(emitted.Add(time.Second), emitted.Location())
		}, "report.period_end is after report.emitted_at"},
		{"report.period_start not before report.period_end", func(s *Snapshot) {
			s.Report.PeriodStart = s.Report.PeriodEnd
		}, "report.period_start is not before report.period_end"},
		{"collection completed before it started", func(s *Snapshot) {
			s.Quality.CollectionStartedAt, s.Quality.CollectionCompletedAt = s.Quality.CollectionCompletedAt, s.Quality.CollectionStartedAt
		}, "quality.collection_completed_at is before quality.collection_started_at"},
		{"report.profile unknown", func(s *Snapshot) { s.Report.Profile = "bogus" }, `report.profile has an unknown value: "bogus"`},
		{"report.min_severity unknown", func(s *Snapshot) { s.Report.MinSeverity = "bogus" }, `report.min_severity has an unknown value: "bogus"`},
		{"scope.filter_mode unknown", func(s *Snapshot) { s.Scope.FilterMode = "bogus" }, `scope.filter_mode has an unknown value: "bogus"`},
		{"scope.confidence unknown", func(s *Snapshot) { s.Scope.Confidence = "bogus" }, `scope.confidence has an unknown value: "bogus"`},
		{"report.settings.endpoint_type unknown", func(s *Snapshot) { s.Report.Settings.EndpointType = "auto" }, `report.settings.endpoint_type has an unknown value: "auto"`},
		{"scope.cluster_name empty", func(s *Snapshot) { s.Scope.ClusterName = "" }, "scope.cluster_name is empty"},
		{"scope.namespaces empty", func(s *Snapshot) { s.Scope.Namespaces = []string{} }, "scope.namespaces is empty"},
		{"scope.namespaces not sorted", func(s *Snapshot) { s.Scope.Namespaces = []string{"b", "a"} }, "scope.namespaces is not sorted or holds a repeated value"},
		{"scope.namespaces holds a repeat", func(s *Snapshot) { s.Scope.Namespaces = []string{"a", "a"} }, "scope.namespaces is not sorted or holds a repeated value"},
		{"quality.capabilities[0].evidence is null", func(s *Snapshot) { s.Quality.Capabilities[0].Evidence = nil },
			"quality.capabilities[0].evidence is null; an empty list is written as []"},
		{"quality.capabilities missing one", func(s *Snapshot) {
			s.Quality.Capabilities = s.Quality.Capabilities[:len(s.Quality.Capabilities)-1]
		}, "quality.capabilities must hold the nine capabilities in their order; found: " + strings.Join(namesOf(t, validSnapshotCapabilities(t))[:8], ", ")},
		{"quality.capabilities[0].state unknown", func(s *Snapshot) { s.Quality.Capabilities[0].State = "bogus" }, `quality.capabilities[0].state has an unknown value: "bogus"`},
		{"quality.capabilities[0].strategy unknown", func(s *Snapshot) { s.Quality.Capabilities[0].Strategy = "bogus" }, `quality.capabilities[0].strategy has an unknown value: "bogus"`},
		{"quality.capabilities[0].reasons unknown", func(s *Snapshot) { s.Quality.Capabilities[0].Reasons = []string{"BOGUS_REASON"} }, `quality.capabilities[0].reasons has an unknown value: "BOGUS_REASON"`},
		{"quality.capabilities[0] available with a reason", func(s *Snapshot) {
			s.Quality.Capabilities[0].Reasons = []string{discovery.ReasonCodes[0]}
		}, "quality.capabilities[0] is available and carries a reason"},
		{"quality.capabilities[0] partial with no reason", func(s *Snapshot) {
			s.Quality.Capabilities[0].State = discovery.StatePartial
		}, "quality.capabilities[0] is partial and carries no reason"},
		{"quality.capabilities[0] unavailable with no reason", func(s *Snapshot) {
			s.Quality.Capabilities[0].State = discovery.StateUnavailable
		}, "quality.capabilities[0] is unavailable and carries no reason"},
		{"quality.capabilities[0] available names no source", func(s *Snapshot) {
			s.Quality.Capabilities[0].Strategy = discovery.StrategyNone
		}, "quality.capabilities[0] is available and names no source"},
		{"quality.capabilities[0] unavailable names a source", func(s *Snapshot) {
			s.Quality.Capabilities[0].State = discovery.StateUnavailable
			s.Quality.Capabilities[0].Reasons = []string{discovery.ReasonCodes[0]}
		}, "quality.capabilities[0] is unavailable and names a source"},
		{"quality.capabilities[0] cannot be configured", func(s *Snapshot) {
			s.Quality.Capabilities[0].Strategy = discovery.StrategyConfigured
		}, "quality.capabilities[0] cannot be configured"},
		{"quality.capabilities[0].reasons repeated", func(s *Snapshot) {
			s.Quality.Capabilities[0].State = discovery.StatePartial
			s.Quality.Capabilities[0].Reasons = []string{discovery.ReasonCodes[0], discovery.ReasonCodes[0]}
		}, "quality.capabilities[0].reasons holds a repeated value"},
		{"report.settings.node_autoscaling unknown", func(s *Snapshot) { s.Report.Settings.NodeAutoscaling = "bogus" }, `report.settings.node_autoscaling has an unknown value: "bogus"`},
		{"report.timezone empty", func(s *Snapshot) { s.Report.Timezone = "" }, "report.timezone is empty"},
		{"scope.cluster_label empty", func(s *Snapshot) { s.Scope.ClusterLabel = "" }, "scope.cluster_label is empty"},
		{"report.settings.timeout_seconds not positive", func(s *Snapshot) { s.Report.Settings.TimeoutSeconds = 0 }, "report.settings.timeout_seconds must be positive"},
		{"report.thresholds percent out of range", func(s *Snapshot) { s.Report.Thresholds.WarningPercent = 101 }, "report.thresholds values must be within 0..100"},
		{"report.thresholds.bursty_gap_percent negative", func(s *Snapshot) { s.Report.Thresholds.BurstyGapPercent = -1 }, "report.thresholds.bursty_gap_percent must not be negative"},
		{"report.thresholds.hpa_replica_divergence negative", func(s *Snapshot) { s.Report.Thresholds.HPAReplicaDivergence = -1 }, "report.thresholds.hpa_replica_divergence must not be negative"},
		{"report.thresholds.warning_percent not below critical_percent", func(s *Snapshot) {
			s.Report.Thresholds.WarningPercent = 90
			s.Report.Thresholds.CriticalPercent = 85
		}, "report.thresholds.warning_percent must be lower than critical_percent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := validSnapshot(t)
			tt.mutate(&snap)

			err := snap.Check()
			if err == nil {
				t.Fatalf("Check() = nil, want error %q", tt.message)
			}
			if err.Error() != tt.message {
				t.Fatalf("Check() = %q, want %q", err.Error(), tt.message)
			}
		})
	}
}

// namesOf returns the capability names of caps, in order.
func namesOf(t *testing.T, caps []Capability) []string {
	t.Helper()
	names := make([]string, len(caps))
	for i, c := range caps {
		names[i] = c.Name
	}
	return names
}

// validSnapshotCapabilities returns the capabilities of a fresh, valid
// snapshot, so TestCheckRefuses can compute the "found:" list of the
// capability-order message without repeating baseInput's own list by hand.
func validSnapshotCapabilities(t *testing.T) []Capability {
	t.Helper()
	return validSnapshot(t).Quality.Capabilities
}

func TestCheckAcceptsBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{"percent zero", func(in *Input) { in.Config.Thresholds.WarningPercent = 0 }},
		{"percent one hundred", func(in *Input) { in.Config.Thresholds.CriticalPercent = 100 }},
		{"period minimum", func(in *Input) { in.Config.Analysis.Period = "1m" }},
		{"timeout minimum", func(in *Input) { in.Config.Metrics.TimeoutSeconds = 0.001 }},
		{"empty organization, classification, author, and nodepool label", func(in *Input) {
			in.Config.Report.Organization = ""
			in.Config.Report.Classification = ""
			in.Config.Report.Author = ""
			in.Config.Cluster.NodepoolLabel = ""
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput()
			tt.mutate(&in)

			if _, err := Build(in); err != nil {
				t.Fatalf("Build() error = %v, want nil", err)
			}
		})
	}
}

func TestCheckAcceptsAReasonCodeAsWarning(t *testing.T) {
	snap := validSnapshot(t)
	snap.Quality.Warnings = []Message{{Code: "METRICS_NOT_FOUND", Params: map[string]float64{}}}
	if err := snap.Check(); err != nil {
		t.Errorf("Check with a reason code as warning = %v, want nil", err)
	}

	snap.Quality.Warnings = []Message{{Code: "NOT_A_CODE", Params: map[string]float64{}}}
	err := snap.Check()
	if err == nil || !strings.Contains(err.Error(), `NOT_A_CODE`) {
		t.Errorf("Check with an unknown code = %v, want a refusal naming NOT_A_CODE", err)
	}
}

func TestBuildOutputAlwaysPassesDecode(t *testing.T) {
	cfg := config.Default()
	cfg.Metrics.URL = "https://prom.example.org"
	cfg.Cluster.Name = "prod"
	cfg.Cluster.Namespaces = []string{"default"}
	cfg.Report.OutputDirectory = "."

	in := Input{
		ID:               "run-defaults",
		GeneratorVersion: "kcr-0.0.0",
		EmittedAt:        time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Config:           cfg,
		EndpointKind:     "prometheus",
		Scope: discovery.Scope{
			ClusterName:      "prod",
			ClusterLabel:     cfg.Cluster.Label,
			FilterMode:       cfg.Cluster.FilterMode,
			MatcherEnabled:   true,
			Confidence:       discovery.ConfidenceExact,
			DiscoveredValues: []string{"prod"},
		},
		Capabilities:        baseCapabilities(),
		CollectionStarted:   time.Date(2025, 12, 31, 23, 55, 0, 0, time.UTC),
		CollectionCompleted: time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC),
	}
	in.Window = windowFor(in.Config.Analysis, in.EmittedAt)
	in.Collected = collect.Collected{
		Autoscaling: unknownAutoscaling(),
		Namespaces:  collect.Result[collect.Namespace]{Rows: baseCollectedNamespaces(in.Config.Cluster.Namespaces)},
	}

	snap, err := Build(in)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}

	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if !reflect.DeepEqual(got, snap) {
		t.Fatalf("Decode(Encode(Build(in))) = %+v, want %+v", got, snap)
	}
}

func TestDecodeAcceptsZeroAndFalse(t *testing.T) {
	in := baseInput()
	in.Config.Metrics.TLSVerify = false
	in.Scope.MatcherEnabled = false
	in.Config.Thresholds.BurstyGapPercent = 0
	in.Config.Thresholds.HPAReplicaDivergence = 0
	in.Config.Thresholds.LowUtilizationPercent = 0

	snap := mustBuild(t, in)
	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if got.Report.Settings.TLSVerify {
		t.Errorf("Settings.TLSVerify = %v, want false", got.Report.Settings.TLSVerify)
	}
	if got.Scope.MatcherEnabled {
		t.Errorf("Scope.MatcherEnabled = %v, want false", got.Scope.MatcherEnabled)
	}
	if got.Report.Thresholds.BurstyGapPercent != 0 {
		t.Errorf("Thresholds.BurstyGapPercent = %v, want 0", got.Report.Thresholds.BurstyGapPercent)
	}
	if got.Report.Thresholds.HPAReplicaDivergence != 0 {
		t.Errorf("Thresholds.HPAReplicaDivergence = %v, want 0", got.Report.Thresholds.HPAReplicaDivergence)
	}
	if got.Report.Thresholds.LowUtilizationPercent != 0 {
		t.Errorf("Thresholds.LowUtilizationPercent = %v, want 0", got.Report.Thresholds.LowUtilizationPercent)
	}
}

// typePaths walks t as Decode's checkKeys does, but statically, with no
// decoded data behind it: at a slice whose element is a struct, it stops
// at the slice field's own path, since the number of elements a real file
// holds is a property of the data, not of the type. Every field of every
// snapshot type is supported, so the ok of supportedKey is not checked
// here; TestKeyWalkRefusesWhatItDoesNotKnow exercises the case where it is
// false, directly against checkKeys.
func typePaths(t reflect.Type, prefix string) []string {
	var paths []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name, _ := supportedKey(field)
		if name == "schema_version" {
			continue
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		paths = append(paths, path)
		if field.Type.Kind() == reflect.Struct && field.Type != reflect.TypeOf(Value{}) {
			paths = append(paths, typePaths(field.Type, path)...)
		}
	}
	return paths
}

// contractKeyPaths lists every key path the contract's types declare, by
// reflect, in the order the types declare them: every field of Snapshot at
// every level (schema_version excluded, since it carries its own dedicated
// message), plus the fields of one Capability element, at index, so that a
// concrete test case can be built for it without needing a decoded file of
// a particular length.
func contractKeyPaths(index int) []string {
	paths := typePaths(reflect.TypeOf(Snapshot{}), "")

	capPrefix := fmt.Sprintf("quality.capabilities[%d]", index)
	for _, p := range typePaths(reflect.TypeOf(Capability{}), "") {
		paths = append(paths, capPrefix+"."+p)
	}
	for _, p := range typePaths(reflect.TypeOf(Recommendation{}), "") {
		paths = append(paths, "recommendations[0]."+p)
	}
	return paths
}

// splitIndex splits a path segment such as "capabilities[3]" into its name
// and index; a segment with no "[" carries no index.
func splitIndex(seg string) (name string, idx int, hasIdx bool) {
	open := strings.IndexByte(seg, '[')
	if open < 0 {
		return seg, 0, false
	}
	closeAt := strings.IndexByte(seg, ']')
	n, err := strconv.Atoi(seg[open+1 : closeAt])
	if err != nil {
		return seg, 0, false
	}
	return seg[:open], n, true
}

// removeKey deletes the key named by path (dot-separated, with an optional
// "[N]" on a segment that names an array) from tree, the generic decoding
// of a document.
func removeKey(tree map[string]any, path string) {
	segments := strings.Split(path, ".")
	var cur any = tree

	for i, seg := range segments {
		name, idx, hasIdx := splitIndex(seg)
		m, ok := cur.(map[string]any)
		if !ok {
			return
		}
		if i == len(segments)-1 {
			delete(m, name)
			return
		}

		next, ok := m[name]
		if !ok {
			return
		}
		if hasIdx {
			arr, ok := next.([]any)
			if !ok || idx >= len(arr) {
				return
			}
			cur = arr[idx]
		} else {
			cur = next
		}
	}
}

// removeJSONKey decodes good into a generic tree, deletes the key at path,
// and re-encodes it.
func removeJSONKey(t *testing.T, good []byte, path string) []byte {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	removeKey(tree, path)
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}
	return data
}

// setKeyNull sets the key named by path (dot-separated, with an optional
// "[N]" on a segment that names an array) to JSON null in tree, the
// generic decoding of a document, in place of removeKey's delete.
func setKeyNull(tree map[string]any, path string) {
	segments := strings.Split(path, ".")
	var cur any = tree

	for i, seg := range segments {
		name, idx, hasIdx := splitIndex(seg)
		m, ok := cur.(map[string]any)
		if !ok {
			return
		}
		if i == len(segments)-1 {
			m[name] = nil
			return
		}

		next, ok := m[name]
		if !ok {
			return
		}
		if hasIdx {
			arr, ok := next.([]any)
			if !ok || idx >= len(arr) {
				return
			}
			cur = arr[idx]
		} else {
			cur = next
		}
	}
}

// nullJSONKey decodes good into a generic tree, sets the value at path to
// null, and re-encodes it.
func nullJSONKey(t *testing.T, good []byte, path string) []byte {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	setKeyNull(tree, path)
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}
	return data
}

func TestDecodeRefusesAMissingKey(t *testing.T) {
	snap := fullSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for _, path := range contractPaths {
		t.Run(path, func(t *testing.T) {
			data := removeJSONKey(t, good, path)

			_, err := Decode(data)
			want := fmt.Sprintf("snapshot lacks the key %s", path)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// contractPaths is the 80 key paths of the snapshot contract, written by
// hand, in the order the types declare them. TestKeyPathsAreTheContract
// compares it against contractKeyPaths(3), the same list built by walking
// the types with reflect, so that the two can actually drift apart if a
// field is ever added to a type without a matching entry here — unlike
// comparing contractKeyPaths against itself, which could never fail.
var contractPaths = []string{
	"report",
	"report.id",
	"report.generator_version",
	"report.emitted_at",
	"report.period_start",
	"report.period_end",
	"report.timezone",
	"report.profile",
	"report.min_severity",
	"report.organization",
	"report.classification",
	"report.author",
	"report.thresholds",
	"report.thresholds.warning_percent",
	"report.thresholds.critical_percent",
	"report.thresholds.low_utilization_percent",
	"report.thresholds.autoscaler_near_max_percent",
	"report.thresholds.nodepool_spread_points",
	"report.thresholds.bursty_gap_percent",
	"report.thresholds.hpa_replica_divergence",
	"report.settings",
	"report.settings.endpoint_type",
	"report.settings.tls_verify",
	"report.settings.timeout_seconds",
	"report.settings.node_autoscaling",
	"report.settings.nodepool_label",
	"scope",
	"scope.cluster_name",
	"scope.cluster_label",
	"scope.filter_mode",
	"scope.matcher_enabled",
	"scope.confidence",
	"scope.discovered_values",
	"scope.namespaces",
	"scope.excluded_containers",
	"quality",
	"quality.capabilities",
	"quality.warnings",
	"quality.notices",
	"quality.collection_started_at",
	"quality.collection_completed_at",
	"summary",
	"summary.namespaces",
	"summary.workloads",
	"summary.hpas",
	"summary.nodes",
	"summary.nodepools",
	"summary.findings",
	"summary.recommendations",
	"summary.healthy",
	"summary.attention",
	"summary.critical",
	"summary.workloads_without_hpa",
	"summary.autoscaling",
	"summary.autoscaling.status",
	"summary.autoscaling.source",
	"summary.autoscaling.confidence",
	"summary.autoscaling.managed_node_groups",
	"summary.autoscaling.current_nodes",
	"summary.autoscaling.min_nodes",
	"summary.autoscaling.max_nodes",
	"summary.autoscaling.scale_events",
	"summary.autoscaling.unschedulable_pods",
	"namespaces",
	"workloads",
	"hpas",
	"nodes",
	"nodepools",
	"findings",
	"recommendations",
	"quality.capabilities[3].name",
	"quality.capabilities[3].state",
	"quality.capabilities[3].strategy",
	"quality.capabilities[3].evidence",
	"quality.capabilities[3].reasons",
	"recommendations[0].code",
	"recommendations[0].severity",
	"recommendations[0].scope_type",
	"recommendations[0].scopes",
	"recommendations[0].confidence",
}

// nullableContractPaths holds the paths of contractPaths whose value is a
// number that may be unknown, so that null is a value of it.
var nullableContractPaths = map[string]bool{
	"summary.autoscaling.managed_node_groups": true,
	"summary.autoscaling.current_nodes":       true,
	"summary.autoscaling.min_nodes":           true,
	"summary.autoscaling.max_nodes":           true,
	"summary.autoscaling.scale_events":        true,
	"summary.autoscaling.unschedulable_pods":  true,
}

// TestKeyPathsAreTheContract compares contractPaths, written by hand, with
// the same list built by walking the types with reflect. This replaces
// TestDecodeMissingKeyTableCoversEveryField, which compared the walk with
// itself and so could never catch a field added to a type without a
// matching row somewhere else.
func TestKeyPathsAreTheContract(t *testing.T) {
	byReflection := contractKeyPaths(3)

	if !reflect.DeepEqual(contractPaths, byReflection) {
		t.Fatalf("contractPaths and the walk of the types differ:\nhand-written: %v\nby reflect:   %v", contractPaths, byReflection)
	}
}

func TestDecodeReportsTheFirstMissingKey(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	removeKey(tree, "report.id")
	removeKey(tree, "scope.cluster_name")
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}

	_, err = Decode(data)
	want := "snapshot lacks the key report.id"
	if err == nil {
		t.Fatalf("Decode() = nil, want error %q", want)
	}
	if err.Error() != want {
		t.Fatalf("Decode() = %q, want %q", err.Error(), want)
	}
}

// TestDecodeRefusesNull mirrors TestDecodeRefusesAMissingKey exactly, but
// leaves every key in place and sets its value to JSON null instead of
// removing it: the writer never writes null, so a snapshot holding one at
// any of the 80 paths of the contract must be refused, by its own message.
func TestDecodeRefusesNull(t *testing.T) {
	snap := fullSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for _, path := range contractPaths {
		t.Run(path, func(t *testing.T) {
			data := nullJSONKey(t, good, path)

			_, err := Decode(data)
			if nullableContractPaths[path] {
				if err != nil {
					t.Fatalf("Decode() error = %v, want nil: a number that is not known is null", err)
				}
				return
			}
			want := fmt.Sprintf("snapshot holds null at %s", path)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestDecodeRefusesNullListElement(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	t.Run("scope.discovered_values[1]", func(t *testing.T) {
		var tree map[string]any
		if err := json.Unmarshal(good, &tree); err != nil {
			t.Fatalf("Unmarshal(good): %v", err)
		}
		tree["scope"].(map[string]any)["discovered_values"] = []any{"prod", nil}
		data, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("Marshal(tree): %v", err)
		}

		_, err = Decode(data)
		want := "snapshot holds null at scope.discovered_values[1]"
		if err == nil {
			t.Fatalf("Decode() = nil, want error %q", want)
		}
		if err.Error() != want {
			t.Fatalf("Decode() = %q, want %q", err.Error(), want)
		}
	})

	t.Run("quality.capabilities[0].evidence[0]", func(t *testing.T) {
		var tree map[string]any
		if err := json.Unmarshal(good, &tree); err != nil {
			t.Fatalf("Unmarshal(good): %v", err)
		}
		capabilities := tree["quality"].(map[string]any)["capabilities"].([]any)
		capabilities[0].(map[string]any)["evidence"] = []any{nil}
		data, err := json.Marshal(tree)
		if err != nil {
			t.Fatalf("Marshal(tree): %v", err)
		}

		_, err = Decode(data)
		want := "snapshot holds null at quality.capabilities[0].evidence[0]"
		if err == nil {
			t.Fatalf("Decode() = nil, want error %q", want)
		}
		if err.Error() != want {
			t.Fatalf("Decode() = %q, want %q", err.Error(), want)
		}
	})
}

func TestDecodeRefusesNullCapability(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	capabilities := tree["quality"].(map[string]any)["capabilities"].([]any)
	capabilities[0] = nil
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}

	_, err = Decode(data)
	want := "snapshot holds null at quality.capabilities[0]"
	if err == nil {
		t.Fatalf("Decode() = nil, want error %q", want)
	}
	if err.Error() != want {
		t.Fatalf("Decode() = %q, want %q", err.Error(), want)
	}
}

// The test types below exist only to give TestKeyWalkRefusesWhatItDoesNotKnow
// a field of a shape checkKeys does not support: a pointer, a map, a
// missing tag, a "-" tag, a tag with an option, and an embedded struct.
type keyWalkPointerField struct {
	Foo *string `json:"foo"`
}

type keyWalkMapField struct {
	Foo map[string]string `json:"foo"`
}

// keyWalkIntKeyMapField gives TestKeyWalkRefusesOtherPointersAndMaps a map
// whose value type is float64, the one the walk supports, but whose key
// type is not a text.
type keyWalkIntKeyMapField struct {
	Foo map[int]float64 `json:"foo"`
}

type keyWalkNoTag struct {
	Foo string
}

type keyWalkDashTag struct {
	Foo string `json:"-"`
}

type keyWalkOptionTag struct {
	Foo string `json:"foo,omitempty"`
}

type keyWalkEmbeddedPart struct {
	Bar string `json:"bar"`
}

type keyWalkEmbedded struct {
	keyWalkEmbeddedPart
}

func TestKeyWalkRefusesWhatItDoesNotKnow(t *testing.T) {
	tests := []struct {
		name  string
		typ   reflect.Type
		field string
	}{
		{"pointer field", reflect.TypeOf(keyWalkPointerField{}), "Foo"},
		{"map field", reflect.TypeOf(keyWalkMapField{}), "Foo"},
		{"no json tag", reflect.TypeOf(keyWalkNoTag{}), "Foo"},
		{"a \"-\" tag", reflect.TypeOf(keyWalkDashTag{}), "Foo"},
		{"a tag with an option", reflect.TypeOf(keyWalkOptionTag{}), "Foo"},
		{"an embedded struct", reflect.TypeOf(keyWalkEmbedded{}), "keyWalkEmbeddedPart"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKeys(tt.typ, map[string]any{}, "")
			want := fmt.Sprintf(unsupportedKeyFmt, tt.typ.Name(), tt.field)
			if err == nil {
				t.Fatalf("checkKeys() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("checkKeys() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// TestKeyWalkRefusesOtherPointersAndMaps covers that the walk's support
// for *float64 and map[string]float64 does not widen to any other pointer
// or any other map: a *string and a map[string]string are refused exactly
// as TestKeyWalkRefusesWhatItDoesNotKnow already covers for the shapes
// the walk does not support.
func TestKeyWalkRefusesOtherPointersAndMaps(t *testing.T) {
	tests := []struct {
		name string
		typ  reflect.Type
	}{
		{"a pointer that is not *float64", reflect.TypeOf(keyWalkPointerField{})},
		{"a map that is not map[string]float64", reflect.TypeOf(keyWalkMapField{})},
		{"a map whose key is not a text", reflect.TypeOf(keyWalkIntKeyMapField{})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKeys(tt.typ, map[string]any{}, "")
			want := fmt.Sprintf(unsupportedKeyFmt, tt.typ.Name(), "Foo")
			if err == nil {
				t.Fatalf("checkKeys() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("checkKeys() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// validSnapshotWithRows returns a Snapshot that passes Check(), with one
// namespace, one workload, one notice, and the eight findings of the
// "direct-complete" scenario (see directCompleteResources in
// build_test.go), so a table test can mutate an element of any of the new
// lists.
func validSnapshotWithRows(t *testing.T) Snapshot {
	t.Helper()
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}

	resources := directCompleteResources()
	namespaces := []collect.Namespace{{
		Name: "payments", Pods: 3, PodsKnown: true,
		RequestsLimitsState: discovery.StateAvailable, Resources: resources,
	}}
	workloads := []collect.Workload{{
		Namespace: "payments", Kind: "Deployment", Name: "payments-web",
		Pods: 1, PodsKnown: true,
		RequestsLimitsState: discovery.StateAvailable, Resources: resources,
	}}
	collected := collect.Collected{
		Autoscaling: unknownAutoscaling(),
		Namespaces: collect.Result[collect.Namespace]{
			Rows:    namespaces,
			Notices: []collect.Message{{Code: collect.NoticeJobsOmitted, Params: map[string]float64{"count": 2}}},
		},
		Workloads: collect.Result[collect.Workload]{Rows: workloads},
	}
	in.Collected = collected
	in.Findings = analysis.All(collected, in.Config.Thresholds)

	return mustBuild(t, in)
}

// unavailableSnapshotWorkload returns a Workload of the snapshot's own
// type, every one of its ten statistics of state unavailable, so a table
// test can add a second, well-formed row to an already-built Snapshot.
func unavailableSnapshotWorkload(namespace, kind, name string) Workload {
	u := Statistics{State: collect.StateUnavailable}
	return Workload{
		Namespace: namespace, Kind: kind, Name: name,
		ObservedUsage: true, HPACoverage: CoverageUnknown,
		CPUUsage: u, MemoryUsage: u, CPURequests: u, MemoryRequests: u,
		CPULimits: u, MemoryLimits: u, CPURequestPercent: u, MemoryRequestPercent: u,
		CPULimitPercent: u, MemoryLimitPercent: u,
	}
}

// fullSnapshot returns a Snapshot that passes Check(), with every new
// section filled: one namespace and two workloads of one name kept apart
// by kind, a finding of the namespace and one of a workload, each with
// evidence, one warning, one notice with its count, a statistic of each
// of the three states, and a pod count that is known (the namespace, and
// the workload "Deployment/api") and one that is not (the workload
// "StatefulSet/api").
func fullSnapshot(t *testing.T) Snapshot {
	t.Helper()
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}

	namespaces := []collect.Namespace{{
		Name: "payments", Pods: 3, PodsKnown: true,
		RequestsLimitsState: discovery.StateAvailable,
		Resources: collect.Resources{
			CPUUsage:             collect.Statistics{State: collect.StateValue, Average: 0.3, Minimum: 0.2, Peak: 0.4, Current: 0.4},
			MemoryUsage:          collect.Statistics{State: collect.StateUnavailable},
			CPURequests:          collect.Statistics{State: collect.StateUnavailable},
			MemoryRequests:       collect.Statistics{State: collect.StateUnavailable},
			CPULimits:            collect.Statistics{State: collect.StateUnavailable},
			MemoryLimits:         collect.Statistics{State: collect.StateUnavailable},
			CPURequestPercent:    collect.Statistics{State: collect.StateValue, Average: 60, Minimum: 40, Peak: 80, Current: 80},
			MemoryRequestPercent: collect.Statistics{State: collect.StateNoBase},
			CPULimitPercent:      collect.Statistics{State: collect.StateValue, Average: 30, Minimum: 20, Peak: 40, Current: 40},
			MemoryLimitPercent:   collect.Statistics{State: collect.StateUnavailable},
		},
	}}
	workloads := []collect.Workload{
		{
			Namespace: "payments", Kind: "Deployment", Name: "api",
			Pods: 2, PodsKnown: true,
			RequestsLimitsState: discovery.StateAvailable, Resources: unavailableResources(),
		},
		{
			Namespace: "payments", Kind: "StatefulSet", Name: "api",
			PodsKnown:           false,
			RequestsLimitsState: discovery.StateAvailable, Resources: unavailableResources(),
		},
	}

	in.Collected = collect.Collected{
		Autoscaling: unknownAutoscaling(),
		Namespaces: collect.Result[collect.Namespace]{
			Rows:     namespaces,
			Warnings: []collect.Message{{Code: collect.WarningNamespaceUsageIncomplete, Params: map[string]float64{}}},
			Notices:  []collect.Message{{Code: collect.NoticeJobsOmitted, Params: map[string]float64{"count": 2}}},
		},
		Workloads: collect.Result[collect.Workload]{Rows: workloads},
	}
	in.Findings = []analysis.Finding{
		{
			Code: "NAMESPACE_CPU_REQUEST_ATTENTION", Severity: analysis.SeverityAttention,
			ScopeType: analysis.ScopeNamespace, ScopeKind: "", ScopeName: "payments",
			Confidence: discovery.ConfidenceExact,
			Evidence:   []analysis.Evidence{{Name: "average_percent", Value: analysis.Number(60)}, {Name: "peak_percent", Value: analysis.Number(80)}},
		},
		{
			Code: "WORKLOAD_CPU_LIMIT_HEALTHY", Severity: analysis.SeverityHealthy,
			ScopeType: analysis.ScopeWorkload, ScopeKind: "Deployment", ScopeName: "payments/api",
			Confidence: discovery.ConfidenceExact,
			Evidence:   []analysis.Evidence{{Name: "average_percent", Value: analysis.Number(30)}, {Name: "peak_percent", Value: analysis.Number(40)}},
		},
	}

	return mustBuild(t, in)
}

// TestEncodeDecodeRoundTrip covers Decode(Encode(s)) == s, compared
// deeply, and that re-encoding the decoded snapshot gives the same bytes
// as the first encoding, compared byte for byte, for a snapshot with
// every new section filled and for one with every new section empty.
//
// Rule 5 makes the second case's namespaces impossible to leave empty:
// scope.namespaces is never empty, and the snapshot's own namespaces must
// name exactly the namespaces of the scope. So "every new section empty"
// here means every one but namespaces: the snapshot has one row for each
// namespace of its scope, every statistic unavailable, and no workload,
// no finding, no warning, no notice; this is exactly validSnapshot's own
// baseline.
func TestEncodeDecodeRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		snap Snapshot
	}{
		{"every new section filled", fullSnapshot(t)},
		{"every new section empty but the namespaces of the scope", validSnapshot(t)},
		{"two recommendations", clusterSnapshot(t)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, err := Encode(tt.snap)
			if err != nil {
				t.Fatalf("Encode() error = %v", err)
			}

			got, err := Decode(first)
			if err != nil {
				t.Fatalf("Decode() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.snap) {
				t.Fatalf("Decode(Encode(s)) = %+v, want %+v", got, tt.snap)
			}

			second, err := Encode(got)
			if err != nil {
				t.Fatalf("Encode(Decode(Encode(s))) error = %v", err)
			}
			if string(second) != string(first) {
				t.Fatalf("Encode(Decode(Encode(s))) != Encode(s):\n%s\nvs\n%s", second, first)
			}
		})
	}
}

// TestDecodeAcceptsNullNumber covers that a *float64 field, such as
// namespaces[0].pods, reads a JSON null as a nil pointer.
func TestDecodeAcceptsNullNumber(t *testing.T) {
	snap := validSnapshotWithRows(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	data := nullJSONKey(t, good, "namespaces[0].pods")

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if got.Namespaces[0].Pods != nil {
		t.Fatalf("Namespaces[0].Pods = %v, want nil", *got.Namespaces[0].Pods)
	}
}

// TestDecodeRefusesNullElsewhere covers that null is refused, by path, at a
// text, at a list, at an object of parameters, and inside it: the only
// place null is accepted is a *float64 field, covered by
// TestDecodeAcceptsNullNumber.
func TestDecodeRefusesNullElsewhere(t *testing.T) {
	snap := validSnapshotWithRows(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"a text", "namespaces[0].name"},
		{"a list", "findings[0].evidence"},
		{"an object of parameters", "quality.notices[0].params"},
		{"inside an object of parameters", "quality.notices[0].params.count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := nullJSONKey(t, good, tt.path)

			_, err := Decode(data)
			want := fmt.Sprintf("snapshot holds null at %s", tt.path)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// setKeyText sets the key named by path (dot-separated, with an optional
// "[N]" on a segment that names an array) to a JSON string in tree, the
// generic decoding of a document, in place of setKeyNull's null.
func setKeyText(tree map[string]any, path, value string) {
	segments := strings.Split(path, ".")
	var cur any = tree

	for i, seg := range segments {
		name, idx, hasIdx := splitIndex(seg)
		m, ok := cur.(map[string]any)
		if !ok {
			return
		}
		if i == len(segments)-1 {
			m[name] = value
			return
		}

		next, ok := m[name]
		if !ok {
			return
		}
		if hasIdx {
			arr, ok := next.([]any)
			if !ok || idx >= len(arr) {
				return
			}
			cur = arr[idx]
		} else {
			cur = next
		}
	}
}

// textJSONKey decodes good into a generic tree, sets the value at path to
// the JSON string value, and re-encodes it.
func textJSONKey(t *testing.T, good []byte, path, value string) []byte {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	setKeyText(tree, path, value)
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}
	return data
}

// TestDecodeRefusesTextAsNumber covers a text where a *float64 field and
// where a map[string]float64 entry expect a number.
func TestDecodeRefusesTextAsNumber(t *testing.T) {
	snap := validSnapshotWithRows(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"a *float64 field", "namespaces[0].cpu_usage.average"},
		{"an entry of a map[string]float64 field", "quality.notices[0].params.count"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := textJSONKey(t, good, tt.path, "1")

			_, err := Decode(data)
			want := fmt.Sprintf("snapshot holds a value of the wrong kind at %s", tt.path)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// TestDecodeRefusesUnknownKeyInNewSections covers a forged key beside a
// real one in each of the six new shapes: a namespace, a workload, a
// statistic, a finding, an evidence, and a message.
func TestDecodeRefusesUnknownKeyInNewSections(t *testing.T) {
	snap := validSnapshotWithRows(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"a namespace", "namespaces[0]"},
		{"a workload", "workloads[0]"},
		{"a statistic", "namespaces[0].cpu_usage"},
		{"a finding", "findings[0]"},
		{"an evidence", "findings[0].evidence[0]"},
		{"a message", "quality.notices[0]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tree map[string]any
			if err := json.Unmarshal(good, &tree); err != nil {
				t.Fatalf("Unmarshal(good): %v", err)
			}
			parent, _ := parentAndKey(t, tree, tt.path+".bogus")
			parent["bogus"] = true
			data, err := json.Marshal(tree)
			if err != nil {
				t.Fatalf("Marshal(tree): %v", err)
			}

			_, err = Decode(data)
			want := fmt.Sprintf("snapshot holds an unknown key: %s.bogus", tt.path)
			if err == nil {
				t.Fatalf("Decode() = nil, want error %q", want)
			}
			if err.Error() != want {
				t.Fatalf("Decode() = %q, want %q", err.Error(), want)
			}
		})
	}
}

// TestDecodeRefusesMissingNewSection reads
// testdata/schema-2.0-before-rows.json, a snapshot whose top level holds
// no namespaces, workloads, or findings, and whose quality holds no
// warnings or notices: checkKeys visits quality's fields in their
// declared order, so the first one this fixture lacks, warnings, is the
// one Decode reports.
func TestDecodeRefusesMissingNewSection(t *testing.T) {
	data, err := os.ReadFile("testdata/schema-2.0-before-rows.json")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	_, err = Decode(data)
	want := "snapshot lacks the key quality.warnings"
	if err == nil {
		t.Fatalf("Decode() = nil, want error %q", want)
	}
	if err.Error() != want {
		t.Fatalf("Decode() = %q, want %q", err.Error(), want)
	}
}

// TestEncodeSortsParams covers that an object of parameters is written
// with its keys sorted, whatever order Params holds them in: this is
// Go's own encoding/json behaviour for a map, so the test exercises it
// with more than one key to actually observe the order.
func TestEncodeSortsParams(t *testing.T) {
	snap := validSnapshot(t)
	snap.Quality.Notices = []Message{{
		Code:   collect.NoticeJobsOmitted,
		Params: map[string]float64{"zebra": 1, "alpha": 2, "middle": 3},
	}}

	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	alpha := strings.Index(string(data), `"alpha"`)
	middle := strings.Index(string(data), `"middle"`)
	zebra := strings.Index(string(data), `"zebra"`)
	if alpha < 0 || middle < 0 || zebra < 0 {
		t.Fatalf("Encode() does not hold every key: %s", data)
	}
	if alpha >= middle || middle >= zebra {
		t.Fatalf("Encode() does not sort the keys of params: %s", data)
	}
}

// TestCheckAcceptsTwoWorkloadsOfOneName covers that two workloads of one
// namespace and one name, kept apart by kind, pass Check: rule 6 refuses a
// repeat only when namespace, name, and kind all agree.
func TestCheckAcceptsTwoWorkloadsOfOneName(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}
	in.Collected = collect.Collected{
		Autoscaling: unknownAutoscaling(),
		Namespaces: collect.Result[collect.Namespace]{Rows: []collect.Namespace{
			{Name: "payments", Resources: unavailableResources()},
		}},
		Workloads: collect.Result[collect.Workload]{Rows: []collect.Workload{
			{Namespace: "payments", Kind: "Deployment", Name: "api", Resources: unavailableResources()},
			{Namespace: "payments", Kind: "StatefulSet", Name: "api", Resources: unavailableResources()},
		}},
	}

	if _, err := Build(in); err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
}

// TestCheckRules covers each of the nine rules Check gained for the
// namespaces, the workloads, and the findings, and each part of a rule
// that carries its own message, against validSnapshotWithRows: one
// namespace ("payments"), one workload ("payments/Deployment/payments-web"),
// one notice, and the eight findings of the "direct-complete" scenario.
func TestCheckRules(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		message string
	}{
		// Rule 1: a warning or a notice code is in its registry; no message
		// stands twice in its list.
		{"quality.warnings[0].code unknown", func(s *Snapshot) {
			s.Quality.Warnings = []Message{{Code: "BOGUS", Params: map[string]float64{}}}
		}, `quality.warnings[0].code has an unknown value: "BOGUS"`},
		{"quality.notices[0].code unknown", func(s *Snapshot) {
			s.Quality.Notices = []Message{{Code: "BOGUS", Params: map[string]float64{}}}
		}, `quality.notices[0].code has an unknown value: "BOGUS"`},
		{"quality.warnings holds a repeated message", func(s *Snapshot) {
			s.Quality.Warnings = []Message{
				{Code: collect.WarningNamespaceUsageIncomplete, Params: map[string]float64{}},
				{Code: collect.WarningNamespaceUsageIncomplete, Params: map[string]float64{}},
			}
		}, fmt.Sprintf("quality.warnings holds a repeated message: %q", collect.WarningNamespaceUsageIncomplete)},
		{"quality.notices holds a repeated message", func(s *Snapshot) {
			s.Quality.Notices = []Message{
				{Code: collect.NoticeJobsOmitted, Params: map[string]float64{"count": 1}},
				{Code: collect.NoticeJobsOmitted, Params: map[string]float64{"count": 1}},
			}
		}, fmt.Sprintf("quality.notices holds a repeated message: %q", collect.NoticeJobsOmitted)},

		// Rule 2: a notice's parameters are exactly the names of
		// collect.NoticeParams; a warning has none; a count is a whole
		// number above zero.
		{"quality.warnings[0].params must be empty", func(s *Snapshot) {
			s.Quality.Warnings = []Message{{Code: collect.WarningNamespaceUsageIncomplete, Params: map[string]float64{"x": 1}}}
		}, "quality.warnings[0].params must be empty"},
		{"quality.notices[0].params must hold exactly", func(s *Snapshot) {
			s.Quality.Notices[0].Params = map[string]float64{}
		}, "quality.notices[0].params must hold exactly: count"},
		{"quality.notices[0].params.count not whole", func(s *Snapshot) {
			s.Quality.Notices[0].Params = map[string]float64{"count": 1.5}
		}, "quality.notices[0].params.count must be a whole number above zero"},
		{"quality.notices[0].params.count not above zero", func(s *Snapshot) {
			s.Quality.Notices[0].Params = map[string]float64{"count": 0}
		}, "quality.notices[0].params.count must be a whole number above zero"},

		// Rule 3: the state of a statistic is one of the three; value has a
		// number; any other state has none; no number is NaN or infinite.
		{"namespaces[0].cpu_usage.state unknown", func(s *Snapshot) {
			s.Namespaces[0].CPUUsage.State = "bogus"
		}, `namespaces[0].cpu_usage.state has an unknown value: "bogus"`},
		{"namespaces[0].cpu_usage has state value and no number", func(s *Snapshot) {
			s.Namespaces[0].CPUUsage = Statistics{State: collect.StateValue}
		}, `namespaces[0].cpu_usage has the state value and lacks a number`},
		{"namespaces[0].cpu_usage.average set for a state that is not value", func(s *Snapshot) {
			s.Namespaces[0].CPUUsage.State = collect.StateUnavailable
		}, `namespaces[0].cpu_usage.average is set for a state that is not "value"`},
		{"namespaces[0].cpu_usage.average not finite", func(s *Snapshot) {
			s.Namespaces[0].CPUUsage.Average = ptr(math.NaN())
		}, "namespaces[0].cpu_usage.average is not a finite number"},

		// Rule 4: a pod count, when present, is a whole number, zero or
		// above.
		{"namespaces[0].pods not whole", func(s *Snapshot) {
			s.Namespaces[0].Pods = ptr(2.5)
		}, "namespaces[0].pods must be a whole number, zero or above"},
		{"namespaces[0].pods negative", func(s *Snapshot) {
			s.Namespaces[0].Pods = ptr(-1)
		}, "namespaces[0].pods must be a whole number, zero or above"},

		// Rule 5: the namespaces are sorted, none twice, and are exactly
		// the namespaces of the scope.
		{"namespaces not sorted or holds a repeat", func(s *Snapshot) {
			s.Namespaces = []Namespace{s.Namespaces[0], s.Namespaces[0]}
		}, "namespaces is not sorted or holds a repeated value"},
		{"namespaces does not match scope.namespaces", func(s *Snapshot) {
			s.Namespaces[0].Name = "other"
		}, "namespaces does not name exactly the namespaces of scope.namespaces"},

		// Rule 6: the workloads are sorted, none twice; each has a name, a
		// kind, and a namespace of the scope; none has the kind Job.
		{"workloads not sorted or holds a repeat", func(s *Snapshot) {
			s.Workloads = append(s.Workloads, s.Workloads[0])
		}, "workloads is not sorted or holds a repeated value"},
		{"workloads[0].name empty", func(s *Snapshot) {
			s.Workloads[0].Name = ""
		}, "workloads[0].name is empty"},
		{"workloads[0].kind empty", func(s *Snapshot) {
			s.Workloads[0].Kind = ""
		}, "workloads[0].kind is empty"},
		{"workloads[0].namespace not of the scope", func(s *Snapshot) {
			s.Workloads[0].Namespace = "other-ns"
		}, "workloads[0].namespace is not a namespace of the scope"},
		{"workloads[0].kind is Job", func(s *Snapshot) {
			s.Workloads[0].Kind = "Job"
		}, "workloads[0].kind must not be Job"},

		// Rule 7: a finding agrees with its definition of analysis.Registry;
		// its confidence is one of the three; its evidence holds no number
		// that is NaN or infinite.
		{"findings[0].code unknown", func(s *Snapshot) {
			s.Findings[0].Code = "BOGUS_CODE"
		}, `findings[0].code has an unknown value: "BOGUS_CODE"`},
		{"findings[0].severity does not match its definition", func(s *Snapshot) {
			s.Findings[0].Severity = analysis.SeverityHealthy
			s.Summary.Critical--
			s.Summary.Healthy++
		}, "findings[0].severity does not match the definition of NAMESPACE_MEMORY_LIMIT_CRITICAL"},
		{"findings[0].scope_type does not match its definition", func(s *Snapshot) {
			s.Findings[0].ScopeType = analysis.ScopeWorkload
		}, "findings[0].scope_type does not match the definition of NAMESPACE_MEMORY_LIMIT_CRITICAL"},
		{"findings[0].evidence does not match its definition", func(s *Snapshot) {
			s.Findings[0].Evidence = []Evidence{{Name: "only_one", Value: numberValue(1)}}
		}, "findings[0].evidence does not match the definition of NAMESPACE_MEMORY_LIMIT_CRITICAL"},
		{"findings[0].evidence in another order", func(s *Snapshot) {
			s.Findings[0].Evidence = []Evidence{
				{Name: "peak_percent", Value: numberValue(12000)},
				{Name: "average_percent", Value: numberValue(11000)},
			}
		}, "findings[0].evidence does not match the definition of NAMESPACE_MEMORY_LIMIT_CRITICAL"},
		{"findings[0].evidence holds one name twice", func(s *Snapshot) {
			s.Findings[0].Evidence = []Evidence{
				{Name: "average_percent", Value: numberValue(11000)},
				{Name: "average_percent", Value: numberValue(12000)},
			}
		}, "findings[0].evidence does not match the definition of NAMESPACE_MEMORY_LIMIT_CRITICAL"},
		{"findings[0].confidence unknown", func(s *Snapshot) {
			s.Findings[0].Confidence = "bogus"
		}, `findings[0].confidence has an unknown value: "bogus"`},
		{"findings[0].evidence[0].value not finite", func(s *Snapshot) {
			s.Findings[0].Evidence[0].Value = numberValue(math.Inf(1))
		}, "findings[0].evidence[0].value is not a finite number"},

		// Rule 8: a namespace finding names a namespace of the snapshot and
		// has an empty kind; a workload finding names a workload of the
		// snapshot.
		{"findings[0].scope_kind must be empty for a namespace finding", func(s *Snapshot) {
			s.Findings[0].ScopeKind = "Deployment"
		}, "findings[0].scope_kind must be empty for a namespace finding"},
		{"findings[0].scope_name not a namespace of the snapshot", func(s *Snapshot) {
			s.Findings[0].ScopeName = "unknown-namespace"
		}, "findings[0].scope_name is not a namespace of the snapshot"},
		{"findings[2].scope_name not a workload of the snapshot", func(s *Snapshot) {
			s.Findings[2].ScopeName = "payments/unknown-workload"
		}, "findings[2].scope_name is not a workload of the snapshot"},
		{"findings[8].scope_name a workload by name, not by this kind", func(s *Snapshot) {
			// findings[8] (WORKLOAD_CPU_LIMIT_HEALTHY) is the only finding
			// of its severity and scope name, so changing its scope kind
			// cannot also disturb the order rule 9 checks: this case is
			// isolated to rule 8 alone.
			s.Workloads = append(s.Workloads, unavailableSnapshotWorkload("payments", "StatefulSet", "payments-web"))
			s.Summary.Workloads = 2
			s.Findings[8].ScopeKind = "ReplicaSet"
		}, "findings[8].scope_name is not a workload of the snapshot"},

		// Rule 9: the findings are in the order of analysis.Sort, none
		// twice.
		{"findings holds a repeated finding", func(s *Snapshot) {
			s.Findings = append(s.Findings, s.Findings[0])
			s.Summary.Findings++
			s.Summary.Critical++
		}, "findings holds a repeated finding"},
		{"findings not in the order of analysis.Sort", func(s *Snapshot) {
			s.Findings[0], s.Findings[1] = s.Findings[1], s.Findings[0]
		}, "findings is not in the order analysis.Sort gives"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := validSnapshotWithRows(t)
			tt.mutate(&snap)

			err := snap.Check()
			if err == nil {
				t.Fatalf("Check() = nil, want error %q", tt.message)
			}
			if err.Error() != tt.message {
				t.Fatalf("Check() = %q, want %q", err.Error(), tt.message)
			}
		})
	}
}

// TestCheckRefusesASlashInAName covers that rule 5 and rule 6 refuse a
// slash in the name of a namespace, and in the namespace, the name, or the
// kind of a workload: no name of a real Kubernetes namespace or object
// holds one, so such a label is a fault of the endpoint or of the
// configuration.
func TestCheckRefusesASlashInAName(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		message string
	}{
		{"namespaces[0].name", func(s *Snapshot) {
			s.Namespaces[0].Name = "a/b"
		}, `namespaces[0].name holds a slash: "a/b"`},
		{"workloads[0].namespace", func(s *Snapshot) {
			s.Workloads[0].Namespace = "a/b"
		}, `workloads[0].namespace holds a slash: "a/b"`},
		{"workloads[0].name", func(s *Snapshot) {
			s.Workloads[0].Name = "a/b"
		}, `workloads[0].name holds a slash: "a/b"`},
		{"workloads[0].kind", func(s *Snapshot) {
			s.Workloads[0].Kind = "a/b"
		}, `workloads[0].kind holds a slash: "a/b"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snap := validSnapshotWithRows(t)
			tt.mutate(&snap)

			err := snap.Check()
			if err == nil {
				t.Fatalf("Check() = nil, want error %q", tt.message)
			}
			if err.Error() != tt.message {
				t.Fatalf("Check() = %q, want %q", err.Error(), tt.message)
			}
		})
	}
}

// TestCheckKeysAWorkloadByItsFields covers that rule 8 matches a finding to
// a workload by its three fields as a struct, not by a text joined from
// them: two workloads that share one namespace and one name, kept apart
// only by kind, each carry their own findings with no collision, and a
// finding whose kind names neither workload is still refused.
func TestCheckKeysAWorkloadByItsFields(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}
	resources := directCompleteResources()
	collected := collect.Collected{
		Autoscaling: unknownAutoscaling(),
		Namespaces: collect.Result[collect.Namespace]{Rows: []collect.Namespace{
			{Name: "payments", Resources: unavailableResources()},
		}},
		Workloads: collect.Result[collect.Workload]{Rows: []collect.Workload{
			{Namespace: "payments", Kind: "Deployment", Name: "api", Resources: resources},
			{Namespace: "payments", Kind: "StatefulSet", Name: "api", Resources: resources},
		}},
	}
	in.Collected = collected
	in.Findings = analysis.All(collected, in.Config.Thresholds)

	snap := mustBuild(t, in)
	if err := snap.Check(); err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}

	workloadFinding := -1
	for i, f := range snap.Findings {
		if f.ScopeType == analysis.ScopeWorkload {
			workloadFinding = i
			break
		}
	}
	if workloadFinding < 0 {
		t.Fatalf("no workload finding built for either workload, want at least one")
	}

	snap.Findings[workloadFinding].ScopeKind = "ReplicaSet"
	err := snap.Check()
	want := fmt.Sprintf("findings[%d].scope_name is not a workload of the snapshot", workloadFinding)
	if err == nil || err.Error() != want {
		t.Fatalf("Check() = %v, want %q", err, want)
	}
}

// clusterSnapshot returns the snapshot of clusterInput: two workloads and
// a coverage-only row, one HPA, three nodes in two pools, an enabled
// autoscaling, and three findings.
func clusterSnapshot(t *testing.T) Snapshot {
	t.Helper()
	return mustBuild(t, clusterInput())
}

// checkRefuses mutates the cluster snapshot and expects Check to refuse it
// with message.
func checkRefuses(t *testing.T, mutate func(*Snapshot), message string) {
	t.Helper()
	snap := clusterSnapshot(t)
	mutate(&snap)
	err := snap.Check()
	if err == nil {
		t.Fatalf("Check() = nil, want error %q", message)
	}
	if err.Error() != message {
		t.Fatalf("Check() = %q, want %q", err.Error(), message)
	}
}

func TestBuildRefusesAPoolNamingAnAbsentNode(t *testing.T) {
	checkRefuses(t, func(s *Snapshot) {
		s.Nodepools[0].Nodes = []string{"ghost", "node-a1", "node-a2"}
	}, `nodepools[0] (a) names node "ghost", which is not a node of the snapshot`)
}

func TestBuildRefusesANodeInTwoPools(t *testing.T) {
	checkRefuses(t, func(s *Snapshot) {
		s.Nodepools[1].Nodes = []string{"node-a1", "node-b1"}
	}, `node "node-a1" is listed by more than one nodepool`)
}

func TestBuildRefusesAWrongSummary(t *testing.T) {
	checkRefuses(t, func(s *Snapshot) {
		s.Summary.Nodes = 4
	}, "summary.nodes is 4, want 3")
}

func TestBuildRefusesAnAutoscalingContradiction(t *testing.T) {
	checkRefuses(t, func(s *Snapshot) {
		s.Summary.Autoscaling.Status = "unknown"
		s.Summary.Autoscaling.Source = "configured"
	}, "summary.autoscaling is configured and its status is unknown")
}

func TestCheckRefusesTheClusterSections(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		message string
	}{
		{"summary.namespaces", func(s *Snapshot) { s.Summary.Namespaces = 1 }, "summary.namespaces is 1, want 2"},
		{"summary.workloads counts observed rows only", func(s *Snapshot) { s.Summary.Workloads = 3 }, "summary.workloads is 3, want 2"},
		{"summary.hpas", func(s *Snapshot) { s.Summary.HPAs = 0 }, "summary.hpas is 0, want 1"},
		{"summary.nodepools", func(s *Snapshot) { s.Summary.Nodepools = 3 }, "summary.nodepools is 3, want 2"},
		{"summary.findings", func(s *Snapshot) { s.Summary.Findings = 4 }, "summary.findings is 4, want 3"},
		{"summary.healthy", func(s *Snapshot) { s.Summary.Healthy = 2 }, "summary.healthy is 2, want 1"},
		{"summary.attention", func(s *Snapshot) { s.Summary.Attention = 0 }, "summary.attention is 0, want 1"},
		{"summary.critical", func(s *Snapshot) { s.Summary.Critical = 3 }, "summary.critical is 3, want 1"},
		{"summary.workloads_without_hpa", func(s *Snapshot) { s.Summary.WorkloadsWithoutHPA = 0 }, "summary.workloads_without_hpa is 0, want 1"},

		{"autoscaling status", func(s *Snapshot) { s.Summary.Autoscaling.Status = "bogus" }, `summary.autoscaling.status has an unknown value: "bogus"`},
		{"autoscaling source", func(s *Snapshot) { s.Summary.Autoscaling.Source = "bogus" }, `summary.autoscaling.source has an unknown value: "bogus"`},
		{"autoscaling source recording_rule", func(s *Snapshot) { s.Summary.Autoscaling.Source = "recording_rule" }, `summary.autoscaling.source has an unknown value: "recording_rule"`},
		{"autoscaling confidence", func(s *Snapshot) { s.Summary.Autoscaling.Confidence = "bogus" }, `summary.autoscaling.confidence has an unknown value: "bogus"`},
		{"autoscaling unknown with a source", func(s *Snapshot) {
			s.Summary.Autoscaling.Status = "unknown"
			s.Summary.Autoscaling.Source = "raw_metrics"
		}, "summary.autoscaling is unknown and has a source or a confidence other than none and unknown"},
		{"autoscaling unknown with a confidence", func(s *Snapshot) {
			s.Summary.Autoscaling.Status = "unknown"
			s.Summary.Autoscaling.Source = "none"
		}, "summary.autoscaling is unknown and has a source or a confidence other than none and unknown"},
		{"autoscaling managed_node_groups not whole", func(s *Snapshot) { s.Summary.Autoscaling.ManagedNodeGroups = ptr(1.5) }, "summary.autoscaling.managed_node_groups must be a finite whole number"},
		{"autoscaling min_nodes infinite", func(s *Snapshot) { s.Summary.Autoscaling.MinNodes = ptr(math.Inf(1)) }, "summary.autoscaling.min_nodes must be a finite whole number"},
		{"autoscaling max_nodes NaN", func(s *Snapshot) { s.Summary.Autoscaling.MaxNodes = ptr(math.NaN()) }, "summary.autoscaling.max_nodes must be a finite whole number"},
		{"autoscaling max_nodes not whole", func(s *Snapshot) { s.Summary.Autoscaling.MaxNodes = ptr(2.5) }, "summary.autoscaling.max_nodes must be a finite whole number"},
		{"autoscaling scale_events infinite", func(s *Snapshot) { s.Summary.Autoscaling.ScaleEvents = ptr(math.Inf(1)) }, "summary.autoscaling.scale_events is not a finite number"},
		{"autoscaling unschedulable_pods NaN", func(s *Snapshot) { s.Summary.Autoscaling.UnschedulablePods = ptr(math.NaN()) }, "summary.autoscaling.unschedulable_pods is not a finite number"},

		{"hpas repeat", func(s *Snapshot) {
			s.HPAs = append(s.HPAs, s.HPAs[0])
			s.Summary.HPAs = 2
		}, "hpas is not sorted or holds a repeated value"},
		{"hpas[0].namespace outside the scope", func(s *Snapshot) { s.HPAs[0].Namespace = "other" }, "hpas[0].namespace is not a namespace of the scope"},
		{"hpas[0].name empty", func(s *Snapshot) { s.HPAs[0].Name = "" }, "hpas[0].name is empty"},
		{"hpas[0].name slash", func(s *Snapshot) { s.HPAs[0].Name = "a/b" }, `hpas[0].name holds a slash: "a/b"`},
		{"hpas[0].namespace slash", func(s *Snapshot) { s.HPAs[0].Namespace = "a/b" }, `hpas[0].namespace holds a slash: "a/b"`},
		{"hpas[0].target_name empty", func(s *Snapshot) { s.HPAs[0].TargetName = "" }, "hpas[0].target_kind and hpas[0].target_name must both be set or both be empty"},
		{"hpas[0].target_kind empty", func(s *Snapshot) { s.HPAs[0].TargetKind = "" }, "hpas[0].target_kind and hpas[0].target_name must both be set or both be empty"},
		{"hpas[0].spec_max not finite", func(s *Snapshot) { s.HPAs[0].SpecMax = ptr(math.Inf(1)) }, "hpas[0].spec_max is not a finite number"},
		{"hpas[0].spec_min not finite", func(s *Snapshot) { s.HPAs[0].SpecMin = ptr(math.NaN()) }, "hpas[0].spec_min is not a finite number"},
		{"hpas[0].current state", func(s *Snapshot) { s.HPAs[0].Current.State = "bogus" }, `hpas[0].current.state has an unknown value: "bogus"`},

		{"nodes repeat", func(s *Snapshot) {
			s.Nodes = append(s.Nodes, s.Nodes[2])
			s.Summary.Nodes = 4
		}, "nodes is not sorted or holds a repeated value"},
		{"nodes[0].name empty", func(s *Snapshot) { s.Nodes[0].Name = "" }, "nodes[0].name is empty"},
		{"nodes[0].pool empty", func(s *Snapshot) { s.Nodes[0].Pool = "" }, "nodes[0].pool is empty"},
		{"nodes[0].pool_confidence", func(s *Snapshot) { s.Nodes[0].PoolConfidence = "bogus" }, `nodes[0].pool_confidence has an unknown value: "bogus"`},
		{"nodes[0].cpu_capacity negative", func(s *Snapshot) { s.Nodes[0].CPUCapacity = ptr(-1) }, "nodes[0].cpu_capacity must be a finite number, zero or above"},
		{"nodes[0].memory_capacity NaN", func(s *Snapshot) { s.Nodes[0].MemoryCapacity = ptr(math.NaN()) }, "nodes[0].memory_capacity must be a finite number, zero or above"},
		{"nodes[0].cpu_allocatable negative", func(s *Snapshot) { s.Nodes[0].CPUAllocatable = ptr(-1) }, "nodes[0].cpu_allocatable must be a finite number, zero or above"},
		{"nodes[0].memory_allocatable infinite", func(s *Snapshot) { s.Nodes[0].MemoryAllocatable = ptr(math.Inf(1)) }, "nodes[0].memory_allocatable must be a finite number, zero or above"},
		{"nodes[0].cpu_request_percent negative", func(s *Snapshot) { s.Nodes[0].CPURequestPercent = ptr(-1) }, "nodes[0].cpu_request_percent must be a finite number, zero or above"},
		{"nodes[0].memory_request_percent negative", func(s *Snapshot) { s.Nodes[0].MemoryRequestPercent = ptr(-1) }, "nodes[0].memory_request_percent must be a finite number, zero or above"},
		{"nodes[0].cpu_usage_percent state", func(s *Snapshot) { s.Nodes[0].CPUUsagePercent.State = "bogus" }, `nodes[0].cpu_usage_percent.state has an unknown value: "bogus"`},
		{"nodes[0].memory_usage_percent not finite", func(s *Snapshot) { s.Nodes[0].MemoryUsagePercent.Peak = ptr(math.NaN()) }, "nodes[0].memory_usage_percent.peak is not a finite number"},

		{"nodepools out of order", func(s *Snapshot) { s.Nodepools[0], s.Nodepools[1] = s.Nodepools[1], s.Nodepools[0] }, "nodepools is not sorted or holds a repeated value"},
		{"nodepools[1].nodes empty", func(s *Snapshot) { s.Nodepools[1].Nodes = []string{} }, "nodepools[1].nodes is empty"},
		{"nodepools[0].nodes not sorted", func(s *Snapshot) { s.Nodepools[0].Nodes = []string{"node-a2", "node-a1"} }, "nodepools[0].nodes is not sorted or holds a repeated value"},
		{"a node listed by no pool", func(s *Snapshot) { s.Nodepools[0].Nodes = []string{"node-a1"} }, `node "node-a2" is listed by no nodepool`},
		{"a pool with a confidence above its nodes", func(s *Snapshot) { s.Nodepools[0].Confidence = "exact" }, `nodepools[0].confidence is "exact", want "inferred", the lowest of its nodes`},
		{"a node row whose pool differs from the pool that lists it", func(s *Snapshot) { s.Nodes[2].Pool = "a" }, `nodes[2].pool is "a", but nodepools[1] (b) lists it`},

		{"workloads[1].hpa_coverage", func(s *Snapshot) { s.Workloads[1].HPACoverage = "bogus" }, `workloads[1].hpa_coverage has an unknown value: "bogus"`},
		{"an uncovered row that an HPA targets", func(s *Snapshot) {
			s.Workloads[1].HPACoverage = "uncovered"
			s.Summary.WorkloadsWithoutHPA = 2
		}, "workloads[1] (payments/web) is uncovered and is the target of hpas[0]"},
		{"an unobserved row that is not uncovered", func(s *Snapshot) {
			s.Workloads[0].HPACoverage = "unknown"
			s.Summary.WorkloadsWithoutHPA = 0
		}, "workloads[0] has no observed usage and is not uncovered"},
		{"an unobserved row with pods", func(s *Snapshot) { s.Workloads[0].Pods = ptr(1) }, "workloads[0] has no observed usage and holds pods"},
		{"an unobserved row with a statistic", func(s *Snapshot) {
			s.Workloads[0].CPUUsage = Statistics{State: collect.StateNoBase}
		}, "workloads[0].cpu_usage must be unavailable for a row with no observed usage"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRefuses(t, tt.mutate, tt.message)
		})
	}
}

func TestDecodeNullBoolean(t *testing.T) {
	good, err := Encode(clusterSnapshot(t))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	got, err := Decode(nullJSONKey(t, good, "nodes[0].ready"))
	if err != nil {
		t.Fatalf("Decode(null ready) error = %v, want nil", err)
	}
	if got.Nodes[0].Ready != nil {
		t.Errorf("Nodes[0].Ready = %v, want nil", *got.Nodes[0].Ready)
	}

	var tree map[string]any
	if err := json.Unmarshal(good, &tree); err != nil {
		t.Fatalf("Unmarshal(good): %v", err)
	}
	tree["nodes"].([]any)[0].(map[string]any)["ready"] = 1.0
	data, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal(tree): %v", err)
	}
	_, err = Decode(data)
	want := "snapshot holds a value of the wrong kind at nodes[0].ready"
	if err == nil || err.Error() != want {
		t.Fatalf("Decode(ready 1) = %v, want error %q", err, want)
	}
}

func TestDecodeReadsTheClusterSections(t *testing.T) {
	snap := clusterSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	got, err := Decode(good)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !reflect.DeepEqual(got, snap) {
		t.Fatalf("Decode(Encode(s)) = %+v, want %+v", got, snap)
	}
}

func TestCheckKeepsANegativeAutoscalerCount(t *testing.T) {
	snap := clusterSnapshot(t)
	snap.Summary.Autoscaling.MaxNodes = ptr(-2)
	snap.Summary.Autoscaling.ScaleEvents = ptr(-1)
	if err := snap.Check(); err != nil {
		t.Fatalf("Check() error = %v, want nil", err)
	}
}

func TestDecodeRefusesAUsageFindingWithNoEvidence(t *testing.T) {
	data, err := os.ReadFile("../../testdata/snapshot/edge-scaling.json")
	if err != nil {
		t.Fatalf("reading the snapshot: %v", err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	emptied := false
	for _, f := range tree["findings"].([]any) {
		finding := f.(map[string]any)
		if strings.HasPrefix(finding["code"].(string), "NODE_CPU_USAGE_") {
			finding["evidence"] = []any{}
			emptied = true
			break
		}
	}
	if !emptied {
		t.Fatal("the snapshot holds no NODE_CPU_USAGE finding")
	}
	mutated, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	_, err = Decode(mutated)
	if err == nil || !strings.Contains(err.Error(), "evidence does not match the definition of NODE_CPU_USAGE_") {
		t.Fatalf("Decode() error = %v, want an evidence error naming the finding", err)
	}
}

func TestDecodeRefusesMissingRecommendations(t *testing.T) {
	good, err := Encode(validSnapshot(t))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	for _, path := range []string{"recommendations", "summary.recommendations"} {
		t.Run(path, func(t *testing.T) {
			_, err := Decode(removeJSONKey(t, good, path))
			want := "snapshot lacks the key " + path
			if err == nil || err.Error() != want {
				t.Fatalf("Decode() error = %v, want %q", err, want)
			}
		})
	}
}

func TestCheckRecomputesTheRecommendations(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Snapshot)
		message string
	}{
		{"an entry removed", func(s *Snapshot) {
			s.Recommendations = s.Recommendations[:1]
			s.Summary.Recommendations = 1
		}, "recommendations holds 1 entries, want 2 from the findings"},
		{"an entry added", func(s *Snapshot) {
			s.Recommendations = append(s.Recommendations, s.Recommendations[0])
			s.Summary.Recommendations = 3
		}, "recommendations holds 3 entries, want 2 from the findings"},
		{"two entries swapped", func(s *Snapshot) {
			s.Recommendations[0], s.Recommendations[1] = s.Recommendations[1], s.Recommendations[0]
		}, "recommendations[0] differs from the findings: got NODE_LOW_UTILIZATION attention node [node-b1] exact, want HPA_MAX_REPLICAS_REACHED critical hpa [payments/web] exact"},
		{"a scope renamed", func(s *Snapshot) {
			s.Recommendations[0].Scopes = []string{"payments/other"}
		}, "recommendations[0] differs from the findings: got HPA_MAX_REPLICAS_REACHED critical hpa [payments/other] exact, want HPA_MAX_REPLICAS_REACHED critical hpa [payments/web] exact"},
		{"a confidence changed", func(s *Snapshot) {
			s.Recommendations[1].Confidence = "inferred"
		}, "recommendations[1] differs from the findings: got NODE_LOW_UTILIZATION attention node [node-b1] inferred, want NODE_LOW_UTILIZATION attention node [node-b1] exact"},
		{"the severity changed", func(s *Snapshot) {
			s.Recommendations[1].Severity = "critical"
		}, "recommendations[1] differs from the findings: got NODE_LOW_UTILIZATION critical node [node-b1] exact, want NODE_LOW_UTILIZATION attention node [node-b1] exact"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRefuses(t, tt.mutate, tt.message)
		})
	}
}

func TestCheckCountsTheRecommendations(t *testing.T) {
	checkRefuses(t, func(s *Snapshot) {
		s.Summary.Recommendations = 3
	}, "summary.recommendations is 3, want 2")
}

func TestCheckRefusesANonFiniteThreshold(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Snapshot)
	}{
		{"report.thresholds.warning_percent", func(s *Snapshot) { s.Report.Thresholds.WarningPercent = math.NaN() }},
		{"report.thresholds.critical_percent", func(s *Snapshot) { s.Report.Thresholds.CriticalPercent = math.NaN() }},
		{"report.thresholds.low_utilization_percent", func(s *Snapshot) { s.Report.Thresholds.LowUtilizationPercent = math.NaN() }},
		{"report.thresholds.autoscaler_near_max_percent", func(s *Snapshot) { s.Report.Thresholds.AutoscalerNearMaxPercent = math.NaN() }},
		{"report.thresholds.nodepool_spread_points", func(s *Snapshot) { s.Report.Thresholds.NodepoolSpreadPoints = math.NaN() }},
		{"report.thresholds.bursty_gap_percent", func(s *Snapshot) { s.Report.Thresholds.BurstyGapPercent = math.NaN() }},
		{"report.thresholds.hpa_replica_divergence", func(s *Snapshot) { s.Report.Thresholds.HPAReplicaDivergence = math.NaN() }},
		{"report.settings.timeout_seconds", func(s *Snapshot) { s.Report.Settings.TimeoutSeconds = math.Inf(1) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validSnapshot(t)
			tt.mutate(&s)
			err := s.Check()
			want := tt.name + " must be a finite number"
			if err == nil || err.Error() != want {
				t.Fatalf("Check() error = %v, want %q", err, want)
			}
		})
	}
}

// goldenBytes reads the golden snapshot of a scenario.
func goldenBytes(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../../testdata/snapshot/" + name + ".json")
	if err != nil {
		t.Fatalf("reading the snapshot of %s: %v", name, err)
	}
	return data
}

// decodeGolden decodes the golden snapshot of a scenario.
func decodeGolden(t *testing.T, name string) Snapshot {
	t.Helper()
	snap, err := Decode(goldenBytes(t, name))
	if err != nil {
		t.Fatalf("Decode(%s) error = %v", name, err)
	}
	return snap
}

// checkRefusesWith expects Check of snap to refuse it with message.
func checkRefusesWith(t *testing.T, snap Snapshot, message string) {
	t.Helper()
	err := snap.Check()
	if err == nil {
		t.Fatalf("Check() = nil, want error %q", message)
	}
	if err.Error() != message {
		t.Fatalf("Check() = %q, want %q", err.Error(), message)
	}
}

func TestDecodeRefusesARepeatedKey(t *testing.T) {
	good := string(goldenBytes(t, "direct-complete"))
	findings := strings.Index(good, `"findings": [`)
	firstCode := findings + strings.Index(good[findings:], `"code": `)
	codeLine := good[firstCode : firstCode+strings.Index(good[firstCode:], "\n")]
	tests := []struct {
		name    string
		data    string
		message string
	}{
		{"at the top", strings.Replace(good, `"schema_version": "2.0",`, `"schema_version": "2.0", "schema_version": "2.0",`, 1), "snapshot repeats the key schema_version"},
		{"inside report", strings.Replace(good, `"profile": "detailed",`, `"profile": "detailed", "profile": "executive",`, 1), "snapshot repeats the key report.profile"},
		{"inside a finding", good[:firstCode] + codeLine + " " + codeLine + good[firstCode+len(codeLine):], "snapshot repeats the key findings[0].code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.data == good {
				t.Fatal("the mutation changed nothing")
			}
			_, err := Decode([]byte(tt.data))
			if err == nil || err.Error() != tt.message {
				t.Fatalf("Decode() error = %v, want %q", err, tt.message)
			}
		})
	}
}

// A key of an object that is made of digits is a key, and an index of a list
// is an index: the path tells them apart.
func TestDecodeNamesARepeatedNumericKeyAsAKey(t *testing.T) {
	good := string(goldenBytes(t, "edge-values"))
	const params = "\"params\": {\n          \"count\": 1\n        }"
	data := strings.Replace(good, params, `"params": {"1": 1, "1": 2}`, 1)
	if data == good {
		t.Fatal("the mutation changed nothing")
	}
	_, err := Decode([]byte(data))
	const want = "snapshot repeats the key quality.notices[0].params.1"
	if err == nil || err.Error() != want {
		t.Fatalf("Decode() error = %v, want %q", err, want)
	}
}

func TestDecodeNamesAWrongVersionBeforeARepeatedKey(t *testing.T) {
	good := string(goldenBytes(t, "direct-complete"))
	findings := strings.Index(good, `"findings": [`)
	firstCode := findings + strings.Index(good[findings:], `"code": `)
	codeLine := good[firstCode : firstCode+strings.Index(good[firstCode:], "\n")]
	repeated := good[:firstCode] + codeLine + " " + codeLine + good[firstCode+len(codeLine):]
	data := strings.Replace(repeated, `"schema_version": "2.0"`, `"schema_version": "0.9"`, 1)
	if data == repeated {
		t.Fatal("the mutation changed nothing")
	}
	_, err := Decode([]byte(data))
	const want = `snapshot has schema_version "0.9"; this version reads "2.0"`
	if err == nil || err.Error() != want {
		t.Fatalf("Decode() error = %v, want %q", err, want)
	}
}

func TestDecodeRefusesTextThatIsNotUTF8(t *testing.T) {
	good := string(goldenBytes(t, "direct-complete"))
	data := strings.Replace(good, `"organization": "ACME"`, "\"organization\": \"AC\xffME\"", 1)
	if data == good {
		t.Fatal("the mutation changed nothing")
	}
	// The offset the message names is where the first invalid byte of the
	// file is, wherever the golden file puts it.
	offset := 0
	for offset < len(data) {
		r, size := utf8.DecodeRuneInString(data[offset:])
		if r == utf8.RuneError && size == 1 {
			break
		}
		offset += size
	}
	if offset == len(data) {
		t.Fatal("the file holds no invalid byte")
	}
	_, err := Decode([]byte(data))
	want := fmt.Sprintf(`snapshot is not valid JSON: jsontext: invalid UTF-8 within "/report/organization" after offset %d`, offset)
	if err == nil || err.Error() != want {
		t.Fatalf("Decode() error = %v, want %q", err, want)
	}
}

func TestCheckRefusesACoveredWorkloadNoHPATargets(t *testing.T) {
	snap := decodeGolden(t, "direct-complete")
	covered := slices.IndexFunc(snap.Workloads, func(w Workload) bool { return w.HPACoverage == CoverageCovered })
	if covered < 0 {
		t.Fatal("the snapshot holds no covered workload")
	}
	wl := snap.Workloads[covered]
	targeting := slices.IndexFunc(snap.HPAs, func(h HPA) bool {
		return h.Namespace == wl.Namespace && h.TargetName == wl.Name && h.TargetKind == wl.Kind
	})
	if targeting < 0 {
		t.Fatal("no HPA targets the covered workload")
	}
	snap.HPAs = slices.Delete(snap.HPAs, targeting, targeting+1)
	snap.Summary.HPAs--
	checkRefusesWith(t, snap, fmt.Sprintf("workloads[%d] (%s/%s) is covered and no HPA targets it", covered, wl.Namespace, wl.Name))
}

func TestCheckRefusesAnAutoscalingShape(t *testing.T) {
	const shape = "summary.autoscaling holds status %s, source %s, and confidence %s, which analysis never writes"
	const count = "summary.autoscaling with source %s holds a count"
	tests := []struct {
		name    string
		mutate  func(*Autoscaling)
		message string
	}{
		{"configured with inferred", func(a *Autoscaling) {
			a.Status, a.Source, a.Confidence = "enabled", "configured", "inferred"
		}, fmt.Sprintf(shape, "enabled", "configured", "inferred")},
		{"raw_metrics with disabled", func(a *Autoscaling) {
			a.Status = "disabled"
		}, fmt.Sprintf(shape, "disabled", "raw_metrics", "inferred")},
		{"configured with a count", func(a *Autoscaling) {
			*a = Autoscaling{Status: "enabled", Source: "configured", Confidence: "exact", MinNodes: ptr(1)}
		}, fmt.Sprintf(count, "configured")},
		{"none with a count", func(a *Autoscaling) {
			*a = Autoscaling{Status: "unknown", Source: "none", Confidence: "unknown", MaxNodes: ptr(1)}
		}, fmt.Sprintf(count, "none")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checkRefuses(t, func(s *Snapshot) { tt.mutate(&s.Summary.Autoscaling) }, tt.message)
		})
	}
}

func TestCheckAcceptsTheAutoscalingShapesAnalysisWrites(t *testing.T) {
	for _, a := range []Autoscaling{
		{Status: "enabled", Source: "configured", Confidence: "exact"},
		{Status: "disabled", Source: "configured", Confidence: "exact"},
		{Status: "unknown", Source: "none", Confidence: "unknown"},
		{Status: "enabled", Source: "raw_metrics", Confidence: "inferred", ManagedNodeGroups: ptr(2), CurrentNodes: ptr(5), MinNodes: ptr(2), MaxNodes: ptr(10), ScaleEvents: ptr(1), UnschedulablePods: ptr(0)},
	} {
		if err := checkAutoscaling(a); err != nil {
			t.Errorf("checkAutoscaling(%+v) = %v, want nil", a, err)
		}
	}
}

func TestCheckRefusesAContraryAutoscalingFinding(t *testing.T) {
	snap := decodeGolden(t, "edge-scaling")
	snap.Summary.Autoscaling = Autoscaling{Status: "disabled", Source: "configured", Confidence: "exact"}
	contrary := slices.IndexFunc(snap.Findings, func(f Finding) bool { return f.Code == "NODE_AUTOSCALING_NEAR_MAX" })
	if contrary < 0 {
		t.Fatal("the snapshot holds no NODE_AUTOSCALING_NEAR_MAX finding")
	}
	checkRefusesWith(t, snap, fmt.Sprintf("findings[%d] is NODE_AUTOSCALING_NEAR_MAX, which the autoscaling status disabled never gives", contrary))
}

func TestCheckRefusesAValueMissingANumber(t *testing.T) {
	for _, missing := range []string{"average", "minimum", "peak", "current"} {
		t.Run(missing, func(t *testing.T) {
			checkRefuses(t, func(s *Snapshot) {
				stat := Statistics{State: collect.StateValue, Average: ptr(1), Minimum: ptr(1), Peak: ptr(1), Current: ptr(1)}
				switch missing {
				case "average":
					stat.Average = nil
				case "minimum":
					stat.Minimum = nil
				case "peak":
					stat.Peak = nil
				case "current":
					stat.Current = nil
				}
				s.Namespaces[0].CPUUsage = stat
			}, "namespaces[0].cpu_usage has the state value and lacks a number")
		})
	}
}

func TestCheckRefusesSpecMinAboveSpecMax(t *testing.T) {
	checkRefuses(t, func(s *Snapshot) {
		s.HPAs[0].SpecMin = ptr(5)
		s.HPAs[0].SpecMax = ptr(2)
	}, "hpas[0].spec_min is greater than hpas[0].spec_max")
}

// setConfidence gives every finding of code the confidence, and every
// recommendation of the code the same, so that the recommendations still
// follow the findings.
func setConfidence(t *testing.T, snap *Snapshot, code, confidence string) int {
	t.Helper()
	index := -1
	for i := range snap.Findings {
		if snap.Findings[i].Code == code {
			snap.Findings[i].Confidence = confidence
			index = i
		}
	}
	if index < 0 {
		t.Fatalf("the snapshot holds no %s finding", code)
	}
	findings := make([]analysis.Finding, len(snap.Findings))
	for i, f := range snap.Findings {
		findings[i] = analysis.Finding{Code: f.Code, Severity: f.Severity, ScopeType: f.ScopeType, ScopeKind: f.ScopeKind, ScopeName: f.ScopeName, Confidence: f.Confidence}
	}
	snap.Recommendations = snap.Recommendations[:0]
	for _, r := range analysis.Recommendations(findings) {
		snap.Recommendations = append(snap.Recommendations, Recommendation{Code: r.Code, Severity: r.Severity, ScopeType: r.ScopeType, Scopes: r.Scopes, Confidence: r.Confidence})
	}
	return index
}

func TestCheckRefusesAConfidenceTheCodeNeverTakes(t *testing.T) {
	missing := decodeGolden(t, "edge-values")
	index := setConfidence(t, &missing, "WORKLOAD_CPU_REQUEST_MISSING", "inferred")
	checkRefusesWith(t, missing, fmt.Sprintf("findings[%d].confidence inferred does not match the definition of WORKLOAD_CPU_REQUEST_MISSING", index))

	healthy := decodeGolden(t, "direct-complete")
	index = setConfidence(t, &healthy, "WORKLOAD_CPU_REQUEST_ATTENTION", "inferred")
	namespace, name, _ := strings.Cut(healthy.Findings[index].ScopeName, "/")
	for _, wl := range healthy.Workloads {
		if wl.Namespace == namespace && wl.Name == name && wl.Approximate {
			t.Fatal("the workload row is approximate")
		}
	}
	checkRefusesWith(t, healthy, fmt.Sprintf("findings[%d].confidence inferred does not match the definition of WORKLOAD_CPU_REQUEST_ATTENTION", index))
}

func TestCheckBindsAFindingConfidenceToItsSource(t *testing.T) {
	uneven := func(f Finding) bool {
		return strings.HasPrefix(f.Code, "NODEPOOL_") && strings.HasSuffix(f.Code, "_UNEVEN")
	}
	t.Run("a pool finding takes the confidence of its pool", func(t *testing.T) {
		snap := decodeGolden(t, "edge-scaling")
		index := slices.IndexFunc(snap.Findings, uneven)
		if index < 0 {
			t.Fatal("the snapshot holds no uneven pool finding")
		}
		if snap.Findings[index].Confidence == "unknown" {
			t.Fatal("the pool of the finding is already unknown")
		}
		code := snap.Findings[index].Code
		setConfidence(t, &snap, code, "unknown")
		checkRefusesWith(t, snap, fmt.Sprintf("findings[%d].confidence unknown does not match the definition of %s", index, code))
	})
	t.Run("an autoscaling finding takes the confidence of the autoscaling", func(t *testing.T) {
		snap := decodeGolden(t, "edge-scaling")
		index := slices.IndexFunc(snap.Findings, func(f Finding) bool { return f.Code == "NODE_AUTOSCALING_NEAR_MAX" })
		if index < 0 {
			t.Fatal("the snapshot holds no NODE_AUTOSCALING_NEAR_MAX finding")
		}
		setConfidence(t, &snap, "NODE_AUTOSCALING_NEAR_MAX", "exact")
		checkRefusesWith(t, snap, fmt.Sprintf("findings[%d].confidence exact does not match the definition of NODE_AUTOSCALING_NEAR_MAX", index))
	})
	t.Run("an approximate row gives inferred", func(t *testing.T) {
		snap := decodeGolden(t, "raw-fallback")
		index := slices.IndexFunc(snap.Findings, func(f Finding) bool { return f.Code == "WORKLOAD_CPU_REQUEST_HEALTHY" })
		if index < 0 {
			t.Fatal("the snapshot holds no WORKLOAD_CPU_REQUEST_HEALTHY finding")
		}
		namespace, name, _ := strings.Cut(snap.Findings[index].ScopeName, "/")
		row := slices.IndexFunc(snap.Workloads, func(w Workload) bool { return w.Namespace == namespace && w.Name == name })
		if snap.Workloads[row].Approximate {
			t.Fatal("the workload row is approximate")
		}
		snap.Workloads[row].Approximate = true
		first := slices.IndexFunc(snap.Findings, func(f Finding) bool {
			def, _ := analysis.Lookup(f.Code)
			return f.ScopeType == analysis.ScopeWorkload && f.ScopeName == snap.Findings[index].ScopeName && def.Confidence.FromApproximate
		})
		checkRefusesWith(t, snap, fmt.Sprintf("findings[%d].confidence exact does not match the definition of %s", first, snap.Findings[first].Code))
	})
}

// shapePaths lists the key path and the type of every field of t, through
// structs, pointers, slices (an element is written "[]"), and maps (an
// element is written "{}"). A struct, a slice, or a map is listed with its
// own type, and a Value, which has a JSON form of its own, is one leaf named
// by its type.
func shapePaths(t reflect.Type, prefix string) []string {
	var paths []string
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name, ok := supportedKey(field)
		if !ok {
			// A field the key walk does not support has no name in the
			// contract, so it is listed by its Go name and the frozen shape
			// no longer matches.
			name = "(unsupported " + field.Name + ")"
		}
		path := name
		if prefix != "" {
			path = prefix + "." + name
		}
		paths = append(paths, path+" "+field.Type.String())
		paths = append(paths, elementPaths(field.Type, path)...)
	}
	return paths
}

// elementPaths follows typ into the fields of the struct it holds, if any.
func elementPaths(typ reflect.Type, path string) []string {
	switch typ.Kind() {
	case reflect.Pointer:
		return elementPaths(typ.Elem(), path)
	case reflect.Slice, reflect.Array:
		return elementPaths(typ.Elem(), path+"[]")
	case reflect.Map:
		return elementPaths(typ.Elem(), path+"{}")
	case reflect.Struct:
		if typ == reflect.TypeOf(Value{}) {
			return nil
		}
		return shapePaths(typ, path)
	}
	return nil
}

// TestSchemaShapeIsFrozen pins every key of the contract and the type of
// each: a change of shape fails until the schema version is raised and the
// new shape is recorded here.
func TestSchemaShapeIsFrozen(t *testing.T) {
	paths := shapePaths(reflect.TypeOf(Snapshot{}), "")
	sort.Strings(paths)
	sum := sha256.Sum256([]byte(strings.Join(paths, "\n")))
	const frozen = "377c249cde3f0ff946d1784d43a29b5804b841a73c8a688ed92669e121abca3e"
	if got := hex.EncodeToString(sum[:]); got != frozen {
		t.Logf("%d paths, hash of the key paths: %s", len(paths), got)
		t.Fatalf("the shape of schema %s changed; raise snapshot.SchemaVersion and record the new shape", SchemaVersion)
	}
}

func TestSnapshotHasNoPeriod(t *testing.T) {
	data, err := Encode(validSnapshot(t))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if tree["schema_version"] != "2.0" {
		t.Errorf("schema_version = %v, want 2.0", tree["schema_version"])
	}
	if _, ok := tree["report"].(map[string]any)["period"]; ok {
		t.Error("report holds the key period")
	}
}

func TestSnapshotEndMayPrecedeTheEmission(t *testing.T) {
	s := validSnapshot(t)
	emitted, _ := Time(s.Report.EmittedAt)
	s.Report.PeriodEnd = formatTime(emitted.Add(-24*time.Hour), emitted.Location())
	s.Report.PeriodStart = formatTime(emitted.Add(-48*time.Hour), emitted.Location())
	data, err := Encode(s)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if _, err := Decode(data); err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
}

func TestSnapshotEndAfterTheEmission(t *testing.T) {
	s := validSnapshot(t)
	emitted, _ := Time(s.Report.EmittedAt)
	s.Report.PeriodEnd = formatTime(emitted.Add(time.Second), emitted.Location())
	err := s.Check()
	if err == nil || err.Error() != "report.period_end is after report.emitted_at" {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestSnapshotNamesVersionOne(t *testing.T) {
	data := mutateJSON(t, goldenBytes(t, "direct-complete"), func(tree map[string]any) { tree["schema_version"] = "1.0" })
	_, err := Decode(data)
	const want = `snapshot has schema_version "1.0"; this version reads "2.0"`
	if err == nil || err.Error() != want {
		t.Fatalf("Decode() error = %v, want %q", err, want)
	}
}

func TestSnapshotRefusesAPeriodKey(t *testing.T) {
	data, err := Encode(validSnapshot(t))
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	data = mutateJSON(t, data, func(tree map[string]any) { tree["report"].(map[string]any)["period"] = "24h" })
	_, err = Decode(data)
	if err == nil || !strings.HasPrefix(err.Error(), "snapshot holds an unknown key: ") {
		t.Fatalf("Decode() error = %v, want an unknown key", err)
	}
}
