package catalog

import (
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestTheme(t *testing.T) {
	tests := []struct {
		lang       Language
		wantLang   string
		wantRegion string
	}{
		{lang: PortugueseBrazil, wantLang: "pt", wantRegion: "BR"},
		{lang: English, wantLang: "en", wantRegion: "US"},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			cat, err := Load(tt.lang)
			if err != nil {
				t.Fatalf("Load(%q): %v", tt.lang, err)
			}
			gotLang, gotRegion := cat.Theme()
			if gotLang != tt.wantLang || gotRegion != tt.wantRegion {
				t.Errorf("Theme() = (%q, %q), want (%q, %q)", gotLang, gotRegion, tt.wantLang, tt.wantRegion)
			}
		})
	}
}

func TestDateTime(t *testing.T) {
	saoPaulo := time.FixedZone("-03:00", -3*60*60)

	tests := []struct {
		name string
		t    time.Time
		pt   string
		en   string
	}{
		{
			name: "15 July 2026, 14:30",
			t:    time.Date(2026, time.July, 15, 14, 30, 0, 0, saoPaulo),
			pt:   "15/07/2026 14:30",
			en:   "07/15/2026 14:30",
		},
		{
			name: "2 January 2026, 03:04",
			t:    time.Date(2026, time.January, 2, 3, 4, 0, 0, time.UTC),
			pt:   "02/01/2026 03:04",
			en:   "01/02/2026 03:04",
		},
	}

	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pt.DateTime(tt.t); got != tt.pt {
				t.Errorf("pt DateTime(%v) = %q, want %q", tt.t, got, tt.pt)
			}
			if got := en.DateTime(tt.t); got != tt.en {
				t.Errorf("en DateTime(%v) = %q, want %q", tt.t, got, tt.en)
			}
		})
	}
}

func TestDateTimeUsesTheWallClockGiven(t *testing.T) {
	saoPaulo := time.FixedZone("-03:00", -3*60*60)
	instant := time.Date(2026, time.July, 15, 17, 30, 0, 0, time.UTC)

	cat, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}

	utcText := cat.DateTime(instant)
	saoPauloText := cat.DateTime(instant.In(saoPaulo))

	if utcText == saoPauloText {
		t.Errorf("DateTime gave the same text (%q) for UTC and Sao Paulo; it must use the wall clock given, not convert it", utcText)
	}
}

func TestDecimal(t *testing.T) {
	tests := []struct {
		value  float64
		digits int
		pt     string
		en     string
	}{
		{value: 3.5, digits: 2, pt: "3,5", en: "3.5"},
		{value: 3, digits: 2, pt: "3", en: "3"},
		{value: 0, digits: 1, pt: "0", en: "0"},
		{value: 1234567.891, digits: 2, pt: "1234567,89", en: "1234567.89"},
	}

	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		if got := pt.Decimal(tt.value, tt.digits); got != tt.pt {
			t.Errorf("pt Decimal(%v, %d) = %q, want %q", tt.value, tt.digits, got, tt.pt)
		}
		if got := en.Decimal(tt.value, tt.digits); got != tt.en {
			t.Errorf("en Decimal(%v, %d) = %q, want %q", tt.value, tt.digits, got, tt.en)
		}
	}
}

// TestDecimalWithZeroDigits checks that Decimal with zero digits trims no
// digit of a whole number: only a zero after a decimal mark is trimmed, and
// with no decimal mark at all, nothing changes.
func TestDecimalWithZeroDigits(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{value: 30, want: "30"},
		{value: 100, want: "100"},
		{value: 0, want: "0"},
		{value: 0.4, want: "0"},
		{value: 12.5, want: "12"},
	}

	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		if got := en.Decimal(tt.value, 0); got != tt.want {
			t.Errorf("Decimal(%v, 0) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestPercent(t *testing.T) {
	tests := []struct {
		value float64
		pt    string
		en    string
	}{
		{value: 78.64, pt: "78,6%", en: "78.6%"},
		{value: 91, pt: "91%", en: "91%"},
		{value: 24000, pt: "24000%", en: "24000%"},
	}

	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		if got := pt.Percent(tt.value); got != tt.pt {
			t.Errorf("pt Percent(%v) = %q, want %q", tt.value, got, tt.pt)
		}
		if got := en.Percent(tt.value); got != tt.en {
			t.Errorf("en Percent(%v) = %q, want %q", tt.value, got, tt.en)
		}
	}
}

func TestCPU(t *testing.T) {
	tests := []struct {
		cores float64
		pt    string
		en    string
	}{
		{cores: 0.5, pt: "500 mCPU", en: "500 mCPU"},
		{cores: 0.0004, pt: "0,4 mCPU", en: "0.4 mCPU"},
		{cores: 0.99996, pt: "1000 mCPU", en: "1000 mCPU"},
		{cores: 1, pt: "1 cores", en: "1 cores"},
		{cores: 3.5, pt: "3,5 cores", en: "3.5 cores"},
		{cores: 4.125, pt: "4,12 cores", en: "4.12 cores"},
		{cores: 0, pt: "0 mCPU", en: "0 mCPU"},
	}

	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		if got := pt.CPU(tt.cores); got != tt.pt {
			t.Errorf("pt CPU(%v) = %q, want %q", tt.cores, got, tt.pt)
		}
		if got := en.CPU(tt.cores); got != tt.en {
			t.Errorf("en CPU(%v) = %q, want %q", tt.cores, got, tt.en)
		}
	}
}

func TestMemory(t *testing.T) {
	tests := []struct {
		bytes float64
		pt    string
		en    string
	}{
		{bytes: 12, pt: "12 B", en: "12 B"},
		{bytes: 1.5, pt: "1,5 B", en: "1.5 B"},
		{bytes: 1023, pt: "1023 B", en: "1023 B"},
		{bytes: 1024, pt: "1 KiB", en: "1 KiB"},
		{bytes: 1342177280, pt: "1,25 GiB", en: "1.25 GiB"},
		{bytes: 2 * 1024 * 1024 * 1024 * 1024 * 1024, pt: "2048 TiB", en: "2048 TiB"},
	}

	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		if got := pt.Memory(tt.bytes); got != tt.pt {
			t.Errorf("pt Memory(%v) = %q, want %q", tt.bytes, got, tt.pt)
		}
		if got := en.Memory(tt.bytes); got != tt.en {
			t.Errorf("en Memory(%v) = %q, want %q", tt.bytes, got, tt.en)
		}
	}
}

func TestCount(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{value: 2.5, want: "2"},
		{value: 3.5, want: "4"},
		{value: 3, want: "3"},
	}

	for _, lang := range Languages() {
		cat, err := Load(lang)
		if err != nil {
			t.Fatalf("Load(%q): %v", lang, err)
		}
		for _, tt := range tests {
			if got := cat.Count(tt.value); got != tt.want {
				t.Errorf("%s Count(%v) = %q, want %q", lang, tt.value, got, tt.want)
			}
		}
	}
}

// TestCountBeyondAnInteger checks that Count writes a value beyond an
// int64's range, or one whose float64 form already lost precision, exactly
// as Python's str(round(value)) does for the same double.
func TestCountBeyondAnInteger(t *testing.T) {
	tests := []struct {
		value float64
		want  string
	}{
		{value: 1e21, want: "1000000000000000000000"},
		{value: 9007199254740993, want: "9007199254740992"},
		{value: 1e15, want: "1000000000000000"},
		{value: 2.5, want: "2"},
		{value: 3.5, want: "4"},
		{value: 0, want: "0"},
		{value: 0.4, want: "0"},
	}

	for _, lang := range Languages() {
		cat, err := Load(lang)
		if err != nil {
			t.Fatalf("Load(%q): %v", lang, err)
		}
		for _, tt := range tests {
			if got := cat.Count(tt.value); got != tt.want {
				t.Errorf("%s Count(%v) = %q, want %q", lang, tt.value, got, tt.want)
			}
		}
	}
}

func TestInteger(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{n: 0, want: "0"},
		{n: 7, want: "7"},
		{n: 1234567, want: "1234567"},
	}

	for _, lang := range Languages() {
		cat, err := Load(lang)
		if err != nil {
			t.Fatalf("Load(%q): %v", lang, err)
		}
		for _, tt := range tests {
			if got := cat.Integer(tt.n); got != tt.want {
				t.Errorf("%s Integer(%d) = %q, want %q", lang, tt.n, got, tt.want)
			}
		}
	}
}

func TestPeriod(t *testing.T) {
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		length time.Duration
		pt     string
		en     string
	}{
		{"24 hours", 24 * time.Hour, "24 horas", "24 hours"},
		{"7 days", 7 * 24 * time.Hour, "7 dias", "7 days"},
		{"6 days", 6 * 24 * time.Hour, "144 horas", "144 hours"},
		{"1 hour", time.Hour, "1 hora", "1 hour"},
		{"90 minutes", 90 * time.Minute, "90 minutos", "90 minutes"},
		{"1 minute", time.Minute, "1 minuto", "1 minute"},
		{"90 seconds", 90 * time.Second, "90 segundos", "90 seconds"},
		{"1 second", time.Second, "1 segundo", "1 second"},
		{"zero", 0, "0 segundos", "0 seconds"},
		{"8 days", 8 * 24 * time.Hour, "8 dias", "8 days"},
		{"7 days and 1 hour", 7*24*time.Hour + time.Hour, "169 horas", "169 hours"},
		{"2.5 seconds", 2500 * time.Millisecond, "2 segundos", "2 seconds"},
		{"3.5 seconds", 3500 * time.Millisecond, "4 segundos", "4 seconds"},
		{"2.6 seconds", 2600 * time.Millisecond, "3 segundos", "3 seconds"},
		{"negative", -5 * time.Hour, "0 segundos", "0 seconds"},
	}

	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			end := start.Add(tt.length)
			if got, err := pt.Period(start, end); err != nil || got != tt.pt {
				t.Errorf("pt Period = %q, %v, want %q", got, err, tt.pt)
			}
			if got, err := en.Period(start, end); err != nil || got != tt.en {
				t.Errorf("en Period = %q, %v, want %q", got, err, tt.en)
			}
		})
	}
}

func TestPeriodNeedsTheEntries(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/en.toml": &fstest.MapFile{Data: []byte("value = \"x\"\n")},
	}
	cat, err := load(fsys, English)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	start := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	_, err = cat.Period(start, start.Add(time.Hour))
	if err == nil || !strings.Contains(err.Error(), string(PeriodHourSingular)) {
		t.Errorf("Period without entries: err = %v, want one naming %q", err, PeriodHourSingular)
	}
}
