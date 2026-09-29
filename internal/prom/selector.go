package prom

import (
	"sort"
	"strings"
)

// Selector builds one PromQL selector. Its matchers are written in the
// order of the original's selector(): Equal, Regex, NotEqual, NotRegex,
// each group by label name.
type Selector struct {
	Metric   string
	Equal    map[string]string
	Regex    map[string][]string // label =~ one of these literal values
	NotEqual map[string]string
	NotRegex map[string][]string // label !~ one of these literal values
}

// String renders the selector, with its four groups of matchers written
// in order, and each group's matchers in the order of their label names,
// whatever order the maps were built in.
func (s Selector) String() string {
	var matchers []string
	matchers = append(matchers, equalMatchers(s.Equal, `="`)...)
	matchers = append(matchers, regexMatchers(s.Regex, `=~"`)...)
	matchers = append(matchers, equalMatchers(s.NotEqual, `!="`)...)
	matchers = append(matchers, regexMatchers(s.NotRegex, `!~"`)...)
	if len(matchers) == 0 {
		return s.Metric
	}
	return s.Metric + "{" + strings.Join(matchers, ",") + "}"
}

// equalMatchers writes one matcher per label of values, in the order of
// their label names, as name+op+Quote(value)+`"`.
func equalMatchers(values map[string]string, op string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	matchers := make([]string, 0, len(names))
	for _, name := range names {
		matchers = append(matchers, name+op+Quote(values[name])+`"`)
	}
	return matchers
}

// regexMatchers writes one matcher per label of values, in the order of
// their label names, as name+op+Literals(values)+`"`.
func regexMatchers(values map[string][]string, op string) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	matchers := make([]string, 0, len(names))
	for _, name := range names {
		matchers = append(matchers, name+op+Literals(values[name])+`"`)
	}
	return matchers
}

// Quote escapes a value for a PromQL string, without the quotes around it.
// It doubles each backslash first, then turns a line break into \n, then
// puts a backslash before each double quote, so that a backslash introduced
// by quoting a line break or a quote is never itself re-escaped.
func Quote(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return value
}

// regexSpecial holds the characters re.escape of Python puts a backslash
// before, and no other: the characters special to a regular expression,
// the space, and the ASCII whitespace control characters.
var regexSpecial = map[rune]bool{
	'(': true, ')': true, '[': true, ']': true, '{': true, '}': true,
	'?': true, '*': true, '+': true, '-': true, '|': true, '^': true,
	'$': true, '\\': true, '.': true, '&': true, '~': true, '#': true,
	' ': true, '\t': true, '\n': true, '\r': true, '\v': true, '\f': true,
}

// Literals writes values as one regular expression that matches any of
// them and nothing else, ready to stand between the quotes of a matcher.
// Each value is escaped for a regular expression as re.escape of Python
// does, then the whole expression is escaped by Quote. regexp.QuoteMeta is
// not used: it does not escape the hyphen.
func Literals(values []string) string {
	escaped := make([]string, len(values))
	for i, value := range values {
		escaped[i] = escapeLiteral(value)
	}
	return Quote("(?:" + strings.Join(escaped, "|") + ")")
}

// escapeLiteral puts a backslash before each character regexSpecial holds,
// and no other.
func escapeLiteral(value string) string {
	var b strings.Builder
	for _, r := range value {
		if regexSpecial[r] {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
