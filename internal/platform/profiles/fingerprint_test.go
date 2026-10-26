package profiles

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

func TestFingerprintChainsTheParent(t *testing.T) {
	a := validExample()
	b := validExample()
	b.DisplayName = "Other"
	p1 := uuid.MustParse("11111111-1111-4111-8111-111111111111")
	p2 := uuid.MustParse("22222222-2222-4222-8222-222222222222")

	if Fingerprint(a, nil) == Fingerprint(a, &p1) {
		t.Fatal("a parent must change the fingerprint")
	}
	first := Fingerprint(a, nil)
	second := Fingerprint(b, &p1)
	third := Fingerprint(a, &p2)
	if first == second || second == third || first == third {
		t.Fatal("chained fingerprints must differ")
	}
	if len(first) != 64 {
		t.Fatalf("length %d", len(first))
	}
	if Fingerprint(a, nil) != first {
		t.Fatal("not deterministic")
	}
}

func TestFingerprintTreatsNilAndEmptyListsAlike(t *testing.T) {
	a := validExample()
	b := validExample()
	a.ExcludedContainers = nil
	b.ExcludedContainers = []string{}
	if Fingerprint(a, nil) != Fingerprint(b, nil) {
		t.Fatal("nil and empty list must hash alike")
	}
}

func TestCanonicalJSONIsStable(t *testing.T) {
	d := validExample()
	d.AllowedNamespaces = []string{"zeta", "alpha"}
	d.DefaultPeriod = nil
	first, err := CanonicalJSON(d)
	if err != nil {
		t.Fatal(err)
	}
	var back Document
	if err := json.Unmarshal(first, &back); err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalJSON(back)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("not stable:\n%s\n%s", first, second)
	}
	if !bytes.Contains(first, []byte(`"allowed_namespaces":["zeta","alpha"]`)) {
		t.Fatalf("list order lost: %s", first)
	}
	if !bytes.Contains(first, []byte(`"default_period":null`)) || bytes.Contains(first, []byte(": ")) {
		t.Fatalf("form wrong: %s", first)
	}
	if !bytes.HasPrefix(first, []byte(`{"active":true,"allowed_namespaces"`)) {
		t.Fatalf("keys not sorted: %s", first)
	}
}

func TestFingerprintUsesTheCapacityRoomDomain(t *testing.T) {
	const want = "60469c00afde439529af05ea88c5fe89459b1b757414d2a7383e97520e51b435"
	if got := Fingerprint(validExample(), nil); got != want {
		t.Fatalf("Fingerprint = %s, want %s", got, want)
	}
}
