// Package catalog turns a code into the sentence a reader of the report
// sees, in Portuguese (Brazil) or in English. It is the only package of the
// program that holds a sentence meant for that reader: everywhere else
// carries a code, and asks this package for the words.
package catalog

import (
	"embed"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locales/*.toml
var localesFS embed.FS

// Language is one of the two languages the report can be written in.
type Language string

// The two languages the catalog serves.
const (
	PortugueseBrazil Language = "pt-BR"
	English          Language = "en"
)

// Languages returns the two languages, Portuguese first.
func Languages() []Language {
	return []Language{PortugueseBrazil, English}
}

// ParseLanguage parses value as one of the two accepted language codes. An
// unrecognised value is an error that lists the accepted ones.
func ParseLanguage(value string) (Language, error) {
	for _, lang := range Languages() {
		if value == string(lang) {
			return lang, nil
		}
	}
	return "", fmt.Errorf("catalog: language %q is not one of %v", value, Languages())
}

// Params are the named values of a sentence, already formatted as text.
type Params map[string]string

// Catalog holds every sentence of one language, read from its TOML file.
type Catalog struct {
	lang      Language
	localizer *i18n.Localizer
	entries   map[Key]string // raw template text of every entry, this language only
}

// paramPattern finds a parameter reference such as {{.Start}}.
var paramPattern = regexp.MustCompile(`\{\{\.(\w+)\}\}`)

// paramNames returns the distinct parameter names that text references, in
// the order they first appear.
func paramNames(text string) []string {
	matches := paramPattern.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool, len(matches))
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		name := m[1]
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names
}

// namePattern is a parameter reference in the one form the catalog accepts
// inside a pair of braces: a dot, then a letter, then letters and digits,
// with no space anywhere inside the braces.
var namePattern = regexp.MustCompile(`^\.[A-Za-z][A-Za-z0-9]*$`)

// templateSpan finds the first span of text that keeps this catalog from
// holding only the plain template form {{.Name}}: a pair of braces whose
// content is a pipeline, a condition, a nested field, a space, or an empty
// reference; an opening "{{" with no closing "}}" after it; or a closing
// "}}" with no opening "{{" before it. It returns the offending span,
// braces included where both exist, and true. It returns "", false once
// every {{ in text is paired with a following }} and every span between
// them matches namePattern.
func templateSpan(text string) (string, bool) {
	i := 0
	for {
		rest := text[i:]
		open := strings.Index(rest, "{{")
		closeIdx := strings.Index(rest, "}}")

		switch {
		case open == -1 && closeIdx == -1:
			return "", false
		case open == -1:
			// A "}}" remains, but no "{{" follows i: a stray close.
			return "}}", true
		case closeIdx != -1 && closeIdx < open:
			// The next marker is a "}}", before the next "{{": a stray
			// close that comes ahead of the next open.
			return "}}", true
		default:
			afterOpen := i + open + 2
			closeInContent := strings.Index(text[afterOpen:], "}}")
			if closeInContent == -1 {
				// This "{{" is never closed.
				return "{{", true
			}
			content := text[afterOpen : afterOpen+closeInContent]
			if !namePattern.MatchString(content) {
				return "{{" + content + "}}", true
			}
			i = afterOpen + closeInContent + 2
		}
	}
}

// flatten walks a TOML document already unmarshalled into a generic map and
// returns every leaf string, keyed by its dotted path. A leaf that is not a
// string is an error, since the catalog holds no plural forms.
func flatten(data map[string]any) (map[Key]string, error) {
	entries := make(map[Key]string)
	var walk func(prefix string, node map[string]any) error
	walk = func(prefix string, node map[string]any) error {
		for k, v := range node {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			switch val := v.(type) {
			case string:
				entries[Key(key)] = val
			case map[string]any:
				if err := walk(key, val); err != nil {
					return err
				}
			default:
				return fmt.Errorf("entry %q is not a string", key)
			}
		}
		return nil
	}
	if err := walk("", data); err != nil {
		return nil, err
	}
	return entries, nil
}

// load reads the message file of lang from fsys. It is split out from Load
// so that a test can point it at a file system that is not the embedded
// one.
func load(fsys fs.FS, lang Language) (*Catalog, error) {
	tag, err := language.Parse(string(lang))
	if err != nil {
		return nil, fmt.Errorf("catalog: %s: %w", lang, err)
	}

	path := "locales/" + string(lang) + ".toml"

	raw, err := fs.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("catalog: %s: %w", lang, err)
	}

	var data map[string]any
	if err := toml.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("catalog: %s: %w", lang, err)
	}
	entries, err := flatten(data)
	if err != nil {
		return nil, fmt.Errorf("catalog: %s: %w", lang, err)
	}

	// A catalog that loads holds only the plain template form {{.Name}}, so
	// Text never needs to ask what syntax an entry used: this is the only
	// place that asks.
	for key, text := range entries {
		if span, bad := templateSpan(text); bad {
			return nil, fmt.Errorf("catalog %s: entry %s uses a template form that is not {{.Name}}: %s", lang, key, span)
		}
	}

	// Each language gets its own bundle, with that language as its only and
	// default language. There is then nothing to fall back to: a key
	// missing in this language is never silently answered from another.
	bundle := i18n.NewBundle(tag)
	bundle.RegisterUnmarshalFunc("toml", toml.Unmarshal)
	if _, err := bundle.LoadMessageFileFS(fsys, path); err != nil {
		return nil, fmt.Errorf("catalog: %s: %w", lang, err)
	}

	return &Catalog{
		lang:      lang,
		localizer: i18n.NewLocalizer(bundle, string(lang)),
		entries:   entries,
	}, nil
}

// Load reads the embedded file of lang.
func Load(lang Language) (*Catalog, error) {
	return load(localesFS, lang)
}

// Language returns the language c was loaded for.
func (c *Catalog) Language() Language {
	return c.lang
}

// Theme returns the language and region codes that drive hyphenation: "pt"
// and "BR", or "en" and "US".
func (c *Catalog) Theme() (lang, region string) {
	if c.lang == PortugueseBrazil {
		return "pt", "BR"
	}
	return "en", "US"
}

// Text returns the entry of key with its parameters filled in. A missing
// entry, a missing parameter, and a parameter the entry does not use are
// errors: go-i18n would otherwise print a missing parameter as the literal
// text "<no value>" and give no error, and would silently ignore a
// parameter the entry does not use.
func (c *Catalog) Text(key Key, params Params) (string, error) {
	raw, ok := c.entries[key]
	if !ok {
		return "", fmt.Errorf("catalog: %s: no entry %q", c.lang, key)
	}

	needed := paramNames(raw)
	for _, name := range needed {
		if _, ok := params[name]; !ok {
			return "", fmt.Errorf("catalog: %s: entry %q needs parameter %q", c.lang, key, name)
		}
	}
	for name := range params {
		used := false
		for _, want := range needed {
			if want == name {
				used = true
				break
			}
		}
		if !used {
			return "", fmt.Errorf("catalog: %s: entry %q does not use parameter %q", c.lang, key, name)
		}
	}

	data := make(map[string]string, len(params))
	for k, v := range params {
		data[k] = v
	}

	text, err := c.localizer.Localize(&i18n.LocalizeConfig{
		MessageID:    string(key),
		TemplateData: data,
	})
	if err != nil {
		return "", fmt.Errorf("catalog: %s: %w", c.lang, err)
	}
	return text, nil
}
