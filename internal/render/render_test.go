package render_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/document"
	"github.com/marceloalvescf/capacity-room/internal/goldentest"
	"github.com/marceloalvescf/capacity-room/internal/render"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

func loadSnapshot(t *testing.T, scenario string) snapshot.Snapshot {
	t.Helper()
	path, err := goldentest.SnapshotPath(scenario)
	if err != nil {
		t.Fatalf("SnapshotPath: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	snap, err := snapshot.Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	return snap
}

func TestDocumentEqualsTheDataFile(t *testing.T) {
	snap := loadSnapshot(t, "direct-complete")
	for _, lang := range catalog.Languages() {
		t.Run(string(lang), func(t *testing.T) {
			got, err := render.Document(snap, lang, document.Options{})
			if err != nil {
				t.Fatalf("Document: %v", err)
			}
			path, err := goldentest.DataFilePath("direct-complete", lang)
			if err != nil {
				t.Fatalf("DataFilePath: %v", err)
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile: %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("the document differs from %s", path)
			}
		})
	}
}
