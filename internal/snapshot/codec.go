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

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
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

	// validStatisticsStates lists the three states of a Statistics, spec
	// 8.1. Like validConfidences, this is not a literal: collect keeps no
	// list of its own, only the three constants StateValue, StateUnavailable,
	// and StateNoBase.
	validStatisticsStates = []string{collect.StateValue, collect.StateUnavailable, collect.StateNoBase}
)

// excludedWorkloadKind is the one workload kind the inventory leaves out,
// spec 8: a Job is one finite execution and never reaches the snapshot.
const excludedWorkloadKind = "Job"

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
// bool, a float, a struct, a slice of strings or of structs, a pointer to
// a float, a map of strings to floats, or a Value (a struct, which checkKeys
// treats as one leaf). Anything else (any other
// pointer, any other map, an interface, an embedded struct, a field with
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
	case reflect.Pointer:
		if field.Type.Elem().Kind() == reflect.Float64 {
			return tag, true
		}
	case reflect.Map:
		if field.Type.Key().Kind() == reflect.String && field.Type.Elem().Kind() == reflect.Float64 {
			return tag, true
		}
	}
	return "", false
}

// kindMatches reports whether value, one leaf of the generic decoding of
// the document, is of the JSON kind that field's Go type expects: a
// string, a bool, a number for a float or for the value a pointer points
// to, an object for a struct or for a map, or an array for a slice. It is
// the one place that decides "the wrong kind", so that a text where a
// number belongs, an object for a list, and so on are all refused by the
// same rule.
func kindMatches(kind reflect.Kind, value any) bool {
	switch kind {
	case reflect.String:
		_, ok := value.(string)
		return ok
	case reflect.Bool:
		_, ok := value.(bool)
		return ok
	case reflect.Float32, reflect.Float64, reflect.Pointer:
		_, ok := value.(float64)
		return ok
	case reflect.Struct, reflect.Map:
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
// JSON value null (the writer never writes one, except at a field of type
// *float64, where null is the value of an unmeasured statistic), or a
// value of the kind kindMatches refuses. A struct field is followed into
// its own object; a slice field whose element is itself a struct is
// followed into each element obj actually holds, named by its index, and
// an element that is itself null, or not itself an object, is refused
// without being read as one; a map field is walked entry by entry, in
// sorted order, refusing a null value or one of the wrong kind, named by
// its key. schema_version is skipped: by the time this walk runs, Decode
// has already read it and confirmed it names the version this program
// reads.
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
			if f.decl.Type.Kind() == reflect.Pointer {
				continue
			}
			return fmt.Errorf("snapshot holds null at %s", path)
		}
		if f.decl.Type == valueType {
			switch value.(type) {
			case float64, bool, string:
				continue
			}
			return fmt.Errorf("snapshot holds a value of the wrong kind at %s", path)
		}
		if !kindMatches(f.decl.Type.Kind(), value) {
			return fmt.Errorf("snapshot holds a value of the wrong kind at %s", path)
		}

		switch f.decl.Type.Kind() {
		case reflect.Struct:
			if err := checkKeys(f.decl.Type, value.(map[string]any), path); err != nil {
				return err
			}
		case reflect.Map:
			elemKind := f.decl.Type.Elem().Kind()
			mapObj := value.(map[string]any)
			for _, key := range sortedKeys(mapObj) {
				elemPath := joinPath(path, key)
				elem := mapObj[key]
				if elem == nil {
					return fmt.Errorf("snapshot holds null at %s", elemPath)
				}
				if !kindMatches(elemKind, elem) {
					return fmt.Errorf("snapshot holds a value of the wrong kind at %s", elemPath)
				}
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

	if err := checkMessageCodes(s.Quality.Warnings, s.Quality.Notices); err != nil {
		return err
	}
	if err := checkMessageParams(s.Quality.Warnings, s.Quality.Notices); err != nil {
		return err
	}
	if err := checkRowStatistics(s.Namespaces, s.Workloads); err != nil {
		return err
	}
	if err := checkPods(s.Namespaces, s.Workloads); err != nil {
		return err
	}
	if err := checkNamespaceRows(s.Namespaces, s.Scope.Namespaces); err != nil {
		return err
	}
	if err := checkWorkloadRows(s.Workloads, s.Scope.Namespaces); err != nil {
		return err
	}
	if err := checkFindingRegistry(s.Findings); err != nil {
		return err
	}
	if err := checkFindingScope(s.Findings, s.Namespaces, s.Workloads); err != nil {
		return err
	}
	if err := checkFindingsOrder(s.Findings); err != nil {
		return err
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

// isFinite reports whether v is neither NaN nor infinite.
func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// paramsEqual reports whether a and b hold the same keys, each with the
// same value.
func paramsEqual(a, b map[string]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if other, ok := b[key]; !ok || other != value {
			return false
		}
	}
	return true
}

// checkMessageCodes refuses a warning or a notice whose code is not one of
// collect.WarningCodes or collect.NoticeCodes, the one registry of what a
// collector can raise, and a message repeated in its own list, by code and
// parameters.
func checkMessageCodes(warnings, notices []Message) error {
	for i, m := range warnings {
		if !slices.Contains(collect.WarningCodes, m.Code) {
			return fmt.Errorf("quality.warnings[%d].code has an unknown value: %q", i, m.Code)
		}
	}
	for i, m := range notices {
		if !slices.Contains(collect.NoticeCodes, m.Code) {
			return fmt.Errorf("quality.notices[%d].code has an unknown value: %q", i, m.Code)
		}
	}
	if err := checkMessagesUnique("quality.warnings", warnings); err != nil {
		return err
	}
	return checkMessagesUnique("quality.notices", notices)
}

// checkMessagesUnique refuses two messages of messages that hold the same
// code and the same parameters.
func checkMessagesUnique(path string, messages []Message) error {
	for i := 0; i < len(messages); i++ {
		for j := i + 1; j < len(messages); j++ {
			if messages[i].Code == messages[j].Code && paramsEqual(messages[i].Params, messages[j].Params) {
				return fmt.Errorf("%s holds a repeated message: %q", path, messages[i].Code)
			}
		}
	}
	return nil
}

// checkMessageParams refuses a warning that carries a parameter, a notice
// whose parameters are not exactly the names of collect.NoticeParams for
// its code, and a count that is not a whole number above zero.
func checkMessageParams(warnings, notices []Message) error {
	for i, m := range warnings {
		if len(m.Params) != 0 {
			return fmt.Errorf("quality.warnings[%d].params must be empty", i)
		}
	}
	for i, m := range notices {
		want := collect.NoticeParams[m.Code]
		if !paramNamesMatch(m.Params, want) {
			return fmt.Errorf("quality.notices[%d].params must hold exactly: %s", i, strings.Join(want, ", "))
		}
		for _, name := range want {
			if name != "count" {
				continue
			}
			value := m.Params[name]
			if value != math.Trunc(value) || value <= 0 {
				return fmt.Errorf("quality.notices[%d].params.count must be a whole number above zero", i)
			}
		}
	}
	return nil
}

// paramNamesMatch reports whether params holds exactly the names of want,
// no more and no fewer.
func paramNamesMatch(params map[string]float64, want []string) bool {
	if len(params) != len(want) {
		return false
	}
	for _, name := range want {
		if _, ok := params[name]; !ok {
			return false
		}
	}
	return true
}

// checkRowStatistics checks the ten statistics of every namespace and every
// workload.
func checkRowStatistics(namespaces []Namespace, workloads []Workload) error {
	for i, ns := range namespaces {
		for _, field := range namespaceStatisticsFields(ns) {
			path := fmt.Sprintf("namespaces[%d].%s", i, field.name)
			if err := checkStatistics(path, field.stat); err != nil {
				return err
			}
		}
	}
	for i, wl := range workloads {
		for _, field := range workloadStatisticsFields(wl) {
			path := fmt.Sprintf("workloads[%d].%s", i, field.name)
			if err := checkStatistics(path, field.stat); err != nil {
				return err
			}
		}
	}
	return nil
}

// statisticsField names one of the ten statistics of a row by its JSON key.
type statisticsField struct {
	name string
	stat Statistics
}

// namespaceStatisticsFields lists the ten statistics of ns, in the order
// Resources declares them.
func namespaceStatisticsFields(ns Namespace) []statisticsField {
	return []statisticsField{
		{"cpu_usage", ns.CPUUsage},
		{"memory_usage", ns.MemoryUsage},
		{"cpu_requests", ns.CPURequests},
		{"memory_requests", ns.MemoryRequests},
		{"cpu_limits", ns.CPULimits},
		{"memory_limits", ns.MemoryLimits},
		{"cpu_request_percent", ns.CPURequestPercent},
		{"memory_request_percent", ns.MemoryRequestPercent},
		{"cpu_limit_percent", ns.CPULimitPercent},
		{"memory_limit_percent", ns.MemoryLimitPercent},
	}
}

// workloadStatisticsFields lists the ten statistics of wl, in the order
// Resources declares them.
func workloadStatisticsFields(wl Workload) []statisticsField {
	return []statisticsField{
		{"cpu_usage", wl.CPUUsage},
		{"memory_usage", wl.MemoryUsage},
		{"cpu_requests", wl.CPURequests},
		{"memory_requests", wl.MemoryRequests},
		{"cpu_limits", wl.CPULimits},
		{"memory_limits", wl.MemoryLimits},
		{"cpu_request_percent", wl.CPURequestPercent},
		{"memory_request_percent", wl.MemoryRequestPercent},
		{"cpu_limit_percent", wl.CPULimitPercent},
		{"memory_limit_percent", wl.MemoryLimitPercent},
	}
}

// checkStatistics refuses a state outside validStatisticsStates, a state
// value with no number, any other state with a number, and a number that is
// NaN or infinite.
func checkStatistics(path string, s Statistics) error {
	if !slices.Contains(validStatisticsStates, s.State) {
		return fmt.Errorf("%s.state has an unknown value: %q", path, s.State)
	}

	numbers := []struct {
		name  string
		value *float64
	}{
		{"average", s.Average},
		{"minimum", s.Minimum},
		{"peak", s.Peak},
		{"current", s.Current},
	}

	if s.State == collect.StateValue {
		if s.Average == nil && s.Minimum == nil && s.Peak == nil && s.Current == nil {
			return fmt.Errorf("%s has state %q and no number", path, collect.StateValue)
		}
	} else {
		for _, n := range numbers {
			if n.value != nil {
				return fmt.Errorf("%s.%s is set for a state that is not %q", path, n.name, collect.StateValue)
			}
		}
	}

	for _, n := range numbers {
		if n.value != nil && !isFinite(*n.value) {
			return fmt.Errorf("%s.%s is not a finite number", path, n.name)
		}
	}
	return nil
}

// checkPods refuses a pod count, present and not a whole number zero or
// above, of any namespace or workload.
func checkPods(namespaces []Namespace, workloads []Workload) error {
	for i, ns := range namespaces {
		if err := checkPodCount(fmt.Sprintf("namespaces[%d].pods", i), ns.Pods); err != nil {
			return err
		}
	}
	for i, wl := range workloads {
		if err := checkPodCount(fmt.Sprintf("workloads[%d].pods", i), wl.Pods); err != nil {
			return err
		}
	}
	return nil
}

// checkPodCount refuses pods, present and not a whole number zero or above.
func checkPodCount(path string, pods *float64) error {
	if pods == nil {
		return nil
	}
	if *pods != math.Trunc(*pods) || *pods < 0 {
		return fmt.Errorf("%s must be a whole number, zero or above", path)
	}
	return nil
}

// checkNamespaceRows refuses a namespaces list that is not sorted by name,
// holds a repeated name, holds a name with a slash, or does not name
// exactly the namespaces of the scope. A slash is refused because no name
// of a real Kubernetes namespace holds one: it is a fault of the endpoint
// or of the configuration.
func checkNamespaceRows(namespaces []Namespace, scopeNamespaces []string) error {
	names := make([]string, len(namespaces))
	for i, ns := range namespaces {
		if strings.Contains(ns.Name, "/") {
			return fmt.Errorf("namespaces[%d].name holds a slash: %q", i, ns.Name)
		}
		names[i] = ns.Name
	}
	if err := checkSorted("namespaces", names); err != nil {
		return err
	}
	if !slices.Equal(names, scopeNamespaces) {
		return errors.New("namespaces does not name exactly the namespaces of scope.namespaces")
	}
	return nil
}

// checkWorkloadRows refuses a workloads list that is not sorted by
// namespace, name, and kind, holds a repeated row, or holds a row with an
// empty name, an empty kind, a namespace outside the scope, the kind Job,
// or a slash in its namespace, name, or kind. A slash is refused because no
// name of a real Kubernetes object holds one: it is a fault of the
// endpoint or of the configuration.
func checkWorkloadRows(workloads []Workload, scopeNamespaces []string) error {
	for i := 1; i < len(workloads); i++ {
		if !workloadLess(workloads[i-1], workloads[i]) {
			return errors.New("workloads is not sorted or holds a repeated value")
		}
	}
	for i, wl := range workloads {
		if strings.Contains(wl.Namespace, "/") {
			return fmt.Errorf("workloads[%d].namespace holds a slash: %q", i, wl.Namespace)
		}
		if wl.Name == "" {
			return fmt.Errorf("workloads[%d].name is empty", i)
		}
		if strings.Contains(wl.Name, "/") {
			return fmt.Errorf("workloads[%d].name holds a slash: %q", i, wl.Name)
		}
		if wl.Kind == "" {
			return fmt.Errorf("workloads[%d].kind is empty", i)
		}
		if strings.Contains(wl.Kind, "/") {
			return fmt.Errorf("workloads[%d].kind holds a slash: %q", i, wl.Kind)
		}
		if !slices.Contains(scopeNamespaces, wl.Namespace) {
			return fmt.Errorf("workloads[%d].namespace is not a namespace of the scope", i)
		}
		if wl.Kind == excludedWorkloadKind {
			return fmt.Errorf("workloads[%d].kind must not be %s", i, excludedWorkloadKind)
		}
	}
	return nil
}

// workloadLess orders a strictly before b, by namespace, then name, then
// kind, following the key the workload collector sorts by.
func workloadLess(a, b Workload) bool {
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	if a.Name != b.Name {
		return a.Name < b.Name
	}
	return a.Kind < b.Kind
}

// checkFindingRegistry refuses a finding whose code is not in
// analysis.Registry, or whose severity, scope type, evidence names, or
// confidence disagree with its definition, or whose evidence holds a
// number that is NaN or infinite.
func checkFindingRegistry(findings []Finding) error {
	for i, f := range findings {
		def, ok := analysis.Lookup(f.Code)
		if !ok {
			return fmt.Errorf("findings[%d].code has an unknown value: %q", i, f.Code)
		}
		if !slices.Contains(def.Severities, f.Severity) {
			return fmt.Errorf("findings[%d].severity does not match the definition of %s", i, f.Code)
		}
		if f.ScopeType != def.ScopeType {
			return fmt.Errorf("findings[%d].scope_type does not match the definition of %s", i, f.Code)
		}
		if !evidenceNamesMatch(f.Evidence, def.Evidence) {
			return fmt.Errorf("findings[%d].evidence does not match the definition of %s", i, f.Code)
		}
		for j, e := range f.Evidence {
			if want := def.Evidence[j].Kind; e.Value.Kind != string(want) {
				return fmt.Errorf("findings[%d].evidence[%d] of %s: %s holds a value of kind %s, want %s", i, j, f.Code, e.Name, e.Value.Kind, want)
			}
		}
		if !slices.Contains(validConfidences, f.Confidence) {
			return fmt.Errorf("findings[%d].confidence has an unknown value: %q", i, f.Confidence)
		}
		for j, e := range f.Evidence {
			if err := checkValue(e.Value); err != nil {
				return fmt.Errorf("findings[%d].evidence[%d].value %w", i, j, err)
			}
		}
	}
	return nil
}

// evidenceNamesMatch reports whether evidence carries exactly the names of
// specs, in order.
func evidenceNamesMatch(evidence []Evidence, specs []analysis.EvidenceSpec) bool {
	if len(evidence) != len(specs) {
		return false
	}
	for i, spec := range specs {
		if evidence[i].Name != spec.Name {
			return false
		}
	}
	return true
}

// workloadIdentity is the three fields that identify a workload: the same
// fields checkWorkloadRows already refuses a slash in, so a namespace and a
// name can be told apart from every other by exact equality alone, with no
// text joined from them standing in as a map key.
type workloadIdentity struct {
	namespace, name, kind string
}

// checkFindingScope refuses a namespace finding with a non-empty scope kind
// or a scope name that is not a namespace of the snapshot, and a workload
// finding whose scope name and scope kind do not name a workload of the
// snapshot.
func checkFindingScope(findings []Finding, namespaces []Namespace, workloads []Workload) error {
	namespaceNames := make(map[string]bool, len(namespaces))
	for _, ns := range namespaces {
		namespaceNames[ns.Name] = true
	}
	workloadKeys := make(map[workloadIdentity]bool, len(workloads))
	for _, wl := range workloads {
		workloadKeys[workloadIdentity{wl.Namespace, wl.Name, wl.Kind}] = true
	}

	for i, f := range findings {
		switch f.ScopeType {
		case analysis.ScopeNamespace:
			if f.ScopeKind != "" {
				return fmt.Errorf("findings[%d].scope_kind must be empty for a namespace finding", i)
			}
			if !namespaceNames[f.ScopeName] {
				return fmt.Errorf("findings[%d].scope_name is not a namespace of the snapshot", i)
			}
		case analysis.ScopeWorkload:
			// A workload finding's scope name is namespace/name. checkWorkloadRows
			// has already refused a slash in either field by the time this rule
			// runs, so the first slash of scope_name is always the join point,
			// and cutting there recovers the two fields exactly, with no other
			// slash left to be ambiguous about.
			namespace, name, cut := strings.Cut(f.ScopeName, "/")
			if !cut || !workloadKeys[workloadIdentity{namespace, name, f.ScopeKind}] {
				return fmt.Errorf("findings[%d].scope_name is not a workload of the snapshot", i)
			}
		}
	}
	return nil
}

// checkFindingsOrder refuses two findings that share a code, scope type,
// scope kind, and scope name, and a findings list that is not in the order
// analysis.Sort gives a copy of it.
func checkFindingsOrder(findings []Finding) error {
	type identity struct {
		code, scopeType, scopeKind, scopeName string
	}
	seen := make(map[identity]bool, len(findings))
	for _, f := range findings {
		id := identity{f.Code, f.ScopeType, f.ScopeKind, f.ScopeName}
		if seen[id] {
			return errors.New("findings holds a repeated finding")
		}
		seen[id] = true
	}

	original := make([]analysis.Finding, len(findings))
	for i, f := range findings {
		original[i] = analysis.Finding{
			Code:      f.Code,
			Severity:  f.Severity,
			ScopeType: f.ScopeType,
			ScopeKind: f.ScopeKind,
			ScopeName: f.ScopeName,
		}
	}
	sorted := append([]analysis.Finding{}, original...)
	analysis.Sort(sorted)

	for i := range sorted {
		if sorted[i].Code != original[i].Code ||
			sorted[i].Severity != original[i].Severity ||
			sorted[i].ScopeType != original[i].ScopeType ||
			sorted[i].ScopeKind != original[i].ScopeKind ||
			sorted[i].ScopeName != original[i].ScopeName {
			return errors.New("findings is not in the order analysis.Sort gives")
		}
	}
	return nil
}
