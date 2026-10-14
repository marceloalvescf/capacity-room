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

// Locate returns the binary named by CAPACITYROOM_TYPST, or "typst" found on PATH.
// getenv and lookPath are injected so tests never touch the real process
// environment or PATH.
func Locate(getenv func(string) string, lookPath func(string) (string, error)) (Toolchain, error) {
	if override := getenv("CAPACITYROOM_TYPST"); override != "" {
		if !filepath.IsAbs(override) {
			return Toolchain{}, fmt.Errorf("CAPACITYROOM_TYPST must be an absolute path: %q", override)
		}
		info, err := os.Stat(override)
		if err != nil {
			return Toolchain{}, fmt.Errorf("CAPACITYROOM_TYPST names no file: %q", override)
		}
		if !info.Mode().IsRegular() {
			return Toolchain{}, fmt.Errorf("CAPACITYROOM_TYPST is not a regular file: %q", override)
		}
		return Toolchain{Binary: override}, nil
	}

	found, err := lookPath("typst")
	if err != nil {
		return Toolchain{}, fmt.Errorf("typst not found: set CAPACITYROOM_TYPST or add typst to PATH")
	}
	abs, err := filepath.Abs(found)
	if err != nil {
		return Toolchain{}, fmt.Errorf("typst found at %q, which could not be made absolute: %w", found, err)
	}
	return Toolchain{Binary: abs}, nil
}

// Version runs "<binary> --version" and returns the second field, e.g.
// "0.15.1" out of "typst 0.15.1 (unknown commit)". The first field must
// read exactly "typst"; a binary that answers with some other program's
// name is treated as unreadable output, not as a version. The context
// deadline is honoured; the caller passes one and this function adds none.
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

	// The context is checked before any other branch: once it has ended, a
	// killed-but-started process and a never-started one both count as
	// "stopped", not "failed" or "could not start". A caller distinguishes
	// this from a real failure with errors.Is(err, ctx.Err()).
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("typst --version stopped: %w", ctxErr)
	}

	// A nil ProcessState means the process never ran at all (e.g. the
	// binary lacks the executable bit), as opposed to having run and
	// exited. The context has already been ruled out above, so this is a
	// distinct failure the operator needs the real cause for.
	if cmd.ProcessState == nil {
		return "", fmt.Errorf("typst --version could not start: %w", runErr)
	}

	fields := strings.Fields(stdout.String())
	if runErr == nil && len(fields) >= 2 && fields[0] == "typst" {
		return fields[1], nil
	}

	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}
	return "", fmt.Errorf("typst --version failed (exit %d): %s", exitCode, firstLineOrFallback(stdout.String(), stderr.String()))
}

// firstLineOrFallback returns the first non-blank line of stdout, falling
// back to the first non-blank line of stderr when stdout has none, and to
// "no output" when neither does. A line is blank when it is empty after
// trimming white space; the returned line is itself trimmed.
func firstLineOrFallback(stdout, stderr string) string {
	if line := firstNonBlankLine(stdout); line != "" {
		return line
	}
	if line := firstNonBlankLine(stderr); line != "" {
		return line
	}
	return "no output"
}

func firstNonBlankLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
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
