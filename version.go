// Package kcr provides module-wide identity for k8s-capacity-report,
// currently limited to the single source of the program's version.
package kcr

import (
	_ "embed"
	"fmt"
	"regexp"
	"strings"
)

//go:embed VERSION
var rawVersion string

var versionPattern = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// ParseVersion trims surrounding white space from raw and validates it
// against the MAJOR.MINOR.PATCH shape. It returns the trimmed value on
// success.
func ParseVersion(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if !versionPattern.MatchString(trimmed) {
		return "", fmt.Errorf("invalid VERSION %q: want MAJOR.MINOR.PATCH", trimmed)
	}
	return trimmed, nil
}

// Version returns the program's version, parsed from the embedded VERSION
// file. It panics if the embedded file is invalid: a broken version must
// stop the program at start rather than run with an unknown identity.
func Version() string {
	v, err := ParseVersion(rawVersion)
	if err != nil {
		panic(err)
	}
	return v
}
