package prom

import "testing"

// TestSelectorSortsMatchers checks that matchers are written in the order
// of their label names, whatever order they were inserted in.
func TestSelectorSortsMatchers(t *testing.T) {
	sel := Selector{
		Metric: "m",
		Equal: map[string]string{
			"resource": "cpu",
			"cluster":  "prod",
		},
	}
	got := sel.String()
	want := `m{cluster="prod",resource="cpu"}`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestSelectorWithoutMatchers checks that a selector with no matchers is
// just the metric name.
func TestSelectorWithoutMatchers(t *testing.T) {
	sel := Selector{Metric: "m"}
	got := sel.String()
	want := "m"
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestQuote covers the three escaping rules, applied in order: backslashes
// doubled first, then a line break turned into \n, then a double quote
// escaped.
func TestQuote(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "quote and backslash",
			input: `prod"west\one`,
			want:  `prod\"west\\one`,
		},
		{
			name:  "line break",
			input: "line one\nline two",
			want:  `line one\nline two`,
		},
		{
			name:  "backslash before quote is not re-escaped",
			input: `a\"b`,
			want:  `a\\\"b`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Quote(tc.input)
			if got != tc.want {
				t.Errorf("Quote(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
