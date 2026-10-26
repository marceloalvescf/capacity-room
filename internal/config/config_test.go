package config

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDefaultMatchesSpec(t *testing.T) {
	got := Default()
	want := Config{
		Metrics: Metrics{
			URL:            "",
			TokenEnv:       "",
			TLSVerify:      true,
			CABundle:       "",
			TimeoutSeconds: 90,
		},
		Cluster: Cluster{
			Name:          "",
			Namespaces:    nil,
			Label:         "cluster",
			FilterMode:    "required",
			NodepoolLabel: "",
		},
		Analysis: Analysis{
			Period:             "24h",
			Timezone:           "America/Sao_Paulo",
			NodeAutoscaling:    "auto",
			ExcludedContainers: nil,
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
			Language:        "pt-BR",
			Organization:    "",
			Classification:  "",
			Author:          "",
			OutputDirectory: ".",
		},
	}

	if got.Metrics != want.Metrics {
		t.Errorf("Default().Metrics = %+v, want %+v", got.Metrics, want.Metrics)
	}
	if got.Cluster.Name != want.Cluster.Name || len(got.Cluster.Namespaces) != 0 ||
		got.Cluster.Label != want.Cluster.Label || got.Cluster.FilterMode != want.Cluster.FilterMode ||
		got.Cluster.NodepoolLabel != want.Cluster.NodepoolLabel {
		t.Errorf("Default().Cluster = %+v, want %+v", got.Cluster, want.Cluster)
	}
	if got.Analysis.Period != want.Analysis.Period || got.Analysis.Timezone != want.Analysis.Timezone ||
		got.Analysis.NodeAutoscaling != want.Analysis.NodeAutoscaling || len(got.Analysis.ExcludedContainers) != 0 {
		t.Errorf("Default().Analysis = %+v, want %+v", got.Analysis, want.Analysis)
	}
	if got.Thresholds != want.Thresholds {
		t.Errorf("Default().Thresholds = %+v, want %+v", got.Thresholds, want.Thresholds)
	}
	if got.Report != want.Report {
		t.Errorf("Default().Report = %+v, want %+v", got.Report, want.Report)
	}
}

func TestDuration(t *testing.T) {
	tests := []struct {
		period string
		want   time.Duration
	}{
		{"30m", 30 * time.Minute},
		{"24h", 24 * time.Hour},
		{"7d", 168 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.period, func(t *testing.T) {
			a := Analysis{Period: tt.period}
			got, err := a.Duration()
			if err != nil {
				t.Fatalf("Duration() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDurationRejectsAMalformedPeriod(t *testing.T) {
	a := Analysis{Period: "not-a-period"}
	if _, err := a.Duration(); err == nil {
		t.Fatalf("Duration() error = nil, want %q", errPeriodShape)
	} else if err.Error() != errPeriodShape.Error() {
		t.Fatalf("Duration() error = %q, want %q", err.Error(), errPeriodShape.Error())
	}
}

// TestDurationRejectsATooLongPeriod proves the only limit on a period's
// magnitude is what a time.Duration (an int64 count of nanoseconds) can
// hold, and that a period that does not fit is refused rather than
// silently wrapped into a negative value: 106752d is the first number of
// days that overflows, and 106751d is the last one that does not.
func TestDurationRejectsATooLongPeriod(t *testing.T) {
	tests := []string{"999999999d", "9223372036854775807m", "106752d"}
	for _, period := range tests {
		t.Run(period, func(t *testing.T) {
			a := Analysis{Period: period}
			d, err := a.Duration()
			if err == nil {
				t.Fatalf("Duration() = %v, err nil, want %q", d, errPeriodTooLong)
			}
			if err.Error() != errPeriodTooLong.Error() {
				t.Fatalf("Duration() error = %q, want %q", err.Error(), errPeriodTooLong.Error())
			}
			if d != 0 {
				t.Fatalf("Duration() = %v, want 0", d)
			}
		})
	}
}

// TestDurationAcceptsTheLongestDayPeriod proves 106751d, the day count
// immediately below the one TestDurationRejectsATooLongPeriod proves
// overflows, is accepted, with the duration it names.
func TestDurationAcceptsTheLongestDayPeriod(t *testing.T) {
	a := Analysis{Period: "106751d"}
	got, err := a.Duration()
	if err != nil {
		t.Fatalf("Duration() error = %v, want nil", err)
	}
	want := 106751 * 24 * time.Hour
	if got != want {
		t.Fatalf("Duration() = %v, want %v", got, want)
	}
}

func TestCheckToken(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{"empty means not given", "", false},
		{"seven characters", "1234567", true},
		{"eight characters", "12345678", false},
		{"seven accented characters", "áéíóúçã", true},
		{"eight accented characters", "áéíóúçãõ", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckToken(tt.token)
			if tt.wantErr && err == nil {
				t.Fatalf("CheckToken(%q) = nil, want an error", tt.token)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("CheckToken(%q) = %v, want nil", tt.token, err)
			}
			if tt.wantErr && err.Error() != "the token must hold at least 8 characters" {
				t.Fatalf("CheckToken(%q) = %q, want %q", tt.token, err.Error(), "the token must hold at least 8 characters")
			}
		})
	}
}

// TestLocationUsesEmbeddedZones sets ZONEINFO to a folder that does not
// exist. On this machine the system also carries zoneinfo files under
// /usr/share/zoneinfo, which time.LoadLocation would find regardless of the
// embedded database, so this test alone does not prove the binary carries
// its own copy; TestConfigImportsEmbeddedZones checks that statically.
func TestLocationUsesEmbeddedZones(t *testing.T) {
	t.Setenv("ZONEINFO", filepath.Join(t.TempDir(), "does-not-exist"))

	a := Analysis{Timezone: "America/Sao_Paulo"}
	loc, err := a.Location()
	if err != nil {
		t.Fatalf("Location() error = %v", err)
	}
	if loc == nil || loc.String() != "America/Sao_Paulo" {
		t.Fatalf("Location() = %v, want America/Sao_Paulo", loc)
	}
}

// TestConfigImportsEmbeddedZones parses every non-test Go file of this
// package and fails unless one of them imports time/tzdata, so the built
// binary always carries the IANA database regardless of the host.
func TestConfigImportsEmbeddedZones(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	found := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", name, err)
		}
		for _, imp := range file.Imports {
			path, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if path == "time/tzdata" {
				found = true
			}
		}
	}

	if !found {
		t.Fatalf("no non-test file of internal/config imports time/tzdata")
	}
}
