package auth

import (
	"encoding/hex"
	"regexp"
	"testing"
)

func TestNewTokenShape(t *testing.T) {
	re := regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	a, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewToken()
	if err != nil {
		t.Fatal(err)
	}
	if !re.MatchString(a) || !re.MatchString(b) || a == b {
		t.Fatalf("tokens %q, %q", a, b)
	}
}

func TestDigestIsSHA256(t *testing.T) {
	want, _ := hex.DecodeString("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad")
	if got := Digest("abc"); hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("got %x", got)
	}
}
