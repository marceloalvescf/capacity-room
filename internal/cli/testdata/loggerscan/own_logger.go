package loggerscan

import (
	"log/slog"
	"os"
)

func build() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}
