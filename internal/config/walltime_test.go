package config

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("LoadLocation(%q): %v", name, err)
	}
	return loc
}

func TestWallTimeShape(t *testing.T) {
	got, err := ParseWallTime("2026-09-01 00:00", time.UTC)
	if err != nil {
		t.Fatalf("ParseWallTime() error = %v", err)
	}
	if want := time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("ParseWallTime() = %v, want %v", got, want)
	}
	for _, text := range []string{
		" 2026-09-01 00:00", "2026-9-01 00:00", "2026-09-01T00:00",
		"2026-09-01 24:00", "2026-02-30 00:00", "2026-09-01 00:00:00", "",
	} {
		if _, err := ParseWallTime(text, time.UTC); !errors.Is(err, ErrWallTimeShape) {
			t.Errorf("ParseWallTime(%q) error = %v, want ErrWallTimeShape", text, err)
		}
	}
}

func TestWallTimeInAZone(t *testing.T) {
	got, err := ParseWallTime("2026-09-01 00:00", mustLoc(t, "America/Sao_Paulo"))
	if err != nil {
		t.Fatalf("ParseWallTime() error = %v", err)
	}
	if want := time.Date(2026, time.September, 1, 3, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("ParseWallTime() = %v, want %v", got, want)
	}
}

func TestWallTimeInAGap(t *testing.T) {
	_, err := ParseWallTime("2026-03-08 02:30", mustLoc(t, "America/New_York"))
	if !errors.Is(err, ErrWallTimeAbsent) {
		t.Fatalf("error = %v, want ErrWallTimeAbsent", err)
	}
}

func TestWallTimeInAnOverlap(t *testing.T) {
	ny := mustLoc(t, "America/New_York")
	if _, err := ParseWallTime("2026-11-01 01:30", ny); !errors.Is(err, ErrWallTimeTwice) {
		t.Fatalf("error = %v, want ErrWallTimeTwice", err)
	}
	for _, text := range []string{"2026-11-01 00:59", "2026-11-01 02:00"} {
		if _, err := ParseWallTime(text, ny); err != nil {
			t.Errorf("ParseWallTime(%q) error = %v", text, err)
		}
	}
}

func rangeFile(analysis string) string {
	return "[metrics]\nurl = \"https://prom.example.org\"\n[cluster]\nname = \"prod\"\nnamespaces = [\"default\"]\n[analysis]\n" + analysis + "\n"
}

func rangeEnv(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

func TestRangeFromTheFile(t *testing.T) {
	loaded, err := Load(Source{File: writeFile(t, rangeFile("start = \"2026-09-01 00:00\"\nend = \"2026-10-01 00:00\"")), Getenv: noEnv})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	a := loaded.Config.Analysis
	if a.Period != "" || a.Start != "2026-09-01 00:00" || a.End != "2026-10-01 00:00" {
		t.Fatalf("Analysis = %+v", a)
	}
	start, end, err := a.Range()
	if err != nil {
		t.Fatalf("Range() error = %v", err)
	}
	if !start.Equal(time.Date(2026, 9, 1, 3, 0, 0, 0, time.UTC)) || !end.Equal(time.Date(2026, 10, 1, 3, 0, 0, 0, time.UTC)) {
		t.Fatalf("Range() = %v, %v", start, end)
	}
}

func TestRangeReplacesALowerPeriod(t *testing.T) {
	loaded, err := Load(Source{
		File:   writeFile(t, rangeFile("period = \"7d\"")),
		Getenv: noEnv,
		Flags:  Flags{Start: strPtr("2026-09-01 00:00"), End: strPtr("2026-10-01 00:00")},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	a := loaded.Config.Analysis
	if a.Period != "" || a.Start != "2026-09-01 00:00" || a.End != "2026-10-01 00:00" {
		t.Fatalf("Analysis = %+v", a)
	}
}

func TestPeriodReplacesALowerRange(t *testing.T) {
	loaded, err := Load(Source{
		File:   writeFile(t, rangeFile("start = \"2026-09-01 00:00\"\nend = \"2026-10-01 00:00\"")),
		Getenv: rangeEnv(map[string]string{"KCR_PERIOD": "36h"}),
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	a := loaded.Config.Analysis
	if a.Period != "36h" || a.Start != "" || a.End != "" {
		t.Fatalf("Analysis = %+v", a)
	}
}

func TestOneSourceSetsBoth(t *testing.T) {
	tests := []struct {
		name string
		src  Source
		want string
	}{
		{"file", Source{File: writeFile(t, rangeFile("period = \"7d\"\nstart = \"2026-09-01 00:00\"")), Getenv: noEnv},
			"the configuration file sets both analysis.period and a range"},
		{"environment", Source{File: writeFile(t, rangeFile("")), Getenv: rangeEnv(map[string]string{"KCR_PERIOD": "7d", "KCR_START": "2026-09-01 00:00"})},
			"the environment sets both KCR_PERIOD and KCR_START or KCR_END"},
		{"flags", Source{File: writeFile(t, rangeFile("")), Getenv: noEnv, Flags: Flags{Period: strPtr("7d"), Start: strPtr("2026-09-01 00:00")}},
			"the flags set both --period and --start or --end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.src)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func loadRange(t *testing.T, flags Flags) error {
	t.Helper()
	_, err := Load(Source{File: writeFile(t, rangeFile("")), Getenv: noEnv, Flags: flags})
	return err
}

func TestRangeNeedsBothEnds(t *testing.T) {
	err := loadRange(t, Flags{Start: strPtr("2026-09-01 00:00")})
	if err == nil || err.Error() != "analysis.start and analysis.end must be set together" {
		t.Fatalf("error = %v", err)
	}
}

func TestRangeOrder(t *testing.T) {
	for _, f := range []Flags{
		{Start: strPtr("2026-10-01 00:00"), End: strPtr("2026-09-01 00:00")},
		{Start: strPtr("2026-10-01 00:00"), End: strPtr("2026-10-01 00:00")},
	} {
		err := loadRange(t, f)
		if err == nil || err.Error() != "analysis.start must come before analysis.end" {
			t.Fatalf("error = %v", err)
		}
	}
}

func TestRangeShapeIsNotEchoed(t *testing.T) {
	err := loadRange(t, Flags{Start: strPtr("tomorrow"), End: strPtr("2026-10-01 00:00")})
	if err == nil || err.Error() != "analysis.start must look like 2026-09-01 00:00" || strings.Contains(err.Error(), "tomorrow") {
		t.Fatalf("error = %v", err)
	}
	err = loadRange(t, Flags{Start: strPtr("2026-09-01 00:00"), End: strPtr("soon")})
	if err == nil || err.Error() != "analysis.end must look like 2026-09-01 00:00" {
		t.Fatalf("error = %v", err)
	}
}

func TestRangeNamesItsZone(t *testing.T) {
	err := loadRange(t, Flags{Timezone: strPtr("America/New_York"), Start: strPtr("2026-03-08 02:30"), End: strPtr("2026-03-09 00:00")})
	if err == nil || err.Error() != "analysis.start 2026-03-08 02:30 does not exist in America/New_York" {
		t.Fatalf("error = %v", err)
	}
	err = loadRange(t, Flags{Timezone: strPtr("America/New_York"), Start: strPtr("2026-10-01 00:00"), End: strPtr("2026-11-01 01:30")})
	if err == nil || err.Error() != "analysis.end 2026-11-01 01:30 occurs twice in America/New_York" {
		t.Fatalf("error = %v", err)
	}
}

func TestDefaultPeriodIsKept(t *testing.T) {
	if got := Default().Analysis.Period; got != "24h" {
		t.Fatalf("Period = %q", got)
	}
}

func TestASourceSetsBothEnds(t *testing.T) {
	const want = "analysis.start and analysis.end must be set together"
	tests := []struct {
		name string
		src  Source
	}{
		{"file start with environment end", Source{File: writeFile(t, rangeFile("start = \"2026-09-01 00:00\"")), Getenv: rangeEnv(map[string]string{"KCR_END": "2026-10-01 00:00"})}},
		{"file end alone", Source{File: writeFile(t, rangeFile("end = \"2026-10-01 00:00\"")), Getenv: noEnv}},
		{"environment start with flag end", Source{File: writeFile(t, rangeFile("")), Getenv: rangeEnv(map[string]string{"KCR_START": "2026-09-01 00:00"}), Flags: Flags{End: strPtr("2026-10-01 00:00")}}},
		{"environment end alone", Source{File: writeFile(t, rangeFile("")), Getenv: rangeEnv(map[string]string{"KCR_END": "2026-10-01 00:00"})}},
		{"file range with flag start alone", Source{File: writeFile(t, rangeFile("start = \"2026-09-01 00:00\"\nend = \"2026-10-01 00:00\"")), Getenv: noEnv, Flags: Flags{Start: strPtr("2026-09-05 00:00")}}},
		{"file range with flag end alone", Source{File: writeFile(t, rangeFile("start = \"2026-09-01 00:00\"\nend = \"2026-10-01 00:00\"")), Getenv: noEnv, Flags: Flags{End: strPtr("2026-09-20 00:00")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.src)
			if err == nil || err.Error() != want {
				t.Fatalf("error = %v, want %q", err, want)
			}
		})
	}
}

func TestAFlagRangeReplacesTheFileRange(t *testing.T) {
	loaded, err := Load(Source{
		File:   writeFile(t, rangeFile("start = \"2026-09-01 00:00\"\nend = \"2026-10-01 00:00\"")),
		Getenv: noEnv,
		Flags:  Flags{Start: strPtr("2026-09-05 00:00"), End: strPtr("2026-09-20 00:00")},
	})
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	a := loaded.Config.Analysis
	if a.Period != "" || a.Start != "2026-09-05 00:00" || a.End != "2026-09-20 00:00" {
		t.Fatalf("Analysis = %+v", a)
	}
}
