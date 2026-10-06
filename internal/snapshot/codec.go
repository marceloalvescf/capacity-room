package snapshot

import (
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
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
// The document is decoded loosely exactly once, into a plain map, by
// encoding/json/v2, which refuses a repeated key at any depth and text that
// is not UTF-8: that map
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
	if err := checkVersionFirst(data); err != nil {
		return Snapshot{}, err
	}
	dec := jsontext.NewDecoder(bytes.NewReader(data))
	var loose any
	if err := jsonv2.UnmarshalDecode(dec, &loose); err != nil {
		if path, ok := repeatedKeyPath(err); ok {
			return Snapshot{}, fmt.Errorf("snapshot repeats the key %s", path)
		}
		return Snapshot{}, fmt.Errorf("snapshot is not valid JSON: %s", err)
	}
	if _, err := dec.ReadToken(); !errors.Is(err, io.EOF) {
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

// checkVersionFirst names a schema_version other than this one before the
// strict reading can refuse the file for a repeated key or for text that is
// not UTF-8, which a file of another version may hold. It reads the file
// leniently and judges nothing but a top-level schema_version that is a
// text; anything else is left to the strict reading, which keeps its order.
func checkVersionFirst(data []byte) error {
	dec := jsontext.NewDecoder(bytes.NewReader(data), jsontext.AllowDuplicateNames(true), jsontext.AllowInvalidUTF8(true))
	var loose any
	if err := jsonv2.UnmarshalDecode(dec, &loose); err != nil {
		return nil
	}
	if _, err := dec.ReadToken(); !errors.Is(err, io.EOF) {
		return nil
	}
	obj, ok := loose.(map[string]any)
	if !ok {
		return nil
	}
	if version, ok := obj["schema_version"].(string); ok && version != SchemaVersion {
		return fmt.Errorf("snapshot has schema_version %q; this version reads %q", version, SchemaVersion)
	}
	return nil
}

// repeatedKeyPath reports whether err is the refusal of a repeated key, and
// where the key is, written as the messages of Check write a path:
// findings[0].code. The JSON pointer of the error names the repeated key.
func repeatedKeyPath(err error) (string, bool) {
	var syntactic *jsontext.SyntacticError
	if !errors.As(err, &syntactic) || !errors.Is(err, jsontext.ErrDuplicateName) {
		return "", false
	}
	path := ""
	for token := range syntactic.JSONPointer.Tokens() {
		if _, err := strconv.ParseUint(token, 10, 64); err == nil && path != "" {
			path += "[" + token + "]"
			continue
		}
		path = joinPath(path, token)
	}
	return path, true
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
// a float or to a bool, a map of strings to floats, or a Value (a struct, which checkKeys
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
		if elem := field.Type.Elem().Kind(); elem == reflect.Float64 || elem == reflect.Bool {
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
// the document, is of the JSON kind that typ expects: a string, a bool, a
// number for a float, a number or a bool for the value a pointer points
// to, an object for a struct or for a map, or an array for a slice. It is
// the one place that decides "the wrong kind", so that a text where a
// number belongs, an object for a list, and so on are all refused by the
// same rule.
func kindMatches(typ reflect.Type, value any) bool {
	switch typ.Kind() {
	case reflect.String:
		_, ok := value.(string)
		return ok
	case reflect.Bool:
		_, ok := value.(bool)
		return ok
	case reflect.Float32, reflect.Float64:
		_, ok := value.(float64)
		return ok
	case reflect.Pointer:
		if typ.Elem().Kind() == reflect.Bool {
			_, ok := value.(bool)
			return ok
		}
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
// JSON value null (the writer never writes one, except at a pointer field,
// where null is the value of a number or a flag that is not known), or a
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
		if !kindMatches(f.decl.Type, value) {
			return fmt.Errorf("snapshot holds a value of the wrong kind at %s", path)
		}

		switch f.decl.Type.Kind() {
		case reflect.Struct:
			if err := checkKeys(f.decl.Type, value.(map[string]any), path); err != nil {
				return err
			}
		case reflect.Map:
			elemType := f.decl.Type.Elem()
			mapObj := value.(map[string]any)
			for _, key := range sortedKeys(mapObj) {
				elemPath := joinPath(path, key)
				elem := mapObj[key]
				if elem == nil {
					return fmt.Errorf("snapshot holds null at %s", elemPath)
				}
				if !kindMatches(elemType, elem) {
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
					if !kindMatches(elemType, elem) {
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
	if !isFinite(s.Report.Settings.TimeoutSeconds) {
		return errors.New("report.settings.timeout_seconds must be a finite number")
	}
	if s.Report.Settings.TimeoutSeconds <= 0 {
		return errors.New("report.settings.timeout_seconds must be positive")
	}

	th := s.Report.Thresholds
	for _, t := range []struct {
		name  string
		value float64
	}{
		{"warning_percent", th.WarningPercent},
		{"critical_percent", th.CriticalPercent},
		{"low_utilization_percent", th.LowUtilizationPercent},
		{"autoscaler_near_max_percent", th.AutoscalerNearMaxPercent},
		{"nodepool_spread_points", th.NodepoolSpreadPoints},
		{"bursty_gap_percent", th.BurstyGapPercent},
		{"hpa_replica_divergence", th.HPAReplicaDivergence},
	} {
		if !isFinite(t.value) {
			return fmt.Errorf("report.thresholds.%s must be a finite number", t.name)
		}
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
	if err := checkSummary(s); err != nil {
		return err
	}
	if err := checkAutoscaling(s.Summary.Autoscaling); err != nil {
		return err
	}
	if err := checkHPARows(s.HPAs, s.Scope.Namespaces); err != nil {
		return err
	}
	if err := checkNodeRows(s.Nodes); err != nil {
		return err
	}
	if err := checkNodepoolRows(s.Nodes, s.Nodepools); err != nil {
		return err
	}
	if err := checkWorkloadCoverage(s.Workloads, s.HPAs); err != nil {
		return err
	}
	if err := checkFindingRegistry(s.Findings); err != nil {
		return err
	}
	if err := checkFindingScope(s); err != nil {
		return err
	}
	if err := checkFindingAutoscaling(s); err != nil {
		return err
	}
	if err := checkFindingConfidence(s); err != nil {
		return err
	}
	if err := checkFindingsOrder(s.Findings); err != nil {
		return err
	}
	if err := checkRecommendations(s); err != nil {
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

// checkMessageCodes refuses a warning whose code is not one of
// collect.WarningCodes or discovery.ReasonCodes, a notice whose code is not
// one of collect.NoticeCodes, the one registry of what a
// collector can raise, and a message repeated in its own list, by code and
// parameters.
func checkMessageCodes(warnings, notices []Message) error {
	for i, m := range warnings {
		if !slices.Contains(collect.WarningCodes, m.Code) && !slices.Contains(discovery.ReasonCodes, m.Code) {
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
// value that lacks any of its four numbers, any other state with a number,
// and a number that is NaN or infinite.
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
		for _, n := range numbers {
			if n.value == nil {
				return fmt.Errorf("%s has the state %s and lacks a number", path, collect.StateValue)
			}
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
		if !evidenceMatches(f.Evidence, def.Evidence) {
			return fmt.Errorf("findings[%d].evidence does not match the definition of %s", i, f.Code)
		}
		for j, e := range f.Evidence {
			spec := def.Evidence[slices.IndexFunc(def.Evidence, func(spec analysis.EvidenceSpec) bool { return spec.Name == e.Name })]
			if e.Value.Kind != string(spec.Kind) {
				return fmt.Errorf("findings[%d].evidence[%d] of %s: %s holds a value of kind %s, want %s", i, j, f.Code, e.Name, e.Value.Kind, spec.Kind)
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

// evidenceMatches reports whether evidence carries the names of specs, in
// order, leaving out only the specs that are optional.
func evidenceMatches(evidence []Evidence, specs []analysis.EvidenceSpec) bool {
	if len(specs) > 0 && len(evidence) == 0 {
		return false
	}
	next := 0
	for _, e := range evidence {
		for next < len(specs) && specs[next].Name != e.Name {
			if !specs[next].Optional {
				return false
			}
			next++
		}
		if next == len(specs) {
			return false
		}
		next++
	}
	for ; next < len(specs); next++ {
		if !specs[next].Optional {
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

// clusterScopeName is the one name of a finding whose scope is the cluster.
const clusterScopeName = "cluster"

// coverageCodes are the two codes of a workload that no HPA targets.
var coverageCodes = []string{"DEPLOYMENT_WITHOUT_HPA", "STATEFULSET_WITHOUT_HPA_INFO"}

// checkFindingScope refuses a finding whose scope does not name a row of
// the snapshot: a namespace finding names a namespace; a workload finding
// names a workload by its scope name and its scope kind, and one of the two
// coverage codes names an uncovered one; an hpa finding names an HPA by
// namespace/name; a node finding names a node; a nodepool finding names a
// pool; a cluster finding is named cluster. Only a workload finding carries
// a scope kind.
func checkFindingScope(s Snapshot) error {
	namespaceNames := make(map[string]bool, len(s.Namespaces))
	for _, ns := range s.Namespaces {
		namespaceNames[ns.Name] = true
	}
	workloadKeys := make(map[workloadIdentity]bool, len(s.Workloads))
	uncoveredKeys := make(map[workloadIdentity]bool)
	for _, wl := range s.Workloads {
		key := workloadIdentity{wl.Namespace, wl.Name, wl.Kind}
		workloadKeys[key] = true
		if wl.HPACoverage == CoverageUncovered {
			uncoveredKeys[key] = true
		}
	}
	hpaNames := make(map[string]bool, len(s.HPAs))
	for _, hpa := range s.HPAs {
		hpaNames[hpa.Namespace+"/"+hpa.Name] = true
	}
	nodeNames := make(map[string]bool, len(s.Nodes))
	for _, node := range s.Nodes {
		nodeNames[node.Name] = true
	}
	poolNames := make(map[string]bool, len(s.Nodepools))
	for _, pool := range s.Nodepools {
		poolNames[pool.Name] = true
	}

	for i, f := range s.Findings {
		if f.ScopeType != analysis.ScopeWorkload && f.ScopeKind != "" {
			return fmt.Errorf("findings[%d].scope_kind must be empty for a %s finding", i, f.ScopeType)
		}
		switch f.ScopeType {
		case analysis.ScopeNamespace:
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
			key := workloadIdentity{namespace, name, f.ScopeKind}
			if slices.Contains(coverageCodes, f.Code) {
				if !cut || !uncoveredKeys[key] {
					return fmt.Errorf("findings[%d].scope_name is not an uncovered workload of the snapshot", i)
				}
				continue
			}
			if !cut || !workloadKeys[key] {
				return fmt.Errorf("findings[%d].scope_name is not a workload of the snapshot", i)
			}
		case analysis.ScopeHPA:
			if !hpaNames[f.ScopeName] {
				return fmt.Errorf("findings[%d].scope_name is not an HPA of the snapshot", i)
			}
		case analysis.ScopeNode:
			if !nodeNames[f.ScopeName] {
				return fmt.Errorf("findings[%d].scope_name is not a node of the snapshot", i)
			}
		case analysis.ScopeNodepool:
			if !poolNames[f.ScopeName] {
				return fmt.Errorf("findings[%d].scope_name is not a nodepool of the snapshot", i)
			}
		case analysis.ScopeCluster:
			if f.ScopeName != clusterScopeName {
				return fmt.Errorf("findings[%d].scope_name must be %s", i, clusterScopeName)
			}
		}
	}
	return nil
}

// autoscalingCodes lists, for each autoscaling status, the autoscaling
// findings that analysis gives under it.
var autoscalingCodes = map[string][]string{
	collect.AutoscalingUnknown:  {"NODE_AUTOSCALING_UNKNOWN"},
	collect.AutoscalingDisabled: {"NODE_AUTOSCALING_DISABLED", "NODE_AUTOSCALING_DISABLED_PRESSURE"},
	collect.AutoscalingEnabled:  {"NODE_AUTOSCALING_NEAR_MAX", "NODE_AUTOSCALING_BOUNDS_UNKNOWN", "NODE_AUTOSCALING_AT_MIN", "NODE_AUTOSCALING_ENABLED"},
}

// checkFindingAutoscaling refuses an autoscaling finding that the status of
// summary.autoscaling never gives.
func checkFindingAutoscaling(s Snapshot) error {
	status := s.Summary.Autoscaling.Status
	for i, f := range s.Findings {
		if strings.HasPrefix(f.Code, "NODE_AUTOSCALING_") && !slices.Contains(autoscalingCodes[status], f.Code) {
			return fmt.Errorf("findings[%d] is %s, which the autoscaling status %s never gives", i, f.Code, status)
		}
	}
	return nil
}

// checkFindingConfidence refuses a finding whose confidence is not one the
// definition of its code allows: a fixed list, the confidence the
// approximate flag of its row decides, the confidence of its node pool, or
// the confidence of the cluster autoscaling.
func checkFindingConfidence(s Snapshot) error {
	approximate := make(map[workloadIdentity]bool, len(s.Workloads))
	for _, wl := range s.Workloads {
		approximate[workloadIdentity{wl.Namespace, wl.Name, wl.Kind}] = wl.Approximate
	}
	namespaces := make(map[string]bool, len(s.Namespaces))
	for _, ns := range s.Namespaces {
		namespaces[ns.Name] = ns.Approximate
	}
	pools := make(map[string]string, len(s.Nodepools))
	for _, pool := range s.Nodepools {
		pools[pool.Name] = pool.Confidence
	}

	for i, f := range s.Findings {
		def, _ := analysis.Lookup(f.Code)
		rule := def.Confidence
		allowed := rule.Fixed
		switch {
		case rule.FromApproximate:
			isApproximate := namespaces[f.ScopeName]
			if f.ScopeType == analysis.ScopeWorkload {
				namespace, name, _ := strings.Cut(f.ScopeName, "/")
				isApproximate = approximate[workloadIdentity{namespace, name, f.ScopeKind}]
			}
			allowed = []string{discovery.ConfidenceExact}
			if isApproximate {
				allowed = []string{discovery.ConfidenceInferred}
			}
		case rule.FromPool:
			allowed = []string{pools[f.ScopeName]}
		case rule.FromAutoscaling:
			allowed = []string{s.Summary.Autoscaling.Confidence}
		}
		if !slices.Contains(allowed, f.Confidence) {
			return fmt.Errorf("findings[%d].confidence %s does not match the definition of %s", i, f.Confidence, f.Code)
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

// checkRecommendations refuses a recommendations list that differs, entry
// by entry, from the one analysis.Recommendations gives for the findings.
func checkRecommendations(s Snapshot) error {
	findings := make([]analysis.Finding, len(s.Findings))
	for i, f := range s.Findings {
		findings[i] = analysis.Finding{
			Code:       f.Code,
			Severity:   f.Severity,
			ScopeType:  f.ScopeType,
			ScopeKind:  f.ScopeKind,
			ScopeName:  f.ScopeName,
			Confidence: f.Confidence,
		}
	}
	want := analysis.Recommendations(findings)
	if len(s.Recommendations) != len(want) {
		return fmt.Errorf("recommendations holds %d entries, want %d from the findings", len(s.Recommendations), len(want))
	}
	for i, got := range s.Recommendations {
		w := want[i]
		if got.Code != w.Code || got.Severity != w.Severity || got.ScopeType != w.ScopeType ||
			!slices.Equal(got.Scopes, w.Scopes) || got.Confidence != w.Confidence {
			return fmt.Errorf("recommendations[%d] differs from the findings: got %s, want %s", i,
				describeRecommendation(got.Code, got.Severity, got.ScopeType, got.Scopes, got.Confidence),
				describeRecommendation(w.Code, w.Severity, w.ScopeType, w.Scopes, w.Confidence))
		}
	}
	return nil
}

// describeRecommendation writes a recommendation as its code, severity,
// scope type, scopes, and confidence.
func describeRecommendation(code, severity, scopeType string, scopes []string, confidence string) string {
	return fmt.Sprintf("%s %s %s %v %s", code, severity, scopeType, scopes, confidence)
}

// validAutoscalingStatuses and validAutoscalingSources are the values of
// summary.autoscaling.status and summary.autoscaling.source.
var (
	validAutoscalingStatuses = []string{collect.AutoscalingEnabled, collect.AutoscalingDisabled, collect.AutoscalingUnknown}
	validAutoscalingSources  = []string{discovery.StrategyConfigured, discovery.StrategyRawMetrics, discovery.StrategyNone}
)

// checkSummary refuses a count of the summary that differs from the number
// of rows, findings, or recommendations it counts: the observed workloads, the
// uncovered workloads, and the findings of each severity.
func checkSummary(s Snapshot) error {
	var observed, uncovered, healthy, attention, critical float64
	for _, wl := range s.Workloads {
		if wl.ObservedUsage {
			observed++
		}
		if wl.HPACoverage == CoverageUncovered {
			uncovered++
		}
	}
	for _, f := range s.Findings {
		switch f.Severity {
		case analysis.SeverityHealthy:
			healthy++
		case analysis.SeverityAttention:
			attention++
		case analysis.SeverityCritical:
			critical++
		}
	}

	counts := []struct {
		name      string
		got, want float64
	}{
		{"namespaces", s.Summary.Namespaces, float64(len(s.Namespaces))},
		{"workloads", s.Summary.Workloads, observed},
		{"hpas", s.Summary.HPAs, float64(len(s.HPAs))},
		{"nodes", s.Summary.Nodes, float64(len(s.Nodes))},
		{"nodepools", s.Summary.Nodepools, float64(len(s.Nodepools))},
		{"findings", s.Summary.Findings, float64(len(s.Findings))},
		{"recommendations", s.Summary.Recommendations, float64(len(s.Recommendations))},
		{"healthy", s.Summary.Healthy, healthy},
		{"attention", s.Summary.Attention, attention},
		{"critical", s.Summary.Critical, critical},
		{"workloads_without_hpa", s.Summary.WorkloadsWithoutHPA, uncovered},
	}
	for _, c := range counts {
		if c.got != c.want {
			return fmt.Errorf("summary.%s is %v, want %v", c.name, c.got, c.want)
		}
	}
	return nil
}

// autoscalingShape is a combination of status, source, and confidence.
type autoscalingShape struct {
	status, source, confidence string
}

// autoscalingShapes are the combinations analysis writes: a configured
// mode, no evidence at all, and the evidence of the raw metrics.
var autoscalingShapes = []autoscalingShape{
	{collect.AutoscalingEnabled, discovery.StrategyConfigured, discovery.ConfidenceExact},
	{collect.AutoscalingDisabled, discovery.StrategyConfigured, discovery.ConfidenceExact},
	{collect.AutoscalingUnknown, discovery.StrategyNone, discovery.ConfidenceUnknown},
	{collect.AutoscalingEnabled, discovery.StrategyRawMetrics, discovery.ConfidenceInferred},
}

// checkAutoscaling refuses an autoscaling whose status, source, or
// confidence is outside its values, whose configured source comes with a
// status that is unknown, whose unknown status comes with a source or a
// confidence, whose combination of the three is not one that analysis
// writes, whose configured or empty source holds a count, or whose counts
// are not numbers of the right kind: the four counts of nodes and groups
// are whole numbers; the two others are finite numbers.
func checkAutoscaling(a Autoscaling) error {
	if err := checkEnum("summary.autoscaling.status", a.Status, validAutoscalingStatuses); err != nil {
		return err
	}
	if err := checkEnum("summary.autoscaling.source", a.Source, validAutoscalingSources); err != nil {
		return err
	}
	if err := checkEnum("summary.autoscaling.confidence", a.Confidence, validConfidences); err != nil {
		return err
	}
	if a.Source == discovery.StrategyConfigured && a.Status == collect.AutoscalingUnknown {
		return errors.New("summary.autoscaling is configured and its status is unknown")
	}
	if a.Status == collect.AutoscalingUnknown && (a.Source != discovery.StrategyNone || a.Confidence != discovery.ConfidenceUnknown) {
		return errors.New("summary.autoscaling is unknown and has a source or a confidence other than none and unknown")
	}
	if !slices.Contains(autoscalingShapes, autoscalingShape{a.Status, a.Source, a.Confidence}) {
		return fmt.Errorf("summary.autoscaling holds status %s, source %s, and confidence %s, which analysis never writes", a.Status, a.Source, a.Confidence)
	}
	if a.Source != discovery.StrategyRawMetrics {
		for _, count := range []*float64{a.ManagedNodeGroups, a.CurrentNodes, a.MinNodes, a.MaxNodes, a.ScaleEvents, a.UnschedulablePods} {
			if count != nil {
				return fmt.Errorf("summary.autoscaling with source %s holds a count", a.Source)
			}
		}
	}

	wholes := []struct {
		name  string
		value *float64
	}{
		{"managed_node_groups", a.ManagedNodeGroups},
		{"current_nodes", a.CurrentNodes},
		{"min_nodes", a.MinNodes},
		{"max_nodes", a.MaxNodes},
	}
	for _, n := range wholes {
		if n.value != nil && (!isFinite(*n.value) || *n.value != math.Trunc(*n.value)) {
			return fmt.Errorf("summary.autoscaling.%s must be a finite whole number", n.name)
		}
	}
	if err := checkFinite("summary.autoscaling.scale_events", a.ScaleEvents); err != nil {
		return err
	}
	return checkFinite("summary.autoscaling.unschedulable_pods", a.UnschedulablePods)
}

// checkNonNegative refuses value, when present, that is NaN, infinite, or
// below zero.
func checkNonNegative(path string, value *float64) error {
	if value != nil && (!isFinite(*value) || *value < 0) {
		return fmt.Errorf("%s must be a finite number, zero or above", path)
	}
	return nil
}

// checkFinite refuses value, when present, that is NaN or infinite.
func checkFinite(path string, value *float64) error {
	if value != nil && !isFinite(*value) {
		return fmt.Errorf("%s is not a finite number", path)
	}
	return nil
}

// checkHPARows refuses an hpas list that is not sorted by namespace and
// name, holds a repeated row, or holds a row with a namespace outside the
// scope, an empty name, a slash in its namespace or name, a target that is
// only half named, a number that is not finite, or a minimum above its
// maximum.
func checkHPARows(hpas []HPA, scopeNamespaces []string) error {
	for i := 1; i < len(hpas); i++ {
		prev, cur := hpas[i-1], hpas[i]
		if prev.Namespace > cur.Namespace || (prev.Namespace == cur.Namespace && prev.Name >= cur.Name) {
			return errors.New("hpas is not sorted or holds a repeated value")
		}
	}
	for i, hpa := range hpas {
		if strings.Contains(hpa.Namespace, "/") {
			return fmt.Errorf("hpas[%d].namespace holds a slash: %q", i, hpa.Namespace)
		}
		if hpa.Name == "" {
			return fmt.Errorf("hpas[%d].name is empty", i)
		}
		if strings.Contains(hpa.Name, "/") {
			return fmt.Errorf("hpas[%d].name holds a slash: %q", i, hpa.Name)
		}
		if !slices.Contains(scopeNamespaces, hpa.Namespace) {
			return fmt.Errorf("hpas[%d].namespace is not a namespace of the scope", i)
		}
		if (hpa.TargetKind == "") != (hpa.TargetName == "") {
			return fmt.Errorf("hpas[%d].target_kind and hpas[%d].target_name must both be set or both be empty", i, i)
		}
		if err := checkFinite(fmt.Sprintf("hpas[%d].spec_min", i), hpa.SpecMin); err != nil {
			return err
		}
		if err := checkFinite(fmt.Sprintf("hpas[%d].spec_max", i), hpa.SpecMax); err != nil {
			return err
		}
		if hpa.SpecMin != nil && hpa.SpecMax != nil && *hpa.SpecMin > *hpa.SpecMax {
			return fmt.Errorf("hpas[%d].spec_min is greater than hpas[%d].spec_max", i, i)
		}
		if err := checkStatistics(fmt.Sprintf("hpas[%d].current", i), hpa.Current); err != nil {
			return err
		}
		if err := checkStatistics(fmt.Sprintf("hpas[%d].desired", i), hpa.Desired); err != nil {
			return err
		}
	}
	return nil
}

// checkNodeRows refuses a nodes list that is not sorted by name, holds a
// repeated name, or holds a row with an empty name or pool, a confidence
// outside its values, or a number that is not finite and zero or above.
func checkNodeRows(nodes []Node) error {
	names := make([]string, len(nodes))
	for i, node := range nodes {
		names[i] = node.Name
	}
	if err := checkSorted("nodes", names); err != nil {
		return err
	}
	for i, node := range nodes {
		if node.Name == "" {
			return fmt.Errorf("nodes[%d].name is empty", i)
		}
		if node.Pool == "" {
			return fmt.Errorf("nodes[%d].pool is empty", i)
		}
		if err := checkEnum(fmt.Sprintf("nodes[%d].pool_confidence", i), node.PoolConfidence, validConfidences); err != nil {
			return err
		}
		numbers := []struct {
			name  string
			value *float64
		}{
			{"cpu_capacity", node.CPUCapacity},
			{"memory_capacity", node.MemoryCapacity},
			{"cpu_allocatable", node.CPUAllocatable},
			{"memory_allocatable", node.MemoryAllocatable},
			{"cpu_request_percent", node.CPURequestPercent},
			{"memory_request_percent", node.MemoryRequestPercent},
		}
		for _, n := range numbers {
			if err := checkNonNegative(fmt.Sprintf("nodes[%d].%s", i, n.name), n.value); err != nil {
				return err
			}
		}
		if err := checkStatistics(fmt.Sprintf("nodes[%d].cpu_usage_percent", i), node.CPUUsagePercent); err != nil {
			return err
		}
		if err := checkStatistics(fmt.Sprintf("nodes[%d].memory_usage_percent", i), node.MemoryUsagePercent); err != nil {
			return err
		}
	}
	return nil
}

// checkNodepoolRows refuses a nodepools list that is not sorted by name,
// holds a pool with no node, a pool that names a node the snapshot does
// not hold, a node listed by two pools or by none, a pool whose confidence
// is not the lowest of its nodes, and a node whose pool is not the pool
// that lists it.
func checkNodepoolRows(nodes []Node, pools []NodePool) error {
	names := make([]string, len(pools))
	for i, pool := range pools {
		names[i] = pool.Name
	}
	if err := checkSorted("nodepools", names); err != nil {
		return err
	}

	nodeIndex := make(map[string]int, len(nodes))
	for i, node := range nodes {
		nodeIndex[node.Name] = i
	}
	owner := make(map[string]int, len(nodes))
	for i, pool := range pools {
		if len(pool.Nodes) == 0 {
			return fmt.Errorf("nodepools[%d].nodes is empty", i)
		}
		if err := checkSorted(fmt.Sprintf("nodepools[%d].nodes", i), pool.Nodes); err != nil {
			return err
		}
		for _, name := range pool.Nodes {
			if _, ok := nodeIndex[name]; !ok {
				return fmt.Errorf("nodepools[%d] (%s) names node %q, which is not a node of the snapshot", i, pool.Name, name)
			}
			if _, ok := owner[name]; ok {
				return fmt.Errorf("node %q is listed by more than one nodepool", name)
			}
			owner[name] = i
		}
	}
	for _, node := range nodes {
		if _, ok := owner[node.Name]; !ok {
			return fmt.Errorf("node %q is listed by no nodepool", node.Name)
		}
	}

	for i, pool := range pools {
		lowest := 0
		for _, name := range pool.Nodes {
			lowest = max(lowest, slices.Index(validConfidences, nodes[nodeIndex[name]].PoolConfidence))
		}
		if want := validConfidences[lowest]; pool.Confidence != want {
			return fmt.Errorf("nodepools[%d].confidence is %q, want %q, the lowest of its nodes", i, pool.Confidence, want)
		}
	}
	for i, node := range nodes {
		if pool := pools[owner[node.Name]]; node.Pool != pool.Name {
			return fmt.Errorf("nodes[%d].pool is %q, but nodepools[%d] (%s) lists it", i, node.Pool, owner[node.Name], pool.Name)
		}
	}
	return nil
}

// checkWorkloadCoverage refuses a workload whose hpa_coverage is outside
// its values, an uncovered workload that an HPA targets, a covered workload
// that no HPA targets, and a row with no
// observed usage that is not uncovered, holds pods, or holds a statistic
// that is not unavailable.
func checkWorkloadCoverage(workloads []Workload, hpas []HPA) error {
	for i, wl := range workloads {
		if err := checkEnum(fmt.Sprintf("workloads[%d].hpa_coverage", i), wl.HPACoverage, validCoverages); err != nil {
			return err
		}
		targeted := false
		for j, hpa := range hpas {
			if hpa.TargetKind != "" && hpa.TargetName != "" &&
				hpa.Namespace == wl.Namespace && hpa.TargetName == wl.Name && hpa.TargetKind == wl.Kind {
				targeted = true
				if wl.HPACoverage == CoverageUncovered {
					return fmt.Errorf("workloads[%d] (%s/%s) is uncovered and is the target of hpas[%d]", i, wl.Namespace, wl.Name, j)
				}
			}
		}
		if wl.HPACoverage == CoverageCovered && !targeted {
			return fmt.Errorf("workloads[%d] (%s/%s) is covered and no HPA targets it", i, wl.Namespace, wl.Name)
		}
		if wl.ObservedUsage {
			continue
		}
		if wl.HPACoverage != CoverageUncovered {
			return fmt.Errorf("workloads[%d] has no observed usage and is not uncovered", i)
		}
		if wl.Pods != nil {
			return fmt.Errorf("workloads[%d] has no observed usage and holds pods", i)
		}
		for _, field := range workloadStatisticsFields(wl) {
			if field.stat.State != collect.StateUnavailable {
				return fmt.Errorf("workloads[%d].%s must be unavailable for a row with no observed usage", i, field.name)
			}
		}
	}
	return nil
}
