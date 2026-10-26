package profiles

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/marceloalvescf/capacity-room/internal/config"
)

// Document is a profile as an administrator writes it and as a revision
// stores it. Its JSON form is the body of the API and the row's document.
type Document struct {
	DisplayName             string            `json:"display_name"`
	Active                  bool              `json:"active"`
	EndpointURL             string            `json:"endpoint_url"`
	EndpointType            string            `json:"endpoint_type"` // auto, prometheus, grafana
	TimeoutSeconds          float64           `json:"timeout_seconds"`
	TLSVerify               bool              `json:"tls_verify"`
	CredentialFile          string            `json:"credential_file"` // "" for none
	CABundleFile            string            `json:"ca_bundle_file"`  // "" for none
	ClusterName             string            `json:"cluster_name"`
	ClusterLabel            string            `json:"cluster_label"`
	FilterMode              string            `json:"filter_mode"` // required, auto, none
	NodepoolLabel           string            `json:"nodepool_label"`
	NodeAutoscaling         string            `json:"node_autoscaling"` // auto, enabled, disabled
	AllowedNamespaces       []string          `json:"allowed_namespaces"`
	ExcludedContainers      []string          `json:"excluded_containers"`
	PermittedPeriods        []string          `json:"permitted_periods"`
	DefaultPeriod           *string           `json:"default_period"`
	PermittedTimezones      []string          `json:"permitted_timezones"`
	DefaultTimezone         *string           `json:"default_timezone"`
	PermittedReportProfiles []string          `json:"permitted_report_profiles"`
	DefaultReportProfile    *string           `json:"default_report_profile"`
	PermittedMinSeverities  []string          `json:"permitted_min_severities"`
	DefaultMinSeverity      *string           `json:"default_min_severity"`
	Thresholds              config.Thresholds `json:"thresholds"`
}

// The defaults of spec 5.2 that a null default resolves to.
const (
	FallbackPeriod        = "24h"
	FallbackTimezone      = "America/Sao_Paulo"
	FallbackReportProfile = "detailed"
	FallbackMinSeverity   = "all"
)

// Defaults are the values a body may leave out.
func Defaults() Document {
	return Document{
		EndpointType:            "auto",
		TimeoutSeconds:          90,
		TLSVerify:               true,
		ClusterLabel:            "cluster",
		FilterMode:              "required",
		NodeAutoscaling:         "auto",
		AllowedNamespaces:       []string{},
		ExcludedContainers:      []string{},
		PermittedPeriods:        []string{},
		PermittedTimezones:      []string{},
		PermittedReportProfiles: []string{},
		PermittedMinSeverities:  []string{},
		Thresholds:              config.Default().Thresholds,
	}
}

// Problem names what is wrong with one field.
type Problem struct {
	Field string
	Word  string // missing, invalid, repeated, empty, not_permitted, incoherent
}

// ReferenceNamePattern is the shape of a credential or CA bundle file name.
var ReferenceNamePattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,61}[a-z0-9])?$`)

var (
	labelNamePattern     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	namespacePattern     = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]{0,61}[a-z0-9])?$`)
	containerNamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,252}$`)
)

// ValidReferenceName reports whether name matches the pattern and holds no "..".
func ValidReferenceName(name string) bool {
	return ReferenceNamePattern.MatchString(name) && !strings.Contains(name, "..")
}

// ValidNamespace reports whether name is a valid namespace name.
func ValidNamespace(name string) bool {
	return namespacePattern.MatchString(name)
}

// ResolvedDefaults returns the four effective defaults of d.
func (d Document) ResolvedDefaults() (period, timezone, reportProfile, minSeverity string) {
	return orFallback(d.DefaultPeriod, FallbackPeriod),
		orFallback(d.DefaultTimezone, FallbackTimezone),
		orFallback(d.DefaultReportProfile, FallbackReportProfile),
		orFallback(d.DefaultMinSeverity, FallbackMinSeverity)
}

func orFallback(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
}

// Validate applies every rule of the document and returns one Problem per
// field, in field order, or nil. It reads no file and resolves no name.
func (d Document) Validate() []Problem {
	var out []Problem
	add := func(field, word string) {
		if word != "" {
			out = append(out, Problem{Field: field, Word: word})
		}
	}

	word := ""
	switch {
	case strings.TrimSpace(d.DisplayName) == "":
		word = "empty"
	case utf8.RuneCountInString(d.DisplayName) > 128 || hasControl(d.DisplayName):
		word = "invalid"
	}
	add("display_name", word)

	if d.EndpointURL == "" {
		add("endpoint_url", "missing")
	}
	add("endpoint_type", oneOf(d.EndpointType, "auto", "prometheus", "grafana"))
	add("timeout_seconds", flag(math.IsNaN(d.TimeoutSeconds) || d.TimeoutSeconds <= 0 || d.TimeoutSeconds > 600))
	add("credential_file", flag(d.CredentialFile != "" && !ValidReferenceName(d.CredentialFile)))

	word = ""
	switch {
	case d.CABundleFile != "" && !ValidReferenceName(d.CABundleFile):
		word = "invalid"
	case d.CABundleFile != "" && !d.TLSVerify:
		word = "incoherent"
	}
	add("ca_bundle_file", word)

	word = ""
	switch {
	case strings.TrimSpace(d.ClusterName) == "":
		word = "empty"
	case hasControl(d.ClusterName):
		word = "invalid"
	}
	add("cluster_name", word)

	add("cluster_label", flag(!labelNamePattern.MatchString(d.ClusterLabel)))
	add("filter_mode", oneOf(d.FilterMode, "required", "auto", "none"))
	add("nodepool_label", flag(d.NodepoolLabel != "" && !labelNamePattern.MatchString(d.NodepoolLabel)))
	add("node_autoscaling", oneOf(d.NodeAutoscaling, "auto", "enabled", "disabled"))

	if len(d.AllowedNamespaces) == 0 {
		add("allowed_namespaces", "empty")
	} else {
		add("allowed_namespaces", checkList(d.AllowedNamespaces, namespacePattern.MatchString))
	}
	add("excluded_containers", checkList(d.ExcludedContainers, containerNamePattern.MatchString))

	period, zone, profile, severity := d.ResolvedDefaults()
	isProfile := func(s string) bool { return s == "detailed" || s == "executive" }
	isSeverity := func(s string) bool { return s == "all" || s == "attention" || s == "critical" }
	validPeriod := func(s string) bool { return config.ValidatePeriod(s) == nil }
	validZone := func(s string) bool { return config.ValidateTimezone(s) == nil }

	add("permitted_periods", checkList(d.PermittedPeriods, validPeriod))
	add("default_period", checkDefault(d.DefaultPeriod, period, d.PermittedPeriods, validPeriod))
	add("permitted_timezones", checkList(d.PermittedTimezones, validZone))
	add("default_timezone", checkDefault(d.DefaultTimezone, zone, d.PermittedTimezones, validZone))
	add("permitted_report_profiles", checkList(d.PermittedReportProfiles, isProfile))
	add("default_report_profile", checkDefault(d.DefaultReportProfile, profile, d.PermittedReportProfiles, isProfile))
	add("permitted_min_severities", checkList(d.PermittedMinSeverities, isSeverity))
	add("default_min_severity", checkDefault(d.DefaultMinSeverity, severity, d.PermittedMinSeverities, isSeverity))

	add("thresholds", flag(config.ValidateThresholds(d.Thresholds) != nil))
	return out
}

func flag(bad bool) string {
	if bad {
		return "invalid"
	}
	return ""
}

func oneOf(v string, allowed ...string) string {
	return flag(!slices.Contains(allowed, v))
}

// checkList returns invalid for the first item that fails ok, else repeated
// for a duplicate, else "".
func checkList(items []string, ok func(string) bool) string {
	for _, it := range items {
		if !ok(it) {
			return "invalid"
		}
	}
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		if seen[it] {
			return "repeated"
		}
		seen[it] = true
	}
	return ""
}

func checkDefault(set *string, resolved string, permitted []string, ok func(string) bool) string {
	if set != nil && !ok(*set) {
		return "invalid"
	}
	if len(permitted) > 0 && !slices.Contains(permitted, resolved) {
		return "not_permitted"
	}
	return ""
}

// RunConfig builds the configuration of a run from the document and the
// selection. The token never enters the configuration, and neither does the
// CA bundle: the caller's transport carries it.
func (d Document) RunConfig(period, start, end, timezone, reportProfile, _ string, namespaces []string) config.Config {
	c := config.Default()
	c.Metrics = config.Metrics{
		URL:            d.EndpointURL,
		EndpointType:   d.EndpointType,
		TLSVerify:      d.TLSVerify,
		TimeoutSeconds: d.TimeoutSeconds,
	}
	c.Cluster = config.Cluster{
		Name:          d.ClusterName,
		Namespaces:    namespaces,
		Label:         d.ClusterLabel,
		FilterMode:    d.FilterMode,
		NodepoolLabel: d.NodepoolLabel,
	}
	c.Analysis = config.Analysis{
		Period:             period,
		Start:              start,
		End:                end,
		Timezone:           timezone,
		NodeAutoscaling:    d.NodeAutoscaling,
		ExcludedContainers: d.ExcludedContainers,
	}
	c.Thresholds = d.Thresholds
	c.Report.Profile = reportProfile
	return c
}
