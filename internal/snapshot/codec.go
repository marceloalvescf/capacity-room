package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// The values report.profile, report.min_severity, scope.filter_mode,
// scope.confidence, and report.settings.endpoint_type may hold. The
// snapshot keeps this list of its own, distinct from config's and
// discovery's, because a stored file must keep the same meaning even if
// those packages change later.
var (
	validProfiles      = []string{"detailed", "executive"}
	validSeverities    = []string{"all", "attention", "critical"}
	validFilterModes   = []string{"required", "auto", "none"}
	validConfidences   = []string{discovery.ConfidenceExact, discovery.ConfidenceInferred, discovery.ConfidenceUnknown}
	validEndpointTypes = []string{"prometheus", "grafana"}
)

// Encode writes s with two spaces of indentation and one line break at the
// end, without escaping "<", ">", or "&" for HTML. The same snapshot always
// gives the same bytes.
func Encode(s Snapshot) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode reads exactly one snapshot of schema_version "1.0" from data and
// checks it, in this order: the JSON is well formed and holds no unknown
// key, nothing follows the document, schema_version is present, it equals
// "1.0", and Check passes.
func Decode(data []byte) (Snapshot, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()

	var snap Snapshot
	if err := dec.Decode(&snap); err != nil {
		if strings.Contains(err.Error(), "unknown field") {
			return Snapshot{}, fmt.Errorf("snapshot holds an unknown key: %s", err)
		}
		return Snapshot{}, fmt.Errorf("snapshot is not valid JSON: %s", err)
	}

	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("snapshot holds data after the document")
	}

	if snap.SchemaVersion == "" {
		return Snapshot{}, errors.New("snapshot has no schema_version")
	}
	if snap.SchemaVersion != SchemaVersion {
		return Snapshot{}, fmt.Errorf("snapshot has schema_version %q; this version reads %q", snap.SchemaVersion, SchemaVersion)
	}

	if err := snap.Check(); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// Time reads one of the time fields of a snapshot: RFC 3339, with an
// offset.
func Time(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}

// Check reports the first integrity failure of s, in the fixed order of
// the rules below.
func (s Snapshot) Check() error {
	if s.Report.ID == "" {
		return errors.New("report.id is empty")
	}
	if s.Report.GeneratorVersion == "" {
		return errors.New("report.generator_version is empty")
	}

	times, err := s.parseTimes()
	if err != nil {
		return err
	}

	if !times.periodEnd.Equal(times.emittedAt) {
		return errors.New("report.period_end differs from report.emitted_at")
	}
	if !times.periodStart.Before(times.periodEnd) {
		return errors.New("report.period_start is not before report.period_end")
	}
	if times.collectionCompleted.Before(times.collectionStarted) {
		return errors.New("quality.collection_completed_at is before quality.collection_started_at")
	}

	if err := checkEnum("report.profile", s.Report.Profile, validProfiles); err != nil {
		return err
	}
	if err := checkEnum("report.min_severity", s.Report.MinSeverity, validSeverities); err != nil {
		return err
	}
	if err := checkEnum("scope.filter_mode", s.Scope.FilterMode, validFilterModes); err != nil {
		return err
	}
	if err := checkEnum("scope.confidence", s.Scope.Confidence, validConfidences); err != nil {
		return err
	}
	if err := checkEnum("report.settings.endpoint_type", s.Report.Settings.EndpointType, validEndpointTypes); err != nil {
		return err
	}

	if s.Scope.ClusterName == "" {
		return errors.New("scope.cluster_name is empty")
	}
	if len(s.Scope.Namespaces) == 0 {
		return errors.New("scope.namespaces is empty")
	}

	if err := checkList("scope.discovered_values", s.Scope.DiscoveredValues); err != nil {
		return err
	}
	if err := checkSorted("scope.namespaces", s.Scope.Namespaces); err != nil {
		return err
	}
	if err := checkList("scope.excluded_containers", s.Scope.ExcludedContainers); err != nil {
		return err
	}

	if err := checkCapabilityNames(s.Quality.Capabilities); err != nil {
		return err
	}
	for i, c := range s.Quality.Capabilities {
		if err := checkCapability(i, c); err != nil {
			return err
		}
	}

	return nil
}

// snapshotTimes holds every time field of a Snapshot, already parsed.
type snapshotTimes struct {
	emittedAt           time.Time
	periodStart         time.Time
	periodEnd           time.Time
	collectionStarted   time.Time
	collectionCompleted time.Time
}

// parseTimes parses every time field of s, in the order they appear in the
// document, and reports the first one that is not an RFC 3339 time.
func (s Snapshot) parseTimes() (snapshotTimes, error) {
	var out snapshotTimes
	fields := []struct {
		path  string
		value string
		dst   *time.Time
	}{
		{"report.emitted_at", s.Report.EmittedAt, &out.emittedAt},
		{"report.period_start", s.Report.PeriodStart, &out.periodStart},
		{"report.period_end", s.Report.PeriodEnd, &out.periodEnd},
		{"quality.collection_started_at", s.Quality.CollectionStartedAt, &out.collectionStarted},
		{"quality.collection_completed_at", s.Quality.CollectionCompletedAt, &out.collectionCompleted},
	}

	for _, f := range fields {
		value, err := Time(f.value)
		if err != nil {
			return snapshotTimes{}, fmt.Errorf("%s is not an RFC 3339 time: %q", f.path, f.value)
		}
		*f.dst = value
	}
	return out, nil
}

// checkEnum refuses a value that is not one of valid.
func checkEnum(path, value string, valid []string) error {
	if slices.Contains(valid, value) {
		return nil
	}
	return fmt.Errorf("%s has an unknown value: %q", path, value)
}

// checkList refuses a nil list, whose distinct message tells it apart from
// a valid, empty one, then checks it is sorted and holds no repeat.
func checkList(path string, values []string) error {
	if values == nil {
		return fmt.Errorf("%s is null; an empty list is written as []", path)
	}
	return checkSorted(path, values)
}

// checkSorted refuses a list that is not sorted or holds a repeated value.
func checkSorted(path string, values []string) error {
	for i := 1; i < len(values); i++ {
		if values[i-1] >= values[i] {
			return fmt.Errorf("%s is not sorted or holds a repeated value", path)
		}
	}
	return nil
}

// checkCapabilityNames refuses a capabilities list that does not hold the
// nine names of discovery.Names, in that order.
func checkCapabilityNames(capabilities []Capability) error {
	names := make([]string, len(capabilities))
	for i, c := range capabilities {
		names[i] = c.Name
	}
	if slices.Equal(names, discovery.Names) {
		return nil
	}
	return fmt.Errorf("quality.capabilities must hold the nine capabilities in their order; found: %s", strings.Join(names, ", "))
}

// checkCapability refuses the capability at index i when its evidence or
// reasons is nil, or when its state, strategy, or a reason falls outside
// the registry.
func checkCapability(i int, c Capability) error {
	if c.Evidence == nil {
		return fmt.Errorf("quality.capabilities[%d].evidence is null; an empty list is written as []", i)
	}
	if c.Reasons == nil {
		return fmt.Errorf("quality.capabilities[%d].reasons is null; an empty list is written as []", i)
	}
	if !slices.Contains(discovery.States, c.State) {
		return fmt.Errorf("quality.capabilities[%d].state has an unknown value: %q", i, c.State)
	}
	if !slices.Contains(discovery.Strategies, c.Strategy) {
		return fmt.Errorf("quality.capabilities[%d].strategy has an unknown value: %q", i, c.Strategy)
	}
	for _, reason := range c.Reasons {
		if !slices.Contains(discovery.ReasonCodes, reason) {
			return fmt.Errorf("quality.capabilities[%d].reasons has an unknown value: %q", i, reason)
		}
	}
	return nil
}
