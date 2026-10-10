// Command fake-prometheus serves one scenario file over HTTP, standing in
// for a Prometheus-compatible API in manual testing and in the end-to-end
// stack.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/promtest"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("fake-prometheus", flag.ContinueOnError)
	flags.SetOutput(stderr)
	fixturePath := flags.String("fixture", "", "path to a scenario file")
	listen := flags.String("listen", "127.0.0.1:9090", "address to listen on")
	probeURL := flags.String("probe", "", "URL to ask once; exit 0 on a 2xx answer")
	delay := flags.Duration("delay", 0, "time to hold each answer other than the ready route")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *probeURL != "" {
		if flags.NFlag() != 1 {
			_, _ = fmt.Fprintln(stderr, "fake-prometheus: --probe takes no other flag")
			return 2
		}
		return probe(*probeURL, stderr)
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

	httpServer := &http.Server{Handler: handler(server, *delay)}
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

// handler answers the ready route itself, so a health check spends no
// recorded answer, and holds every other answer for delay before the
// recorded server writes it.
func handler(server *promtest.Server, delay time.Duration) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /-/ready", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if delay > 0 {
			time.Sleep(delay)
		}
		server.ServeHTTP(w, r)
	})
	return mux
}

// probe asks url once and reports whether it answered with a 2xx status.
func probe(url string, stderr io.Writer) int {
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "fake-prometheus: %v\n", err)
		return 1
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode > 299 {
		_, _ = fmt.Fprintf(stderr, "fake-prometheus: %s answered %d\n", url, response.StatusCode)
		return 1
	}
	return 0
}
