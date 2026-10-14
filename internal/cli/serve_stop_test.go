package cli

import (
	"bytes"
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/redact"
)

// syncBuffer is a bytes.Buffer safe for the server goroutine to write while
// the test reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// stalledServer serves a handler that writes far more than a socket holds,
// to a client that never reads.
func stalledServer(t *testing.T, logger *slog.Logger) (*http.Server, *connCount, net.Conn, <-chan struct{}) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	httpServer, conns := newServeServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		chunk := make([]byte, 1<<20)
		for range 256 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}), logger)
	go func() { _ = httpServer.Serve(listener) }()

	client, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if _, err := client.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	return httpServer, conns, client, started
}

func TestStopClosesAStalledDownload(t *testing.T) {
	var log syncBuffer
	httpServer, conns, _, started := stalledServer(t, newLogger(&log, redact.New()))
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not start")
	}
	begun := time.Now()
	if err := stopServer(context.Background(), httpServer, conns, 200*time.Millisecond, newLogger(&log, redact.New())); err != nil {
		t.Fatalf("stopServer = %v, want nil", err)
	}
	if took := time.Since(begun); took < 200*time.Millisecond || took > time.Second {
		t.Fatalf("the stop took %v", took)
	}
	if got := log.String(); !strings.Contains(got, "serve: connections closed before they finished: 1") {
		t.Fatalf("log = %q", got)
	}
}

func TestStopWithNothingOpen(t *testing.T) {
	var log syncBuffer
	logger := newLogger(&log, redact.New())
	httpServer, conns := newServeServer(http.NotFoundHandler(), logger)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = httpServer.Serve(listener) }()
	if err := stopServer(context.Background(), httpServer, conns, 200*time.Millisecond, logger); err != nil {
		t.Fatalf("stopServer = %v, want nil", err)
	}
	if got := log.String(); got != "" {
		t.Fatalf("log = %q, want none", got)
	}
}

// TestStopCountsOnlyConnectionsInUse leaves one stalled download and one
// idle keep-alive connection and one new connection that sent nothing open
// at the stop; only the download counts.
func TestStopCountsOnlyConnectionsInUse(t *testing.T) {
	var log syncBuffer
	logger := newLogger(&log, redact.New())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	httpServer, conns := newServeServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/quick" {
			return
		}
		close(started)
		chunk := make([]byte, 1<<20)
		for range 256 {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}), logger)
	idle := make(chan struct{})
	track := httpServer.ConnState
	fresh := make(chan struct{})
	var once sync.Once
	var news int
	httpServer.ConnState = func(c net.Conn, s http.ConnState) {
		track(c, s)
		if s == http.StateNew {
			news++
			if news == 2 {
				close(fresh)
			}
		}
		if s == http.StateIdle {
			once.Do(func() { close(idle) })
		}
	}
	go func() { _ = httpServer.Serve(listener) }()
	addr := listener.Addr().String()

	idleClient, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idleClient.Close() })
	if _, err := idleClient.Write([]byte("GET /quick HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-idle:
	case <-time.After(5 * time.Second):
		t.Fatal("the connection did not become idle")
	}

	freshClient, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = freshClient.Close() })
	select {
	case <-fresh:
	case <-time.After(5 * time.Second):
		t.Fatal("the new connection was not seen")
	}

	stalled, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stalled.Close() })
	if _, err := stalled.Write([]byte("GET /slow HTTP/1.1\r\nHost: x\r\n\r\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the handler did not start")
	}
	if err := stopServer(context.Background(), httpServer, conns, 200*time.Millisecond, logger); err != nil {
		t.Fatalf("stopServer = %v, want nil", err)
	}
	if got := log.String(); !strings.Contains(got, "serve: connections closed before they finished: 1") {
		t.Fatalf("log = %q", got)
	}
}
