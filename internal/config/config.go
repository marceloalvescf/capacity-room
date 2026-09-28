// Package config reads the settings of the report command: a TOML file,
// then the environment, then the command-line flags, in that order of
// precedence, and validates the result against spec 5.2.
package config

import (
	"regexp"
	"strconv"
	"time"

	// Imported for its effect: it embeds the IANA time zone database in the
	// binary, so analysis.timezone resolves the same way on every machine,
	// whether or not the host itself carries zoneinfo files.
	_ "time/tzdata"
)

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

// Duration returns the length of analysis.period.
func (a Analysis) Duration() (time.Duration, error) {
	if !periodRe.MatchString(a.Period) {
		return 0, errPeriodShape
	}

	value, err := strconv.Atoi(a.Period[:len(a.Period)-1])
	if err != nil {
		return 0, errPeriodShape
	}

	switch a.Period[len(a.Period)-1] {
	case 'm':
		return time.Duration(value) * time.Minute, nil
	case 'h':
		return time.Duration(value) * time.Hour, nil
	default: // 'd', the only other letter periodRe accepts
		return time.Duration(value) * 24 * time.Hour, nil
	}
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
