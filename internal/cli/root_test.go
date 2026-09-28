package cli

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
)

func TestRunVersionFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), Env{}, []string{"--version"}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if want := "k8s-capacity-report 0.0.0\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want %q", stdout.String(), want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestRunNoArgumentsPrintsHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), Env{}, []string{}, &stdout, &stderr)

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), "Usage:")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), Env{}, []string{"nope"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if want := "error: unknown command; the commands are: report\n"; stderr.String() != want {
		t.Fatalf("stderr = %q, want %q", stderr.String(), want)
	}
	if strings.Contains(stdout.String(), "nope") || strings.Contains(stderr.String(), "nope") {
		t.Fatalf("a stream held the argument: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
}

// TestRootRepeatsNoArgument proves that a stray first argument at the root
// never reaches a message, whatever else is wrong with the rest of the
// line: a bad flag elsewhere (one meant for the command the user
// misspelled) would otherwise make cobra's own flag parsing fail first,
// with a different message, before the root's own check ever saw the
// argument.
func TestRootRepeatsNoArgument(t *testing.T) {
	const want = "error: unknown command; the commands are: report\n"

	tests := []struct {
		name string
		args []string
	}{
		{"bare argument", []string{"hunter2"}},
		{"argument then a valid command and flag", []string{"hunter2", "report", "--check"}},
		{"misspelled command with a flag", []string{"reprot", "--check"}},
		{"argument then an unknown flag", []string{"hunter2", "--token=abc"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), Env{}, tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("code = %d, want 2; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
			if stderr.String() != want {
				t.Fatalf("stderr = %q, want %q", stderr.String(), want)
			}
			if strings.Contains(stdout.String(), "hunter2") || strings.Contains(stderr.String(), "hunter2") {
				t.Fatalf("a stream held the first argument: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

// TestRootListsItsCommands proves availableCommands lists every real
// subcommand of the root, sorted, and never the ones cobra provides by
// itself.
func TestRootListsItsCommands(t *testing.T) {
	cmd := newRootCmd(context.Background(), Env{}, redact.New())
	cmd.AddCommand(&cobra.Command{Use: "second"})

	got := availableCommands(cmd)
	want := []string{"report", "second"}
	if !slices.Equal(got, want) {
		t.Fatalf("availableCommands() = %v, want %v", got, want)
	}
}

// TestHelpAndVersionStillWork confirms the paths TestRootRepeatsNoArgument's
// new checks must not disturb: help, in every accepted form, and the
// version flag, all still succeed exactly as before.
func TestHelpAndVersionStillWork(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"--help", []string{"--help"}},
		{"help", []string{"help"}},
		{"help report", []string{"help", "report"}},
		{"--version", []string{"--version"}},
		{"no argument", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), Env{}, tt.args, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			if stdout.Len() == 0 {
				t.Fatalf("stdout is empty, want it to hold something")
			}
		})
	}
}

func TestRunUnknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), Env{}, []string{"--nope"}, &stdout, &stderr)

	if code != 2 {
		t.Fatalf("Run() code = %d, want 2", code)
	}
	if want := "unknown flag"; !strings.Contains(stderr.String(), want) {
		t.Fatalf("stderr = %q, want it to contain %q", stderr.String(), want)
	}
}
