package typst

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeScript writes an executable shell script into a fresh temp directory
// and returns its absolute path. Scripts start with #!/bin/sh and run with
// an empty environment, so they use shell built-ins only and name any other
// program (see toolPath) by absolute path.
func writeScript(t *testing.T, script string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "typst")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

// toolPath returns the absolute path of the named program, found on the
// test process's PATH, for a script that cannot search for it.
func toolPath(t *testing.T, name string) string {
	t.Helper()
	path, err := exec.LookPath(name)
	if err != nil {
		t.Fatalf("finding %s on PATH: %v", name, err)
	}
	return path
}

func unreachableLookPath(string) (string, error) {
	return "", errors.New("lookPath should not be called")
}

func TestLocateUsesEnvironment(t *testing.T) {
	path := writeScript(t, "#!/bin/sh\nexit 0\n")
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TYPST" {
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

func TestLocateRejectsRelativeOverride(t *testing.T) {
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TYPST" {
			return "bin/typst"
		}
		return ""
	}

	_, err := Locate(getenv, unreachableLookPath)
	if err == nil || !strings.Contains(err.Error(), "must be an absolute path") {
		t.Fatalf("Locate() error = %v, want it to contain %q", err, "must be an absolute path")
	}
}

func TestLocateRejectsMissingOverride(t *testing.T) {
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TYPST" {
			return "/nonexistent/typst"
		}
		return ""
	}

	_, err := Locate(getenv, unreachableLookPath)
	if err == nil || !strings.Contains(err.Error(), "names no file") {
		t.Fatalf("Locate() error = %v, want it to contain %q", err, "names no file")
	}
}

func TestLocateRejectsFolderOverride(t *testing.T) {
	dir := t.TempDir()
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TYPST" {
			return dir
		}
		return ""
	}

	_, err := Locate(getenv, unreachableLookPath)
	if err == nil || !strings.Contains(err.Error(), "is not a regular file") {
		t.Fatalf("Locate() error = %v, want it to contain %q", err, "is not a regular file")
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
	want := "typst not found: set CAPACITYROOM_TYPST or add typst to PATH"
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

func TestVersionRejectsImpostorProgramName(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'im-not-typst 0.15.1 (x)'\n")}

	_, err := tc.Version(context.Background())
	if err == nil {
		t.Fatalf("Version() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "im-not-typst 0.15.1 (x)") {
		t.Fatalf("Version() error = %v, want it to contain the line received %q", err, "im-not-typst 0.15.1 (x)")
	}
}

func TestProbeRejectsImpostorProgramName(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'im-not-typst 0.15.1 (x)'\n")}

	err := tc.Probe(context.Background())
	if err == nil {
		t.Fatalf("Probe() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "im-not-typst 0.15.1 (x)") {
		t.Fatalf("Probe() error = %v, want it to contain the line received %q", err, "im-not-typst 0.15.1 (x)")
	}
}

func TestVersionRejectsWrongCaseProgramName(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'Typst 0.15.1'\n")}

	_, err := tc.Version(context.Background())
	if err == nil {
		t.Fatalf("Version() error = nil, want an error")
	}
}

func TestVersionAcceptsUnknownCommitSuffix(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'typst 0.15.1 (unknown commit)'\n")}

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
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\n"+toolPath(t, "sleep")+" 5\n")}
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
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Version() error = %v, want errors.Is(err, context.DeadlineExceeded)", err)
	}
	if !strings.HasPrefix(err.Error(), "typst --version stopped:") {
		t.Fatalf("Version() error = %v, want it to start with %q", err, "typst --version stopped:")
	}
}

func TestVersionWithEndedContext(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'typst 0.15.1 (x)'\n")}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := tc.Version(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Version() error = %v, want errors.Is(err, context.Canceled)", err)
	}
	if strings.Contains(err.Error(), "could not start") {
		t.Fatalf("Version() error = %v, want it not to contain %q", err, "could not start")
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

func TestLocateMakesRelativeLookupAbsolute(t *testing.T) {
	// lookPath can return a relative path (e.g. found via a relative PATH
	// entry); Locate must resolve it against the working directory before
	// handing it back, since a later os/exec call needs an absolute path.
	relative := filepath.Join("bin", "typst")
	getenv := func(string) string { return "" }
	lookPath := func(string) (string, error) {
		return relative, nil
	}

	got, err := Locate(getenv, lookPath)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if !filepath.IsAbs(got.Binary) {
		t.Fatalf("Binary = %q, want an absolute path", got.Binary)
	}
	if !strings.HasSuffix(got.Binary, relative) {
		t.Fatalf("Binary = %q, want it to end in %q", got.Binary, relative)
	}
}

func TestLocateAcceptsSymbolicLinkOverride(t *testing.T) {
	target := writeScript(t, "#!/bin/sh\nexit 0\n")
	link := filepath.Join(t.TempDir(), "typst-link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	getenv := func(key string) string {
		if key == "CAPACITYROOM_TYPST" {
			return link
		}
		return ""
	}

	got, err := Locate(getenv, unreachableLookPath)
	if err != nil {
		t.Fatalf("Locate() error = %v", err)
	}
	if got.Binary != link {
		t.Fatalf("Binary = %q, want %q", got.Binary, link)
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
