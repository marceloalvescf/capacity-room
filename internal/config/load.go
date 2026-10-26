package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// sectionOrder lists the five sections of spec 5.2, in the order their
// fields appear in Config. It doubles as the set of known section names and
// fixes the order in which a structural problem (an unknown section, a
// section that is not a table, an unknown key) is reported when more than
// one section has one.
var sectionOrder = []string{"metrics", "cluster", "analysis", "thresholds", "report"}

// sectionKeys is the allow-list of keys for each section of sectionOrder.
var sectionKeys = map[string][]string{
	"metrics":    {"url", "token_env", "tls_verify", "ca_bundle", "timeout_seconds"},
	"cluster":    {"name", "namespaces", "label", "filter_mode", "nodepool_label"},
	"analysis":   {"period", "start", "end", "timezone", "node_autoscaling", "excluded_containers"},
	"thresholds": {"warning_percent", "critical_percent", "low_utilization_percent", "autoscaler_near_max_percent", "nodepool_spread_points", "bursty_gap_percent", "hpa_replica_divergence"},
	"report":     {"profile", "language", "organization", "classification", "author", "output_directory"},
}

// tomlConfig mirrors Config for decoding, with the lower snake case keys of
// spec 5.2. A key the file does not name leaves the field untouched, so the
// caller decodes into a value already populated from the current Config.
type tomlConfig struct {
	Metrics struct {
		URL            string  `toml:"url"`
		TokenEnv       string  `toml:"token_env"`
		TLSVerify      bool    `toml:"tls_verify"`
		CABundle       string  `toml:"ca_bundle"`
		TimeoutSeconds float64 `toml:"timeout_seconds"`
	}
	Cluster struct {
		Name          string   `toml:"name"`
		Namespaces    []string `toml:"namespaces"`
		Label         string   `toml:"label"`
		FilterMode    string   `toml:"filter_mode"`
		NodepoolLabel string   `toml:"nodepool_label"`
	}
	Analysis struct {
		Period             string   `toml:"period"`
		Start              string   `toml:"start"`
		End                string   `toml:"end"`
		Timezone           string   `toml:"timezone"`
		NodeAutoscaling    string   `toml:"node_autoscaling"`
		ExcludedContainers []string `toml:"excluded_containers"`
	}
	Thresholds struct {
		WarningPercent           float64 `toml:"warning_percent"`
		CriticalPercent          float64 `toml:"critical_percent"`
		LowUtilizationPercent    float64 `toml:"low_utilization_percent"`
		AutoscalerNearMaxPercent float64 `toml:"autoscaler_near_max_percent"`
		NodepoolSpreadPoints     float64 `toml:"nodepool_spread_points"`
		BurstyGapPercent         float64 `toml:"bursty_gap_percent"`
		HPAReplicaDivergence     float64 `toml:"hpa_replica_divergence"`
	}
	Report struct {
		Profile         string `toml:"profile"`
		Language        string `toml:"language"`
		Organization    string `toml:"organization"`
		Classification  string `toml:"classification"`
		Author          string `toml:"author"`
		OutputDirectory string `toml:"output_directory"`
	}
}

func fromConfig(c Config) tomlConfig {
	var t tomlConfig
	t.Metrics.URL = c.Metrics.URL
	t.Metrics.TokenEnv = c.Metrics.TokenEnv
	t.Metrics.TLSVerify = c.Metrics.TLSVerify
	t.Metrics.CABundle = c.Metrics.CABundle
	t.Metrics.TimeoutSeconds = c.Metrics.TimeoutSeconds
	t.Cluster.Name = c.Cluster.Name
	t.Cluster.Namespaces = c.Cluster.Namespaces
	t.Cluster.Label = c.Cluster.Label
	t.Cluster.FilterMode = c.Cluster.FilterMode
	t.Cluster.NodepoolLabel = c.Cluster.NodepoolLabel
	t.Analysis.Period = c.Analysis.Period
	t.Analysis.Start = c.Analysis.Start
	t.Analysis.End = c.Analysis.End
	t.Analysis.Timezone = c.Analysis.Timezone
	t.Analysis.NodeAutoscaling = c.Analysis.NodeAutoscaling
	t.Analysis.ExcludedContainers = c.Analysis.ExcludedContainers
	t.Thresholds.WarningPercent = c.Thresholds.WarningPercent
	t.Thresholds.CriticalPercent = c.Thresholds.CriticalPercent
	t.Thresholds.LowUtilizationPercent = c.Thresholds.LowUtilizationPercent
	t.Thresholds.AutoscalerNearMaxPercent = c.Thresholds.AutoscalerNearMaxPercent
	t.Thresholds.NodepoolSpreadPoints = c.Thresholds.NodepoolSpreadPoints
	t.Thresholds.BurstyGapPercent = c.Thresholds.BurstyGapPercent
	t.Thresholds.HPAReplicaDivergence = c.Thresholds.HPAReplicaDivergence
	t.Report.Profile = c.Report.Profile
	t.Report.Language = c.Report.Language
	t.Report.Organization = c.Report.Organization
	t.Report.Classification = c.Report.Classification
	t.Report.Author = c.Report.Author
	t.Report.OutputDirectory = c.Report.OutputDirectory
	return t
}

func toConfig(t tomlConfig) Config {
	return Config{
		Metrics: Metrics{
			URL:            t.Metrics.URL,
			TokenEnv:       t.Metrics.TokenEnv,
			TLSVerify:      t.Metrics.TLSVerify,
			CABundle:       t.Metrics.CABundle,
			TimeoutSeconds: t.Metrics.TimeoutSeconds,
		},
		Cluster: Cluster{
			Name:          t.Cluster.Name,
			Namespaces:    t.Cluster.Namespaces,
			Label:         t.Cluster.Label,
			FilterMode:    t.Cluster.FilterMode,
			NodepoolLabel: t.Cluster.NodepoolLabel,
		},
		Analysis: Analysis{
			Period:             t.Analysis.Period,
			Start:              t.Analysis.Start,
			End:                t.Analysis.End,
			Timezone:           t.Analysis.Timezone,
			NodeAutoscaling:    t.Analysis.NodeAutoscaling,
			ExcludedContainers: t.Analysis.ExcludedContainers,
		},
		Thresholds: Thresholds{
			WarningPercent:           t.Thresholds.WarningPercent,
			CriticalPercent:          t.Thresholds.CriticalPercent,
			LowUtilizationPercent:    t.Thresholds.LowUtilizationPercent,
			AutoscalerNearMaxPercent: t.Thresholds.AutoscalerNearMaxPercent,
			NodepoolSpreadPoints:     t.Thresholds.NodepoolSpreadPoints,
			BurstyGapPercent:         t.Thresholds.BurstyGapPercent,
			HPAReplicaDivergence:     t.Thresholds.HPAReplicaDivergence,
		},
		Report: Report{
			Profile:         t.Report.Profile,
			Language:        t.Report.Language,
			Organization:    t.Report.Organization,
			Classification:  t.Report.Classification,
			Author:          t.Report.Author,
			OutputDirectory: t.Report.OutputDirectory,
		},
	}
}

// wrongTypeRe matches the message BurntSushi/toml produces when a value in
// the file cannot be unified with a struct field's type: it always starts
// with the line and the dotted key, since decoding always happens with a
// non-empty context here (every field of tomlConfig sits one level under a
// section).
var wrongTypeRe = regexp.MustCompile(`^toml: line (\d+) \(last key "([^"]*)"\): (incompatible types:.*)$`)

// Load reads the file, then the environment, then the flags, and validates
// the result.
func Load(src Source) (Loaded, error) {
	getenv := src.Getenv
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	cfg := Default()

	if src.File != "" {
		if err := applyFile(src.File, &cfg); err != nil {
			return Loaded{}, err
		}
	}

	if err := applyEnvironment(&cfg, getenv); err != nil {
		return Loaded{}, err
	}
	if err := applyFlags(&cfg, src.Flags); err != nil {
		return Loaded{}, err
	}

	if err := cfg.Validate(); err != nil {
		return Loaded{}, err
	}

	token, err := resolveToken(getenv, src.Flags.Token, cfg.Metrics.TokenEnv)
	if err != nil {
		return Loaded{}, err
	}

	return Loaded{Config: cfg, Token: token}, nil
}

// applyFile reads the TOML file at path, checks its structure, and merges
// the keys it names into cfg. A key the file does not name is left as cfg
// already had it.
func applyFile(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		cause := err.Error()
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) {
			cause = pathErr.Err.Error()
		}
		return fmt.Errorf("configuration file cannot be read: %s: %s", path, cause)
	}
	text := string(data)

	var raw map[string]any
	if _, err := toml.Decode(text, &raw); err != nil {
		var parseErr toml.ParseError
		if errors.As(err, &parseErr) {
			return fmt.Errorf("configuration file is not valid TOML: line %d, column %d: %s",
				parseErr.Position.Line, parseErr.Position.Col, parseErr.Message)
		}
		return fmt.Errorf("configuration file is not valid TOML: %s", err)
	}

	if err := checkStructure(raw); err != nil {
		return err
	}

	target := fromConfig(*cfg)
	if _, err := toml.Decode(text, &target); err != nil {
		return wrongTypeError(err)
	}
	*cfg = toConfig(target)

	table, _ := raw["analysis"].(map[string]any)
	_, hasPeriod := table["period"]
	_, hasStart := table["start"]
	_, hasEnd := table["end"]
	if err := setWindow(&cfg.Analysis, hasPeriod, hasStart, hasEnd, errFileWindow); err != nil {
		return err
	}
	return nil
}

// setWindow settles one source's say on the window: a source that sets a
// period and a range together is refused with refusal, and one that sets a
// start without an end, or an end without a start, is refused; a source that
// sets a range replaces both ends and clears the period, and one that sets a
// period clears the range.
func setWindow(a *Analysis, period, start, end bool, refusal error) error {
	rng := start || end
	switch {
	case period && rng:
		return refusal
	case start != end:
		return errRangeTogether
	case rng:
		a.Period = ""
	case period:
		a.Start, a.End = "", ""
	}
	return nil
}

// checkStructure rejects an unknown top-level section, a known section that
// is not a table, the forbidden metrics.token key, and an unknown key
// inside a known section, in that order. Sections are checked in
// sectionOrder, so the result is deterministic when more than one section
// has a problem.
func checkStructure(raw map[string]any) error {
	known := make(map[string]bool, len(sectionOrder))
	for _, section := range sectionOrder {
		known[section] = true
	}

	var unknownSections []string
	for key := range raw {
		if !known[key] {
			unknownSections = append(unknownSections, key)
		}
	}
	if len(unknownSections) > 0 {
		sort.Strings(unknownSections)
		return fmt.Errorf("unknown configuration sections: %s", strings.Join(unknownSections, ", "))
	}

	for _, section := range sectionOrder {
		value, present := raw[section]
		if !present {
			continue
		}
		table, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("%s must be a TOML table", section)
		}

		if section == "metrics" {
			if _, ok := table["token"]; ok {
				return errors.New("metrics.token is forbidden; use metrics.token_env")
			}
		}

		allowed := make(map[string]bool, len(sectionKeys[section]))
		for _, key := range sectionKeys[section] {
			allowed[key] = true
		}
		var unknownKeys []string
		for key := range table {
			if !allowed[key] {
				unknownKeys = append(unknownKeys, key)
			}
		}
		if len(unknownKeys) > 0 {
			sort.Strings(unknownKeys)
			return fmt.Errorf("unknown %s keys: %s", section, strings.Join(unknownKeys, ", "))
		}
	}
	return nil
}

// wrongTypeError reformats the internal message BurntSushi/toml produces
// for a value that does not match its destination field's type into spec
// 5.2's fixed shape. err is not expected to fail the match, since
// checkStructure has already ruled out every other way toml.Decode can
// fail against tomlConfig; the fallback exists only so a future library
// change is reported instead of panicking.
func wrongTypeError(err error) error {
	if m := wrongTypeRe.FindStringSubmatch(err.Error()); m != nil {
		return fmt.Errorf("configuration file has a wrong type at %s: line %s: %s", m[2], m[1], m[3])
	}
	return fmt.Errorf("configuration file has a wrong type: %s", err)
}

// applyEnvironment overrides cfg with every CAPACITYROOM_ variable of spec 5.2 that
// is set to a non-empty value.
func applyEnvironment(cfg *Config, getenv func(string) string) error {
	if v := getenv("CAPACITYROOM_METRICS_URL"); v != "" {
		cfg.Metrics.URL = v
	}
	if v := getenv("CAPACITYROOM_CLUSTER"); v != "" {
		cfg.Cluster.Name = v
	}
	if v := getenv("CAPACITYROOM_CLUSTER_LABEL"); v != "" {
		cfg.Cluster.Label = v
	}
	if v := getenv("CAPACITYROOM_CLUSTER_FILTER"); v != "" {
		cfg.Cluster.FilterMode = v
	}
	if v := getenv("CAPACITYROOM_NAMESPACES"); v != "" {
		cfg.Cluster.Namespaces = splitList(v)
	}
	period, start, end := getenv("CAPACITYROOM_PERIOD"), getenv("CAPACITYROOM_START"), getenv("CAPACITYROOM_END")
	if period != "" {
		cfg.Analysis.Period = period
	}
	if start != "" {
		cfg.Analysis.Start = start
	}
	if end != "" {
		cfg.Analysis.End = end
	}
	if err := setWindow(&cfg.Analysis, period != "", start != "", end != "", errEnvWindow); err != nil {
		return err
	}
	if v := getenv("CAPACITYROOM_TIMEZONE"); v != "" {
		cfg.Analysis.Timezone = v
	}
	if v := getenv("CAPACITYROOM_NODE_AUTOSCALING"); v != "" {
		cfg.Analysis.NodeAutoscaling = v
	}
	if v := getenv("CAPACITYROOM_REPORT_PROFILE"); v != "" {
		cfg.Report.Profile = v
	}
	if v := getenv("CAPACITYROOM_OUTPUT_DIRECTORY"); v != "" {
		cfg.Report.OutputDirectory = v
	}
	if v := getenv("CAPACITYROOM_LANGUAGE"); v != "" {
		cfg.Report.Language = v
	}
	return nil
}

// applyFlags overrides cfg with every flag that was given.
func applyFlags(cfg *Config, flags Flags) error {
	if flags.MetricsURL != nil {
		cfg.Metrics.URL = *flags.MetricsURL
	}
	if flags.Cluster != nil {
		cfg.Cluster.Name = *flags.Cluster
	}
	if flags.ClusterLabel != nil {
		cfg.Cluster.Label = *flags.ClusterLabel
	}
	if flags.ClusterFilter != nil {
		cfg.Cluster.FilterMode = *flags.ClusterFilter
	}
	if flags.Namespaces != nil {
		cfg.Cluster.Namespaces = splitList(*flags.Namespaces)
	}
	if flags.Period != nil {
		cfg.Analysis.Period = *flags.Period
	}
	if flags.Start != nil {
		cfg.Analysis.Start = *flags.Start
	}
	if flags.End != nil {
		cfg.Analysis.End = *flags.End
	}
	if err := setWindow(&cfg.Analysis, flags.Period != nil, flags.Start != nil, flags.End != nil, errFlagWindow); err != nil {
		return err
	}
	if flags.Timezone != nil {
		cfg.Analysis.Timezone = *flags.Timezone
	}
	if flags.NodeAutoscaling != nil {
		cfg.Analysis.NodeAutoscaling = *flags.NodeAutoscaling
	}
	if flags.ReportProfile != nil {
		cfg.Report.Profile = *flags.ReportProfile
	}
	if flags.Language != nil {
		cfg.Report.Language = *flags.Language
	}
	if flags.Insecure {
		cfg.Metrics.TLSVerify = false
	}
	return nil
}

// splitList splits s on commas, trims each item, drops an empty item, keeps
// the input order, and keeps a repeated item once, at its first place.
func splitList(s string) []string {
	var result []string
	seen := make(map[string]bool)
	for _, item := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

// resolveToken picks the token: flagToken when given and not empty;
// otherwise the variable named by tokenEnv; otherwise empty. It fails when
// tokenEnv names a variable that is unset or empty and flagToken was not
// given, and it refuses either source of a token shorter than
// MinTokenLength characters, by the same rule CheckToken applies.
func resolveToken(getenv func(string) string, flagToken *string, tokenEnv string) (string, error) {
	if flagToken != nil && *flagToken != "" {
		if err := CheckToken(*flagToken); err != nil {
			return "", err
		}
		return *flagToken, nil
	}
	if tokenEnv == "" {
		return "", nil
	}
	value := getenv(tokenEnv)
	if value == "" {
		return "", fmt.Errorf("metrics.token_env names %s, which is unset or empty", tokenEnv)
	}
	if err := CheckToken(value); err != nil {
		return "", err
	}
	return value, nil
}

// RenderOptions are the settings that --from-snapshot reads: no file, the
// environment, then the flags. A value left empty means "as stored in the
// snapshot" for Profile.
type RenderOptions struct {
	Language        string
	Profile         string
	OutputDirectory string
}

// LoadRenderOptions builds RenderOptions from the environment, then the
// flags, and validates its language and its profile; an empty profile is
// left as stored.
func LoadRenderOptions(getenv func(string) string, flags Flags) (RenderOptions, error) {
	if getenv == nil {
		getenv = func(string) string { return "" }
	}

	opts := RenderOptions{Language: "pt-BR"}

	if v := getenv("CAPACITYROOM_LANGUAGE"); v != "" {
		opts.Language = v
	}
	if v := getenv("CAPACITYROOM_REPORT_PROFILE"); v != "" {
		opts.Profile = v
	}
	if v := getenv("CAPACITYROOM_OUTPUT_DIRECTORY"); v != "" {
		opts.OutputDirectory = v
	}

	if flags.Language != nil {
		opts.Language = *flags.Language
	}
	if flags.ReportProfile != nil {
		opts.Profile = *flags.ReportProfile
	}

	if opts.Language != "pt-BR" && opts.Language != "en" {
		return RenderOptions{}, errReportLanguage
	}
	switch opts.Profile {
	case "", "detailed", "executive":
	default:
		return RenderOptions{}, errReportProfile
	}
	return opts, nil
}
