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

// TestRootListsItsCommands proves commandNames' listed half names every
// real subcommand of the root, sorted, and never a hidden one or "help".
func TestRootListsItsCommands(t *testing.T) {
	cmd := newRootCmd(context.Background(), Env{}, redact.New())
	cmd.AddCommand(&cobra.Command{Use: "second", Args: func(*cobra.Command, []string) error { return nil }})

	_, listed := commandNames(cmd)
	want := []string{"report", "second"}
	if !slices.Equal(listed, want) {
		t.Fatalf("commandNames() listed = %v, want %v", listed, want)
	}
}

// TestCompletionCommandIsGone proves cobra's default completion command,
// added to every program unless turned off, does not exist here: this
// program has the commands its design names, and no other. Every row gives
// the exact same message as a plain, unrecognised command would, since
// "completion" is no longer special.
func TestCompletionCommandIsGone(t *testing.T) {
	const want = "error: unknown command; the commands are: report\n"

	tests := []struct {
		name string
		args []string
	}{
		{"completion alone", []string{"completion"}},
		{"completion bash", []string{"completion", "bash"}},
		{"completion hunter2", []string{"completion", "hunter2"}},
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
				t.Fatalf("a stream held the argument: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

// commandPaths walks cmd's own subcommands, recursively, and returns the
// invocation path of each one (its own name, preceded by its parents').
func commandPaths(cmd *cobra.Command, prefix []string) [][]string {
	var paths [][]string
	for _, c := range cmd.Commands() {
		path := append(append([]string{}, prefix...), c.Name())
		paths = append(paths, path)
		paths = append(paths, commandPaths(c, path)...)
	}
	return paths
}

// TestNoCommandRepeatsAnArgument is the guard the ruling asks for: it walks
// every command of the root, recursively, and runs each one, by its own
// path, with one extra argument no command of this program is ever meant
// to accept. Comparing function values is not possible in Go, so the
// behaviour is tested directly: whatever a command's own Args produces, no
// stream may ever hold this argument. help is included in the walk (see
// newRootCmd's early InitDefaultHelpCmd) and is exempt from having its own
// Args by construction — its own Run resolves an unrecognised topic to the
// nearest matching command's help text and never echoes it — but it is
// still run here, since the ruling asks for every command to be walked,
// not skipped.
func TestNoCommandRepeatsAnArgument(t *testing.T) {
	const marker = "zz-not-an-argument-zz"
	root := newRootCmd(context.Background(), Env{}, redact.New())

	paths := commandPaths(root, nil)
	if len(paths) == 0 {
		t.Fatal("commandPaths found no command; a walk that finds nothing guards nothing")
	}

	for _, path := range paths {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			args := append(append([]string{}, path...), marker)
			var stdout, stderr bytes.Buffer
			_ = Run(context.Background(), Env{}, args, &stdout, &stderr)
			if strings.Contains(stdout.String(), marker) || strings.Contains(stderr.String(), marker) {
				t.Fatalf("a stream held the argument: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

// TestRootHelpListsOnlyItsCommands proves the root's own --help output
// names the commands a reader can actually use (report, help) and never
// the completion command, which this program disables.
func TestRootHelpListsOnlyItsCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), Env{}, []string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, want 0; stderr = %q", code, stderr.String())
	}

	text := stdout.String()
	if !strings.Contains(text, "report") {
		t.Fatalf("help text does not mention %q: %q", "report", text)
	}
	if !strings.Contains(text, "help") {
		t.Fatalf("help text does not mention %q: %q", "help", text)
	}
	if strings.Contains(text, "completion") {
		t.Fatalf("help text mentions %q, which this program never provides: %q", "completion", text)
	}
}

// TestHiddenCommandIsStillRun proves Finding 2's ruling: a hidden command
// is still one the raw pre-check (unknownRootCommand) accepts as a real
// command to dispatch to, and cobra still runs it by name, even though the
// message for a genuinely unknown command never names it.
func TestHiddenCommandIsStillRun(t *testing.T) {
	ran := false
	hidden := &cobra.Command{
		Use:    "secret",
		Hidden: true,
		Args:   func(*cobra.Command, []string) error { return nil },
		Run:    func(*cobra.Command, []string) { ran = true },
	}

	cmd := newRootCmd(context.Background(), Env{}, redact.New())
	cmd.AddCommand(hidden)

	if msg, unknown := unknownRootCommand(cmd, []string{"secret"}); unknown {
		t.Fatalf("unknownRootCommand treated the hidden command as unknown: %q", msg)
	}

	var stdout, stderr bytes.Buffer
	cmd.SetArgs([]string{"secret"})
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !ran {
		t.Fatal("the hidden command did not run")
	}

	msg, unknown := unknownRootCommand(cmd, []string{"nope"})
	if !unknown {
		t.Fatal("unknownRootCommand accepted a genuinely unknown command")
	}
	if strings.Contains(msg, "secret") {
		t.Fatalf("the message named the hidden command: %q", msg)
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
