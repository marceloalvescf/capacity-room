package catalog

import (
	"testing"
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
