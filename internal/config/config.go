// Package config reads the settings of the report command: a TOML file,
// then the environment, then the command-line flags, in that order of
// precedence, and validates the result against spec 5.2.
package config

import (
	"math"
	"regexp"
	"strconv"
	"time"
	"unicode/utf8"

	// Imported for its effect: it embeds the IANA time zone database in the
	// binary, so analysis.timezone resolves the same way on every machine,
	// whether or not the host itself carries zoneinfo files.
	_ "time/tzdata"
)

// MinTokenLength is the fewest characters a non-empty --token or a
// non-empty variable named by metrics.token_env may hold. A token shorter
// than this cannot be told apart from ordinary text once every occurrence
// of it is redacted: a token of one letter, or a common short word, turns
// every unrelated use of that letter or word in a diagnostic into
// [redacted] too.
const MinTokenLength = 8

// CheckToken refuses a token shorter than MinTokenLength characters. An
// empty token is not refused here: an empty --token or metrics.token_env
// value means "not given", not "too short". The count is of characters,
// not of bytes, so a token of seven accented letters is refused exactly
// like one of seven plain ones.
func CheckToken(token string) error {
	if token == "" {
		return nil
	}
	if utf8.RuneCountInString(token) < MinTokenLength {
		return errTokenTooShort
	}
	return nil
}

// Config is the fully resolved, validated configuration of a report run.
type Config struct {
	Metrics    Metrics
	Cluster    Cluster
	Analysis   Analysis
	Thresholds Thresholds
	Report     Report
}

// Metrics configures the Prometheus-compatible endpoint the report reads.
type Metrics struct {
	URL            string // metrics.url
	EndpointType   string // metrics.endpoint_type: auto, prometheus, grafana
	TokenEnv       string // metrics.token_env
	TLSVerify      bool   // metrics.tls_verify
	CABundle       string // metrics.ca_bundle
	TimeoutSeconds float64
}

// Cluster names the cluster the report covers and how it is matched.
type Cluster struct {
	Name          string
	Namespaces    []string
	Label         string
	FilterMode    string // required, auto, none
	NodepoolLabel string
}

// Analysis configures the time window and scope of the analysis.
type Analysis struct {
	Period             string // for example "24h"
	Timezone           string
	NodeAutoscaling    string // auto, enabled, disabled
	ExcludedContainers []string
}

// Thresholds holds the numeric limits that decide a finding's severity.
type Thresholds struct {
	WarningPercent           float64
	CriticalPercent          float64
	LowUtilizationPercent    float64
	AutoscalerNearMaxPercent float64
	NodepoolSpreadPoints     float64
	BurstyGapPercent         float64
	HPAReplicaDivergence     float64
}

// Report configures how the report is written.
type Report struct {
	Profile         string // detailed, executive
	MinSeverity     string // all, attention, critical
	Language        string // pt-BR, en
	Organization    string
	Classification  string
	Author          string
	OutputDirectory string
}

// Default returns the defaults of spec 5.2. Required keys are left empty.
func Default() Config {
	return Config{
		Metrics: Metrics{
			EndpointType:   "auto",
			TLSVerify:      true,
			TimeoutSeconds: 90,
		},
		Cluster: Cluster{
			Label:      "cluster",
			FilterMode: "required",
		},
		Analysis: Analysis{
			Period:          "24h",
			Timezone:        "America/Sao_Paulo",
			NodeAutoscaling: "auto",
		},
		Thresholds: Thresholds{
			WarningPercent:           70,
			CriticalPercent:          85,
			LowUtilizationPercent:    30,
			AutoscalerNearMaxPercent: 80,
			NodepoolSpreadPoints:     30,
			BurstyGapPercent:         100,
			HPAReplicaDivergence:     1,
		},
		Report: Report{
			Profile:         "detailed",
			MinSeverity:     "all",
			Language:        "pt-BR",
			OutputDirectory: ".",
		},
	}
}

// periodRe matches a period of spec 5.2: a positive integer with no leading
// zero, followed by m, h, or d.
var periodRe = regexp.MustCompile(`^[1-9][0-9]*[mhd]$`)

// Flags holds the values given on the command line. A nil field was not
// given.
type Flags struct {
	MetricsURL, EndpointType, Token               *string
	Cluster, ClusterLabel, ClusterFilter          *string
	Namespaces, Period, Timezone, NodeAutoscaling *string
	MinSeverity, ReportProfile, Language          *string
	Insecure                                      bool
}

// Source names where Load reads a configuration from.
type Source struct {
	File   string // path of the TOML file
	Getenv func(string) string
	Flags  Flags
}

// Loaded is a valid configuration and the token, which is kept out of
// Config so that nothing that stores or prints a Config can hold it.
type Loaded struct {
	Config Config
	Token  string
}

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

// Duration returns the length of analysis.period. The design sets no upper
// limit of its own on a period, so the only limit is what a time.Duration
// (an int64 count of nanoseconds) can hold: a period whose number of
// digits, or whose product with its unit, does not fit is refused rather
// than silently wrapped into a negative value.
func (a Analysis) Duration() (time.Duration, error) {
	if !periodRe.MatchString(a.Period) {
		return 0, errPeriodShape
	}

	value, err := strconv.ParseInt(a.Period[:len(a.Period)-1], 10, 64)
	if err != nil {
		return 0, errPeriodTooLong
	}

	unit := periodUnitNanos(a.Period[len(a.Period)-1])
	if value > math.MaxInt64/unit {
		return 0, errPeriodTooLong
	}
	return time.Duration(value * unit), nil
}

// Location returns the zone of analysis.timezone, from the data embedded
// in the binary by the time/tzdata import above.
func (a Analysis) Location() (*time.Location, error) {
	loc, err := time.LoadLocation(a.Timezone)
	if err != nil {
		return nil, errTimezoneUnknown
	}
	return loc, nil
}
