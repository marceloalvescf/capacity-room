package audit_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/audit"
)

func TestEncodeAcceptsScalars(t *testing.T) {
	got, err := audit.Encode(audit.Metadata{"role": "operator", "count": 2, "ok": true, "none": nil})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"count":2,"none":null,"ok":true,"role":"operator"}`; string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestEncodeAcceptsOneLevel(t *testing.T) {
	for name, nested := range map[string]any{
		"metadata": audit.Metadata{"role": "reader"},
		"map":      map[string]any{"role": "reader"},
	} {
		got, err := audit.Encode(audit.Metadata{"changed": nested})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if want := `{"changed":{"role":"reader"}}`; string(got) != want {
			t.Fatalf("%s: got %s, want %s", name, got, want)
		}
	}
}

func TestEncodeRefusesForbiddenKeys(t *testing.T) {
	for _, key := range []string{"password", "new_token", "Cookie", "DIGEST", "secret_ref", "dsn", "password_hash", "HASH"} {
		_, err := audit.Encode(audit.Metadata{key: "x"})
		if !errors.Is(err, audit.ErrMetadata) || !strings.Contains(err.Error(), key) {
			t.Errorf("key %q: got %v", key, err)
		}
	}
}

func TestEncodeRefusesNestedKeys(t *testing.T) {
	_, err := audit.Encode(audit.Metadata{"changed": audit.Metadata{"password": "x"}})
	if !errors.Is(err, audit.ErrMetadata) || !strings.Contains(err.Error(), "password") {
		t.Fatalf("got %v", err)
	}
}

func TestEncodeRefusesDeepNesting(t *testing.T) {
	_, err := audit.Encode(audit.Metadata{"a": map[string]any{"b": map[string]any{"c": 1}}})
	if !errors.Is(err, audit.ErrMetadata) {
		t.Fatalf("got %v", err)
	}
}

func TestEncodeRefusesOtherTypes(t *testing.T) {
	type pair struct{ A int }
	for name, v := range map[string]any{"slice": []string{"a"}, "struct": pair{1}, "time": time.Unix(0, 0)} {
		_, err := audit.Encode(audit.Metadata{name: v})
		if !errors.Is(err, audit.ErrMetadata) || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: got %v", name, err)
		}
	}
}

func TestEncodeEmpty(t *testing.T) {
	for name, m := range map[string]audit.Metadata{"nil": nil, "empty": {}} {
		got, err := audit.Encode(m)
		if err != nil || string(got) != `{}` {
			t.Errorf("%s: got %s, %v", name, got, err)
		}
	}
}
