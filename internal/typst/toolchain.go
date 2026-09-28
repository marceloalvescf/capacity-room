// Package typst locates the Typst compiler binary and checks that it is
// exactly the version this project's PDF output has been pinned to, so
// reports stay byte-reproducible across machines.
package typst

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RequiredVersion is the only Typst version this project accepts.
const RequiredVersion = "0.15.1"

// Toolchain names one located typst binary.
type Toolchain struct {
	Binary string // absolute path
}

// Locate returns the binary named by KCR_TYPST, or "typst" found on PATH.
// getenv and lookPath are injected so tests never touch the real process
// environment or PATH.
func Locate(getenv func(string) string, lookPath func(string) (string, error)) (Toolchain, error) {
	if override := getenv("KCR_TYPST"); override != "" {
		info, err := os.Stat(override)
		if !filepath.IsAbs(override) || err != nil || !info.Mode().IsRegular() {
			return Toolchain{}, fmt.Errorf("KCR_TYPST must be an absolute path to the typst binary: %q", override)
		}
		return Toolchain{Binary: override}, nil
	}

	found, err := lookPath("typst")
	if err != nil {
		return Toolchain{}, fmt.Errorf("typst not found: set KCR_TYPST or add typst to PATH")
	}
	abs, err := filepath.Abs(found)
	if err != nil {
		return Toolchain{}, fmt.Errorf("typst not found: set KCR_TYPST or add typst to PATH")
	}
	return Toolchain{Binary: abs}, nil
}

// Version runs "<binary> --version" and returns the second field, e.g.
// "0.15.1" out of "typst 0.15.1 (unknown commit)". The context deadline is
// honoured; the caller passes one and this function adds none.
func (t Toolchain) Version(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, t.Binary, "--version")
	cmd.Env = []string{} // run with an empty environment, not the parent's

	// WaitDelay bounds how long Wait blocks on the I/O-copying goroutines
	// after the process is killed. Without it, a fake script that starts
	// /bin/sleep as a child can leave that grandchild holding the output
	// pipes open, and Wait would block until it exits on its own.
	cmd.WaitDelay = 200 * time.Millisecond

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	fields := strings.Fields(stdout.String())
	if runErr == nil && len(fields) >= 2 {
		return fields[1], nil
	}

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return "", fmt.Errorf("typst --version failed (exit %d): %s", exitCode, firstLineOrFallback(stdout.String(), stderr.String()))
}

// firstLineOrFallback returns the first line of stdout, falling back to the
// first line of stderr when stdout is empty, and to "no output" when both
// are empty.
func firstLineOrFallback(stdout, stderr string) string {
	if line := firstLine(stdout); line != "" {
		return line
	}
	if line := firstLine(stderr); line != "" {
		return line
	}
	return "no output"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// Probe fails unless Version returns RequiredVersion.
func (t Toolchain) Probe(ctx context.Context) error {
	version, err := t.Version(ctx)
	if err != nil {
		return err
	}
	if version != RequiredVersion {
		return fmt.Errorf("typst %s found at %s; this build requires %s", version, t.Binary, RequiredVersion)
	}
	return nil
}
