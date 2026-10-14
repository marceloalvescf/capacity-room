package api

import (
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/platform/store"
)

func TestSlotsEqualTheDownloadPool(t *testing.T) {
	if got := cap((&Server{}).downloads()); got != store.DownloadConns {
		t.Fatalf("download slots = %d, want %d", got, store.DownloadConns)
	}
}
