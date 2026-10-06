package cli

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// stringer is a fmt.Stringer whose text holds a secret.
type stringer string

func (s stringer) String() string { return string(s) }

// A text handler escapes a quote, a backslash, and a format character
// before the stream redactor sees the line, so the logger redacts every
// value before it is formatted.
func TestLoggerRedactsBeforeFormatting(t *testing.T) {
	tokens := []string{
		`s3cr3t"t0ken-quote`,
		`s3cr3t\t0ken-backslash`,
		"s3cr3t​t0ken-format",
	}
	for _, token := range tokens {
		t.Run(token, func(t *testing.T) {
			var out bytes.Buffer
			redactor := redact.New()
			logger := newLogger(redactor.Writer(&out), redactor)
			redactor.Add(token)

			logger.Warn("attempt failed near "+token,
				slog.String("value", "near "+token),
				slog.Any("cause", errors.New("bad query near "+token)),
				slog.Any("stringer", stringer("near "+token)),
				slog.Group("outer", slog.String("inner", token)),
			)
			logger.WithGroup("group").Info("again", slog.Any("cause", errors.New(token)))

			got := out.String()
			if strings.Contains(got, "t0ken") {
				t.Fatalf("the token reached the log:\n%s", got)
			}
			if n := strings.Count(got, redact.Mark); n != 6 {
				t.Fatalf("%d redaction marks, want 6:\n%s", n, got)
			}
		})
	}
}
