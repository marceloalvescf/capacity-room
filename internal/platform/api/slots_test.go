package api

import (
	"testing"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/store"
)

func TestSlotsEqualTheDownloadPool(t *testing.T) {
	if got := cap((&Server{}).downloads()); got != store.DownloadConns {
		t.Fatalf("download slots = %d, want %d", got, store.DownloadConns)
	}
}
