package cli

import (
	"io"
	"log/slog"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// newLogger returns the logger of serve and worker, which River's logger
// also writes through: text lines on w, the run's redacted stream, with the
// message and every value passed through redactor before the handler quotes
// it. It is the only place that builds a handler of a command's logger.
func newLogger(w io.Writer, redactor *redact.Redactor) *slog.Logger {
	return slog.New(redact.NewHandler(redactor, slog.NewTextHandler(w, nil)))
}
