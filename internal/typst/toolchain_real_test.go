//go:build toolchain

package typst

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

// TestRealTypstIsThePinnedVersion exercises Locate and Probe against the
// real typst binary installed on this machine. It requires the toolchain
// build tag because it depends on machine state, not just source code.
func TestRealTypstIsThePinnedVersion(t *testing.T) {
	tc, err := Locate(os.Getenv, exec.LookPath)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if err := tc.Probe(context.Background()); err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
}

// TestRealPdftotextExists checks that pdftotext, used by a later phase to
// verify rendered PDFs, is installed on this machine.
func TestRealPdftotextExists(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Fatalf("LookPath(\"pdftotext\") error = %v", err)
	}
}
