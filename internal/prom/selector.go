package prom

import (
	"sort"
	"strings"
)

// Selector builds one PromQL selector.
type Selector struct {
	Metric string
	Equal  map[string]string
}

// String renders the selector, with its matchers written in the order of
// their label names, whatever the order Equal was built in.
func (s Selector) String() string {
	if len(s.Equal) == 0 {
		return s.Metric
	}
	names := make([]string, 0, len(s.Equal))
	for name := range s.Equal {
		names = append(names, name)
	}
	sort.Strings(names)

	matchers := make([]string, 0, len(names))
	for _, name := range names {
		matchers = append(matchers, name+`="`+Quote(s.Equal[name])+`"`)
	}
	return s.Metric + "{" + strings.Join(matchers, ",") + "}"
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
