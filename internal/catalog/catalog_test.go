package catalog

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestParseLanguage(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Language
		wantErr bool
	}{
		{name: "pt-BR", input: "pt-BR", want: PortugueseBrazil},
		{name: "en", input: "en", want: English},
		{name: "wrong case", input: "pt-br", wantErr: true},
		{name: "unknown", input: "fr", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLanguage(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseLanguage(%q) = %q, nil; want error", tt.input, got)
				}
				for _, lang := range Languages() {
					if !strings.Contains(err.Error(), string(lang)) {
						t.Errorf("error %q does not list accepted value %q", err, lang)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseLanguage(%q) unexpected error: %v", tt.input, err)
			}
			if got != tt.want {
				t.Errorf("ParseLanguage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestLoadBothLanguages(t *testing.T) {
	for _, lang := range Languages() {
		if _, err := Load(lang); err != nil {
			t.Errorf("Load(%q) unexpected error: %v", lang, err)
		}
	}
}

func TestTextPlainEntry(t *testing.T) {
	tests := []struct {
		lang Language
		want string
	}{
		{lang: PortugueseBrazil, want: "LEVANTAMENTO DE CAPACIDADE"},
		{lang: English, want: "CAPACITY REPORT"},
	}
	for _, tt := range tests {
		t.Run(string(tt.lang), func(t *testing.T) {
			cat, err := Load(tt.lang)
			if err != nil {
				t.Fatalf("Load(%q): %v", tt.lang, err)
			}
			got, err := cat.Text(CoverTitle, nil)
			if err != nil {
				t.Fatalf("Text(CoverTitle): %v", err)
			}
			if got != tt.want {
				t.Errorf("Text(CoverTitle) = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTextFillsParameters(t *testing.T) {
	pt, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	got, err := pt.Text(FrontMetadataPeriodRange, Params{"Start": "14/07/2026 14:30", "End": "15/07/2026 14:30"})
	if err != nil {
		t.Fatalf("Text(FrontMetadataPeriodRange): %v", err)
	}
	want := "14/07/2026 14:30 a 15/07/2026 14:30"
	if got != want {
		t.Errorf("pt Text(FrontMetadataPeriodRange) = %q, want %q", got, want)
	}

	en, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}
	got, err = en.Text(FrontMetadataPeriodRange, Params{"Start": "14/07/2026 14:30", "End": "15/07/2026 14:30"})
	if err != nil {
		t.Fatalf("Text(FrontMetadataPeriodRange): %v", err)
	}
	want = "14/07/2026 14:30 to 15/07/2026 14:30"
	if got != want {
		t.Errorf("en Text(FrontMetadataPeriodRange) = %q, want %q", got, want)
	}
}

func TestTextRefusesMissingEntry(t *testing.T) {
	cat, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}
	_, err = cat.Text(Key("nope"), nil)
	if err == nil {
		t.Fatal("Text(\"nope\") = nil error, want one")
	}
	if !strings.Contains(err.Error(), "nope") {
		t.Errorf("error %q does not name the key", err)
	}
	if !strings.Contains(err.Error(), string(English)) {
		t.Errorf("error %q does not name the language", err)
	}
}

func TestTextRefusesMissingParameter(t *testing.T) {
	cat, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	text, err := cat.Text(FrontMetadataPeriodRange, Params{"Start": "14/07/2026 14:30"})
	if err == nil {
		t.Fatal("Text(FrontMetadataPeriodRange) with Start only = nil error, want one")
	}
	if !strings.Contains(err.Error(), "End") {
		t.Errorf("error %q does not name the missing parameter End", err)
	}
	if strings.Contains(text, "<no value>") {
		t.Errorf("Text returned %q, which holds <no value>", text)
	}
}

func TestTextRefusesUnusedParameter(t *testing.T) {
	cat, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}
	_, err = cat.Text(CoverTitle, Params{"Extra": "x"})
	if err == nil {
		t.Fatal("Text(CoverTitle) with an unused parameter = nil error, want one")
	}
	if !strings.Contains(err.Error(), "Extra") {
		t.Errorf("error %q does not name the unused parameter Extra", err)
	}
}

func TestTextDoesNotReadMarkup(t *testing.T) {
	cat, err := Load(PortugueseBrazil)
	if err != nil {
		t.Fatalf("Load(pt-BR): %v", err)
	}
	got, err := cat.Text(FrontMetadataPeriodRange, Params{"Start": "{{.End}}", "End": "valor"})
	if err != nil {
		t.Fatalf("Text(FrontMetadataPeriodRange): %v", err)
	}
	want := "{{.End}} a valor"
	if got != want {
		t.Errorf("Text(FrontMetadataPeriodRange) = %q, want %q (the value must not be read as a template)", got, want)
	}
}

// TestLoadDoesNotFallBackBetweenLanguages proves that a Catalog for one
// language never answers a key from the other language's text: each
// language is loaded into a bundle that carries only that language, so
// there is nothing for go-i18n to fall back to.
func TestLoadDoesNotFallBackBetweenLanguages(t *testing.T) {
	fsys := fstest.MapFS{
		"locales/pt-BR.toml": &fstest.MapFile{Data: []byte("only_in_portuguese = \"Só em português\"\n")},
		"locales/en.toml":    &fstest.MapFile{Data: []byte("cover_title = \"placeholder\"\n")},
	}

	pt, err := load(fsys, PortugueseBrazil)
	if err != nil {
		t.Fatalf("load(pt-BR): %v", err)
	}
	if _, err := pt.Text(Key("only_in_portuguese"), nil); err != nil {
		t.Fatalf("pt Text(only_in_portuguese): %v", err)
	}

	en, err := load(fsys, English)
	if err != nil {
		t.Fatalf("load(en): %v", err)
	}
	_, err = en.Text(Key("only_in_portuguese"), nil)
	if err == nil {
		t.Fatal("en Text(only_in_portuguese) = nil error; the English catalog must not fall back to Portuguese")
	}
}
