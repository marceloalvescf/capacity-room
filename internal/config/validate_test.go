package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validConfig returns a Config that passes Validate(), so each table test
// below can start from a known-good baseline and change exactly one field.
func validConfig(t *testing.T) Config {
	t.Helper()

	dir := t.TempDir()
	caBundle := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(caBundle, []byte("cert"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg := Default()
	cfg.Metrics.URL = "https://prom.example.org"
	cfg.Metrics.CABundle = caBundle
	cfg.Cluster.Name = "prod"
	cfg.Cluster.Namespaces = []string{"default"}
	cfg.Report.OutputDirectory = dir

	if err := cfg.Validate(); err != nil {
		t.Fatalf("baseline Validate() error = %v, want nil", err)
	}
	return cfg
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		message string
	}{
		{"metrics.url missing", func(c *Config) { c.Metrics.URL = "" }, "metrics.url is required"},
		{"metrics.url not absolute", func(c *Config) { c.Metrics.URL = "ftp://prom.example.org" }, "metrics.url must be an absolute HTTP or HTTPS URL"},
		{"metrics.url has credentials", func(c *Config) { c.Metrics.URL = "https://user:pass@prom.example.org" }, "metrics.url must not embed credentials"},
		{"metrics.url has a query", func(c *Config) { c.Metrics.URL = "https://prom.example.org?x=1" }, "metrics.url must not include a query string"},
		{"metrics.url has a fragment", func(c *Config) { c.Metrics.URL = "https://prom.example.org#f" }, "metrics.url must not include a fragment"},
		{"metrics.endpoint_type invalid", func(c *Config) { c.Metrics.EndpointType = "bogus" }, "metrics.endpoint_type must be auto, prometheus, or grafana"},
		{"grafana url without datasource or proxy", func(c *Config) {
			c.Metrics.EndpointType = "grafana"
			c.Metrics.URL = "https://prom.example.org/api/v1"
		}, "metrics.url must identify a Grafana datasource proxy API root"},
		{"metrics.ca_bundle missing", func(c *Config) { c.Metrics.CABundle = "/no/such/file.pem" }, "metrics.ca_bundle must be an existing file"},
		{"metrics.timeout_seconds not positive", func(c *Config) { c.Metrics.TimeoutSeconds = 0 }, "metrics.timeout_seconds must be positive"},
		{"cluster.name blank", func(c *Config) { c.Cluster.Name = "" }, "cluster.name is required"},
		{"cluster.namespaces empty", func(c *Config) { c.Cluster.Namespaces = nil }, "cluster.namespaces must contain at least one non-empty namespace"},
		{"cluster.namespaces has a blank item", func(c *Config) { c.Cluster.Namespaces = []string{"a", ""} }, "cluster.namespaces must contain at least one non-empty namespace"},
		{"cluster.label invalid", func(c *Config) { c.Cluster.Label = "1bad" }, "cluster.label must be a valid Prometheus label name"},
		{"cluster.filter_mode invalid", func(c *Config) { c.Cluster.FilterMode = "bogus" }, "cluster.filter_mode must be required, auto, or none"},
		{"cluster.nodepool_label invalid", func(c *Config) { c.Cluster.NodepoolLabel = "1bad" }, "cluster.nodepool_label must be a valid Prometheus label name"},
		{"analysis.period invalid", func(c *Config) { c.Analysis.Period = "abc" }, "analysis.period must look like 30m, 4h, 24h, or 7d"},
		{"analysis.timezone unknown", func(c *Config) { c.Analysis.Timezone = "Not/AZone" }, "analysis.timezone is unknown"},
		{"analysis.node_autoscaling invalid", func(c *Config) { c.Analysis.NodeAutoscaling = "bogus" }, "analysis.node_autoscaling must be auto, enabled, or disabled"},
		{"threshold percent out of range", func(c *Config) { c.Thresholds.WarningPercent = 101 }, "threshold values must be within 0..100"},
		{"thresholds.bursty_gap_percent negative", func(c *Config) { c.Thresholds.BurstyGapPercent = -1 }, "thresholds.bursty_gap_percent must not be negative"},
		{"thresholds.hpa_replica_divergence negative", func(c *Config) { c.Thresholds.HPAReplicaDivergence = -1 }, "thresholds.hpa_replica_divergence must not be negative"},
		{"warning not below critical", func(c *Config) {
			c.Thresholds.WarningPercent = 90
			c.Thresholds.CriticalPercent = 85
		}, "thresholds.warning_percent must be lower than critical_percent"},
		{"report.profile invalid", func(c *Config) { c.Report.Profile = "bogus" }, "report.profile must be detailed or executive"},
		{"report.min_severity invalid", func(c *Config) { c.Report.MinSeverity = "bogus" }, "report.min_severity must be all, attention, or critical"},
		{"report.language invalid", func(c *Config) { c.Report.Language = "fr" }, "report.language must be pt-BR or en"},
		{"report.output_directory missing", func(c *Config) { c.Report.OutputDirectory = "/no/such/directory" }, "report.output_directory must exist and be writable"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig(t)
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want error %q", tt.message)
			}
			if err.Error() != tt.message {
				t.Fatalf("Validate() = %q, want %q", err.Error(), tt.message)
			}
		})
	}
}

func TestValidateReportsFirstFailure(t *testing.T) {
	cfg := validConfig(t)
	cfg.Cluster.Name = ""      // rule 10 in the table
	cfg.Report.Language = "fr" // rule 24 in the table, comes later

	err := cfg.Validate()
	if err == nil {
		t.Fatalf("Validate() = nil, want error")
	}
	if want := "cluster.name is required"; err.Error() != want {
		t.Fatalf("Validate() = %q, want %q (the earlier rule)", err.Error(), want)
	}
}

func TestValidateAcceptsBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"percent zero", func(c *Config) { c.Thresholds.WarningPercent = 0 }},
		{"percent one hundred", func(c *Config) { c.Thresholds.CriticalPercent = 100 }},
		{"period minimum", func(c *Config) { c.Analysis.Period = "1m" }},
		{"timeout minimum", func(c *Config) { c.Metrics.TimeoutSeconds = 0.001 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig(t)
			tt.mutate(&cfg)

			if err := cfg.Validate(); err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func TestValidateGrafanaPath(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		message string // empty means valid
	}{
		{"datasource path", "https://grafana.example.org/api/datasources/uid/x", ""},
		{"proxy substring path", "https://grafana.example.org/myproxything", ""},
		{"neither", "https://grafana.example.org/prom", "metrics.url must identify a Grafana datasource proxy API root"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig(t)
			cfg.Metrics.EndpointType = "grafana"
			cfg.Metrics.URL = tt.url

			err := cfg.Validate()
			if tt.message == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil || err.Error() != tt.message {
				t.Fatalf("Validate() = %v, want %q", err, tt.message)
			}
		})
	}
}

// TestMessagesNeverRepeatTheAddress exercises each metrics.url rule with a
// URL built from the secrets of
// "https://user:hunter2@h.example/?token=abc#f" and checks that the
// returned message, which is always a fixed sentence, never contains any of
// them.
func TestMessagesNeverRepeatTheAddress(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{"not absolute", "ftp://user:hunter2@h.example/?token=abc#f"},
		{"credentials", "https://user:hunter2@h.example/"},
		{"query", "https://h.example/?token=abc"},
		{"fragment", "https://h.example/#f"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig(t)
			cfg.Metrics.URL = tt.url

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() = nil, want an error")
			}
			for _, secret := range []string{"hunter2", "abc", "h.example"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("Validate() error = %q, must not contain %q", err.Error(), secret)
				}
			}
		})
	}
}

func TestWritableCheckLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()

	if err := checkWritableDir(dir); err != nil {
		t.Fatalf("checkWritableDir() error = %v, want nil", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("dir has %d entries after checkWritableDir, want 0: %v", len(entries), entries)
	}
}

func TestWritableCheckReportsAProbeItCannotRemove(t *testing.T) {
	var probePath string
	original := removeFile
	removeFile = func(name string) error {
		probePath = name
		return errors.New("permission denied")
	}
	t.Cleanup(func() { removeFile = original })

	dir := t.TempDir()
	err := checkWritableDir(dir)
	if err == nil {
		t.Fatalf("checkWritableDir() = nil, want an error")
	}
	if probePath == "" {
		t.Fatalf("removeFile was never called")
	}
	want := "report.output_directory holds a probe file that could not be removed: " + probePath + ": permission denied"
	if err.Error() != want {
		t.Fatalf("checkWritableDir() = %q, want %q", err.Error(), want)
	}
}

func TestWritableCheckRefusesAFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	err := checkWritableDir(path)
	want := "report.output_directory must exist and be writable"
	if err == nil || err.Error() != want {
		t.Fatalf("checkWritableDir() = %v, want %q", err, want)
	}
}

func TestWritableCheckRefusesAReadOnlyFolder(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	err := checkWritableDir(dir)
	want := "report.output_directory must exist and be writable"
	if err == nil || err.Error() != want {
		t.Fatalf("checkWritableDir() = %v, want %q", err, want)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		t.Fatalf("ReadDir: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("dir has %d entries after checkWritableDir, want 0: %v", len(entries), entries)
	}
}
