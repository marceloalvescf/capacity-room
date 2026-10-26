package config

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// noEnv answers every environment lookup with "not set".
func noEnv(string) string { return "" }

// fullConfig is the Config that testdata/full.toml describes: every key set
// to a value that is not its default.
func fullConfig() Config {
	return Config{
		Metrics: Metrics{
			URL:            "https://prom.example.org/api/v1",
			EndpointType:   "prometheus",
			TokenEnv:       "CAPACITYROOM_FULL_TOKEN",
			TLSVerify:      false,
			CABundle:       "testdata/ca.pem",
			TimeoutSeconds: 45,
		},
		Cluster: Cluster{
			Name:          "prod",
			Namespaces:    []string{"payments", "billing"},
			Label:         "k8s_cluster",
			FilterMode:    "auto",
			NodepoolLabel: "nodepool",
		},
		Analysis: Analysis{
			Period:             "7d",
			Timezone:           "UTC",
			NodeAutoscaling:    "enabled",
			ExcludedContainers: []string{"istio-proxy"},
		},
		Thresholds: Thresholds{
			WarningPercent:           60,
			CriticalPercent:          90,
			LowUtilizationPercent:    20,
			AutoscalerNearMaxPercent: 75,
			NodepoolSpreadPoints:     15,
			BurstyGapPercent:         50,
			HPAReplicaDivergence:     2,
		},
		Report: Report{
			Profile:         "executive",
			Language:        "en",
			Organization:    "Acme",
			Classification:  "internal",
			Author:          "Test Author",
			OutputDirectory: "testdata",
		},
	}
}

// getenvWithToken answers CAPACITYROOM_FULL_TOKEN (which full.toml's token_env
// names) and, optionally, one more variable, "" for everything else.
func getenvWithToken(extraKey, extraVal string) func(string) string {
	return func(key string) string {
		switch key {
		case "CAPACITYROOM_FULL_TOKEN":
			return "s3cr3t12"
		case extraKey:
			return extraVal
		default:
			return ""
		}
	}
}

// writeFile writes a TOML fixture into a fresh temp directory and returns
// its path.
func writeFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestLoadReadsEveryKey(t *testing.T) {
	loaded, err := Load(Source{File: "testdata/full.toml", Getenv: getenvWithToken("", "")})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(loaded.Config, fullConfig()) {
		t.Fatalf("Load().Config = %+v, want %+v", loaded.Config, fullConfig())
	}
}

func TestLoadMinimalFile(t *testing.T) {
	loaded, err := Load(Source{File: "testdata/minimal.toml", Getenv: noEnv})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := Default()
	want.Metrics.URL = "https://prom.example.org"
	want.Cluster.Name = "prod"
	want.Cluster.Namespaces = []string{"default"}

	if !reflect.DeepEqual(loaded.Config, want) {
		t.Fatalf("Load().Config = %+v, want %+v", loaded.Config, want)
	}
}

func TestLoadEnvironmentOverridesFile(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		envVal func(t *testing.T) string
		mutate func(*Config, string)
	}{
		{"metrics.url", "CAPACITYROOM_METRICS_URL", fixedVal("https://override.example/metrics"), func(c *Config, v string) { c.Metrics.URL = v }},
		{"metrics.endpoint_type", "CAPACITYROOM_ENDPOINT_TYPE", fixedVal("auto"), func(c *Config, v string) { c.Metrics.EndpointType = v }},
		{"cluster.name", "CAPACITYROOM_CLUSTER", fixedVal("staging"), func(c *Config, v string) { c.Cluster.Name = v }},
		{"cluster.label", "CAPACITYROOM_CLUSTER_LABEL", fixedVal("team"), func(c *Config, v string) { c.Cluster.Label = v }},
		{"cluster.filter_mode", "CAPACITYROOM_CLUSTER_FILTER", fixedVal("none"), func(c *Config, v string) { c.Cluster.FilterMode = v }},
		{"cluster.namespaces", "CAPACITYROOM_NAMESPACES", fixedVal("alpha,beta"), func(c *Config, v string) { c.Cluster.Namespaces = splitList(v) }},
		{"analysis.period", "CAPACITYROOM_PERIOD", fixedVal("4h"), func(c *Config, v string) { c.Analysis.Period = v }},
		{"analysis.timezone", "CAPACITYROOM_TIMEZONE", fixedVal("Europe/Lisbon"), func(c *Config, v string) { c.Analysis.Timezone = v }},
		{"analysis.node_autoscaling", "CAPACITYROOM_NODE_AUTOSCALING", fixedVal("disabled"), func(c *Config, v string) { c.Analysis.NodeAutoscaling = v }},
		{"report.profile", "CAPACITYROOM_REPORT_PROFILE", fixedVal("detailed"), func(c *Config, v string) { c.Report.Profile = v }},
		{"report.output_directory", "CAPACITYROOM_OUTPUT_DIRECTORY", func(t *testing.T) string { return t.TempDir() }, func(c *Config, v string) { c.Report.OutputDirectory = v }},
		{"report.language", "CAPACITYROOM_LANGUAGE", fixedVal("pt-BR"), func(c *Config, v string) { c.Report.Language = v }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val := tt.envVal(t)
			loaded, err := Load(Source{File: "testdata/full.toml", Getenv: getenvWithToken(tt.envVar, val)})
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			want := fullConfig()
			tt.mutate(&want, val)

			if !reflect.DeepEqual(loaded.Config, want) {
				t.Fatalf("Load().Config = %+v, want %+v", loaded.Config, want)
			}
		})
	}
}

func fixedVal(s string) func(*testing.T) string {
	return func(*testing.T) string { return s }
}

func TestLoadFlagOverridesEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		envVal  string
		flagVal string
		setFlag func(*Flags, string)
		mutate  func(*Config, string)
	}{
		{"metrics.url", "CAPACITYROOM_METRICS_URL", "https://env.example/metrics", "https://flag.example/metrics",
			func(f *Flags, v string) { f.MetricsURL = &v }, func(c *Config, v string) { c.Metrics.URL = v }},
		{"metrics.endpoint_type", "CAPACITYROOM_ENDPOINT_TYPE", "grafana", "auto",
			func(f *Flags, v string) { f.EndpointType = &v }, func(c *Config, v string) { c.Metrics.EndpointType = v }},
		{"cluster.name", "CAPACITYROOM_CLUSTER", "envcluster", "flagcluster",
			func(f *Flags, v string) { f.Cluster = &v }, func(c *Config, v string) { c.Cluster.Name = v }},
		{"cluster.label", "CAPACITYROOM_CLUSTER_LABEL", "envlabel", "flaglabel",
			func(f *Flags, v string) { f.ClusterLabel = &v }, func(c *Config, v string) { c.Cluster.Label = v }},
		{"cluster.filter_mode", "CAPACITYROOM_CLUSTER_FILTER", "none", "required",
			func(f *Flags, v string) { f.ClusterFilter = &v }, func(c *Config, v string) { c.Cluster.FilterMode = v }},
		{"cluster.namespaces", "CAPACITYROOM_NAMESPACES", "envns1,envns2", "flagns1,flagns2",
			func(f *Flags, v string) { f.Namespaces = &v }, func(c *Config, v string) { c.Cluster.Namespaces = splitList(v) }},
		{"analysis.period", "CAPACITYROOM_PERIOD", "1h", "2h",
			func(f *Flags, v string) { f.Period = &v }, func(c *Config, v string) { c.Analysis.Period = v }},
		{"analysis.timezone", "CAPACITYROOM_TIMEZONE", "Asia/Tokyo", "Europe/Lisbon",
			func(f *Flags, v string) { f.Timezone = &v }, func(c *Config, v string) { c.Analysis.Timezone = v }},
		{"analysis.node_autoscaling", "CAPACITYROOM_NODE_AUTOSCALING", "auto", "disabled",
			func(f *Flags, v string) { f.NodeAutoscaling = &v }, func(c *Config, v string) { c.Analysis.NodeAutoscaling = v }},
		// report.profile has only two valid values; the file's own value
		// ("executive") is reused for the flag so that reaching it proves
		// the flag won rather than the file surviving untouched, since the
		// intervening environment value ("detailed") differs from both.
		{"report.profile", "CAPACITYROOM_REPORT_PROFILE", "detailed", "executive",
			func(f *Flags, v string) { f.ReportProfile = &v }, func(c *Config, v string) { c.Report.Profile = v }},
		{"report.language", "CAPACITYROOM_LANGUAGE", "pt-BR", "en",
			func(f *Flags, v string) { f.Language = &v }, func(c *Config, v string) { c.Report.Language = v }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var flags Flags
			tt.setFlag(&flags, tt.flagVal)

			loaded, err := Load(Source{
				File:   "testdata/full.toml",
				Getenv: getenvWithToken(tt.envVar, tt.envVal),
				Flags:  flags,
			})
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}

			want := fullConfig()
			tt.mutate(&want, tt.flagVal)

			if !reflect.DeepEqual(loaded.Config, want) {
				t.Fatalf("Load().Config = %+v, want %+v", loaded.Config, want)
			}
		})
	}
}

func TestLoadEmptyVariableIsNotSet(t *testing.T) {
	getenv := func(key string) string {
		if key == "CAPACITYROOM_FULL_TOKEN" {
			return "s3cr3t12"
		}
		if key == "CAPACITYROOM_CLUSTER" {
			return ""
		}
		return ""
	}

	loaded, err := Load(Source{File: "testdata/full.toml", Getenv: getenv})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Config.Cluster.Name != "prod" {
		t.Fatalf("Cluster.Name = %q, want %q (the file's value)", loaded.Config.Cluster.Name, "prod")
	}
}

func TestLoadSplitsNamespaces(t *testing.T) {
	tests := []struct {
		input   string
		want    []string
		wantErr string // empty means no error expected
	}{
		{"a,b", []string{"a", "b"}, ""},
		{" a , b ", []string{"a", "b"}, ""},
		{"a,,b", []string{"a", "b"}, ""},
		{"a,b,a", []string{"a", "b"}, ""},
		{",", nil, "cluster.namespaces must contain at least one non-empty namespace"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			getenv := func(key string) string {
				if key == "CAPACITYROOM_NAMESPACES" {
					return tt.input
				}
				return ""
			}

			loaded, err := Load(Source{File: "testdata/minimal.toml", Getenv: getenv})
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("Load() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Load() error = %v, want nil", err)
			}
			if !reflect.DeepEqual(loaded.Config.Cluster.Namespaces, tt.want) {
				t.Fatalf("Namespaces = %v, want %v", loaded.Config.Cluster.Namespaces, tt.want)
			}
		})
	}
}

func TestLoadInsecureTurnsVerificationOff(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
tls_verify = true

[cluster]
name = "prod"
namespaces = ["default"]
`)

	loaded, err := Load(Source{File: path, Getenv: noEnv, Flags: Flags{Insecure: true}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Config.Metrics.TLSVerify {
		t.Fatalf("Metrics.TLSVerify = true, want false")
	}
}

func TestLoadTokenFromVariable(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
token_env = "CAPACITYROOM_TOKEN"

[cluster]
name = "prod"
namespaces = ["default"]
`)
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TOKEN" {
			return "abcdefgh"
		}
		return ""
	}

	loaded, err := Load(Source{File: path, Getenv: getenv})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Token != "abcdefgh" {
		t.Fatalf("Token = %q, want %q", loaded.Token, "abcdefgh")
	}
}

func TestLoadTokenFlagWins(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
token_env = "CAPACITYROOM_TOKEN"

[cluster]
name = "prod"
namespaces = ["default"]
`)
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TOKEN" {
			return "abcdefgh"
		}
		return ""
	}
	token := "xyzxyzxy"

	loaded, err := Load(Source{File: path, Getenv: getenv, Flags: Flags{Token: &token}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Token != "xyzxyzxy" {
		t.Fatalf("Token = %q, want %q", loaded.Token, "xyzxyzxy")
	}
}

// TestLoadRejectsATooShortTokenVariable proves resolveToken applies the
// same MinTokenLength rule to a token read from metrics.token_env as
// cli.Run applies, before anything else, to one read from --token.
func TestLoadRejectsATooShortTokenVariable(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
token_env = "CAPACITYROOM_TOKEN"

[cluster]
name = "prod"
namespaces = ["default"]
`)
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TOKEN" {
			return "abc"
		}
		return ""
	}

	_, err := Load(Source{File: path, Getenv: getenv})
	want := "the token must hold at least 8 characters"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadTokenVariableUnset(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
token_env = "CAPACITYROOM_TOKEN"

[cluster]
name = "prod"
namespaces = ["default"]
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "metrics.token_env names CAPACITYROOM_TOKEN, which is unset or empty"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadTokenVariableUnsetWithFlag(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
token_env = "CAPACITYROOM_TOKEN"

[cluster]
name = "prod"
namespaces = ["default"]
`)
	token := "xyzxyzxy"

	loaded, err := Load(Source{File: path, Getenv: noEnv, Flags: Flags{Token: &token}})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Token != "xyzxyzxy" {
		t.Fatalf("Token = %q, want %q", loaded.Token, "xyzxyzxy")
	}
}

func TestLoadNoTokenAtAll(t *testing.T) {
	loaded, err := Load(Source{File: "testdata/minimal.toml", Getenv: noEnv})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Token != "" {
		t.Fatalf("Token = %q, want empty", loaded.Token)
	}
}

func TestLoadRejectsTokenInFile(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
token = "abc"

[cluster]
name = "prod"
namespaces = ["default"]
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "metrics.token is forbidden; use metrics.token_env"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
	if strings.Contains(err.Error(), "abc") {
		t.Fatalf("Load() error = %q, must not contain the token value", err.Error())
	}
}

func TestLoadRejectsSectionNotTable(t *testing.T) {
	path := writeFile(t, `
metrics = "wrong"

[cluster]
name = "prod"
namespaces = ["default"]
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "metrics must be a TOML table"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadRejectsUnknownSection(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[extra]
foo = 1
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "unknown configuration sections: extra"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadRejectsUnknownKey(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
colour = 1

[cluster]
name = "prod"
namespaces = ["default"]
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "unknown metrics keys: colour"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadRejectsTheMinimumSeverityKey(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[report]
min_severity = "all"
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "unknown report keys: min_severity"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadRejectsOldPackageKey(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[report]
graphite_package = "@local/graphite-report:1.0.0"
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	want := "unknown report keys: graphite_package"
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadReportsLineAndColumn(t *testing.T) {
	_, err := Load(Source{File: "testdata/broken.toml", Getenv: noEnv})
	if err == nil {
		t.Fatalf("Load() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "line 3") {
		t.Fatalf("Load() error = %q, want it to contain %q", err.Error(), "line 3")
	}

	m := regexp.MustCompile(`column (\d+)`).FindStringSubmatch(err.Error())
	if m == nil {
		t.Fatalf("Load() error = %q, want it to name a column", err.Error())
	}
	col, convErr := strconv.Atoi(m[1])
	if convErr != nil || col <= 0 {
		t.Fatalf("Load() error names column %q, want a number above zero", m[1])
	}
}

func TestLoadReportsWrongType(t *testing.T) {
	path := writeFile(t, `
[metrics]
url = "https://prom.example.org"
timeout_seconds = "fast"

[cluster]
name = "prod"
namespaces = ["default"]
`)

	_, err := Load(Source{File: path, Getenv: noEnv})
	if err == nil || !strings.Contains(err.Error(), "metrics.timeout_seconds") {
		t.Fatalf("Load() error = %v, want it to name metrics.timeout_seconds", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.toml")

	_, err := Load(Source{File: path, Getenv: noEnv})
	if err == nil {
		t.Fatalf("Load() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "configuration file cannot be read") || !strings.Contains(err.Error(), path) {
		t.Fatalf("Load() error = %q, want it to name %q", err.Error(), path)
	}
}

func TestLoadRenderOptions(t *testing.T) {
	tests := []struct {
		name    string
		getenv  func(string) string
		flags   Flags
		want    RenderOptions
		wantErr string
	}{
		{"nothing set", noEnv, Flags{}, RenderOptions{Language: "pt-BR"}, ""},
		{"environment sets the language", func(key string) string {
			if key == "CAPACITYROOM_LANGUAGE" {
				return "en"
			}
			return ""
		}, Flags{}, RenderOptions{Language: "en"}, ""},
		{"flag wins over environment", func(key string) string {
			if key == "CAPACITYROOM_LANGUAGE" {
				return "en"
			}
			return ""
		}, Flags{Language: strPtr("pt-BR")}, RenderOptions{Language: "pt-BR"}, ""},
		{"invalid language", noEnv, Flags{Language: strPtr("fr")}, RenderOptions{}, "report.language must be pt-BR or en"},
		{"invalid profile", noEnv, Flags{ReportProfile: strPtr("Executive")}, RenderOptions{}, "report.profile must be detailed or executive"},
		{"invalid profile from the environment", func(key string) string {
			if key == "CAPACITYROOM_REPORT_PROFILE" {
				return "weird"
			}
			return ""
		}, Flags{}, RenderOptions{}, "report.profile must be detailed or executive"},
		{"valid profile", noEnv, Flags{ReportProfile: strPtr("executive")}, RenderOptions{Language: "pt-BR", Profile: "executive"}, ""},
		{"empty profile kept as stored", noEnv, Flags{ReportProfile: strPtr("")}, RenderOptions{Language: "pt-BR"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadRenderOptions(tt.getenv, tt.flags)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("LoadRenderOptions() error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LoadRenderOptions() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Fatalf("LoadRenderOptions() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// TestLoadKeepsExplicitZeroValues checks that a key the file sets to the
// zero value of its type wins over the (non-zero, for most of these)
// default, rather than being mistaken for "not set". Each row is
// testdata/minimal.toml plus exactly one extra line.
func TestLoadKeepsExplicitZeroValues(t *testing.T) {
	tests := []struct {
		name    string
		content string
		check   func(Config) bool
	}{
		{
			name: "metrics.tls_verify = false",
			content: `
[metrics]
url = "https://prom.example.org"
tls_verify = false

[cluster]
name = "prod"
namespaces = ["default"]
`,
			check: func(c Config) bool { return !c.Metrics.TLSVerify },
		},
		{
			name: "thresholds.bursty_gap_percent = 0",
			content: `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[thresholds]
bursty_gap_percent = 0
`,
			check: func(c Config) bool { return c.Thresholds.BurstyGapPercent == 0 },
		},
		{
			name: "thresholds.hpa_replica_divergence = 0",
			content: `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[thresholds]
hpa_replica_divergence = 0
`,
			check: func(c Config) bool { return c.Thresholds.HPAReplicaDivergence == 0 },
		},
		{
			name: "thresholds.low_utilization_percent = 0",
			content: `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[thresholds]
low_utilization_percent = 0
`,
			check: func(c Config) bool { return c.Thresholds.LowUtilizationPercent == 0 },
		},
		{
			name: "analysis.excluded_containers = []",
			content: `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[analysis]
excluded_containers = []
`,
			check: func(c Config) bool { return len(c.Analysis.ExcludedContainers) == 0 },
		},
		{
			name: `report.organization = ""`,
			content: `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]

[report]
organization = ""
`,
			check: func(c Config) bool { return c.Report.Organization == "" },
		},
		{
			name: `cluster.nodepool_label = ""`,
			content: `
[metrics]
url = "https://prom.example.org"

[cluster]
name = "prod"
namespaces = ["default"]
nodepool_label = ""
`,
			check: func(c Config) bool { return c.Cluster.NodepoolLabel == "" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeFile(t, tt.content)

			loaded, err := Load(Source{File: path, Getenv: noEnv})
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !tt.check(loaded.Config) {
				t.Fatalf("Load().Config = %+v, did not keep the explicit zero value", loaded.Config)
			}
		})
	}
}
