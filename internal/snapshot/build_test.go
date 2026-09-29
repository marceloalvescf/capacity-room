package snapshot

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// windowFor computes the query window for analysis at emittedAt, the same
// call runSnapshot and runPDF make before building a snapshot. It panics on
// an invalid window: every case in this file starts from a period and a
// time zone that agree.
func windowFor(analysis config.Analysis, emittedAt time.Time) collect.Window {
	w, err := collect.NewWindow(analysis, emittedAt)
	if err != nil {
		panic(err)
	}
	return w
}

// unavailableResources gives all ten statistics of a row the state
// unavailable and no number.
func unavailableResources() collect.Resources {
	u := collect.Statistics{State: collect.StateUnavailable}
	return collect.Resources{
		CPUUsage: u, MemoryUsage: u, CPURequests: u, MemoryRequests: u,
		CPULimits: u, MemoryLimits: u, CPURequestPercent: u, MemoryRequestPercent: u,
		CPULimitPercent: u, MemoryLimitPercent: u,
	}
}

// knownResources gives all ten statistics of a row the state value and a
// full, known number: no field of it is null.
func knownResources() collect.Resources {
	v := collect.Statistics{State: collect.StateValue, Average: 1, Minimum: 1, Peak: 1, Current: 1}
	return collect.Resources{
		CPUUsage: v, MemoryUsage: v, CPURequests: v, MemoryRequests: v,
		CPULimits: v, MemoryLimits: v, CPURequestPercent: v, MemoryRequestPercent: v,
		CPULimitPercent: v, MemoryLimitPercent: v,
	}
}

// knownResourcesNamespaces returns one row for each name of names, sorted,
// with a known pod count and a full, known statistic: no field of any row
// is null.
func knownResourcesNamespaces(names []string) []collect.Namespace {
	sorted := append([]string{}, names...)
	sort.Strings(sorted)
	rows := make([]collect.Namespace, len(sorted))
	for i, name := range sorted {
		rows[i] = collect.Namespace{Name: name, Pods: 1, PodsKnown: true, Resources: knownResources()}
	}
	return rows
}

// baseCollectedNamespaces returns one row for each name of names, sorted,
// every statistic unavailable, so that a snapshot built from these rows
// passes Check's rule that the namespaces of the snapshot are exactly the
// namespaces of the scope.
func baseCollectedNamespaces(names []string) []collect.Namespace {
	sorted := append([]string{}, names...)
	sort.Strings(sorted)
	rows := make([]collect.Namespace, len(sorted))
	for i, name := range sorted {
		rows[i] = collect.Namespace{Name: name, Resources: unavailableResources()}
	}
	return rows
}

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
	in := Input{
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
	in.Window = windowFor(in.Config.Analysis, in.EmittedAt)
	in.Collected = collect.Collected{
		Namespaces: collect.Result[collect.Namespace]{Rows: baseCollectedNamespaces(in.Config.Cluster.Namespaces)},
	}
	return in
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
		{"Input.EmittedAt", func(in *Input) {
			in.EmittedAt = in.EmittedAt.Add(time.Hour)
			in.Window = windowFor(in.Config.Analysis, in.EmittedAt)
		}},
		{"Input.EndpointKind", func(in *Input) { in.EndpointKind = "grafana" }},
		{"Input.Scope.ClusterName", func(in *Input) {
			in.Scope.ClusterName = "other"
		}},
		{"Input.Scope.ClusterLabel", func(in *Input) { in.Scope.ClusterLabel = "k8s_cluster" }},
		{"Input.Scope.FilterMode", func(in *Input) { in.Scope.FilterMode = "auto" }},
		{"Input.Scope.MatcherEnabled", func(in *Input) { in.Scope.MatcherEnabled = false }},
		{"Input.Scope.Confidence", func(in *Input) { in.Scope.Confidence = discovery.ConfidenceInferred }},
		{"Input.Scope.DiscoveredValues", func(in *Input) { in.Scope.DiscoveredValues = []string{"only"} }},
		{"Input.Capabilities[0].State", func(in *Input) {
			// A partial capability must carry a reason, so the reason is
			// set alongside the state being tested here.
			in.Capabilities[0].State = discovery.StatePartial
			in.Capabilities[0].Reasons = []string{discovery.ReasonCodes[0]}
		}},
		{"Input.Capabilities[0].Strategy", func(in *Input) { in.Capabilities[0].Strategy = discovery.StrategyRecordingRule }},
		{"Input.Capabilities[0].Evidence", func(in *Input) { in.Capabilities[0].Evidence = []string{"other_metric"} }},
		{"Input.Capabilities[0].Reasons", func(in *Input) {
			// An available capability may carry no reason, so the state
			// and strategy are moved to unavailable alongside the reason
			// being tested here.
			in.Capabilities[0].State = discovery.StateUnavailable
			in.Capabilities[0].Strategy = discovery.StrategyNone
			in.Capabilities[0].Reasons = []string{discovery.ReasonCodes[1]}
		}},
		{"Input.CollectionStarted", func(in *Input) { in.CollectionStarted = in.CollectionStarted.Add(time.Minute) }},
		{"Input.CollectionCompleted", func(in *Input) { in.CollectionCompleted = in.CollectionCompleted.Add(time.Minute) }},
		{"Config.Metrics.TLSVerify", func(in *Input) { in.Config.Metrics.TLSVerify = false }},
		{"Config.Metrics.TimeoutSeconds", func(in *Input) { in.Config.Metrics.TimeoutSeconds = 99 }},
		{"Config.Cluster.Namespaces", func(in *Input) {
			in.Config.Cluster.Namespaces = []string{"other-ns"}
			in.Collected.Namespaces.Rows = baseCollectedNamespaces(in.Config.Cluster.Namespaces)
		}},
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
	in.Window = windowFor(in.Config.Analysis, in.EmittedAt)

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

// TestBuildTakesThePeriodFromTheWindow covers that Build takes
// report.period_start and report.period_end from Input.Window, not from a
// computation of its own: the window given here holds values a fresh
// computation from Input.EmittedAt and Input.Config.Analysis would never
// produce, so the test only passes if Build actually reads the window.
func TestBuildTakesThePeriodFromTheWindow(t *testing.T) {
	in := baseInput()
	in.Config.Analysis.Timezone = "UTC"
	in.Window = collect.Window{
		Start: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		End:   time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
	}
	in.EmittedAt = in.Window.End

	snap := mustBuild(t, in)

	if snap.Report.PeriodStart != "2020-01-01T00:00:00Z" {
		t.Fatalf("Report.PeriodStart = %q, want %q", snap.Report.PeriodStart, "2020-01-01T00:00:00Z")
	}
	if snap.Report.PeriodEnd != "2020-01-02T00:00:00Z" {
		t.Fatalf("Report.PeriodEnd = %q, want %q", snap.Report.PeriodEnd, "2020-01-02T00:00:00Z")
	}
}

func TestBuildSortsLists(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"b", "a"}
	in.Config.Analysis.ExcludedContainers = []string{"y", "x"}
	in.Collected.Namespaces.Rows = baseCollectedNamespaces(in.Config.Cluster.Namespaces)

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
	// A row with an unmeasured statistic or an unknown pod count does write
	// null, on purpose: that is covered by TestBuildWritesNullForNoNumber
	// and TestBuildWritesUnknownPodsAsNull, not by this test.
	in.Collected.Namespaces.Rows = knownResourcesNamespaces(in.Config.Cluster.Namespaces)

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

// directCompleteResources gives the ten statistics of the namespace
// "payments" and of its single workload "payments-web" of the
// "direct-complete" scenario: the numbers TestNamespaceNumbersOfTheScenario
// of internal/collect/namespaces_test.go transcribes for the namespace.
// The workload consumes the whole namespace, so it carries the same
// numbers, as TestFindingsOfDirectComplete of
// internal/analysis/analysis_test.go notes for its own findings.
func directCompleteResources() collect.Resources {
	return collect.Resources{
		CPUUsage:             collect.Statistics{State: collect.StateValue, Average: 0.30000000000000004, Minimum: 0.2, Peak: 0.4, Current: 0.4},
		MemoryUsage:          collect.Statistics{State: collect.StateValue, Average: 110.0, Minimum: 100.0, Peak: 120.0, Current: 120.0},
		CPURequests:          collect.Statistics{State: collect.StateValue, Average: 0.5, Minimum: 0.5, Peak: 0.5, Current: 0.5},
		MemoryRequests:       collect.Statistics{State: collect.StateValue, Average: 0.5, Minimum: 0.5, Peak: 0.5, Current: 0.5},
		CPULimits:            collect.Statistics{State: collect.StateValue, Average: 1.0, Minimum: 1.0, Peak: 1.0, Current: 1.0},
		MemoryLimits:         collect.Statistics{State: collect.StateValue, Average: 1.0, Minimum: 1.0, Peak: 1.0, Current: 1.0},
		CPURequestPercent:    collect.Statistics{State: collect.StateValue, Average: 60.0, Minimum: 40.0, Peak: 80.0, Current: 80.0},
		MemoryRequestPercent: collect.Statistics{State: collect.StateValue, Average: 22000.0, Minimum: 20000.0, Peak: 24000.0, Current: 24000.0},
		CPULimitPercent:      collect.Statistics{State: collect.StateValue, Average: 30.0, Minimum: 20.0, Peak: 40.0, Current: 40.0},
		MemoryLimitPercent:   collect.Statistics{State: collect.StateValue, Average: 11000.0, Minimum: 10000.0, Peak: 12000.0, Current: 12000.0},
	}
}

// checkSnapshotStatistics compares got, one of a row's ten statistics,
// against want, the collect.Statistics it must have come from: every
// number of want is present in got, since every statistic directComplete
// gives is of state value.
func checkSnapshotStatistics(t *testing.T, label string, got Statistics, want collect.Statistics) {
	t.Helper()
	if got.State != want.State {
		t.Errorf("%s.State = %q, want %q", label, got.State, want.State)
	}
	checkPtr(t, label+".Average", got.Average, want.Average)
	checkPtr(t, label+".Minimum", got.Minimum, want.Minimum)
	checkPtr(t, label+".Peak", got.Peak, want.Peak)
	checkPtr(t, label+".Current", got.Current, want.Current)
}

func checkPtr(t *testing.T, label string, got *float64, want float64) {
	t.Helper()
	if got == nil || *got != want {
		t.Errorf("%s = %v, want %v", label, got, want)
	}
}

// TestBuildCopiesEveryRow builds a snapshot from the one namespace and the
// one workload of the "direct-complete" scenario and checks every field of
// each row, and of each of its eight findings, against the numbers
// TestNamespaceNumbersOfTheScenario (internal/collect/namespaces_test.go)
// and TestFindingsOfDirectComplete (internal/analysis/analysis_test.go)
// give for that same scenario.
func TestBuildCopiesEveryRow(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}

	resources := directCompleteResources()
	namespaces := []collect.Namespace{{
		Name: "payments", Pods: 3, PodsKnown: true, Approximate: false,
		RequestsLimitsState: discovery.StateAvailable, Resources: resources,
	}}
	workloads := []collect.Workload{{
		Namespace: "payments", Kind: "Deployment", Name: "payments-web",
		Pods: 1, PodsKnown: true, Approximate: false,
		RequestsLimitsState: discovery.StateAvailable, Resources: resources,
	}}
	collected := collect.Collected{
		Namespaces: collect.Result[collect.Namespace]{Rows: namespaces},
		Workloads:  collect.Result[collect.Workload]{Rows: workloads},
	}
	in.Collected = collected
	in.Findings = analysis.All(collected, in.Config.Thresholds)

	snap := mustBuild(t, in)

	if len(snap.Namespaces) != 1 {
		t.Fatalf("len(Namespaces) = %d, want 1", len(snap.Namespaces))
	}
	ns := snap.Namespaces[0]
	if ns.Name != "payments" {
		t.Errorf("Namespaces[0].Name = %q, want %q", ns.Name, "payments")
	}
	if ns.Pods == nil || *ns.Pods != 3 {
		t.Errorf("Namespaces[0].Pods = %v, want 3", ns.Pods)
	}
	if ns.Approximate {
		t.Errorf("Namespaces[0].Approximate = true, want false")
	}
	checkSnapshotStatistics(t, "Namespaces[0].CPUUsage", ns.CPUUsage, resources.CPUUsage)
	checkSnapshotStatistics(t, "Namespaces[0].MemoryUsage", ns.MemoryUsage, resources.MemoryUsage)
	checkSnapshotStatistics(t, "Namespaces[0].CPURequests", ns.CPURequests, resources.CPURequests)
	checkSnapshotStatistics(t, "Namespaces[0].MemoryRequests", ns.MemoryRequests, resources.MemoryRequests)
	checkSnapshotStatistics(t, "Namespaces[0].CPULimits", ns.CPULimits, resources.CPULimits)
	checkSnapshotStatistics(t, "Namespaces[0].MemoryLimits", ns.MemoryLimits, resources.MemoryLimits)
	checkSnapshotStatistics(t, "Namespaces[0].CPURequestPercent", ns.CPURequestPercent, resources.CPURequestPercent)
	checkSnapshotStatistics(t, "Namespaces[0].MemoryRequestPercent", ns.MemoryRequestPercent, resources.MemoryRequestPercent)
	checkSnapshotStatistics(t, "Namespaces[0].CPULimitPercent", ns.CPULimitPercent, resources.CPULimitPercent)
	checkSnapshotStatistics(t, "Namespaces[0].MemoryLimitPercent", ns.MemoryLimitPercent, resources.MemoryLimitPercent)

	if len(snap.Workloads) != 1 {
		t.Fatalf("len(Workloads) = %d, want 1", len(snap.Workloads))
	}
	wl := snap.Workloads[0]
	if wl.Namespace != "payments" || wl.Kind != "Deployment" || wl.Name != "payments-web" {
		t.Errorf("Workloads[0] = %s/%s/%s, want payments/Deployment/payments-web", wl.Namespace, wl.Kind, wl.Name)
	}
	if wl.Pods == nil || *wl.Pods != 1 {
		t.Errorf("Workloads[0].Pods = %v, want 1", wl.Pods)
	}
	if wl.Approximate {
		t.Errorf("Workloads[0].Approximate = true, want false")
	}
	checkSnapshotStatistics(t, "Workloads[0].CPUUsage", wl.CPUUsage, resources.CPUUsage)
	checkSnapshotStatistics(t, "Workloads[0].MemoryUsage", wl.MemoryUsage, resources.MemoryUsage)
	checkSnapshotStatistics(t, "Workloads[0].CPURequests", wl.CPURequests, resources.CPURequests)
	checkSnapshotStatistics(t, "Workloads[0].MemoryRequests", wl.MemoryRequests, resources.MemoryRequests)
	checkSnapshotStatistics(t, "Workloads[0].CPULimits", wl.CPULimits, resources.CPULimits)
	checkSnapshotStatistics(t, "Workloads[0].MemoryLimits", wl.MemoryLimits, resources.MemoryLimits)
	checkSnapshotStatistics(t, "Workloads[0].CPURequestPercent", wl.CPURequestPercent, resources.CPURequestPercent)
	checkSnapshotStatistics(t, "Workloads[0].MemoryRequestPercent", wl.MemoryRequestPercent, resources.MemoryRequestPercent)
	checkSnapshotStatistics(t, "Workloads[0].CPULimitPercent", wl.CPULimitPercent, resources.CPULimitPercent)
	checkSnapshotStatistics(t, "Workloads[0].MemoryLimitPercent", wl.MemoryLimitPercent, resources.MemoryLimitPercent)

	wantCodes := []string{
		"NAMESPACE_MEMORY_LIMIT_CRITICAL", "NAMESPACE_MEMORY_REQUEST_CRITICAL",
		"WORKLOAD_MEMORY_LIMIT_CRITICAL", "WORKLOAD_MEMORY_REQUEST_CRITICAL",
		"NAMESPACE_CPU_REQUEST_ATTENTION", "WORKLOAD_CPU_REQUEST_ATTENTION",
		"NAMESPACE_CPU_LIMIT_HEALTHY", "WORKLOAD_CPU_LIMIT_HEALTHY",
	}
	if len(snap.Findings) != len(wantCodes) {
		t.Fatalf("len(Findings) = %d, want %d: %+v", len(snap.Findings), len(wantCodes), snap.Findings)
	}
	for i, want := range in.Findings {
		got := snap.Findings[i]
		if got.Code != wantCodes[i] {
			t.Errorf("Findings[%d].Code = %q, want %q", i, got.Code, wantCodes[i])
		}
		if got.Severity != want.Severity {
			t.Errorf("Findings[%d].Severity = %q, want %q", i, got.Severity, want.Severity)
		}
		if got.ScopeType != want.ScopeType {
			t.Errorf("Findings[%d].ScopeType = %q, want %q", i, got.ScopeType, want.ScopeType)
		}
		if got.ScopeKind != want.ScopeKind {
			t.Errorf("Findings[%d].ScopeKind = %q, want %q", i, got.ScopeKind, want.ScopeKind)
		}
		if got.ScopeName != want.ScopeName {
			t.Errorf("Findings[%d].ScopeName = %q, want %q", i, got.ScopeName, want.ScopeName)
		}
		if got.Confidence != want.Confidence {
			t.Errorf("Findings[%d].Confidence = %q, want %q", i, got.Confidence, want.Confidence)
		}
		if len(got.Evidence) != len(want.Evidence) {
			t.Errorf("Findings[%d].Evidence = %+v, want %+v", i, got.Evidence, want.Evidence)
			continue
		}
		for j, e := range want.Evidence {
			if got.Evidence[j].Name != e.Name || got.Evidence[j].Value != e.Value {
				t.Errorf("Findings[%d].Evidence[%d] = %+v, want %+v", i, j, got.Evidence[j], e)
			}
		}
	}
}

// TestBuildWritesNullForNoNumber covers a statistic of state no_base and
// one of state unavailable: Encode writes null for all four numbers of
// each, never a number.
func TestBuildWritesNullForNoNumber(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}
	resources := unavailableResources()
	resources.CPURequestPercent = collect.Statistics{State: collect.StateNoBase}
	in.Collected = collect.Collected{
		Namespaces: collect.Result[collect.Namespace]{Rows: []collect.Namespace{{
			Name: "payments", Resources: resources,
		}}},
	}

	snap := mustBuild(t, in)
	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}

	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	namespace := tree["namespaces"].([]any)[0].(map[string]any)

	noBase := namespace["cpu_request_percent"].(map[string]any)
	if noBase["state"] != collect.StateNoBase {
		t.Errorf("cpu_request_percent.state = %v, want %q", noBase["state"], collect.StateNoBase)
	}
	unavailable := namespace["cpu_usage"].(map[string]any)
	if unavailable["state"] != collect.StateUnavailable {
		t.Errorf("cpu_usage.state = %v, want %q", unavailable["state"], collect.StateUnavailable)
	}
	for _, stat := range []map[string]any{noBase, unavailable} {
		for _, key := range []string{"average", "minimum", "peak", "current"} {
			if value, ok := stat[key]; !ok || value != nil {
				t.Errorf("%s = %v, want null", key, value)
			}
		}
	}
}

// TestBuildWritesUnknownPodsAsNull covers a row whose pod count is not
// known: Build writes a nil Pods, and Encode writes null for it.
func TestBuildWritesUnknownPodsAsNull(t *testing.T) {
	in := baseInput()
	in.Config.Cluster.Namespaces = []string{"payments"}
	in.Collected = collect.Collected{
		Namespaces: collect.Result[collect.Namespace]{Rows: []collect.Namespace{{
			Name: "payments", PodsKnown: false, Resources: unavailableResources(),
		}}},
	}

	snap := mustBuild(t, in)
	if snap.Namespaces[0].Pods != nil {
		t.Fatalf("Namespaces[0].Pods = %v, want nil", snap.Namespaces[0].Pods)
	}

	data, err := Encode(snap)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	namespace := tree["namespaces"].([]any)[0].(map[string]any)
	if value, ok := namespace["pods"]; !ok || value != nil {
		t.Fatalf("namespaces[0].pods = %v, want null", value)
	}
}
