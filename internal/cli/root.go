// Package cli wires the k8s-capacity-report command line: argument parsing,
// output routing, and exit codes.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcr "github.com/marceloalvescf/k8s-capacity-report"
)

// Run builds the root command, executes it against args, and returns the
// process exit code. Output streams are passed explicitly rather than using
// os.Stdout/os.Stderr so callers (including tests) control where they go.
func Run(args []string, stdout, stderr io.Writer) int {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// Cobra's own error/usage printing is silenced; the error is printed
	// once here so it always lands on stderr regardless of the command
	// that produced it.
	if err := cmd.Execute(); err != nil {
		// Discard the error from writing to stderr; if it fails,
		// there is nowhere left to report the failure.
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "k8s-capacity-report",
		Short:   "Generate Kubernetes capacity reports from Prometheus metrics.",
		Version: kcr.Version(),
		// No subcommands exist yet, so any positional argument is a
		// mistake; NoArgs turns it into an "unknown command" error
		// instead of silently accepting it.
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		Run: func(cmd *cobra.Command, _ []string) {
			// A Run function is required for cobra to validate Args at
			// all (an unrunnable command short-circuits straight to
			// help); with no subcommands yet, running with no arguments
			// just shows the help.
			// Help is written to the caller's writer and cannot report
			// a write failure anywhere else, so discard the error.
			_ = cmd.Help()
		},
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	return cmd
}
