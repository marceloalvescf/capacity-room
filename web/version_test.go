package web

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestBuildVersion(t *testing.T) {
	raw, err := os.ReadFile("../VERSION")
	if err != nil {
		t.Fatal(err)
	}
	want := strings.TrimSpace(string(raw))
	m := regexp.MustCompile(`<meta name="capacityroom-version" content="([^"]*)"`).FindStringSubmatch(readIndex(t))
	if m == nil {
		t.Fatal("index.html has no capacityroom-version meta")
	}
	if m[1] != want {
		t.Fatalf("capacityroom-version %q, want %q", m[1], want)
	}
}
