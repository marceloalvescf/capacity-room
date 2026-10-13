package cli

import (
	"context"
	"os"
	"os/signal"
)

// SignalContext returns a context that ends at the first of signals. The
// signals have their default action again before the context ends, so a
// second one ends the process while a command is still stopping.
func SignalContext(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	received := make(chan os.Signal, 1)
	signal.Notify(received, signals...)
	go func() {
		select {
		case <-received:
		case <-ctx.Done():
		}
		signal.Stop(received)
		cancel()
	}()
	return ctx, cancel
}
