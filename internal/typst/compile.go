package typst

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Format names an output format of the compiler.
type Format string

// The output formats Compile can produce.
const (
	PDF Format = "pdf"
	PNG Format = "png"
)

// Job describes a single Typst compilation.
type Job struct {
	Root        string // absolute
	FontPath    string // absolute
	Source      string // absolute, inside Root
	Output      string // absolute; for PNG it contains "{p}"
	Format      Format
	Inputs      map[string]string // passed as --input key=value
	PackagePath string            // optional, absolute
	Pages       string            // optional, PNG only, for example "1,2"
	PPI         int               // optional, PNG only
}

// CompileError reports a non-zero typst exit.
type CompileError struct {
	ExitCode int
	Stderr   string
}

func (e *CompileError) Error() string {
	return fmt.Sprintf("typst compile failed (exit %d): %s", e.ExitCode, firstLineOrFallback("", e.Stderr))
}

// ErrWarning marks a compile that exited zero but reported a warning. A
// warning is treated as a failure so drifting fonts or markup never reach a
// shipped report silently.
var ErrWarning = errors.New("typst reported a warning")

// isAbsolute returns an error naming field when path is not absolute.
func isAbsolute(field, path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%s must be absolute: %q", field, path)
	}
	return nil
}

// sourceInsideRoot reports whether source lies inside root, comparing
// cleaned paths by path segment (via filepath.Rel) so that a root like
// "/a/b" does not accept a source under the sibling "/a/bc".
func sourceInsideRoot(root, source string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(source))
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// Args returns the arguments after the binary name. It is pure: it neither
// touches the filesystem nor runs anything, so it can be unit tested and
// reused to check what Compile is about to run.
func Args(job Job) ([]string, error) {
	for _, f := range []struct{ name, path string }{
		{"Root", job.Root},
		{"FontPath", job.FontPath},
		{"Source", job.Source},
		{"Output", job.Output},
	} {
		if err := isAbsolute(f.name, f.path); err != nil {
			return nil, err
		}
	}
	if job.PackagePath != "" {
		if err := isAbsolute("PackagePath", job.PackagePath); err != nil {
			return nil, err
		}
	}
	if !sourceInsideRoot(job.Root, job.Source) {
		return nil, errors.New("source must be inside root")
	}

	switch job.Format {
	case PDF, PNG:
	default:
		return nil, fmt.Errorf("unknown format: %q", job.Format)
	}

	if job.Format == PNG && !strings.Contains(job.Output, "{p}") {
		return nil, fmt.Errorf("PNG output must contain %q: %q", "{p}", job.Output)
	}
	if job.Format == PDF && job.Pages != "" {
		return nil, errors.New("pages applies only to PNG")
	}
	if job.Format == PDF && job.PPI != 0 {
		return nil, errors.New("ppi applies only to PNG")
	}
	if job.PPI < 0 {
		return nil, fmt.Errorf("ppi must not be negative: %d", job.PPI)
	}
	for key := range job.Inputs {
		if key == "" {
			return nil, errors.New("input key must not be empty")
		}
		if strings.Contains(key, "=") {
			return nil, fmt.Errorf("input key must not contain %q: %q", "=", key)
		}
	}

	args := []string{
		"compile",
		"--diagnostic-format", "short",
		"--root", job.Root,
		"--font-path", job.FontPath,
		"--ignore-system-fonts",
		"--ignore-embedded-fonts",
	}
	if job.PackagePath != "" {
		args = append(args, "--package-path", job.PackagePath)
	}
	args = append(args, "--format", string(job.Format))
	if job.Pages != "" {
		args = append(args, "--pages", job.Pages)
	}
	if job.PPI != 0 {
		args = append(args, "--ppi", strconv.Itoa(job.PPI))
	}

	keys := make([]string, 0, len(job.Inputs))
	for key := range job.Inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		args = append(args, "--input", key+"="+job.Inputs[key])
	}

	args = append(args, job.Source, job.Output)
	return args, nil
}

// hasWarning reports whether stderr, in typst's short diagnostic format,
// holds a warning: any line containing "warning:".
func hasWarning(stderr string) bool {
	for _, line := range strings.Split(stderr, "\n") {
		if strings.Contains(line, "warning:") {
			return true
		}
	}
	return false
}

// Compile runs the job with the fixed flags Args produces. It returns
// stderr in every case, so callers can log it even on success.
func (t Toolchain) Compile(ctx context.Context, job Job) (string, error) {
	args, err := Args(job)
	if err != nil {
		return "", err
	}

	cmd := exec.CommandContext(ctx, t.Binary, args...)
	cmd.Env = []string{} // run with an empty environment, not the parent's

	// See Toolchain.Version for why WaitDelay is needed: it bounds how long
	// Wait blocks on the I/O-copying goroutines after the process is killed.
	cmd.WaitDelay = 200 * time.Millisecond

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	out := stderr.String()

	// The context is checked before any other branch: once it has ended, a
	// killed-but-started process and a never-started one both count as
	// "stopped", not "failed" or "could not start". A caller distinguishes
	// this from a real compile failure with errors.Is(err, ctx.Err()).
	if ctxErr := ctx.Err(); ctxErr != nil {
		return out, fmt.Errorf("typst compile stopped: %w", ctxErr)
	}

	if runErr != nil {
		if cmd.ProcessState != nil {
			return out, &CompileError{ExitCode: cmd.ProcessState.ExitCode(), Stderr: out}
		}
		return out, fmt.Errorf("typst compile could not start: %w", runErr)
	}

	if hasWarning(out) {
		return out, fmt.Errorf("%w: %s", ErrWarning, out)
	}
	return out, nil
}
