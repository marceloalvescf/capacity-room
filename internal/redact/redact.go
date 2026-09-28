// Package redact removes secrets from text before it reaches a terminal or
// a log.
package redact

import (
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Mark replaces every secret found.
const Mark = "[redacted]"

// The regular expressions below port the six rules of the original Python
// sanitizer (see spec 5.4). RE2 has no back-reference, so the original's
// single quoted-authorization pattern, which matches the value's closing
// quote with \2, is split into two alternatives here: one that requires a
// double-quoted value, one that requires a single-quoted value. Each stops
// at a newline, matching the original's un-flagged "." there.
var (
	quotedAuthDoubleRe = regexp.MustCompile(`(?i)(['"]authorization['"]\s*:\s*)"[^"\n]*"`)
	quotedAuthSingleRe = regexp.MustCompile(`(?i)(['"]authorization['"]\s*:\s*)'[^'\n]*'`)
	bareAuthRe         = regexp.MustCompile(`(?im)(\bauthorization\b\s*[:=]\s*)[^\r\n]+`)
	assignmentRe       = regexp.MustCompile(`(?i)\b(token|access_token|api_key)\s*[:=]\s*[^\s,;]+`)
	queryStringRe      = regexp.MustCompile(`(https?://[^\s?]+)\?[^\s]+`)
	userinfoRe         = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@`)
)

// Redactor removes a fixed set of literal secrets and every pattern of
// spec 5.4 from text. A Redactor is safe for use by several goroutines: a
// caller may hold one for the life of a whole run, calling Add as it learns
// of a new secret, while Text and the writers Writer returns are already in
// use from other goroutines.
type Redactor struct {
	mu sync.Mutex
	// secrets is sorted longest first, so a short secret never leaves a
	// suffix of a longer one behind in the text.
	secrets []string
}

// New keeps the secrets that are not empty.
func New(secrets ...string) *Redactor {
	r := &Redactor{}
	r.Add(secrets...)
	return r
}

// Add keeps the secrets that are not empty, adding them to the ones r
// already holds, and keeps the whole set sorted longest first.
func (r *Redactor) Add(secrets ...string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, secret := range secrets {
		if secret != "" {
			r.secrets = append(r.secrets, secret)
		}
	}
	sort.SliceStable(r.secrets, func(i, j int) bool { return len(r.secrets[i]) > len(r.secrets[j]) })
}

// Text returns s with every secret removed. It reads the current set of
// secrets under lock, so a concurrent Add is either fully reflected or not
// reflected at all, never partially.
func (r *Redactor) Text(s string) string {
	r.mu.Lock()
	secrets := append([]string(nil), r.secrets...)
	r.mu.Unlock()

	for _, secret := range secrets {
		s = strings.ReplaceAll(s, secret, Mark)
	}
	s = quotedAuthDoubleRe.ReplaceAllString(s, `${1}"`+Mark+`"`)
	s = quotedAuthSingleRe.ReplaceAllString(s, `${1}'`+Mark+`'`)
	s = bareAuthRe.ReplaceAllString(s, "${1}"+Mark)
	s = assignmentRe.ReplaceAllString(s, "${1}="+Mark)
	s = queryStringRe.ReplaceAllString(s, "${1}?"+Mark)
	s = userinfoRe.ReplaceAllString(s, "${1}"+Mark+"@")
	return s
}

// Writer returns a writer that redacts each Write call as one text and
// passes the result to w. It reports len(p) on success, so that a caller
// never sees a short write caused by the redaction.
func (r *Redactor) Writer(w io.Writer) io.Writer {
	return &redactingWriter{redactor: r, inner: w}
}

// redactingWriter is the io.Writer returned by Redactor.Writer.
type redactingWriter struct {
	redactor *Redactor
	inner    io.Writer
}

// Write redacts p as one text and forwards the result to the inner writer.
// It reports len(p) rather than the number of bytes actually forwarded,
// since redaction can change the byte count without the caller's input
// being any less fully consumed.
func (w *redactingWriter) Write(p []byte) (int, error) {
	if _, err := w.inner.Write([]byte(w.redactor.Text(string(p)))); err != nil {
		return 0, err
	}
	return len(p), nil
}
