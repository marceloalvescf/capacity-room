package snapshot

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strconv"
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

// periodUnitNanos returns the length, in nanoseconds, of one unit of a
// period's trailing letter: 'm' for minutes, 'h' for hours, and anything
// else (periodRe accepts only 'd' besides those two) for days.
func periodUnitNanos(unit byte) int64 {
	switch unit {
	case 'm':
		return int64(time.Minute)
	case 'h':
		return int64(time.Hour)
	default: // 'd', the only other letter periodRe accepts
		return int64(24 * time.Hour)
	}
}

// periodTooLong reports whether period, already known to match periodRe,
// names a length that does not fit in a time.Duration: more digits than an
// int64 can hold, or a value whose product with its unit overflows. The
// design sets no upper limit of its own on a period, so the only limit is
// what the type that carries it can hold.
func periodTooLong(period string) bool {
	value, err := strconv.ParseInt(period[:len(period)-1], 10, 64)
	if err != nil {
		return true
	}
	unit := periodUnitNanos(period[len(period)-1])
	return value > math.MaxInt64/unit
}

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

// Decode reads exactly one snapshot from data and checks it.
//
// The document is decoded loosely exactly once, into a plain map: that map
// is the one reading of the file, and everything Decode judges afterwards
// — the schema version, every key of the contract, and the typed Snapshot
// itself — is read from it, never from the bytes a second time. Two
// independent readings of the same bytes can disagree on which value a
// repeated, differently-cased key carries (encoding/json's own struct
// decode matches a field's name without regard to letter case; a map does
// not), and a second reading that keeps validating the value the first
// reading discarded is a check that says yes to what it did not look at.
//
// The order below is fixed: a file that is not JSON, or is cut in the
// middle; anything after the first document; a document that is not a JSON
// object; a missing, null, or non-string schema_version; another
// schema_version; then one walk of the map against the contract's types
// (an unknown key, a missing key, a null, or a value of the wrong kind, in
// that order at each key); the typed Snapshot, filled from the same map
// once it has passed that walk; and finally Check.
func Decode(data []byte) (Snapshot, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var loose any
	if err := dec.Decode(&loose); err != nil {
		return Snapshot{}, fmt.Errorf("snapshot is not valid JSON: %s", err)
	}
	if err := dec.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		return Snapshot{}, errors.New("snapshot holds data after the document")
	}

	obj, ok := loose.(map[string]any)
	if !ok {
		return Snapshot{}, errors.New("snapshot is not a JSON object")
	}

	version, ok := obj["schema_version"].(string)
	if !ok {
		return Snapshot{}, errors.New("snapshot has no schema_version")
	}
	if version != SchemaVersion {
		return Snapshot{}, fmt.Errorf("snapshot has schema_version %q; this version reads %q", version, SchemaVersion)
	}

	if err := checkKeys(reflect.TypeOf(Snapshot{}), obj, ""); err != nil {
		return Snapshot{}, err
	}

	snap, err := decodeFromMap(obj)
	if err != nil {
		return Snapshot{}, err
	}

	if err := snap.Check(); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// decodeFromMap fills a Snapshot from obj, the same map checkKeys has
// already walked against the contract's types. obj holds only keys that
// match the contract letter for letter, of the kind the contract expects,
// so encoding it again and decoding it strictly is the simplest way to
// turn it into a Snapshot: no value of the result comes from the bytes of
// the original file by any other path.
func decodeFromMap(obj map[string]any) (Snapshot, error) {
	raw, err := json.Marshal(obj)
	if err != nil {
		return Snapshot{}, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var snap Snapshot
	if err := dec.Decode(&snap); err != nil {
		return Snapshot{}, err
	}
	return snap, nil
}

// unsupportedKeyFmt is the message checkKeys and supportedKey give for a
// field of a shape the walk does not know how to check: a fault of the
// program, not of a file, so it names the type and the field rather than a
// path of a document.
const unsupportedKeyFmt = "snapshot type %s has a field the key walk does not support: %s"

// supportedKey returns the plain name of field's json tag, and whether the
// walk knows how to check a field shaped like it: exported, not embedded,
// tagged with a bare name (no "-", no option such as omitempty), and of a
// kind whose JSON value the walk can look at on its own — a string, a
// bool, a float, a struct, or a slice of strings or of structs. Anything
// else (a pointer, a map, an interface, an embedded struct, a field with
// no json tag, a "-" tag, or a tag carrying an option) would let the walk
// say yes without ever having looked, so the caller refuses it instead.
func supportedKey(field reflect.StructField) (string, bool) {
	if field.Anonymous {
		return "", false
	}
	tag, hasTag := field.Tag.Lookup("json")
	if !hasTag || tag == "-" || strings.Contains(tag, ",") {
		return "", false
	}

	switch field.Type.Kind() {
	case reflect.String, reflect.Bool, reflect.Float32, reflect.Float64, reflect.Struct:
		return tag, true
	case reflect.Slice:
		switch field.Type.Elem().Kind() {
		case reflect.String, reflect.Struct:
			return tag, true
		}
	}
	return "", false
}

// kindMatches reports whether value, one leaf of the generic decoding of
// the document, is of the JSON kind that field's Go type expects: a
// string, a bool, a number for a float, an object for a struct, or an
// array for a slice. It is the one place that decides "the wrong kind", so
// that a text where a number belongs, an object for a list, and so on are
// all refused by the same rule.
func kindMatches(kind reflect.Kind, value any) bool {
	switch kind {
	case reflect.String:
		_, ok := value.(string)
		return ok
	case reflect.Bool:
		_, ok := value.(bool)
		return ok
	case reflect.Float32, reflect.Float64:
		_, ok := value.(float64)
		return ok
	case reflect.Struct:
		_, ok := value.(map[string]any)
		return ok
	case reflect.Slice:
		_, ok := value.([]any)
		return ok
	}
	return false
}

// sortedKeys returns the keys of obj, sorted: obj comes from encoding/json,
// which gives no order of its own, and the unknown-key scan below must
// visit them in a fixed order for Decode to report the same fault on every
// run of the same bytes.
func sortedKeys(obj map[string]any) []string {
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// joinPath appends name to prefix, dot-separated, or returns name alone
// when prefix is empty.
func joinPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

// checkKeys walks t's fields against obj, the generic decoding of the same
// document at this level, and reports the first problem, in this fixed
// order: a key obj holds that no field of t declares, compared letter for
// letter (obj's keys are visited in sorted order, so the same file always
// names the same key first); then, for each field of t in its declared
// order, a key t declares that obj does not hold, a key present with the
// JSON value null (the writer never writes one), or a value of the kind
// kindMatches refuses. A struct field is followed into its own object; a
// slice field whose element is itself a struct (only
// quality.capabilities) is followed into each element obj actually holds,
// named by its index, and an element that is itself null, or not itself
// an object, is refused without being read as one. schema_version is
// skipped: by the time this walk runs, Decode has already read it and
// confirmed it names the version this program reads.
func checkKeys(t reflect.Type, obj map[string]any, prefix string) error {
	type field struct {
		decl reflect.StructField
		name string
	}

	known := make(map[string]bool, t.NumField())
	var fields []field
	for i := 0; i < t.NumField(); i++ {
		decl := t.Field(i)
		name, ok := supportedKey(decl)
		if !ok {
			return fmt.Errorf(unsupportedKeyFmt, t.Name(), decl.Name)
		}
		known[name] = true
		if name == "schema_version" {
			continue
		}
		fields = append(fields, field{decl, name})
	}

	for _, key := range sortedKeys(obj) {
		if !known[key] {
			return fmt.Errorf("snapshot holds an unknown key: %s", joinPath(prefix, key))
		}
	}

	for _, f := range fields {
		path := joinPath(prefix, f.name)

		value, present := obj[f.name]
		if !present {
			return fmt.Errorf("snapshot lacks the key %s", path)
		}
		if value == nil {
			return fmt.Errorf("snapshot holds null at %s", path)
		}
		if !kindMatches(f.decl.Type.Kind(), value) {
			return fmt.Errorf("snapshot holds a value of the wrong kind at %s", path)
		}

		switch f.decl.Type.Kind() {
		case reflect.Struct:
			if err := checkKeys(f.decl.Type, value.(map[string]any), path); err != nil {
				return err
			}
		case reflect.Slice:
			elemType := f.decl.Type.Elem()
			for idx, elem := range value.([]any) {
				elemPath := fmt.Sprintf("%s[%d]", path, idx)
				if elem == nil {
					return fmt.Errorf("snapshot holds null at %s", elemPath)
				}
				if elemType.Kind() != reflect.Struct {
					if !kindMatches(elemType.Kind(), elem) {
						return fmt.Errorf("snapshot holds a value of the wrong kind at %s", elemPath)
					}
					continue
				}
				elemObj, ok := elem.(map[string]any)
				if !ok {
					return fmt.Errorf("snapshot holds a value of the wrong kind at %s", elemPath)
				}
				if err := checkKeys(elemType, elemObj, elemPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
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
	if periodTooLong(s.Report.Period) {
		return fmt.Errorf("report.period is too long: %q", s.Report.Period)
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
// the registry, and then, once every value is known to be in its
// registry, when the capability's state, strategy, evidence, and reasons
// disagree with each other: a state, a strategy, and a set of reasons are
// each individually valid on their own and can still describe a
// capability that could never come out of Detect.
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

	switch c.State {
	case discovery.StateAvailable:
		if len(c.Reasons) > 0 {
			return fmt.Errorf("quality.capabilities[%d] is available and carries a reason", i)
		}
	case discovery.StatePartial, discovery.StateUnavailable:
		if len(c.Reasons) == 0 {
			return fmt.Errorf("quality.capabilities[%d] is %s and carries no reason", i, c.State)
		}
	}
	if c.State == discovery.StateAvailable && c.Strategy == discovery.StrategyNone {
		return fmt.Errorf("quality.capabilities[%d] is available and names no source", i)
	}
	if c.State == discovery.StateUnavailable && c.Strategy != discovery.StrategyNone {
		return fmt.Errorf("quality.capabilities[%d] is unavailable and names a source", i)
	}
	if c.Strategy == discovery.StrategyConfigured && c.Name != "node_autoscaling" {
		return fmt.Errorf("quality.capabilities[%d] cannot be configured", i)
	}
	seenReasons := make(map[string]bool, len(c.Reasons))
	for _, reason := range c.Reasons {
		if seenReasons[reason] {
			return fmt.Errorf("quality.capabilities[%d].reasons holds a repeated value", i)
		}
		seenReasons[reason] = true
	}
	return nil
}
