package snapshot

import (
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// baseCapabilities returns the nine capabilities in the order of
// discovery.Names, each with a distinct, valid, non-nil evidence and
// reasons list, so that a table test can mutate exactly one field of one
// capability.
func baseCapabilities() []discovery.Capability {
	caps := make([]discovery.Capability, len(discovery.Names))
	for i, name := range discovery.Names {
		caps[i] = discovery.Capability{
			Name:     name,
			State:    discovery.StateAvailable,
			Strategy: discovery.StrategyRawMetrics,
			Evidence: []string{"metric_" + name},
			Reasons:  []string{},
		}
	}
	return caps
}

// baseInput returns an Input that Build accepts, with every field holding a
// distinct value so that a swapped pair of fields is caught by a
// field-by-field comparison.
func baseInput() Input {
	return Input{
		ID:               "run-001",
		GeneratorVersion: "kcr-9.9.9",
		EmittedAt:        time.Date(2026, 7, 15, 14, 30, 0, 0, time.UTC),
		Config: config.Config{
			Metrics: config.Metrics{
				URL:            "https://prom.example.org", // never read by Build
				EndpointType:   "auto",                     // never read by Build
				TLSVerify:      true,
				TimeoutSeconds: 42,
			},
			Cluster: config.Cluster{
				Name:          "prod", // never read by Build directly
				Namespaces:    []string{"payments", "checkout"},
				Label:         "cluster", // never read by Build directly
				NodepoolLabel: "eks.amazonaws.com/nodegroup",
			},
			Analysis: config.Analysis{
				Period:             "24h",
				Timezone:           "America/Sao_Paulo",
				NodeAutoscaling:    "auto",
				ExcludedContainers: []string{"istio-proxy"},
			},
			Thresholds: config.Thresholds{
				WarningPercent:           70,
				CriticalPercent:          85,
				LowUtilizationPercent:    30,
				AutoscalerNearMaxPercent: 80,
				NodepoolSpreadPoints:     30,
				BurstyGapPercent:         100,
				HPAReplicaDivergence:     1,
			},
			Report: config.Report{
				Profile:        "detailed",
				MinSeverity:    "all",
				Organization:   "Example Org",
				Classification: "internal",
				Author:         "capacity-bot",
			},
		},
		EndpointKind: "prometheus",
		Scope: discovery.Scope{
			ClusterName:      "prod",
			ClusterLabel:     "cluster",
			FilterMode:       "required",
			MatcherEnabled:   true,
			Confidence:       discovery.ConfidenceExact,
			DiscoveredValues: []string{"prod", "staging"},
		},
		Capabilities:        baseCapabilities(),
		CollectionStarted:   time.Date(2026, 7, 15, 14, 25, 0, 0, time.UTC),
		CollectionCompleted: time.Date(2026, 7, 15, 14, 29, 0, 0, time.UTC),
	}
}

func mustBuild(t *testing.T, in Input) Snapshot {
	t.Helper()
	snap, err := Build(in)
	if err != nil {
		t.Fatalf("Build() error = %v, want nil", err)
	}
	return snap
}

func TestBuildCopiesEveryField(t *testing.T) {
	in := baseInput()
	snap := mustBuild(t, in)

	if snap.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", snap.SchemaVersion, SchemaVersion)
	}
	if snap.Report.ID != in.ID {
		t.Errorf("Report.ID = %q, want %q", snap.Report.ID, in.ID)
	}
	if snap.Report.GeneratorVersion != in.GeneratorVersion {
		t.Errorf("Report.GeneratorVersion = %q, want %q", snap.Report.GeneratorVersion, in.GeneratorVersion)
	}
	if snap.Report.Period != in.Config.Analysis.Period {
		t.Errorf("Report.Period = %q, want %q", snap.Report.Period, in.Config.Analysis.Period)
	}
	if snap.Report.Timezone != in.Config.Analysis.Timezone {
		t.Errorf("Report.Timezone = %q, want %q", snap.Report.Timezone, in.Config.Analysis.Timezone)
	}
	if snap.Report.Profile != in.Config.Report.Profile {
		t.Errorf("Report.Profile = %q, want %q", snap.Report.Profile, in.Config.Report.Profile)
	}
	if snap.Report.MinSeverity != in.Config.Report.MinSeverity {
		t.Errorf("Report.MinSeverity = %q, want %q", snap.Report.MinSeverity, in.Config.Report.MinSeverity)
	}
	if snap.Report.Organization != in.Config.Report.Organization {
		t.Errorf("Report.Organization = %q, want %q", snap.Report.Organization, in.Config.Report.Organization)
	}
	if snap.Report.Classification != in.Config.Report.Classification {
		t.Errorf("Report.Classification = %q, want %q", snap.Report.Classification, in.Config.Report.Classification)
	}
	if snap.Report.Author != in.Config.Report.Author {
		t.Errorf("Report.Author = %q, want %q", snap.Report.Author, in.Config.Report.Author)
	}

	th := in.Config.Thresholds
	if snap.Report.Thresholds.WarningPercent != th.WarningPercent {
		t.Errorf("Thresholds.WarningPercent = %v, want %v", snap.Report.Thresholds.WarningPercent, th.WarningPercent)
	}
	if snap.Report.Thresholds.CriticalPercent != th.CriticalPercent {
		t.Errorf("Thresholds.CriticalPercent = %v, want %v", snap.Report.Thresholds.CriticalPercent, th.CriticalPercent)
	}
	if snap.Report.Thresholds.LowUtilizationPercent != th.LowUtilizationPercent {
		t.Errorf("Thresholds.LowUtilizationPercent = %v, want %v", snap.Report.Thresholds.LowUtilizationPercent, th.LowUtilizationPercent)
	}
	if snap.Report.Thresholds.AutoscalerNearMaxPercent != th.AutoscalerNearMaxPercent {
		t.Errorf("Thresholds.AutoscalerNearMaxPercent = %v, want %v", snap.Report.Thresholds.AutoscalerNearMaxPercent, th.AutoscalerNearMaxPercent)
	}
	if snap.Report.Thresholds.NodepoolSpreadPoints != th.NodepoolSpreadPoints {
		t.Errorf("Thresholds.NodepoolSpreadPoints = %v, want %v", snap.Report.Thresholds.NodepoolSpreadPoints, th.NodepoolSpreadPoints)
	}
	if snap.Report.Thresholds.BurstyGapPercent != th.BurstyGapPercent {
		t.Errorf("Thresholds.BurstyGapPercent = %v, want %v", snap.Report.Thresholds.BurstyGapPercent, th.BurstyGapPercent)
	}
	if snap.Report.Thresholds.HPAReplicaDivergence != th.HPAReplicaDivergence {
		t.Errorf("Thresholds.HPAReplicaDivergence = %v, want %v", snap.Report.Thresholds.HPAReplicaDivergence, th.HPAReplicaDivergence)
	}

	if snap.Report.Settings.EndpointType != in.EndpointKind {
		t.Errorf("Settings.EndpointType = %q, want %q (the answering kind, not config.metrics.endpoint_type)", snap.Report.Settings.EndpointType, in.EndpointKind)
	}
	if snap.Report.Settings.TLSVerify != in.Config.Metrics.TLSVerify {
		t.Errorf("Settings.TLSVerify = %v, want %v", snap.Report.Settings.TLSVerify, in.Config.Metrics.TLSVerify)
	}
	if snap.Report.Settings.TimeoutSeconds != in.Config.Metrics.TimeoutSeconds {
		t.Errorf("Settings.TimeoutSeconds = %v, want %v", snap.Report.Settings.TimeoutSeconds, in.Config.Metrics.TimeoutSeconds)
	}
	if snap.Report.Settings.NodeAutoscaling != in.Config.Analysis.NodeAutoscaling {
		t.Errorf("Settings.NodeAutoscaling = %q, want %q", snap.Report.Settings.NodeAutoscaling, in.Config.Analysis.NodeAutoscaling)
	}
	if snap.Report.Settings.NodepoolLabel != in.Config.Cluster.NodepoolLabel {
		t.Errorf("Settings.NodepoolLabel = %q, want %q", snap.Report.Settings.NodepoolLabel, in.Config.Cluster.NodepoolLabel)
	}

	if snap.Scope.ClusterName != in.Scope.ClusterName {
		t.Errorf("Scope.ClusterName = %q, want %q", snap.Scope.ClusterName, in.Scope.ClusterName)
	}
	if snap.Scope.ClusterLabel != in.Scope.ClusterLabel {
		t.Errorf("Scope.ClusterLabel = %q, want %q", snap.Scope.ClusterLabel, in.Scope.ClusterLabel)
	}
	if snap.Scope.FilterMode != in.Scope.FilterMode {
		t.Errorf("Scope.FilterMode = %q, want %q", snap.Scope.FilterMode, in.Scope.FilterMode)
	}
	if snap.Scope.MatcherEnabled != in.Scope.MatcherEnabled {
		t.Errorf("Scope.MatcherEnabled = %v, want %v", snap.Scope.MatcherEnabled, in.Scope.MatcherEnabled)
	}
	if snap.Scope.Confidence != in.Scope.Confidence {
		t.Errorf("Scope.Confidence = %q, want %q", snap.Scope.Confidence, in.Scope.Confidence)
	}
	if strings.Join(snap.Scope.DiscoveredValues, ",") != strings.Join(in.Scope.DiscoveredValues, ",") {
		t.Errorf("Scope.DiscoveredValues = %v, want %v", snap.Scope.DiscoveredValues, in.Scope.DiscoveredValues)
	}
	if strings.Join(snap.Scope.Namespaces, ",") != "checkout,payments" {
		t.Errorf("Scope.Namespaces = %v, want sorted %v", snap.Scope.Namespaces, in.Config.Cluster.Namespaces)
	}
	if strings.Join(snap.Scope.ExcludedContainers, ",") != strings.Join(in.Config.Analysis.ExcludedContainers, ",") {
		t.Errorf("Scope.ExcludedContainers = %v, want %v", snap.Scope.ExcludedContainers, in.Config.Analysis.ExcludedContainers)
	}

	if len(snap.Quality.Capabilities) != len(in.Capabilities) {
		t.Fatalf("len(Quality.Capabilities) = %d, want %d", len(snap.Quality.Capabilities), len(in.Capabilities))
	}
	for i, c := range in.Capabilities {
		got := snap.Quality.Capabilities[i]
		if got.Name != c.Name {
			t.Errorf("Quality.Capabilities[%d].Name = %q, want %q", i, got.Name, c.Name)
		}
		if got.State != c.State {
			t.Errorf("Quality.Capabilities[%d].State = %q, want %q", i, got.State, c.State)
		}
		if got.Strategy != c.Strategy {
			t.Errorf("Quality.Capabilities[%d].Strategy = %q, want %q", i, got.Strategy, c.Strategy)
		}
		if strings.Join(got.Evidence, ",") != strings.Join(c.Evidence, ",") {
			t.Errorf("Quality.Capabilities[%d].Evidence = %v, want %v", i, got.Evidence, c.Evidence)
		}
		if strings.Join(got.Reasons, ",") != strings.Join(c.Reasons, ",") {
			t.Errorf("Quality.Capabilities[%d].Reasons = %v, want %v", i, got.Reasons, c.Reasons)
		}
	}

	wantStarted := in.CollectionStarted.In(mustLoc(t, in)).Format(time.RFC3339)
	if snap.Quality.CollectionStartedAt != wantStarted {
		t.Errorf("Quality.CollectionStartedAt = %q, want %q", snap.Quality.CollectionStartedAt, wantStarted)
	}
	wantCompleted := in.CollectionCompleted.In(mustLoc(t, in)).Format(time.RFC3339)
	if snap.Quality.CollectionCompletedAt != wantCompleted {
		t.Errorf("Quality.CollectionCompletedAt = %q, want %q", snap.Quality.CollectionCompletedAt, wantCompleted)
	}
}

func mustLoc(t *testing.T, in Input) *time.Location {
	t.Helper()
	loc, err := in.Config.Analysis.Location()
	if err != nil {
		t.Fatalf("Location() error = %v", err)
	}
	return loc
}

// TestBuildEveryFieldChangesTheBytes is the test of the rule "a field that
// changes nothing is a defect of the contract". Every row of Input, and
// every field of config.Config that Build reads, is exercised once.
func TestBuildEveryFieldChangesTheBytes(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{"Input.ID", func(in *Input) { in.ID = "run-002" }},
		{"Input.GeneratorVersion", func(in *Input) { in.GeneratorVersion = "kcr-1.0.0" }},
		{"Input.EmittedAt", func(in *Input) { in.EmittedAt = in.EmittedAt.Add(time.Hour) }},
		{"Input.EndpointKind", func(in *Input) { in.EndpointKind = "grafana" }},
		{"Input.Scope.ClusterName", func(in *Input) {
			in.Scope.ClusterName = "other"
		}},
		{"Input.Scope.ClusterLabel", func(in *Input) { in.Scope.ClusterLabel = "k8s_cluster" }},
		{"Input.Scope.FilterMode", func(in *Input) { in.Scope.FilterMode = "auto" }},
		{"Input.Scope.MatcherEnabled", func(in *Input) { in.Scope.MatcherEnabled = false }},
		{"Input.Scope.Confidence", func(in *Input) { in.Scope.Confidence = discovery.ConfidenceInferred }},
		{"Input.Scope.DiscoveredValues", func(in *Input) { in.Scope.DiscoveredValues = []string{"only"} }},
		{"Input.Capabilities[0].State", func(in *Input) { in.Capabilities[0].State = discovery.StatePartial }},
		{"Input.Capabilities[0].Strategy", func(in *Input) { in.Capabilities[0].Strategy = discovery.StrategyRecordingRule }},
		{"Input.Capabilities[0].Evidence", func(in *Input) { in.Capabilities[0].Evidence = []string{"other_metric"} }},
		{"Input.Capabilities[0].Reasons", func(in *Input) {
			in.Capabilities[0].Reasons = []string{discovery.ReasonCodes[0]}
		}},
		{"Input.CollectionStarted", func(in *Input) { in.CollectionStarted = in.CollectionStarted.Add(time.Minute) }},
		{"Input.CollectionCompleted", func(in *Input) { in.CollectionCompleted = in.CollectionCompleted.Add(time.Minute) }},
		{"Config.Metrics.TLSVerify", func(in *Input) { in.Config.Metrics.TLSVerify = false }},
		{"Config.Metrics.TimeoutSeconds", func(in *Input) { in.Config.Metrics.TimeoutSeconds = 99 }},
		{"Config.Cluster.Namespaces", func(in *Input) { in.Config.Cluster.Namespaces = []string{"other-ns"} }},
		{"Config.Cluster.NodepoolLabel", func(in *Input) { in.Config.Cluster.NodepoolLabel = "other-label" }},
		{"Config.Analysis.Period", func(in *Input) { in.Config.Analysis.Period = "7d" }},
		{"Config.Analysis.Timezone", func(in *Input) { in.Config.Analysis.Timezone = "America/New_York" }},
		{"Config.Analysis.NodeAutoscaling", func(in *Input) { in.Config.Analysis.NodeAutoscaling = "enabled" }},
		{"Config.Analysis.ExcludedContainers", func(in *Input) { in.Config.Analysis.ExcludedContainers = []string{"other-sidecar"} }},
		{"Config.Thresholds.WarningPercent", func(in *Input) { in.Config.Thresholds.WarningPercent = 71 }},
		{"Config.Thresholds.CriticalPercent", func(in *Input) { in.Config.Thresholds.CriticalPercent = 86 }},
		{"Config.Thresholds.LowUtilizationPercent", func(in *Input) { in.Config.Thresholds.LowUtilizationPercent = 31 }},
		{"Config.Thresholds.AutoscalerNearMaxPercent", func(in *Input) { in.Config.Thresholds.AutoscalerNearMaxPercent = 81 }},
		{"Config.Thresholds.NodepoolSpreadPoints", func(in *Input) { in.Config.Thresholds.NodepoolSpreadPoints = 31 }},
		{"Config.Thresholds.BurstyGapPercent", func(in *Input) { in.Config.Thresholds.BurstyGapPercent = 101 }},
		{"Config.Thresholds.HPAReplicaDivergence", func(in *Input) { in.Config.Thresholds.HPAReplicaDivergence = 2 }},
		{"Config.Report.Profile", func(in *Input) { in.Config.Report.Profile = "executive" }},
		{"Config.Report.MinSeverity", func(in *Input) { in.Config.Report.MinSeverity = "critical" }},
		{"Config.Report.Organization", func(in *Input) { in.Config.Report.Organization = "Other Org" }},
		{"Config.Report.Classification", func(in *Input) { in.Config.Report.Classification = "public" }},
		{"Config.Report.Author", func(in *Input) { in.Config.Report.Author = "other-author" }},
	}

	base := mustBuild(t, baseInput())
	baseBytes, err := Encode(base)
	if err != nil {
		t.Fatalf("Encode(base) error = %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput()
			tt.mutate(&in)

			changed := mustBuild(t, in)
			changedBytes, err := Encode(changed)
			if err != nil {
				t.Fatalf("Encode(changed) error = %v", err)
			}

			if string(changedBytes) == string(baseBytes) {
				t.Fatalf("changing %s did not change the encoded bytes", tt.name)
			}
		})
	}
}

func TestBuildWritesTimesInTheZone(t *testing.T) {
	in := baseInput()
	in.EmittedAt = time.Date(2026, 7, 15, 17, 30, 0, 0, time.UTC) // 14:30 in Sao Paulo (-03:00)
	in.Config.Analysis.Timezone = "America/Sao_Paulo"

	snap := mustBuild(t, in)

	if !strings.HasSuffix(snap.Report.EmittedAt, "-03:00") {
		t.Fatalf("Report.EmittedAt = %q, want a -03:00 offset", snap.Report.EmittedAt)
	}

	got, err := Time(snap.Report.EmittedAt)
	if err != nil {
		t.Fatalf("Time() error = %v", err)
	}
	if !got.Equal(in.EmittedAt) {
		t.Fatalf("Time(Report.EmittedAt) = %v, want the same instant as %v", got, in.EmittedAt)
	}
}

func TestBuildCutsToSeconds(t *testing.T) {
	in := baseInput()
	in.EmittedAt = time.Date(2026, 7, 15, 14, 30, 0, 700_000_000, time.UTC)

	snap := mustBuild(t, in)

	if strings.Contains(snap.Report.EmittedAt, ".") {
		t.Fatalf("Report.EmittedAt = %q, want no fractional seconds", snap.Report.EmittedAt)
	}
}

func TestBuildPeriodStart(t *testing.T) {
	saoPaulo, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("LoadLocation(Sao Paulo): %v", err)
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation(New York): %v", err)
	}

	tests := []struct {
		name      string
		emittedAt time.Time
		timezone  string
		period    string
		want      string
	}{
		{
			name:      "24h in Sao Paulo",
			emittedAt: time.Date(2026, 7, 15, 14, 30, 0, 0, saoPaulo),
			timezone:  "America/Sao_Paulo",
			period:    "24h",
			want:      "2026-07-14T14:30:00-03:00",
		},
		{
			name:      "7d in Sao Paulo",
			emittedAt: time.Date(2026, 7, 15, 14, 30, 0, 0, saoPaulo),
			timezone:  "America/Sao_Paulo",
			period:    "7d",
			want:      "2026-07-08T14:30:00-03:00",
		},
		{
			name:      "24h across a New York DST start",
			emittedAt: time.Date(2026, 3, 8, 12, 0, 0, 0, newYork),
			timezone:  "America/New_York",
			period:    "24h",
			want:      "2026-03-07T12:00:00-05:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := baseInput()
			in.EmittedAt = tt.emittedAt
			in.Config.Analysis.Timezone = tt.timezone
			in.Config.Analysis.Period = tt.period

			snap := mustBuild(t, in)
			if snap.Report.PeriodStart != tt.want {
				t.Fatalf("Report.PeriodStart = %q, want %q", snap.Report.PeriodStart, tt.want)
			}
		})
	}
}

func TestBuildSortsLists(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"b", "a"}
	in.Config.Analysis.ExcludedContainers = []string{"y", "x"}

	snap := mustBuild(t, in)

	if got, want := snap.Scope.Namespaces, []string{"a", "b"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Scope.Namespaces = %v, want %v", got, want)
	}
	if got, want := snap.Scope.ExcludedContainers, []string{"x", "y"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("Scope.ExcludedContainers = %v, want %v", got, want)
	}
}

func TestBuildNeverWritesNull(t *testing.T) {
	in := baseInput()
	in.Config.Analysis.ExcludedContainers = nil
	in.Capabilities[0].Evidence = nil
	in.Capabilities[0].Reasons = nil

	snap := mustBuild(t, in)
	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	if strings.Contains(string(data), "null") {
		t.Fatalf("Encode() contains \"null\": %s", data)
	}
	if !strings.Contains(string(data), "[]") {
		t.Fatalf("Encode() has no \"[]\" for the empty lists: %s", data)
	}
}

func TestBuildKeepsSecretsOut(t *testing.T) {
	in := baseInput()
	in.Config.Metrics.URL = "https://metrics.internal.example:9443/prom"
	in.Config.Metrics.TokenEnv = "KCR_SECRET_NAME"
	in.Config.Metrics.CABundle = "/etc/ssl/private/ca.pem"
	in.Config.Report.OutputDirectory = "/var/reports"

	snap := mustBuild(t, in)
	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	for _, secret := range []string{
		"metrics.internal.example", "9443", "hunter2", "KCR_SECRET_NAME", "ca.pem", "/var/reports",
	} {
		if strings.Contains(string(data), secret) {
			t.Fatalf("Encode() contains %q: %s", secret, data)
		}
	}
}
