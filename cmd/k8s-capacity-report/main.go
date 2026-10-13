// Command k8s-capacity-report generates Kubernetes capacity reports from a Prometheus-compatible API.
package main

import (
	"context"
	"os"
	"syscall"

	"github.com/marceloalvescf/k8s-capacity-report/internal/cli"
)

func main() {
	ctx, stop := cli.SignalContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, cli.RealEnv(), os.Args[1:], os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}
