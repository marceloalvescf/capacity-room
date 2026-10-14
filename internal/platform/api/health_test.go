//go:build postgres

package api

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/marceloalvescf/capacity-room/internal/platform/storetest"
)

func ready(t *testing.T, pool *pgxpool.Pool) *httptest.ResponseRecorder {
	t.Helper()
	return get(t, &Server{Pool: pool}, "GET", "/health/ready")
}

func TestHealthReady(t *testing.T) {
	w := ready(t, storetest.New(t))
	want := `{"status":"ready","checks":{"database":true,"migrations":true}}` + "\n"
	if w.Code != 200 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
	if w.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("content type %q", w.Header().Get("Content-Type"))
	}
}

func TestHealthReadyWithoutMigrations(t *testing.T) {
	w := ready(t, storetest.NewEmpty(t))
	want := `{"status":"not_ready","checks":{"database":true,"migrations":false}}` + "\n"
	if w.Code != 503 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}

func TestHealthReadyDatabaseDown(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://x:y@127.0.0.1:1/x?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	w := ready(t, pool)
	want := `{"status":"not_ready","checks":{"database":false,"migrations":false}}` + "\n"
	if w.Code != 503 || w.Body.String() != want {
		t.Fatalf("%d %q", w.Code, w.Body.String())
	}
}
