//go:build postgres

package api

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// stalledDownload opens a connection that asks for the PDF of the report and
// never reads the answer.
func stalledDownload(t *testing.T, srv *httptest.Server, a *actor, path string) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: x\r\nCookie: %s=%s\r\n\r\n", path, SessionCookie, a.cookie.Value)
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatal(err)
	}
	return conn
}

func waitFor(t *testing.T, what string, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatalf("no %s after 10 s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestFifthDownloadWaitsForASlot(t *testing.T) {
	ts := newTestServer(t)
	c := ts.cast(t)
	big := bytes.Repeat([]byte("x"), 48<<20)
	id := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: []byte("{}"), pdfBody: big})
	srv := httptest.NewServer(ts.Handler())
	t.Cleanup(srv.Close)
	path := "/api/v1/reports/" + id.String() + "/pdf?lang=en"

	var conns []net.Conn
	for i := 0; i < 4; i++ {
		conns = append(conns, stalledDownload(t, srv, &c.eve, path))
	}
	waitFor(t, "four slots taken", func() bool { return len(ts.downloads()) == 4 })

	req, err := http.NewRequest("GET", srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(c.eve.cookie)
	answered := make(chan error, 1)
	go func() {
		resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
		if err == nil {
			_, err = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		answered <- err
	}()
	select {
	case err := <-answered:
		t.Fatalf("the fifth download was answered while four held the slots: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
	for _, conn := range conns {
		_ = conn.Close()
	}
	select {
	case err := <-answered:
		if err != nil {
			t.Fatalf("the fifth download after a slot came free: %v", err)
		}
	case <-time.After(20 * time.Second):
		t.Fatal("the fifth download never got a slot")
	}
	waitFor(t, "every slot back", func() bool { return len(ts.downloads()) == 0 })
}

func TestStalledReaderIsCutAtTheDeadline(t *testing.T) {
	ts := newTestServer(t)
	ts.downloadSlots, ts.downloadDeadline = 1, 300*time.Millisecond
	c := ts.cast(t)
	big := bytes.Repeat([]byte("x"), 48<<20)
	id := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: []byte("{}"), pdfBody: big})
	srv := httptest.NewServer(ts.Handler())
	t.Cleanup(srv.Close)
	path := "/api/v1/reports/" + id.String() + "/pdf?lang=en"

	stalledDownload(t, srv, &c.eve, path)
	waitFor(t, "the slot taken", func() bool { return len(ts.downloads()) == 1 })
	waitFor(t, "the slot given back after the deadline", func() bool { return len(ts.downloads()) == 0 })

	req, err := http.NewRequest("GET", srv.URL+path+"&x=1", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(c.eve.cookie)
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	n, err := io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if err != nil || n != int64(len(big)) {
		t.Fatalf("the download after the cut: %d bytes, %v", n, err)
	}
}

func TestDownloadWaitingForASlotEndsWithItsContext(t *testing.T) {
	ts := newTestServer(t)
	ts.downloadSlots = 1
	c := ts.cast(t)
	id := ts.seedReport(t, stored{owner: c.ops.user.ID, snapshot: []byte("{}")})
	ts.downloads() <- struct{}{} // the one slot is held
	ctx, cancel := context.WithCancel(context.Background())
	r := ts.requestAs(&c.eve, "GET", "/api/v1/reports/"+id.String()+"/pdf?lang=en").WithContext(ctx)
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		ts.Handler().ServeHTTP(w, r)
		close(done)
	}()
	select {
	case <-done:
		t.Fatalf("answered while the slot was held: %d", w.Code)
	case <-time.After(300 * time.Millisecond):
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("still waiting after the context ended")
	}
	if w.Body.Len() != 0 || len(ts.downloads()) != 1 {
		t.Fatalf("body %d bytes, %d slots held", w.Body.Len(), len(ts.downloads()))
	}
}
