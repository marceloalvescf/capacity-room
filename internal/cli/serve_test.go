//go:build postgres

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

// freeAddress returns a loopback address nothing listens on.
func freeAddress(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().String()
}

// startServe runs serve in a goroutine and waits until /health/ready answers
// 200. The returned stop cancels it and returns the exit code and the log.
func startServe(t *testing.T, env Env, config string) (address string, stop func() (int, string)) {
	t.Helper()
	address = freeAddress(t)
	ctx, cancel := context.WithCancel(context.Background())
	var stdout, stderr syncBuffer
	done := make(chan int, 1)
	go func() {
		done <- Run(ctx, env, []string{"serve", "--config", config, "--listen", address}, &stdout, &stderr)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		resp, err := http.Get("http://" + address + "/health/ready")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatalf("the server was not ready in 5 s; log = %q", stderr.String())
		}
		time.Sleep(20 * time.Millisecond)
	}

	stopped := false
	stop = func() (int, string) {
		stopped = true
		cancel()
		select {
		case code := <-done:
			return code, stderr.String()
		case <-time.After(35 * time.Second):
			t.Fatal("serve did not return after the context was cancelled")
			return 0, ""
		}
	}
	t.Cleanup(func() {
		if !stopped {
			stop()
		}
	})
	return address, stop
}

func TestServeListensAndStops(t *testing.T) {
	pool := storetest.New(t)
	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": pool.Config().ConnString()}, "")
	address, stop := startServe(t, env, writePlatformFiles(t))

	code, log := stop()
	if code != 0 {
		t.Fatalf("code = %d, log = %q", code, log)
	}
	for _, want := range []string{"listening on", address, "the outbound allowlists are empty", "stopping"} {
		if !strings.Contains(log, want) {
			t.Errorf("log = %q, want it to hold %q", log, want)
		}
	}
}

func TestServeCannotListen(t *testing.T) {
	pool := storetest.New(t)
	taken, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = taken.Close() }()
	address := taken.Addr().String()

	env := platformEnv(map[string]string{"KCR_DATABASE_DSN": pool.Config().ConnString()}, "")
	code, stdout, stderr := runCommand(context.Background(), env,
		"serve", "--config", writePlatformFiles(t), "--listen", address)
	if code != 1 || stdout != "" || !strings.HasPrefix(stderr, "error: cannot listen on "+address+": ") {
		t.Fatalf("code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}
}

// reader is an HTTP client that keeps cookies by hand: the server marks them
// Secure, which a cookie jar would not send over plain HTTP.
type reader struct {
	t       *testing.T
	base    string
	cookies map[string]string
	header  map[string]string // sent with every request
}

// apiClient is the client of the proofs of the API: no request waits for ever.
var apiClient = &http.Client{Timeout: 30 * time.Second}

func (r *reader) do(method, path, csrf string, body any) (int, map[string]any) {
	r.t.Helper()
	var payload io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			r.t.Fatal(err)
		}
		payload = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, r.base+path, payload)
	if err != nil {
		r.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if csrf != "" {
		req.Header.Set("X-CSRF-Token", csrf)
	}
	for name, value := range r.header {
		req.Header.Set(name, value)
	}
	for name, value := range r.cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	resp, err := apiClient.Do(req)
	if err != nil {
		r.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 || c.Value == "" {
			delete(r.cookies, c.Name)
		} else {
			r.cookies[c.Name] = c.Value
		}
	}
	var decoded map[string]any
	raw, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(raw, &decoded)
	return resp.StatusCode, decoded
}

func (r *reader) mustStatus(want int, method, path, csrf string, body any) map[string]any {
	r.t.Helper()
	status, decoded := r.do(method, path, csrf, body)
	if status != want {
		r.t.Fatalf("%s %s = %d (%v), want %d", method, path, status, decoded, want)
	}
	return decoded
}

// signIn runs the anonymous session and the login and returns the CSRF token
// of the new session.
func (r *reader) signIn(username, password string) string {
	r.t.Helper()
	anonymous := r.mustStatus(200, "GET", "/api/v1/auth/session", "", nil)
	if anonymous["authenticated"] != false {
		r.t.Fatalf("anonymous session = %v", anonymous)
	}
	loginCSRF, _ := anonymous["login_csrf"].(string)
	signed := r.mustStatus(200, "POST", "/api/v1/auth/login", loginCSRF,
		map[string]string{"username": username, "password": password})
	csrf, _ := signed["csrf_token"].(string)
	if csrf == "" {
		r.t.Fatalf("login = %v, want a csrf_token", signed)
	}
	return csrf
}

func TestServeEndToEnd(t *testing.T) {
	pool := storetest.New(t)
	env := platformEnv(map[string]string{
		"KCR_BOOTSTRAP_MODE": "1",
		"KCR_DATABASE_DSN":   pool.Config().ConnString(),
	}, "admin password one\n")
	config := writePlatformFiles(t)

	code, stdout, stderr := runCommand(context.Background(), env,
		"admin", "bootstrap-admin", "--username", "admin", "--password-stdin", "--config", config)
	if code != 0 || stdout != "created administrator admin\n" {
		t.Fatalf("bootstrap: code = %d, stdout = %q, stderr = %q", code, stdout, stderr)
	}

	address, stop := startServe(t, env, config)
	base := "http://" + address

	admin := &reader{t: t, base: base, cookies: map[string]string{}}
	adminCSRF := admin.signIn("admin", "admin password one")
	admin.mustStatus(201, "POST", "/api/v1/admin/users", adminCSRF, map[string]string{
		"username": "olivia", "password": "olivia password one", "role": "operator", "language": "en",
	})

	operator := &reader{t: t, base: base, cookies: map[string]string{}}
	operatorCSRF := operator.signIn("olivia", "olivia password one")
	oldCookie := operator.cookies["kcr_session"]
	operator.mustStatus(200, "PATCH", "/api/v1/account", operatorCSRF, map[string]string{
		"current_password": "olivia password one", "password": "olivia password two",
	})
	if _, kept := operator.cookies["kcr_session"]; kept {
		t.Fatal("the password change did not clear the session cookie")
	}

	stale := &reader{t: t, base: base, cookies: map[string]string{"kcr_session": oldCookie}}
	if anonymous := stale.mustStatus(200, "GET", "/api/v1/auth/session", "", nil); anonymous["authenticated"] != false {
		t.Fatalf("the old cookie still signs in: %v", anonymous)
	}

	code, log := stop()
	if code != 0 || !strings.Contains(log, "listening on") || !strings.Contains(log, "stopping") {
		t.Fatalf("code = %d, log = %q", code, log)
	}
}
