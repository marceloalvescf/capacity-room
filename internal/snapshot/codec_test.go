package snapshot

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

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
	snap := validSnapshot(t)

	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if !reflect.DeepEqual(got, snap) {
		t.Fatalf("Decode(Encode(s)) = %+v, want %+v", got, snap)
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
		{"schema_version another version", []byte(`{"schema_version": "2.0"}`), `snapshot has schema_version "2.0"; this version reads "1.0"`, false},
		{
			"another version, a missing key, and an unknown key at once",
			[]byte(`{"schema_version": "2.0", "report": {}, "extra": 1}`),
			`snapshot has schema_version "2.0"; this version reads "1.0"`,
			false,
		},
		{
			"a valid snapshot of another version",
			mutateJSON(t, good, func(tree map[string]any) { tree["schema_version"] = "2.0" }),
			`snapshot has schema_version "2.0"; this version reads "1.0"`,
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
			append(mutateJSON(t, good, func(tree map[string]any) { tree["schema_version"] = "2.0" }), []byte("{}")...),
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
				tree["schema_version"] = "2.0"
			}),
			want: `snapshot has schema_version "2.0"; this version reads "1.0"`,
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

// TestDecodeRefusesAnotherLetterCase builds, for each of the 45 paths of
// the contract, a valid snapshot with one more key beside the real one: the
// same name, its first letter capitalised, holding a forged value. The
// walk of the map compares a key letter for letter, so every one of these
// is refused as an unknown key, never silently merged into the real one by
// a case-insensitive struct decode.
func TestDecodeRefusesAnotherLetterCase(t *testing.T) {
	if len(contractPaths) != 45 {
		t.Fatalf("contractPaths has %d entries, want 45", len(contractPaths))
	}

	snap := validSnapshot(t)
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

// TestDecodeReadsWhatTheMapHolds writes report.classification twice, with
// the same letters and two different values: not something Encode ever
// produces, but something a hand-edited or foreign file legally could,
// since JSON itself does not forbid a repeated key. The map is the one
// reading of the file, and a map keeps the last value written to a key, so
// the snapshot must hold that last value too — one reading, the last one,
// never the first one a second, independent reading might have kept.
func TestDecodeReadsWhatTheMapHolds(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	original := fmt.Sprintf("%q: %q,", "classification", snap.Report.Classification)
	if !strings.Contains(string(good), original) {
		t.Fatalf("did not find %s in the encoded snapshot", original)
	}
	duplicated := original + "\n    " + fmt.Sprintf("%q: %q,", "classification", "a repeated value wins")
	data := []byte(strings.Replace(string(good), original, duplicated, 1))

	got, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode() error = %v, want nil", err)
	}
	if got.Report.Classification != "a repeated value wins" {
		t.Fatalf("Report.Classification = %q, want %q (the last value written)", got.Report.Classification, "a repeated value wins")
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
		{"report.period_end differs from report.emitted_at", func(s *Snapshot) {
			s.Report.PeriodEnd = "2099-01-01T00:00:00Z"
		}, "report.period_end differs from report.emitted_at"},
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
		{"report.period unknown", func(s *Snapshot) { s.Report.Period = "bogus" }, `report.period has an unknown value: "bogus"`},
		{"report.period too long", func(s *Snapshot) { s.Report.Period = "999999999d" }, `report.period is too long: "999999999d"`},
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
		if field.Type.Kind() == reflect.Struct {
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
	snap := validSnapshot(t)
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

// contractPaths is the 45 key paths of the snapshot contract, written by
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
	"report.period",
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
	"quality.collection_started_at",
	"quality.collection_completed_at",
	"quality.capabilities[3].name",
	"quality.capabilities[3].state",
	"quality.capabilities[3].strategy",
	"quality.capabilities[3].evidence",
	"quality.capabilities[3].reasons",
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
// any of the 45 paths of the contract must be refused, by its own message.
func TestDecodeRefusesNull(t *testing.T) {
	snap := validSnapshot(t)
	good, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for _, path := range contractPaths {
		t.Run(path, func(t *testing.T) {
			data := nullJSONKey(t, good, path)

			_, err := Decode(data)
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
