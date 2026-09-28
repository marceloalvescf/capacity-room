package typst

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// validJob returns a Job that passes Args validation, rooted and sourced
// inside t.TempDir() so callers can mutate one field at a time to exercise
// a single rejection rule.
func validJob(t *testing.T) Job {
	t.Helper()
	root := t.TempDir()
	fonts := t.TempDir()
	return Job{
		Root:     root,
		FontPath: fonts,
		Source:   filepath.Join(root, "doc.typ"),
		Output:   filepath.Join(root, "out.pdf"),
		Format:   PDF,
	}
}

func TestArgsOrderForPDF(t *testing.T) {
	job := Job{
		Root:     "/root",
		FontPath: "/fonts",
		Source:   "/root/doc.typ",
		Output:   "/root/out.pdf",
		Format:   PDF,
		Inputs:   map[string]string{"b": "2", "a": "1"},
	}
	want := []string{
		"compile",
		"--diagnostic-format", "short",
		"--root", "/root",
		"--font-path", "/fonts",
		"--ignore-system-fonts", "--ignore-embedded-fonts",
		"--format", "pdf",
		"--input", "a=1",
		"--input", "b=2",
		"/root/doc.typ",
		"/root/out.pdf",
	}
	got, err := Args(job)
	if err != nil {
		t.Fatalf("Args() error = %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Args() = %#v, want %#v", got, want)
	}
}

func TestArgsOrderForPNG(t *testing.T) {
	job := Job{
		Root:        "/root",
		FontPath:    "/fonts",
		Source:      "/root/doc.typ",
		Output:      "/root/{p}.png",
		Format:      PNG,
		Pages:       "1,2",
		PPI:         144,
		PackagePath: "/pkg",
	}
	want := []string{
		"compile",
		"--diagnostic-format", "short",
		"--root", "/root",
		"--font-path", "/fonts",
		"--ignore-system-fonts", "--ignore-embedded-fonts",
		"--package-path", "/pkg",
		"--format", "png",
		"--pages", "1,2",
		"--ppi", "144",
		"/root/doc.typ",
		"/root/{p}.png",
	}
	got, err := Args(job)
	if err != nil {
		t.Fatalf("Args() error = %v", err)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("Args() = %#v, want %#v", got, want)
	}
}

func TestArgsAlwaysHasTheFourFixedFlags(t *testing.T) {
	job := validJob(t)
	got, err := Args(job)
	if err != nil {
		t.Fatalf("Args() error = %v", err)
	}
	for _, flag := range []string{"--root", "--font-path", "--ignore-system-fonts", "--ignore-embedded-fonts"} {
		if !slices.Contains(got, flag) {
			t.Fatalf("Args() = %#v, missing %q", got, flag)
		}
	}
}

func TestArgsRejectsRelativePath(t *testing.T) {
	cases := []struct {
		name   string
		field  string
		mutate func(*Job)
	}{
		{"Root", "Root", func(j *Job) { j.Root = "root" }},
		{"FontPath", "FontPath", func(j *Job) { j.FontPath = "fonts" }},
		{"Source", "Source", func(j *Job) { j.Source = "doc.typ" }},
		{"Output", "Output", func(j *Job) { j.Output = "out.pdf" }},
		{"PackagePath", "PackagePath", func(j *Job) { j.PackagePath = "pkg" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			job := validJob(t)
			tc.mutate(&job)
			_, err := Args(job)
			if err == nil {
				t.Fatalf("Args() error = nil, want error naming %q", tc.field)
			}
			if !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Args() error = %v, want it to name %q", err, tc.field)
			}
		})
	}
}

func TestArgsRejectsSourceOutsideRoot(t *testing.T) {
	job := Job{
		Root:     "/a",
		FontPath: "/fonts",
		Source:   "/b/x.typ",
		Output:   "/a/out.pdf",
		Format:   PDF,
	}
	_, err := Args(job)
	want := "source must be inside root"
	if err == nil || err.Error() != want {
		t.Fatalf("Args() error = %v, want %q", err, want)
	}
}

func TestArgsRejectsSiblingPrefix(t *testing.T) {
	job := Job{
		Root:     "/a/b",
		FontPath: "/fonts",
		Source:   "/a/bc/x.typ",
		Output:   "/a/b/out.pdf",
		Format:   PDF,
	}
	_, err := Args(job)
	want := "source must be inside root"
	if err == nil || err.Error() != want {
		t.Fatalf("Args() error = %v, want %q", err, want)
	}
}

func TestArgsRejectsUnknownFormat(t *testing.T) {
	job := validJob(t)
	job.Format = Format("svg")
	_, err := Args(job)
	if err == nil {
		t.Fatalf("Args() error = nil, want error")
	}
}

func TestArgsRejectsPNGWithoutPagePlaceholder(t *testing.T) {
	job := validJob(t)
	job.Format = PNG
	job.Output = filepath.Join(job.Root, "page.png")
	_, err := Args(job)
	if err == nil {
		t.Fatalf("Args() error = nil, want error")
	}
}

func TestArgsRejectsPagesForPDF(t *testing.T) {
	job := validJob(t)
	job.Pages = "1,2"
	_, err := Args(job)
	if err == nil {
		t.Fatalf("Args() error = nil, want error")
	}
}

func TestArgsRejectsPPIForPDF(t *testing.T) {
	job := validJob(t)
	job.PPI = 144
	_, err := Args(job)
	if err == nil {
		t.Fatalf("Args() error = nil, want error")
	}
}

func TestArgsRejectsNegativePPI(t *testing.T) {
	job := validJob(t)
	job.Format = PNG
	job.Output = filepath.Join(job.Root, "{p}.png")
	job.PPI = -1
	_, err := Args(job)
	if err == nil {
		t.Fatalf("Args() error = nil, want error")
	}
}

func TestArgsRejectsBadInputKey(t *testing.T) {
	cases := []string{"", "a=b"}
	for _, key := range cases {
		t.Run(strconv.Quote(key), func(t *testing.T) {
			job := validJob(t)
			job.Inputs = map[string]string{key: "value"}
			_, err := Args(job)
			if err == nil {
				t.Fatalf("Args() error = nil, want error for key %q", key)
			}
		})
	}
}

func TestCompileReturnsStderrOnSuccess(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\nexit 0\n")}
	job := validJob(t)

	stderr, err := tc.Compile(context.Background(), job)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	if stderr != "" {
		t.Fatalf("Compile() stderr = %q, want empty", stderr)
	}
}

func TestCompileFailsOnNonZeroExit(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'x.typ:1:1: error: boom' >&2\nexit 1\n")}
	job := validJob(t)

	_, err := tc.Compile(context.Background(), job)
	var compileErr *CompileError
	if !errors.As(err, &compileErr) {
		t.Fatalf("Compile() error = %v, want *CompileError", err)
	}
	if compileErr.ExitCode != 1 {
		t.Fatalf("CompileError.ExitCode = %d, want 1", compileErr.ExitCode)
	}
	if !strings.Contains(compileErr.Error(), "boom") {
		t.Fatalf("CompileError.Error() = %q, want it to contain %q", compileErr.Error(), "boom")
	}
}

func TestCompileFailsOnWarning(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\necho 'x.typ:2:1: warning: unused' >&2\nexit 0\n")}
	job := validJob(t)

	_, err := tc.Compile(context.Background(), job)
	if !errors.Is(err, ErrWarning) {
		t.Fatalf("Compile() error = %v, want errors.Is(err, ErrWarning)", err)
	}
}

func TestCompilePassesArguments(t *testing.T) {
	recorded := filepath.Join(t.TempDir(), "args")
	script := "#!/bin/sh\nfor a in \"$@\"; do echo \"$a\" >> \"" + recorded + "\"; done\nexit 0\n"
	tc := Toolchain{Binary: writeScript(t, script)}
	job := validJob(t)

	if _, err := tc.Compile(context.Background(), job); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	want, err := Args(job)
	if err != nil {
		t.Fatalf("Args() error = %v", err)
	}
	raw, err := os.ReadFile(recorded)
	if err != nil {
		t.Fatalf("reading recorded args: %v", err)
	}
	got := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	if !slices.Equal(got, want) {
		t.Fatalf("recorded args = %#v, want %#v", got, want)
	}
}

func TestCompileUsesEmptyEnvironment(t *testing.T) {
	// See TestVersionUsesEmptyEnvironment in toolchain_test.go: /bin/sh
	// supplies its own PATH when none is inherited, so these sentinels are
	// how non-inheritance is observed rather than an empty PATH itself.
	const sentinelHome = "/should-not-be-inherited-home"
	const sentinelPath = "/should-not-be-inherited-path"
	t.Setenv("HOME", sentinelHome)
	t.Setenv("PATH", sentinelPath)

	script := "#!/bin/sh\n" +
		"if [ -n \"$HOME\" ] || [ \"$PATH\" = \"" + sentinelPath + "\" ]; then\n" +
		"  exit 9\n" +
		"fi\n" +
		"exit 0\n"
	tc := Toolchain{Binary: writeScript(t, script)}
	job := validJob(t)

	_, err := tc.Compile(context.Background(), job)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func TestCompileHonoursDeadline(t *testing.T) {
	tc := Toolchain{Binary: writeScript(t, "#!/bin/sh\n/bin/sleep 5\n")}
	job := validJob(t)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := tc.Compile(ctx, job)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("Compile() error = nil, want an error")
	}
	if elapsed > time.Second {
		t.Fatalf("Compile() took %s, want under 1s", elapsed)
	}
}
