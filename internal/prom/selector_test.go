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

// TestSelectorWritesFourGroupsInOrder checks that the four kinds of
// matcher are written in the order of selector() of the original:
// Equal, Regex, NotEqual, NotRegex.
func TestSelectorWritesFourGroupsInOrder(t *testing.T) {
	sel := Selector{
		Metric: "m",
		Equal: map[string]string{
			"cluster": "prod",
		},
		Regex: map[string][]string{
			"namespace": {"payments", "orders"},
		},
		NotRegex: map[string][]string{
			"container": {"", "POD", "istio-proxy"},
		},
	}
	got := sel.String()
	want := `m{cluster="prod",namespace=~"(?:payments|orders)",container!~"(?:|POD|istio\\-proxy)"}`
	if got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

// TestLiteralsEscapesTwice checks the exact text of the original's
// literal_regex(): each value escaped for a regular expression, then the
// whole escaped again by Quote.
func TestLiteralsEscapesTwice(t *testing.T) {
	got := Literals([]string{"payments", "orders.v2"})
	want := `(?:payments|orders\\.v2)`
	if got != want {
		t.Errorf("Literals(...) = %q, want %q", got, want)
	}
}

// TestLiteralsEscapesAQuote checks a value holding a double quote, which
// is not special to a regular expression and so is left to Quote, applied
// once to the whole expression.
func TestLiteralsEscapesAQuote(t *testing.T) {
	got := Literals([]string{`a"b`})
	want := `(?:a\"b)`
	if got != want {
		t.Errorf("Literals(...) = %q, want %q", got, want)
	}
}

// TestLiteralsEscapesEveryListedCharacter checks every character
// re.escape of Python puts a backslash before: each gets a backslash from
// the value-level escape, doubled by Quote.
func TestLiteralsEscapesEveryListedCharacter(t *testing.T) {
	chars := []string{
		"(", ")", "[", "]", "{", "}", "?", "*", "+", "-", "|", "^", "$",
		`\`, ".", "&", "~", "#", " ", "\t", "\n", "\r", "\v", "\f",
	}
	for _, c := range chars {
		got := Literals([]string{c})
		want := "(?:\\\\" + c + ")"
		switch c {
		case `\`:
			want = `(?:\\\\)`
		case "\n":
			want = `(?:\\\n)`
		}
		if got != want {
			t.Errorf("Literals([%q]) = %q, want %q", c, got, want)
		}
	}
}

// TestLiteralsLeavesOtherCharactersAlone checks that a value holding none
// of the escaped characters passes through unchanged.
func TestLiteralsLeavesOtherCharactersAlone(t *testing.T) {
	got := Literals([]string{"a_b/c:d,e=f"})
	want := "(?:a_b/c:d,e=f)"
	if got != want {
		t.Errorf("Literals(...) = %q, want %q", got, want)
	}
}

// assertPanicsWith fails the test unless fn panics with a value holding
// want.
func assertPanicsWith(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("want a panic with %q, got none", want)
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("panic value = %v (%T), want a string", r, r)
		}
		if msg != want {
			t.Errorf("panic value = %q, want %q", msg, want)
		}
	}()
	fn()
}

// TestLiteralsPanicsOnAnEmptyList checks that Literals refuses an empty
// list of values, as literal_regex of the original does.
func TestLiteralsPanicsOnAnEmptyList(t *testing.T) {
	const want = "prom: a regular expression of literals needs at least one value"
	t.Run("nil", func(t *testing.T) {
		assertPanicsWith(t, want, func() { Literals(nil) })
	})
	t.Run("empty", func(t *testing.T) {
		assertPanicsWith(t, want, func() { Literals([]string{}) })
	})
}

// TestLiteralsAcceptsOneEmptyValue checks that a list holding one value
// that is itself the empty text is not an empty list.
func TestLiteralsAcceptsOneEmptyValue(t *testing.T) {
	got := Literals([]string{""})
	want := "(?:)"
	if got != want {
		t.Errorf("Literals([\"\"]) = %q, want %q", got, want)
	}
}

// TestSelectorPanicsOnAnEmptyRegexList checks that a Selector whose Regex
// or NotRegex holds a label with an empty list panics through Literals
// when String is called.
func TestSelectorPanicsOnAnEmptyRegexList(t *testing.T) {
	const want = "prom: a regular expression of literals needs at least one value"
	t.Run("Regex", func(t *testing.T) {
		sel := Selector{Metric: "m", Regex: map[string][]string{"namespace": {}}}
		assertPanicsWith(t, want, func() { _ = sel.String() })
	})
	t.Run("NotRegex", func(t *testing.T) {
		sel := Selector{Metric: "m", NotRegex: map[string][]string{"container": nil}}
		assertPanicsWith(t, want, func() { _ = sel.String() })
	})
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
