package config

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"regexp"
	"strings"
)

// labelRe matches a valid Prometheus label name.
var labelRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// The messages below are returned by more than one function (Validate,
// Duration, Location), so each is defined once here to guarantee the exact
// wording never drifts between call sites.
var (
	errPeriodShape          = errors.New("analysis.period must look like 30m, 4h, 24h, or 7d")
	errPeriodTooLong        = errors.New("analysis.period is too long")
	errTimezoneUnknown      = errors.New("analysis.timezone is unknown")
	errTokenTooShort        = errors.New("the token must hold at least 8 characters")
	errMetricsURLShape      = errors.New("metrics.url must be an absolute HTTP or HTTPS URL")
	errMetricsURLCreds      = errors.New("metrics.url must not embed credentials")
	errMetricsURLQuery      = errors.New("metrics.url must not include a query string")
	errMetricsURLFrag       = errors.New("metrics.url must not include a fragment")
	errMetricsURLGraf       = errors.New("metrics.url must identify a Grafana datasource proxy API root")
	errMetricsCABundle      = errors.New("metrics.ca_bundle must be an existing file")
	errMetricsTimeout       = errors.New("metrics.timeout_seconds must be positive")
	errMetricsURLReq        = errors.New("metrics.url is required")
	errEndpointType         = errors.New("metrics.endpoint_type must be auto, prometheus, or grafana")
	errClusterName          = errors.New("cluster.name is required")
	errClusterNS            = errors.New("cluster.namespaces must contain at least one non-empty namespace")
	errClusterLabel         = errors.New("cluster.label must be a valid Prometheus label name")
	errClusterFilter        = errors.New("cluster.filter_mode must be required, auto, or none")
	errClusterNodepool      = errors.New("cluster.nodepool_label must be a valid Prometheus label name")
	errNodeAutoscaling      = errors.New("analysis.node_autoscaling must be auto, enabled, or disabled")
	errThresholdRange       = errors.New("threshold values must be within 0..100")
	errThresholdFinite      = errors.New("threshold values must be finite numbers")
	errMetricsTimeoutFinite = errors.New("metrics.timeout_seconds must be a finite number")
	errBurstyGap            = errors.New("thresholds.bursty_gap_percent must not be negative")
	errHPADivergence        = errors.New("thresholds.hpa_replica_divergence must not be negative")
	errWarningCritical      = errors.New("thresholds.warning_percent must be lower than critical_percent")
	errReportProfile        = errors.New("report.profile must be detailed or executive")
	errReportSeverity       = errors.New("report.min_severity must be all, attention, or critical")
	errReportLanguage       = errors.New("report.language must be pt-BR or en")
	errReportOutputDir      = errors.New("report.output_directory must exist and be writable")
)

// Validate checks every rule of spec 5.2 and returns the first failure, in
// the order of the table of rules. It has no access to the environment, so
// it does not check the token (that is Load's responsibility).
func (c Config) Validate() error {
	if err := validateMetrics(c.Metrics); err != nil {
		return err
	}
	if err := validateCluster(c.Cluster); err != nil {
		return err
	}
	if err := validateAnalysis(c.Analysis); err != nil {
		return err
	}
	if err := validateThresholds(c.Thresholds); err != nil {
		return err
	}
	return validateReport(c.Report)
}

func validateMetrics(m Metrics) error {
	if m.URL == "" {
		return errMetricsURLReq
	}

	parsed, err := url.Parse(m.URL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return errMetricsURLShape
	}
	if parsed.User != nil {
		return errMetricsURLCreds
	}
	if parsed.RawQuery != "" {
		return errMetricsURLQuery
	}
	if parsed.Fragment != "" {
		return errMetricsURLFrag
	}

	switch m.EndpointType {
	case "auto", "prometheus", "grafana":
	default:
		return errEndpointType
	}
	if m.EndpointType == "grafana" && !strings.Contains(parsed.Path, "datasource") && !strings.Contains(parsed.Path, "proxy") {
		return errMetricsURLGraf
	}

	if m.CABundle != "" {
		info, err := os.Stat(m.CABundle)
		if err != nil || info.IsDir() {
			return errMetricsCABundle
		}
	}
	if math.IsNaN(m.TimeoutSeconds) || math.IsInf(m.TimeoutSeconds, 0) {
		return errMetricsTimeoutFinite
	}
	if m.TimeoutSeconds <= 0 {
		return errMetricsTimeout
	}
	return nil
}

func validateCluster(cl Cluster) error {
	if cl.Name == "" {
		return errClusterName
	}
	if len(cl.Namespaces) == 0 {
		return errClusterNS
	}
	for _, ns := range cl.Namespaces {
		if ns == "" {
			return errClusterNS
		}
		if strings.Contains(ns, "/") {
			return fmt.Errorf("cluster.namespaces holds a namespace with a slash: %q", ns)
		}
	}
	if !labelRe.MatchString(cl.Label) {
		return errClusterLabel
	}
	switch cl.FilterMode {
	case "required", "auto", "none":
	default:
		return errClusterFilter
	}
	if cl.NodepoolLabel != "" && !labelRe.MatchString(cl.NodepoolLabel) {
		return errClusterNodepool
	}
	return nil
}

func validateAnalysis(a Analysis) error {
	if !periodRe.MatchString(a.Period) {
		return errPeriodShape
	}
	if _, err := a.Duration(); err != nil {
		return errPeriodTooLong
	}
	if _, err := a.Location(); err != nil {
		return errTimezoneUnknown
	}
	switch a.NodeAutoscaling {
	case "auto", "enabled", "disabled":
	default:
		return errNodeAutoscaling
	}
	return nil
}

func validateThresholds(th Thresholds) error {
	for _, v := range []float64{
		th.WarningPercent,
		th.CriticalPercent,
		th.LowUtilizationPercent,
		th.AutoscalerNearMaxPercent,
		th.NodepoolSpreadPoints,
		th.BurstyGapPercent,
		th.HPAReplicaDivergence,
	} {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return errThresholdFinite
		}
	}
	percents := []float64{
		th.WarningPercent,
		th.CriticalPercent,
		th.LowUtilizationPercent,
		th.AutoscalerNearMaxPercent,
		th.NodepoolSpreadPoints,
	}
	for _, p := range percents {
		if p < 0 || p > 100 {
			return errThresholdRange
		}
	}
	if th.BurstyGapPercent < 0 {
		return errBurstyGap
	}
	if th.HPAReplicaDivergence < 0 {
		return errHPADivergence
	}
	if th.WarningPercent >= th.CriticalPercent {
		return errWarningCritical
	}
	return nil
}

func validateReport(r Report) error {
	switch r.Profile {
	case "detailed", "executive":
	default:
		return errReportProfile
	}
	switch r.MinSeverity {
	case "all", "attention", "critical":
	default:
		return errReportSeverity
	}
	switch r.Language {
	case "pt-BR", "en":
	default:
		return errReportLanguage
	}
	if err := checkWritableDir(r.OutputDirectory); err != nil {
		return err
	}
	return nil
}

// removeFile is the seam TestWritableCheckReportsAProbeItCannotRemove
// replaces to force the clean-up path of checkWritableDir to fail without
// needing a real read-only-after-create filesystem.
var removeFile = os.Remove

// checkWritableDir reports whether path exists, is a folder, and a file can
// be created in it. A permission bit alone is not checked; a temporary file
// is actually created, then closed and removed. The folder being missing,
// not being a folder, or refusing the file's creation all return the same
// "must exist and be writable" error. A probe file that was created but
// then could not be closed or removed is reported by name instead of being
// left behind silently: removal is attempted even when the close fails, so
// a close-only failure still gets one more chance to clean up.
func checkWritableDir(path string) error {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return errReportOutputDir
	}

	probe, err := os.CreateTemp(path, ".kcr-write-test-*")
	if err != nil {
		return errReportOutputDir
	}
	name := probe.Name()

	closeErr := probe.Close()
	removeErr := removeFile(name)

	cause := closeErr
	if cause == nil {
		cause = removeErr
	}
	if cause != nil {
		return fmt.Errorf("report.output_directory holds a probe file that could not be removed: %s: %s", name, cause)
	}
	return nil
}
