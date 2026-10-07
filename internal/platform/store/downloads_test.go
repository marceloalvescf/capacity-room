//go:build postgres

package store_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/storetest"
)

func TestDownloadPoolSize(t *testing.T) {
	pool := storetest.New(t)
	downloads, err := store.OpenDownloads(context.Background(), pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(downloads.Close)
	cfg := downloads.Config()
	if cfg.MaxConns != 4 || cfg.MinConns != 0 {
		t.Fatalf("MaxConns = %d, MinConns = %d, want 4 and 0", cfg.MaxConns, cfg.MinConns)
	}
}

func TestDownloadPoolKeepsNoIdleConnection(t *testing.T) {
	ctx := context.Background()
	pool := storetest.New(t)
	f := newReportFixture(t, pool)
	r := collectedReport(t, pool, f, now.Add(time.Hour), map[string]string{"en": "pending"}, map[string]int64{"en": 1})
	body := append([]byte("%PDF-"), bytes.Repeat([]byte("x"), 8<<20)...)
	if err := store.FinishPDF(ctx, pool, r.ID, "en", body, now); err != nil {
		t.Fatal(err)
	}

	downloads, err := store.OpenDownloads(ctx, pool.Config().ConnString())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(downloads.Close)
	for i := range 3 {
		got, err := store.ReadPDF(ctx, downloads, r.ID, "en", now)
		if err != nil || len(got.Body) != len(body) {
			t.Fatalf("read %d: %d bytes, %v", i, len(got.Body), err)
		}
		if idle := downloads.Stat().IdleConns(); idle != 0 {
			t.Fatalf("after read %d: %d idle connections, want none", i, idle)
		}
		// A released connection is closed in the background.
		deadline := time.Now().Add(5 * time.Second)
		for downloads.Stat().TotalConns() != 0 {
			if time.Now().After(deadline) {
				t.Fatalf("after read %d: %d connections still open, want none", i, downloads.Stat().TotalConns())
			}
			time.Sleep(5 * time.Millisecond)
		}
	}
}
