package auth

import (
	"errors"
	"regexp"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var usernamePattern = regexp.MustCompile("^[a-z0-9][a-z0-9._-]{0,63}$")

// ErrInvalidUsername is returned when the normalised name does not match
// ^[a-z0-9][a-z0-9._-]{0,63}$.
var ErrInvalidUsername = errors.New("invalid username")

// NormalizeUsername applies NFKC, Unicode case folding, and trims white
// space, as the original's normalize_username does.
func NormalizeUsername(raw string) string {
	return strings.TrimSpace(cases.Fold().String(norm.NFKC.String(raw)))
}

// ValidateUsername normalises raw and returns the stored form, or
// ErrInvalidUsername.
func ValidateUsername(raw string) (string, error) {
	n := NormalizeUsername(raw)
	if !UsernameMatchesPattern(n) {
		return "", ErrInvalidUsername
	}
	return n, nil
}

// UsernameMatchesPattern reports whether a normalised name matches the
// pattern, for the sign-in path, which never refuses a name but skips the
// lookup of one that cannot be stored.
func UsernameMatchesPattern(normalized string) bool {
	return usernamePattern.MatchString(normalized)
}
