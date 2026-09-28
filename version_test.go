package kcr

import (
	"os"
	"strings"
	"testing"
)

func TestParseVersionAcceptsPlainVersion(t *testing.T) {
	got, err := ParseVersion("1.2.3")
	if err != nil {
		t.Fatalf("ParseVersion returned error: %v", err)
	}
	if got != "1.2.3" {
		t.Fatalf("ParseVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestParseVersionTrimsWhitespace(t *testing.T) {
	cases := []string{" 0.0.0\n", "0.0.0\r\n"}
	for _, raw := range cases {
		got, err := ParseVersion(raw)
		if err != nil {
			t.Fatalf("ParseVersion(%q) returned error: %v", raw, err)
		}
		if got != "0.0.0" {
			t.Fatalf("ParseVersion(%q) = %q, want %q", raw, got, "0.0.0")
		}
	}
}

func TestParseVersionRejects(t *testing.T) {
	cases := []string{"", "v0.1.0", "1.2", "1.2.3.4", "01.2.3", "1.2.3-rc1", "1.2.3\n4.5.6"}
	for _, raw := range cases {
		_, err := ParseVersion(raw)
		if err == nil {
			t.Fatalf("ParseVersion(%q) returned no error, want error", raw)
		}
		if !strings.Contains(err.Error(), "want MAJOR.MINOR.PATCH") {
			t.Fatalf("ParseVersion(%q) error = %q, want it to contain %q", raw, err.Error(), "want MAJOR.MINOR.PATCH")
		}
	}
}

func TestVersionMatchesFile(t *testing.T) {
	raw, err := os.ReadFile("VERSION")
	if err != nil {
		t.Fatalf("reading VERSION: %v", err)
	}
	want, err := ParseVersion(string(raw))
	if err != nil {
		t.Fatalf("ParseVersion(VERSION contents) returned error: %v", err)
	}
	if got := Version(); got != want {
		t.Fatalf("Version() = %q, want %q", got, want)
	}
}
