//go:build postgres

package cli

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/auth"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func TestServeWiresTheProfiles(t *testing.T) {
	pool := storetest.New(t)
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	hash, err := auth.HashPassword("reader password one")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.InsertUser(context.Background(), pool, store.User{
		ID: uuid.New(), Username: "rita", PasswordHash: hash, Role: auth.RoleReader,
		Language: "en", Active: true, CreatedAt: now, UpdatedAt: now,
	}); err != nil {
		t.Fatal(err)
	}

	loaded := settings.Settings{
		SessionTTL: 12 * time.Hour, SessionIdle: time.Hour,
		CredentialDir: t.TempDir(), CABundleDir: t.TempDir(),
	}
	server := buildServer(pool, loaded, []byte("0123456789abcdef0123456789abcdef"),
		func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))
	front := httptest.NewServer(server.Handler())
	defer front.Close()

	client := &reader{t: t, base: front.URL, cookies: map[string]string{}}
	client.mustStatus(401, "GET", "/api/v1/profiles", "", nil)

	client.signIn("rita", "reader password one")
	listed := client.mustStatus(200, "GET", "/api/v1/profiles", "", nil)
	if want := map[string]any{"profiles": []any{}}; !reflect.DeepEqual(listed, want) {
		t.Fatalf("GET /api/v1/profiles = %v, want %v", listed, want)
	}
}
