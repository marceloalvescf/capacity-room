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
			EndpointType:   "auto",
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
			MinSeverity:     "all",
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
