package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeUsernameFoldsAndTrims(t *testing.T) {
	cases := []struct{ in, want string }{{"  Admin ", "admin"}, {"Ⅷ", "viii"}, {"ﬁsh", "fish"}, {"Straße", "strasse"}}
	for _, c := range cases {
		in, want := c.in, c.want
		if got := NormalizeUsername(in); got != want {
			t.Errorf("NormalizeUsername(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateUsernameAccepts(t *testing.T) {
	long := strings.Repeat("a", 64)
	for _, in := range []string{"a", "a.b_c-d", long} {
		got, err := ValidateUsername(in)
		if err != nil || got != in {
			t.Errorf("ValidateUsername(%q) = %q, %v", in, got, err)
		}
	}
}

func TestValidateUsernameRefuses(t *testing.T) {
	for _, in := range []string{"", ".a", strings.Repeat("a", 65), "a b", "a\x00b", "José"} {
		if _, err := ValidateUsername(in); !errors.Is(err, ErrInvalidUsername) {
			t.Errorf("ValidateUsername(%q) err = %v", in, err)
		}
	}
}

func TestValidateUsernameRefusesControlCharacters(t *testing.T) {
	for _, in := range []string{"a\nb", "a\tb"} {
		if _, err := ValidateUsername(in); !errors.Is(err, ErrInvalidUsername) {
			t.Errorf("ValidateUsername(%q) err = %v", in, err)
		}
	}
}

func TestUsernameMatchesPattern(t *testing.T) {
	for in, want := range map[string]bool{"admin": true, "Admin": false, "a\x00": false} {
		if got := UsernameMatchesPattern(in); got != want {
			t.Errorf("UsernameMatchesPattern(%q) = %v", in, got)
		}
	}
}
