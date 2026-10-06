package loggerscan

import (
	"io"
	"log"
	"log/slog"
	"os"
)

func others() {
	_ = slog.Default()
	slog.SetDefault(nil)
	_ = slog.NewJSONHandler(os.Stdout, nil)
	log.Printf("x")
	_ = slog.NewTextHandler(io.Discard, nil)
	_ = slog.NewJSONHandler(io.Discard, nil)
}
