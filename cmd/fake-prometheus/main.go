// Command fake-prometheus serves one scenario file over HTTP, standing in
// for a Prometheus-compatible API in manual testing.
package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr *os.File) int {
	flags := flag.NewFlagSet("fake-prometheus", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fixturePath := flags.String("fixture", "", "path to a scenario file")
	listen := flags.String("listen", "127.0.0.1:9090", "address to listen on")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *fixturePath == "" {
		// Discard the error from writing to stderr; if it fails, there is
		// nowhere left to report the failure.
		_, _ = fmt.Fprintln(stderr, "fake-prometheus: --fixture is required")
		return 2
	}

	fixture, err := promtest.Load(*fixturePath)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fake-prometheus: %v\n", err)
		return 1
	}
	server := promtest.NewServer(fixture)

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fake-prometheus: %v\n", err)
		return 1
	}

	httpServer := &http.Server{Handler: server}
	go func() {
		// ErrServerClosed is the expected outcome of the signal handling
		// below; nothing else can be done with any other error here since
		// Serve runs in the background.
		_ = httpServer.Serve(listener)
	}()

	_, _ = fmt.Fprintf(stdout, "listening on http://%s\n", listener.Addr())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	_, _ = fmt.Fprintf(stdout, "remaining: %d\n", server.Remaining())
	_ = httpServer.Close()
	return 0
}
