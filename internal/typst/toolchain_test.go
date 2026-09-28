package typst

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeScript writes an executable shell script into a fresh temp directory
// and returns its absolute path. Scripts start with #!/bin/sh and run with
// an empty environment, so they use shell built-ins only and name any other
// program (e.g. /bin/sleep) by absolute path.
func writeScript(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func unreachableLookPath(string) (string, error) {
	return "", errors.New("lookPath should not be called")
}

func TestLocateUsesEnvironment(t *testing.T) {
	path := writeScript(t, "#!/bin/sh\nexit 0\n")
	getenv := func(key string) string {
		if key == "KCR_TYPST" {
			return path
		}
		return ""
	}

	got, err := Locate(getenv, unreachableLookPath)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if got.Binary != path {
		t.Fatalf("Binary = %q, want %q", got.Binary, path)
	}
}

func TestLocateRejectsRelativePath(t *testing.T) {
	getenv := func(key string) string {
		if key == "KCR_TYPST" {
			return "bin/typst"
		}
		return ""
	}

	_, err := Locate(getenv, unreachableLookPath)
	if err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("Locate() error = %v, want it to contain %q", err, "must be an absolute path")
	}
}

func TestLocateRejectsMissingFile(t *testing.T) {
	getenv := func(key string) string {
		if key == "KCR_TYPST" {
			return "/nonexistent/typst"
		}
		return ""
	}

	_, err := Locate(getenv, unreachableLookPath)
	if err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("Locate() error = %v, want it to contain %q", err, "must be an absolute path")
	}
}

func TestLocateRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	getenv := func(key string) string {
		if key == "KCR_TYPST" {
			return dir
		}
		return ""
	}

	_, err := Locate(getenv, unreachableLookPath)
	if err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("Locate() error = %v, want it to contain %q", err, "must be an absolute path")
	}
}

func TestLocateFallsBackToPath(t *testing.T) {
	path := writeScript(t, "#!/bin/sh\nexit 0\n")
	getenv := func(string) string { return "" }
	lookPath := func(name string) (string, error) {
		if name != "typst" {
			t.Fatalf("lookPath called with %q, want %q", name, "typst")
		}
		return path, nil
	}

	got, err := Locate(getenv, lookPath)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if got.Binary != path {
		t.Fatalf("Binary = %q, want %q", got.Binary, path)
	}
	if !filepath.IsAbs(got.Binary) {
		t.Fatalf("Binary = %q, want an absolute path", got.Binary)
	}
}

func TestLocateNotFound(t *testing.T) {
	getenv := func(string) string { return "" }
	lookPath := func(string) (string, error) {
		return "", errors.New("not found")
	}

	_, err := Locate(getenv, lookPath)
	want := "typst not found: set KCR_TYPST or add typst to PATH"
	if err == nil || err.Error() != want {
		t.Fatalf("Locate() error = %v, want %q", err, want)
	}
}

func TestVersionParsesSecondField(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'typst 0.15.1 (abc123)'\n")}

	got, err := tc.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got != "0.15.1" {
		t.Fatalf("Version() = %q, want %q", got, "0.15.1")
	}
}

func TestVersionFailsOnNonZeroExit(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho boom >&2\nexit 3\n")}

	_, err := tc.Version(context.Background())
	if err == nil {
		t.Fatalf("Version() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "exit 3") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Version() error = %v, want it to contain %q and %q", err, "exit 3", "boom")
	}
}

func TestVersionFailsOnShortOutput(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho typst\n")}

	_, err := tc.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "typst --version failed") {
		t.Fatalf("Version() error = %v, want it to contain %q", err, "typst --version failed")
	}
}

func TestVersionFailsOnNoOutput(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\nexit 0\n")}

	_, err := tc.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no output") {
		t.Fatalf("Version() error = %v, want it to contain %q", err, "no output")
	}
}

func TestVersionMessageSkipsLeadingBlankLines(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho\necho boom\nexit 3\n")}

	_, err := tc.Version(context.Background())
	if err == nil {
		t.Fatalf("Version() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "exit 3") || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Version() error = %v, want it to contain %q and %q", err, "exit 3", "boom")
	}
}

func TestVersionMessageFallsBackToStderrWhenStdoutIsBlank(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho\necho\necho 'from stderr' >&2\nexit 3\n")}

	_, err := tc.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "from stderr") {
		t.Fatalf("Version() error = %v, want it to contain %q", err, "from stderr")
	}
}

func TestVersionMessageSaysNoOutputWhenBothAreBlank(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho\necho >&2\nexit 0\n")}

	_, err := tc.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no output") {
		t.Fatalf("Version() error = %v, want it to contain %q", err, "no output")
	}
}

func TestVersionReportsWhyItCouldNotStart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	tc := Toolchain{Binary: path}

	_, err := tc.Version(context.Background())
	if err == nil || !strings.Contains(err.Error(), "typst --version could not start:") {
		t.Fatalf("Version() error = %v, want it to contain %q", err, "typst --version could not start:")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("Version() error = %v, want errors.Is(err, fs.ErrPermission)", err)
	}
	if strings.Contains(err.Error(), "exit -1") {
		t.Fatalf("Version() error = %v, want it not to contain %q", err, "exit -1")
	}
}

func TestVersionHonoursDeadline(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\n/bin/sleep 5\n")}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := tc.Version(ctx)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Version() error = nil, want an error")
	}
	if elapsed > time.Second {
		t.Fatalf("Version() took %s, want under 1s", elapsed)
	}
}

func TestVersionUsesEmptyEnvironment(t *testing.T) {
	// /bin/sh (bash in POSIX mode on this machine) fills in its own default
	// PATH whenever none is inherited, so an empty PATH cannot be observed
	// directly. Instead these sentinels prove non-inheritance: if cmd.Env
	// were nil rather than an empty slice, the child would see these exact
	// parent values instead of an empty HOME and a shell-supplied PATH.
	const sentinelHome = "/should-not-be-inherited-home"
	const sentinelPath = "/should-not-be-inherited-path"
	t.Setenv("HOME", sentinelHome)
	t.Setenv("PATH", sentinelPath)

	script := "#!/bin/sh\n" +
		"if [ -z \"$HOME\" ] && [ \"$PATH\" != \"" + sentinelPath + "\" ]; then\n" +
		"  echo 'typst 0.15.1 (x)'\n" +
		"else\n" +
		"  exit 9\n" +
		"fi\n"
	tc := Toolchain{Binary: writeScript(t, script)}

	got, err := tc.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if got != "0.15.1" {
		t.Fatalf("Version() = %q, want %q", got, "0.15.1")
	}
}

func TestProbeAcceptsRequiredVersion(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'typst 0.15.1 (x)'\n")}

	if err := tc.Probe(context.Background()); err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
}

func TestProbeRejectsOtherVersion(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'typst 0.15.0 (x)'\n")}

	err := tc.Probe(context.Background())
	if err == nil {
		t.Fatalf("Probe() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "typst 0.15.0 found at") || !strings.Contains(err.Error(), "requires 0.15.1") {
		t.Fatalf("Probe() error = %v, want it to contain %q and %q", err, "typst 0.15.0 found at", "requires 0.15.1")
	}
}
