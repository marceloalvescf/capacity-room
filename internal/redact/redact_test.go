package redact_test

import (
	"errors"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

func TestTextRemovesLiteralSecret(t *testing.T) {
	r := redact.New("s3cr3t")
	got := r.Text("token was s3cr3t here")
	want := "token was [redacted] here"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextIgnoresEmptySecret(t *testing.T) {
	r := redact.New("")
	got := r.Text("abc")
	want := "abc"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRemovesLongerSecretFirst(t *testing.T) {
	r := redact.New("abc", "abcdef")
	got := r.Text("x abcdef y")
	want := "x [redacted] y"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRedactsQuotedAuthorization(t *testing.T) {
	var r redact.Redactor
	got := r.Text(`{"Authorization": "Bearer x.y"}`)
	want := `{"Authorization": "[redacted]"}`
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRedactsSingleQuotedAuthorization(t *testing.T) {
	var r redact.Redactor
	got := r.Text(`{'authorization': 'Bearer x'}`)
	want := `{'authorization': '[redacted]'}`
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRedactsBareAuthorizationLine(t *testing.T) {
	var r redact.Redactor
	got := r.Text("Authorization: Bearer abc\nnext")
	want := "Authorization: [redacted]\nnext"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRedactsAssignments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "token colon-less equals", in: "token=abc", want: "token=[redacted]"},
		{name: "access_token colon", in: "access_token: abc", want: "access_token=[redacted]"},
		{name: "API_KEY spaced equals", in: "API_KEY = abc", want: "API_KEY=[redacted]"},
	}
	var r redact.Redactor
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.Text(tt.in)
			if got != tt.want {
				t.Fatalf("Text(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestTextAssignmentStopsAtSeparator(t *testing.T) {
	var r redact.Redactor
	got := r.Text("token=abc, next")
	want := "token=[redacted], next"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRedactsQueryString(t *testing.T) {
	var r redact.Redactor
	got := r.Text("https://h.example/p?a=1&b=2 done")
	want := "https://h.example/p?[redacted] done"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextRedactsUserInfo(t *testing.T) {
	var r redact.Redactor
	got := r.Text("connection failed: postgresql+psycopg://kcr_admin:SuperSecretPass123@10.0.0.5/kcr")
	want := "connection failed: postgresql+psycopg://[redacted]@10.0.0.5/kcr"
	if got != want {
		t.Fatalf("Text() = %q, want %q", got, want)
	}
}

func TestTextLeavesPlainTextAlone(t *testing.T) {
	tests := []string{
		"tokenizer",
		"the token bucket",
		"https://h.example/p",
		"a@b",
	}
	var r redact.Redactor
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			got := r.Text(in)
			if got != in {
				t.Fatalf("Text(%q) = %q, want unchanged", in, got)
			}
		})
	}
}

func TestTextIsStableWhenAppliedTwice(t *testing.T) {
	values := []string{
		"token was [redacted] here",
		"abc",
		"x [redacted] y",
		`{"Authorization": "[redacted]"}`,
		`{'authorization': '[redacted]'}`,
		"Authorization: [redacted]\nnext",
		"token=[redacted]",
		"access_token=[redacted]",
		"API_KEY=[redacted]",
		"token=[redacted], next",
		"https://h.example/p?[redacted] done",
		"connection failed: postgresql+psycopg://[redacted]@10.0.0.5/kcr",
		"tokenizer",
		"the token bucket",
		"https://h.example/p",
		"a@b",
	}
	var r redact.Redactor
	for _, v := range values {
		t.Run(v, func(t *testing.T) {
			once := r.Text(v)
			twice := r.Text(once)
			if once != twice {
				t.Fatalf("Text(Text(%q)) = %q, want %q", v, twice, once)
			}
		})
	}
}

func TestWriterRedactsEachWrite(t *testing.T) {
	r := redact.New("s3cr3t")
	inner := &captureWriter{}
	w := r.Writer(inner)

	n, err := w.Write([]byte("a s3cr3t b\n"))
	if err != nil {
		t.Fatalf("Write() error = %v, want nil", err)
	}
	if n != 11 {
		t.Fatalf("Write() n = %d, want 11", n)
	}
	want := "a [redacted] b\n"
	if inner.String() != want {
		t.Fatalf("inner received %q, want %q", inner.String(), want)
	}
}

func TestWriterReturnsInnerError(t *testing.T) {
	r := redact.New("s3cr3t")
	failure := errors.New("boom")
	w := r.Writer(&failingWriter{err: failure})

	_, err := w.Write([]byte("a s3cr3t b"))
	if !errors.Is(err, failure) {
		t.Fatalf("Write() error = %v, want %v", err, failure)
	}
}

// captureWriter is an io.Writer that records every byte slice it receives.
type captureWriter struct {
	data []byte
}

func (c *captureWriter) Write(p []byte) (int, error) {
	c.data = append(c.data, p...)
	return len(p), nil
}

func (c *captureWriter) String() string {
	return string(c.data)
}

// failingWriter is an io.Writer that always fails with err.
type failingWriter struct {
	err error
}

func (f *failingWriter) Write(_ []byte) (int, error) {
	return 0, f.err
}
