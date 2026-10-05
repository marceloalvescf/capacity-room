package cli

import (
	"bytes"
	"context"
	kcr "github.com/marceloalvescf/k8s-capacity-report"
	"slices"
	"sort"
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
	if want := "k8s-capacity-report " + kcr.Version() + "\n"; stdout.String() != want {
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
	if want := "error: unknown command; the commands are: admin, report, serve\n"; stderr.String() != want {
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
	const want = "error: unknown command; the commands are: admin, report, serve\n"

	tests := []struct {
		name string
		args []string
	}{
		{"bare argument", []string{"hunter2"}},
		{"argument then a valid command and flag", []string{"hunter2", "report", "--check"}},
		{"misspelled command with a flag", []string{"reprot", "--check"}},
		{"argument then an unknown flag", []string{"hunter2", "--token=abcdefgh"}},
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
	want := []string{"admin", "report", "second", "serve"}
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
	const want = "error: unknown command; the commands are: admin, report, serve\n"

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

// walkedCommandNames returns the space-joined path of every command
// commandPaths finds under root, sorted.
func walkedCommandNames(root *cobra.Command) []string {
	paths := commandPaths(root, nil)
	names := make([]string, len(paths))
	for i, path := range paths {
		names[i] = strings.Join(path, " ")
	}
	sort.Strings(names)
	return names
}

// TestNoCommandRepeatsAnArgument walks every command of the root,
// recursively, checks that the set of commands found is exactly the
// expected one, and then runs each command, by its own path, with two
// extra arguments no command of this program accepts: no stream may hold
// that argument, whatever the command's own Args produces. Comparing
// function values is not possible in Go, so this is the behaviour itself,
// tested directly. A command added to the tree later changes the set this
// test expects, which fails until its name is added here — the moment to
// look at its Args.
func TestNoCommandRepeatsAnArgument(t *testing.T) {
	const marker = "zz-not-an-argument-zz"
	root := newRootCmd(context.Background(), Env{}, redact.New())

	got := walkedCommandNames(root)
	want := []string{"admin", "admin bootstrap-admin", "admin migrate", "admin probe", "help", "report", "serve"}
	if !slices.Equal(got, want) {
		t.Fatalf("walkedCommandNames() = %v, want %v", got, want)
	}

	for _, path := range commandPaths(root, nil) {
		t.Run(strings.Join(path, " "), func(t *testing.T) {
			// Two arguments, so that probe, which takes one URL and repeats
			// it on purpose, refuses by count instead.
			args := append(append([]string{}, path...), marker, marker)
			var stdout, stderr bytes.Buffer
			_ = Run(context.Background(), Env{}, args, &stdout, &stderr)
			if strings.Contains(stdout.String(), marker) || strings.Contains(stderr.String(), marker) {
				t.Fatalf("a stream held the argument: stdout=%q stderr=%q", stdout.String(), stderr.String())
			}
		})
	}
}

// TestGuardNoticesAMissingCommand proves that walkedCommandNames's
// comparison, the one TestNoCommandRepeatsAnArgument relies on, actually
// notices a command dropping out of the walk: with "help" removed from a
// root built the same way, the set walkedCommandNames returns no longer
// equals the expected set. This test passes precisely because the
// comparison fails on the altered root; it does not run
// TestNoCommandRepeatsAnArgument itself, which must keep passing on the
// real, unaltered root.
func TestGuardNoticesAMissingCommand(t *testing.T) {
	root := newRootCmd(context.Background(), Env{}, redact.New())
	for _, c := range root.Commands() {
		if c.Name() == "help" {
			root.RemoveCommand(c)
		}
	}

	got := walkedCommandNames(root)
	want := []string{"admin", "admin bootstrap-admin", "admin migrate", "admin probe", "help", "report", "serve"}
	if slices.Equal(got, want) {
		t.Fatalf("walkedCommandNames() = %v, want it to differ from %v now that help is missing", got, want)
	}
}

// TestHiddenCompletionIsRefused proves that cobra's own hidden shell-
// completion commands, __complete and __completeNoDesc, added inside
// Execute regardless of any option, never reach that point here: the raw
// pre-check (unknownRootCommand) runs first and rejects them exactly as it
// would any other name it does not yet recognise.
func TestHiddenCompletionIsRefused(t *testing.T) {
	const want = "error: unknown command; the commands are: admin, report, serve\n"

	tests := []struct {
		name string
		args []string
	}{
		{"__complete with an argument", []string{"__complete", "hunter2"}},
		{"__completeNoDesc with an argument", []string{"__completeNoDesc", "hunter2"}},
		{"__complete naming report", []string{"__complete", "report", "hunter2"}},
		{"__complete naming report and a flag", []string{"__complete", "report", "--token", "hunter22"}},
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

// TestHiddenCommandIsStillRun proves that a hidden command is still one the
// raw pre-check (unknownRootCommand) accepts as a real command to dispatch
// to, and cobra still runs it by name, even though the message for a
// genuinely unknown command never names it.
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
