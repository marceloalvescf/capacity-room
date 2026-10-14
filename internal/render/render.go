// Package render turns a snapshot into the data file of the report, and the
// data file into PDF bytes.
package render

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/document"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
	"github.com/marceloalvescf/capacity-room/internal/typst"
	"github.com/marceloalvescf/capacity-room/template"
)

// MaxPDFBytes is the largest PDF the platform stores.
const MaxPDFBytes = 64 << 20

// ErrNotPDF is returned when the compiler wrote bytes that do not start as a PDF does.
var ErrNotPDF = errors.New("the compiler wrote a file that is not a PDF")

// pdfMagic is the first bytes of every real PDF file.
var pdfMagic = []byte("%PDF-")

// Document makes report.json from the snapshot in the language, narrowed by opts.
func Document(s snapshot.Snapshot, lang catalog.Language, opts document.Options) ([]byte, error) {
	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, err
	}
	doc, err := document.Build(s, cat, opts)
	if err != nil {
		return nil, err
	}
	return document.Encode(doc)
}

// Compile unpacks the template into a fresh folder under tempDir, compiles
// data, and returns the PDF bytes; the folder is removed before it returns.
// An empty tempDir means os.TempDir().
func Compile(ctx context.Context, tc typst.Toolchain, tempDir string, data []byte) (pdf []byte, err error) {
	err = inWorkFolder(tempDir, func(dir string) error {
		if err := template.Unpack(dir); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "report.json"), data, 0o644); err != nil {
			return err
		}

		output := filepath.Join(dir, "report.pdf")
		job := typst.Job{
			Root:     dir,
			FontPath: template.FontPath(dir),
			Source:   template.Entry(dir),
			Output:   output,
			Format:   typst.PDF,
			Inputs:   map[string]string{"data": "/report.json"},
		}
		if _, err := tc.Compile(ctx, job); err != nil {
			return err
		}

		out, err := os.ReadFile(output)
		if err != nil {
			return err
		}
		if !bytes.HasPrefix(out, pdfMagic) {
			return ErrNotPDF
		}
		pdf = out
		return nil
	})
	if err != nil {
		return nil, err
	}
	return pdf, nil
}

// inWorkFolder makes a fresh, empty folder under parent (os.TempDir() when
// parent is empty), runs work with its path, and removes the folder in every
// case: success, an error from work, or a panic that unwinds through it.
func inWorkFolder(parent string, work func(dir string) error) error {
	if parent == "" {
		parent = os.TempDir()
	}
	dir, err := os.MkdirTemp(parent, "capacity-room-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	return work(dir)
}
