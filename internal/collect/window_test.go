package collect

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
)

func TestStepTable(t *testing.T) {
	tests := []struct {
		name   string
		period time.Duration
		want   string
	}{
		{"30m", 30 * time.Minute, "30s"},
		{"30m1s", 30*time.Minute + time.Second, "2m"},
		{"6h", 6 * time.Hour, "2m"},
		{"6h1s", 6*time.Hour + time.Second, "5m"},
		{"24h", 24 * time.Hour, "5m"},
		{"24h1s", 24*time.Hour + time.Second, "15m"},
		{"7d", 7 * 24 * time.Hour, "15m"},
		{"7d1s", 7*24*time.Hour + time.Second, "1h"},
		{"30d", 30 * 24 * time.Hour, "1h"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Step(tt.period); got != tt.want {
				t.Fatalf("Step(%v) = %q, want %q", tt.period, got, tt.want)
			}
		})
	}
}

func TestNewWindowOfTheGoldenInput(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	emittedAt := time.Date(2026, 7, 15, 14, 30, 0, 0, loc)
	analysis := config.Analysis{Period: "24h", Timezone: "America/Sao_Paulo"}

	got, err := NewWindow(analysis, emittedAt)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	if got.Start.Unix() != 1784050200 {
		t.Errorf("Start.Unix() = %d, want %d", got.Start.Unix(), 1784050200)
	}
	if got.End.Unix() != 1784136600 {
		t.Errorf("End.Unix() = %d, want %d", got.End.Unix(), 1784136600)
	}
	if got.Step != "5m" {
		t.Errorf("Step = %q, want %q", got.Step, "5m")
	}
}

func TestNewWindowCountsTheWallClock(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	emittedAt := time.Date(2026, 3, 8, 12, 0, 0, 0, loc)
	analysis := config.Analysis{Period: "24h", Timezone: "America/New_York"}

	got, err := NewWindow(analysis, emittedAt)
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	want := time.Date(2026, 3, 7, 12, 0, 0, 0, loc)
	if !got.Start.Equal(want) {
		t.Errorf("Start = %v, want %v", got.Start, want)
	}
	if diff := got.End.Sub(got.Start); diff != 23*time.Hour {
		t.Errorf("End.Sub(Start) = %v, want %v", diff, 23*time.Hour)
	}
}

func TestNewWindowRefusesAnEmptyPeriod(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	emittedAt := time.Date(2026, 3, 29, 3, 30, 0, 0, loc)
	analysis := config.Analysis{Period: "1h", Timezone: "Europe/Berlin"}

	_, err = NewWindow(analysis, emittedAt)
	if err == nil {
		t.Fatal("NewWindow: error = nil, want an error")
	}
	if !errors.Is(err, ErrEmptyPeriod) {
		t.Errorf("errors.Is(err, ErrEmptyPeriod) = false, err = %v", err)
	}
	if !strings.Contains(err.Error(), "1h") {
		t.Errorf("err.Error() = %q, want it to name %q", err.Error(), "1h")
	}
	if !strings.Contains(err.Error(), "Europe/Berlin") {
		t.Errorf("err.Error() = %q, want it to name %q", err.Error(), "Europe/Berlin")
	}
}

func TestNewWindowRefusesAStartAfterTheEnd(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	emittedAt := time.Date(2026, 3, 29, 3, 0, 0, 0, loc)
	analysis := config.Analysis{Period: "1m", Timezone: "Europe/Berlin"}

	_, err = NewWindow(analysis, emittedAt)
	if err == nil {
		t.Fatal("NewWindow: error = nil, want an error")
	}
	if !errors.Is(err, ErrEmptyPeriod) {
		t.Errorf("errors.Is(err, ErrEmptyPeriod) = false, err = %v", err)
	}
}

func TestNewWindowAcrossTheSkippedHour(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	newYork, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	tests := []struct {
		name       string
		zone       string
		period     string
		emitted    time.Time
		wantOffset int // seconds east of UTC
	}{
		{
			name:       "Europe/Berlin",
			zone:       "Europe/Berlin",
			period:     "2h",
			emitted:    time.Date(2026, 3, 29, 3, 30, 0, 0, berlin),
			wantOffset: 3600, // +01:00
		},
		{
			name:       "America/New_York",
			zone:       "America/New_York",
			period:     "1h",
			emitted:    time.Date(2026, 3, 8, 3, 30, 0, 0, newYork),
			wantOffset: -5 * 3600, // -05:00
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analysis := config.Analysis{Period: tt.period, Timezone: tt.zone}

			got, err := NewWindow(analysis, tt.emitted)
			if err != nil {
				t.Fatalf("NewWindow: %v", err)
			}
			if got.Start.Hour() != 1 || got.Start.Minute() != 30 {
				t.Errorf("Start = %v, want 01:30", got.Start)
			}
			if _, offset := got.Start.Zone(); offset != tt.wantOffset {
				t.Errorf("Start offset = %d, want %d", offset, tt.wantOffset)
			}
			if diff := got.End.Sub(got.Start); diff != time.Hour {
				t.Errorf("End.Sub(Start) = %v, want %v", diff, time.Hour)
			}
		})
	}
}
