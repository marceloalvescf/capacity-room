//go:build postgres

package cli

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

func TestServeServesTheInterface(t *testing.T) {
	pool := storetest.New(t)
	loaded := settings.Settings{
		SessionTTL: 12 * time.Hour, SessionIdle: time.Hour,
		CredentialDir: t.TempDir(), CABundleDir: t.TempDir(),
	}
	server, err := buildServer(pool, openDownloads(t, pool), loaded, []byte("0123456789abcdef0123456789abcdef"),
		time.Now, slog.New(slog.NewTextHandler(io.Discard, nil)), redact.New())
	if err != nil {
		t.Fatal(err)
	}
	front := httptest.NewServer(server.Handler())
	defer front.Close()

	for _, path := range []string{"/", "/account"} {
		res, err := http.Get(front.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()
		if res.StatusCode != 200 || !strings.Contains(string(body), `<div id="root">`) {
			t.Errorf("GET %s = %d %q", path, res.StatusCode, body)
		}
	}

	res, err := http.Get(front.URL + "/api/v1/auth/session")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("session Content-Type %q, body %q", ct, body)
	}
}
