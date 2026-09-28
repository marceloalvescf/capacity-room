package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
)

// periodRe matches report.period: a positive integer with no leading zero,
// followed by m, h, or d. internal/config carries the same rule for the
// configuration; this copy is deliberate, not a shortcut: the snapshot is a
// stored contract and must be able to judge a file that no configuration of
// this version produced.
var periodRe = regexp.MustCompile(`^[1-9][0-9]*[mhd]$`)

// validProfiles, validSeverities, validFilterModes, validEndpointTypes, and
// validNodeAutoscaling are the values of the configuration that report and
// settings may hold. They are written here as literals, not read from
// internal/config, so that a stored file keeps its meaning even if config's
// own rules change later.
//
// validConfidences is the one list that is not a literal: discovery keeps
// no list of the confidences, only the three constants ConfidenceExact,
// ConfidenceInferred, and ConfidenceUnknown, so this is the closest a list
// of its own can come. It is grouped here with the other four registries of
// discovery's this file reuses (Names, States, Strategies, ReasonCodes, in
// checkCapabilityNames and checkCapability below): spec 7.2 makes
// discovery's registry the single list of codes for what discovery itself
// decides, and confidence is one such code, not a value of the
// configuration.
var (
	validProfiles        = []string{"detailed", "executive"}
	validSeverities      = []string{"all", "attention", "critical"}
	validFilterModes     = []string{"required", "auto", "none"}
	validEndpointTypes   = []string{"prometheus", "grafana"}
	validNodeAutoscaling = []string{"auto", "enabled", "disabled"}
	validConfidences     = []string{discovery.ConfidenceExact, discovery.ConfidenceInferred, discovery.ConfidenceUnknown}
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
// key, nothing follows the document, the file holds every key of the
// contract, schema_version is present, it equals "1.0", and Check passes.
//
// encoding/json reads a missing key as the zero value, and zero is a valid
// threshold, a valid boolean, and a valid empty list, so a decoded Snapshot
// alone cannot tell "the run wrote this" from "the key was left out". The
// key-presence check below reads data a second time, into a plain
// map[string]any, and compares it against Snapshot's own fields by
// reflect, so that a field added to a type is covered without a second
// list to keep.
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

	var tree map[string]any
	if err := json.Unmarshal(data, &tree); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot is not valid JSON: %s", err)
	}
	if path, missing := firstMissingKey(reflect.TypeOf(Snapshot{}), tree, ""); missing {
		return Snapshot{}, fmt.Errorf("snapshot lacks the key %s", path)
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

// jsonName returns the name a struct field's json tag holds, ignoring any
// option after a comma (such as omitempty; no field of this package uses
// one, but a bare split keeps this correct if one is ever added).
func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

// firstMissingKey walks t's fields in their declared order, following tree
// (the generic decoding of the same document) at each level, and reports
// the path of the first key t declares that tree does not hold. A struct
// field is followed into its own object; a slice field whose element is
// itself a struct (only quality.capabilities) is followed into as many
// elements as tree actually holds, each named by its index, so a short or
// long array is checked exactly as far as it goes. schema_version is
// skipped: its absence keeps the dedicated message Decode already gives it.
func firstMissingKey(t reflect.Type, tree any, prefix string) (string, bool) {
	obj, ok := tree.(map[string]any)
	if !ok {
		return "", false
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		name := jsonName(field)
		if name == "schema_version" {
			continue
		}

		path := name
		if prefix != "" {
			path = prefix + "." + name
		}

		value, present := obj[name]
		if !present {
			return path, true
		}

		switch field.Type.Kind() {
		case reflect.Struct:
			if p, found := firstMissingKey(field.Type, value, path); found {
				return p, true
			}
		case reflect.Slice:
			if field.Type.Elem().Kind() != reflect.Struct {
				continue
			}
			arr, ok := value.([]any)
			if !ok {
				continue
			}
			for idx, elem := range arr {
				elemPath := fmt.Sprintf("%s[%d]", path, idx)
				if p, found := firstMissingKey(field.Type.Elem(), elem, elemPath); found {
					return p, true
				}
			}
		}
	}
	return "", false
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

	if !periodRe.MatchString(s.Report.Period) {
		return fmt.Errorf("report.period has an unknown value: %q", s.Report.Period)
	}
	if err := checkEnum("report.settings.node_autoscaling", s.Report.Settings.NodeAutoscaling, validNodeAutoscaling); err != nil {
		return err
	}
	if s.Report.Timezone == "" {
		return errors.New("report.timezone is empty")
	}
	if s.Scope.ClusterLabel == "" {
		return errors.New("scope.cluster_label is empty")
	}
	if s.Report.Settings.TimeoutSeconds <= 0 {
		return errors.New("report.settings.timeout_seconds must be positive")
	}

	percents := []float64{
		s.Report.Thresholds.WarningPercent,
		s.Report.Thresholds.CriticalPercent,
		s.Report.Thresholds.LowUtilizationPercent,
		s.Report.Thresholds.AutoscalerNearMaxPercent,
		s.Report.Thresholds.NodepoolSpreadPoints,
	}
	for _, p := range percents {
		if p < 0 || p > 100 {
			return errors.New("report.thresholds values must be within 0..100")
		}
	}
	if s.Report.Thresholds.BurstyGapPercent < 0 {
		return errors.New("report.thresholds.bursty_gap_percent must not be negative")
	}
	if s.Report.Thresholds.HPAReplicaDivergence < 0 {
		return errors.New("report.thresholds.hpa_replica_divergence must not be negative")
	}
	if s.Report.Thresholds.WarningPercent >= s.Report.Thresholds.CriticalPercent {
		return errors.New("report.thresholds.warning_percent must be lower than critical_percent")
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
