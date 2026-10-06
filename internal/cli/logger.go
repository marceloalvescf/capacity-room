package cli

import (
	"io"
	"log/slog"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

// newLogger returns the logger of serve and worker, which River's logger
// also writes through: text lines on w, the run's redacted stream, with the
// message and every value passed through redactor before the handler quotes
// it.
func newLogger(w io.Writer, redactor *redact.Redactor) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{ReplaceAttr: redactor.ReplaceAttr}))
}
