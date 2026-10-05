package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexedwards/argon2id"
)

func TestCheckPassword(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want error
	}{
		{"11", strings.Repeat("a", 11), ErrPasswordTooShort},
		{"12", strings.Repeat("a", 12), nil},
		{"1024", strings.Repeat("a", 1024), nil},
		{"1025", strings.Repeat("a", 1025), ErrPasswordTooLong},
		{"12 runes", strings.Repeat("á", 12), nil},
		{"1024 runes", strings.Repeat("é", 1024), nil},
	}
	for _, c := range cases {
		if err := CheckPassword(c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v, want %v", c.name, err, c.want)
		}
	}
}

const testPassword = "correct horse battery staple"

func TestHashPasswordPins(t *testing.T) {
	h, err := HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("hash prefix: %q", h)
	}
}

func TestVerifyPasswordMatches(t *testing.T) {
	h, err := HashPassword(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if m, r := VerifyPassword(testPassword, h); !m || r {
		t.Errorf("right password: %v, %v", m, r)
	}
	if m, r := VerifyPassword("another horse battery staple", h); m || r {
		t.Errorf("wrong password: %v, %v", m, r)
	}
}

func TestVerifyPasswordNeedsRehash(t *testing.T) {
	h, err := argon2id.CreateHash(testPassword, &argon2id.Params{Memory: 65536, Iterations: 1, Parallelism: 4, SaltLength: 16, KeyLength: 32})
	if err != nil {
		t.Fatal(err)
	}
	if m, r := VerifyPassword(testPassword, h); !m || !r {
		t.Errorf("got %v, %v", m, r)
	}
}

func TestVerifyPasswordGarbage(t *testing.T) {
	if m, r := VerifyPassword(testPassword, "nothash"); m || !r {
		t.Errorf("got %v, %v", m, r)
	}
}

func TestDummyHashIsStableAndRandom(t *testing.T) {
	a, b := DummyHash(), DummyHash()
	if a != b {
		t.Fatal("dummy hash changed between calls")
	}
	empty, err := HashPassword("")
	if err != nil {
		t.Fatal(err)
	}
	if a == empty {
		t.Fatal("dummy hash equals a hash of the empty string")
	}
}
