package redact

import "testing"

// A long-lived process adds the same credential on every use, so the set
// must hold it once however often it is added.
func TestAddKeepsOneCopyOfASecret(t *testing.T) {
	r := New("first-secret")
	for range 1000 {
		r.Add("same-credential-value", "first-secret")
	}
	r.Add("other-credential-value-longer")
	if len(r.secrets) != 3 {
		t.Fatalf("held %d secrets, want 3", len(r.secrets))
	}
	want := []string{"other-credential-value-longer", "same-credential-value", "first-secret"}
	for i, s := range want {
		if r.secrets[i] != s {
			t.Fatalf("secrets = %q, want %q", r.secrets, want)
		}
	}
}
