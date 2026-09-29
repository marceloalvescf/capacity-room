package catalog

import (
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
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

// TestTextForWarningAndNoticeEntries proves that a warning key and both
// entries of every notice code resolve through the embedded files, not
// only through the entries map catalog.go builds on its own: the locale
// files hold the "singular" and "plural" entries of a notice as an
// ordinary table, the same shape as every other family, so a lookup by the
// exact key must still work through go-i18n's own bundle.
func TestTextForWarningAndNoticeEntries(t *testing.T) {
	for _, lang := range Languages() {
		cat, err := Load(lang)
		if err != nil {
			t.Fatalf("Load(%q): %v", lang, err)
		}

		if _, err := cat.Text(Warning("NAMESPACE_USAGE_INCOMPLETE"), nil); err != nil {
			t.Errorf("%s Text(Warning) unexpected error: %v", lang, err)
		}

		for _, code := range collect.NoticeCodes {
			singular, err := cat.Text(Notice(code, 1), nil)
			if err != nil {
				t.Fatalf("%s Text(Notice(%s), singular) unexpected error: %v", lang, code, err)
			}
			if strings.Contains(singular, "{{") {
				t.Errorf("%s Text(Notice(%s), singular) = %q, holds an unfilled template", lang, code, singular)
			}

			plural, err := cat.Text(Notice(code, 3), Params{"Count": "3"})
			if err != nil {
				t.Fatalf("%s Text(Notice(%s), plural) unexpected error: %v", lang, code, err)
			}
			if !strings.Contains(plural, "3") {
				t.Errorf("%s Text(Notice(%s), plural) = %q, does not hold the filled count", lang, code, plural)
			}
		}
	}
}

// TestTextRefusesMissingEntry compares the whole error text by equality,
// not with two separate Contains checks: "en" is a substring of the fixed
// word "entry" in the message's own wording, so a Contains check for the
// language would pass even if the language were never actually
// interpolated into the message at all.
func TestTextRefusesMissingEntry(t *testing.T) {
	cat, err := Load(English)
	if err != nil {
		t.Fatalf("Load(en): %v", err)
	}
	_, err = cat.Text(Key("nope"), nil)
	if err == nil {
		t.Fatal("Text(\"nope\") = nil error, want one")
	}
	want := fmt.Sprintf("catalog: %s: no entry %q", English, Key("nope"))
	if err.Error() != want {
		t.Errorf("error = %q, want %q", err, want)
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

// tomlString renders value as a TOML basic string, escaping the backslashes
// and double quotes a test input below may contain.
func tomlString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return `"` + value + `"`
}

// fsWithEntry builds a one-language, one-entry test file system, as
// TestLoadDoesNotFallBackBetweenLanguages does, so load can be pointed at a
// single entry without disturbing the real locale files.
func fsWithEntry(text string) fstest.MapFS {
	return fstest.MapFS{
		"locales/en.toml": &fstest.MapFile{Data: []byte("value = " + tomlString(text) + "\n")},
	}
}

// TestLoadRefusesOtherTemplateForms proves that Load rejects every
// {{...}} span that is not the exact plain form {{.Name}}: a space inside
// the braces, a pipeline, a condition, a nested field, an empty reference,
// an unterminated "{{", a stray "}}", and one bad span among otherwise good
// ones. wantSpan is a literal, hand-written per row, never computed by
// calling templateSpan: a span taken from templateSpan itself would still
// match whatever wrong span a regression in templateSpan produced, since
// load's error is built from that very call. The whole error text is then
// compared by equality against the one message Load produces for a bad
// span, so that neither a wrong span nor a wrong surrounding text can
// hide inside a Contains check — "{{" and "}}" both already appear, coincidentally, in
// the fixed wording "...that is not {{.Name}}: ...", which is exactly what
// let two rows of an earlier version of this test pass regardless of what
// span Load actually reported.
func TestLoadRefusesOtherTemplateForms(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		wantSpan string
	}{
		{"spaces inside the braces", "{{ .Start }}", "{{ .Start }}"},
		{"a pipeline", `{{.Start | printf "%s"}}`, `{{.Start | printf "%s"}}`},
		{"a condition", "{{if .Start}}x{{end}}", "{{if .Start}}"},
		{"a nested field", "{{.start.end}}", "{{.start.end}}"},
		{"an empty reference", "{{.}}", "{{.}}"},
		{"an unterminated open", "{{.Start}", "{{"},
		{"a stray close", "Start}}", "}}"},
		{"one bad span among good ones", "{{.Start}} and {{ .End}}", "{{ .End}}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := fmt.Sprintf("catalog %s: entry %s uses a template form that is not {{.Name}}: %s", English, Key("value"), tt.wantSpan)

			_, err := load(fsWithEntry(tt.text), English)
			if err == nil {
				t.Fatalf("load() with entry %q = nil error, want %q", tt.text, want)
			}
			if err.Error() != want {
				t.Errorf("load() with entry %q error = %q, want %q", tt.text, err.Error(), want)
			}
		})
	}
}

// TestLoadAcceptsThePlainForm proves that Load still accepts every entry
// this catalog actually needs: two plain parameters, the same parameter
// referenced twice, no brace at all, and a single unpaired brace character,
// which is not a {{ or }} marker at all.
func TestLoadAcceptsThePlainForm(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"two parameters", "{{.Start}} a {{.End}}"},
		{"the same parameter twice, adjacent", "{{.Start}}{{.Start}}"},
		{"no brace at all", "Plain text with no braces."},
		{"one open brace alone", "Value: {approx}"},
		{"one close brace alone", "Value: approx}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := load(fsWithEntry(tt.text), English); err != nil {
				t.Fatalf("load() with entry %q: %v", tt.text, err)
			}
		})
	}
}

// TestTextWithTheSameParameterTwice proves that an entry may reference the
// same parameter more than once: Text still requires it exactly once from
// the caller, and fills every occurrence.
func TestTextWithTheSameParameterTwice(t *testing.T) {
	cat, err := load(fsWithEntry("{{.A}} and {{.A}}"), English)
	if err != nil {
		t.Fatalf("load(): %v", err)
	}

	got, err := cat.Text(Key("value"), Params{"A": "x"})
	if err != nil {
		t.Fatalf("Text(value): %v", err)
	}
	want := "x and x"
	if got != want {
		t.Errorf("Text(value) = %q, want %q", got, want)
	}
}
