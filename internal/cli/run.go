// Package cli wires the k8s-capacity-report command line: argument parsing,
// output routing, and exit codes.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	kcr "github.com/marceloalvescf/k8s-capacity-report"
	"github.com/marceloalvescf/k8s-capacity-report/internal/catalog"
	collectpkg "github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/pipeline"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
	"github.com/marceloalvescf/k8s-capacity-report/internal/render"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
)

// Env is everything the command takes from outside itself. Every field is
// injected so a test never touches the real process environment, PATH,
// clock, or random source.
type Env struct {
	Getenv   func(string) string
	LookPath func(string) (string, error)
	Now      func() time.Time
	NewID    func() (string, error)
	Stdin    io.Reader // read by the commands that take a secret from it
	TempDir  string    // parent of the work folder; empty means os.TempDir()
	Version  string    // written to the snapshot as the generator version
}

// RealEnv returns the environment of the running process.
func RealEnv() Env {
	return Env{
		Getenv:   os.Getenv,
		LookPath: exec.LookPath,
		Now:      time.Now,
		Stdin:    os.Stdin,
		NewID: func() (string, error) {
			id, err := uuid.NewRandom()
			if err != nil {
				return "", err
			}
			return id.String(), nil
		},
		Version: kcr.Version(),
	}
}

// tokenValuesFromArgs finds every value a --token flag holds in args,
// however it was spelled: --token=<value> or --token <value>. It reads the
// raw argument list itself, before cobra parses anything, so that a token
// is known to the redactor even when a broken flag elsewhere stops cobra
// from ever calling report's own RunE.
func tokenValuesFromArgs(args []string) []string {
	var values []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case strings.HasPrefix(arg, "--token="):
			values = append(values, strings.TrimPrefix(arg, "--token="))
		case arg == "--token":
			if i+1 < len(args) {
				values = append(values, args[i+1])
			}
		}
	}
	return values
}

// Run builds the root command, executes it against args, and returns the
// process exit code.
//
// A --token value too short to survive redaction without mangling every
// unrelated occurrence of its own text is refused first, on the raw
// arguments, before anything else below: before the Redactor is built, so
// that value is never added to it, and before cobra reads anything, so
// that a broken flag elsewhere never reports its own message instead. An
// empty --token counts as not given, not as too short.
//
// One redact.Redactor is made here, from every remaining --token value
// tokenValuesFromArgs can find in args, and both streams are wrapped by it
// before cobra ever sees them: nothing below this function writes to a raw
// stream. The same Redactor is handed down to the report command, which
// adds the configuration's own token to it, in place, the moment
// config.Load returns (see runCheck, runSnapshot, and runPDF); every writer
// already built from that Redactor sees the addition immediately,
// including one the panic recovery below reaches for after the fact — so a
// panic anywhere after the token is known is redacted, not only a panic
// that happens to occur before any mode function is even called.
//
// The recovery sits here, above cmd.Execute(), rather than inside the
// report command's own logic, so that it also catches a panic from
// anything cobra itself might do while resolving or parsing.
func Run(ctx context.Context, env Env, args []string, stdout, stderr io.Writer) (code int) {
	tokenValues := tokenValuesFromArgs(args)
	for _, value := range tokenValues {
		if err := config.CheckToken(value); err != nil {
			_, _ = fmt.Fprintf(stderr, "error: %s\n", err)
			return 2
		}
	}

	redactor := redact.New(tokenValues...)
	wrappedOut := redactor.Writer(stdout)
	wrappedErr := redactor.Writer(stderr)

	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(wrappedErr, "error: internal failure: %s\n", oneLine(redactor, fmt.Sprint(r)))
			code = 1
		}
	}()

	cmd := newRootCmd(ctx, env, redactor)

	// A stray first argument that names no command is caught here, before
	// cobra ever runs: if a later, unrelated flag is also wrong (one meant
	// for the command the user misspelled, such as `hunter2 --check`),
	// pflag would otherwise fail on that flag while parsing root's own
	// flags — root has no --check — and its "unknown flag" message would
	// fire first, before root's own Args below ever saw the stray argument
	// at all. Checking here, on the raw arguments, means the one message
	// below is what every such invocation gets, regardless of what else is
	// wrong with the rest of the line.
	if msg, ok := unknownRootCommand(cmd, args); ok {
		_, _ = fmt.Fprintf(wrappedErr, "error: %s\n", msg)
		return 2
	}

	cmd.SetArgs(args)
	cmd.SetOut(wrappedOut)
	cmd.SetErr(wrappedErr)

	if err := cmd.Execute(); err != nil {
		var status *exitStatus
		if errors.As(err, &status) {
			return status.code
		}
		var failure *exitError
		if errors.As(err, &failure) {
			_, _ = fmt.Fprintf(wrappedErr, "error: %s\n", oneLine(redactor, failure.Error()))
			return failure.code
		}
		// Every failure prints exactly one "error: <message>" line, cobra's
		// own errors included (an unknown flag, or report's own refusal of
		// a positional argument), each as a single Write through the
		// redactor already wrapping wrappedErr.
		_, _ = fmt.Fprintf(wrappedErr, "error: %s\n", oneLine(redactor, err.Error()))
		return 2
	}
	return 0
}

// oneLine redacts message and then replaces every run of white space in it,
// newlines included, by one space, so that a driver's or the system's
// message of several lines still prints as one line. Redacting first means
// that no secret escapes the redactor by having its white space changed.
func oneLine(redactor *redact.Redactor, message string) string {
	return strings.Join(strings.Fields(redactor.Text(message)), " ")
}

func newRootCmd(ctx context.Context, env Env, redactor *redact.Redactor) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "k8s-capacity-report",
		Short:   "Generate Kubernetes capacity reports from Prometheus metrics.",
		Version: kcr.Version(),
		// No positional argument belongs at the root. Run's own check above
		// (unknownRootCommand) catches every case this project's tests
		// exercise before cobra ever gets here; this is the same check,
		// kept as the command's own backstop for whatever reaches it
		// anyway, so this Args never falls back to cobra.NoArgs, whose
		// message quotes the argument itself (see report's own Args, which
		// closes the same class of leak one level down).
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return errors.New(unknownCommandMessage(cmd))
			}
			return nil
		},
		// Cobra's own "Did you mean this?" suggestions are one more text it
		// would otherwise write on its own; turned off, on principle, along
		// with everything else this package already writes itself.
		DisableSuggestions: true,
		// This program has the commands its design names (admin,
		// report, and serve) and no other. Cobra's
		// default completion command is not one of them, and it carries
		// its own unmodified cobra.NoArgs, which repeats a stray argument
		// verbatim — the same leak the root's own Args function above
		// avoids, one command over. Removed outright, rather than given
		// its own Args, since the design never asked for it.
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		SilenceErrors:     true,
		SilenceUsage:      true,
		Run: func(cmd *cobra.Command, _ []string) {
			// A Run function is required for cobra to validate Args at all
			// (an unrunnable command short-circuits straight to help); with
			// no arguments, running the bare command just shows the help.
			// Help is written to the caller's writer and cannot report a
			// write failure anywhere else, so discard the error.
			_ = cmd.Help()
		},
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.AddCommand(newAdminCmd(ctx, env, redactor))
	cmd.AddCommand(newReportCmd(ctx, env, redactor))
	cmd.AddCommand(newServeCmd(ctx, env, redactor))
	// Cobra only adds its default "help" command lazily, inside Execute();
	// registering it here instead means commandNames' one walk of
	// cmd.Commands() already sees it before Run's own pre-check
	// (unknownRootCommand) ever runs, with no need to name it by a literal
	// string. Cobra calls this again inside Execute, which removes the
	// help command just added and adds the same one back, leaving exactly
	// one: calling it twice is harmless.
	cmd.InitDefaultHelpCmd()
	return cmd
}

// commandNames walks cmd's own subcommands once and returns two lists: all
// of them, by the name cobra dispatches on (hidden commands included, since
// cobra runs a hidden command by name exactly like any other; help
// included, since it is always a valid first argument too), and the ones
// worth naming in a message to the reader (neither hidden nor help). A
// command added to the program later appears in both lists, as
// appropriate, without a change here.
func commandNames(cmd *cobra.Command) (all, listed []string) {
	for _, c := range cmd.Commands() {
		all = append(all, c.Name())
		if !c.Hidden && c.Name() != "help" {
			listed = append(listed, c.Name())
		}
	}
	sort.Strings(all)
	sort.Strings(listed)
	return all, listed
}

// unknownCommandMessage is the one message every "this is not a command"
// refusal of this program prints; written once, so root's own Args and
// unknownRootCommand's raw pre-check can never drift apart.
func unknownCommandMessage(cmd *cobra.Command) string {
	_, listed := commandNames(cmd)
	return fmt.Sprintf("unknown command; the commands are: %s", strings.Join(listed, ", "))
}

// unknownRootCommand reports the message and true when args names, as its
// first element, something that is not a command: not one of cmd's own
// subcommands, hidden ones and help included. A first element that starts
// with "-" is a flag, not a command name, and is left to cobra itself to
// parse and judge, so that an existing, unrelated flag error (such as a
// bare --nope, with no command named at all) keeps its own message. Cobra
// also adds two hidden commands of its own, __complete and
// __completeNoDesc, but only inside Execute; since this check runs before
// Execute, it rejects those two exactly as it would any other name it does
// not recognise yet.
func unknownRootCommand(cmd *cobra.Command, args []string) (string, bool) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", false
	}
	all, _ := commandNames(cmd)
	if slices.Contains(all, args[0]) {
		return "", false
	}
	return unknownCommandMessage(cmd), true
}

// exitStatus is a sentinel error carrying a process exit code chosen by a
// command that has already printed its own diagnostic, redacted. Run reads
// the code and prints nothing further.
type exitStatus struct{ code int }

func (e *exitStatus) Error() string { return "" }

// exitError wraps err with the exit code the step that produced it maps to,
// for the failures the report command itself classifies rather than
// recognising by type (a configuration problem, a mode conflict, a refused
// profile, a bad snapshot, an output or toolchain problem).
type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

// runMode is one of the four ways `report` can be invoked.
type runMode int

const (
	modePDF runMode = iota
	modeCheck
	modeSnapshot
	modeFromSnapshot
)

// collectingFlagNames lists the flags of spec 5.2 that configure a
// collecting run; --from-snapshot refuses every one of them.
var collectingFlagNames = []string{
	"metrics-url", "endpoint-type", "token", "insecure", "cluster",
	"cluster-label", "cluster-filter", "namespaces", "period",
	"timezone", "node-autoscaling",
}

// determineMode validates the mode-selecting flags against every
// combination the command refuses and picks the mode the surviving flags
// select.
func determineMode(cmd *cobra.Command) (runMode, error) {
	changed := cmd.Flags().Changed
	hasCheck := changed("check")
	hasSnapshot := changed("snapshot")
	hasFromSnapshot := changed("from-snapshot")

	if hasCheck {
		for _, name := range []string{"output", "snapshot", "from-snapshot"} {
			if changed(name) {
				return 0, &exitError{2, fmt.Errorf("--check cannot be combined with --%s", name)}
			}
		}
	}
	if hasSnapshot && !hasCheck && changed("output") {
		return 0, &exitError{2, errors.New("--snapshot cannot be combined with --output")}
	}
	if hasFromSnapshot {
		conflicts := append([]string{"snapshot", "config"}, collectingFlagNames...)
		for _, name := range conflicts {
			if changed(name) {
				return 0, &exitError{2, fmt.Errorf("--from-snapshot cannot be combined with --%s", name)}
			}
		}
	}
	if changed("lang") && (hasCheck || hasSnapshot) {
		return 0, &exitError{2, errors.New("--lang applies only when a PDF is written")}
	}

	switch {
	case hasCheck:
		return modeCheck, nil
	case hasFromSnapshot:
		return modeFromSnapshot, nil
	case hasSnapshot:
		return modeSnapshot, nil
	default:
		return modePDF, nil
	}
}

// stringFlagOrNil returns a pointer to value when name was given on the
// command line, or nil otherwise, matching config.Flags' convention that nil
// means "not given".
func stringFlagOrNil(cmd *cobra.Command, name, value string) *string {
	if !cmd.Flags().Changed(name) {
		return nil
	}
	return &value
}

// buildConfigFlags reads every flag of f into a config.Flags, leaving a
// field nil when its flag was not given.
func buildConfigFlags(cmd *cobra.Command, f *reportFlags) config.Flags {
	return config.Flags{
		MetricsURL:      stringFlagOrNil(cmd, "metrics-url", f.metricsURL),
		EndpointType:    stringFlagOrNil(cmd, "endpoint-type", f.endpointType),
		Token:           stringFlagOrNil(cmd, "token", f.token),
		Cluster:         stringFlagOrNil(cmd, "cluster", f.cluster),
		ClusterLabel:    stringFlagOrNil(cmd, "cluster-label", f.clusterLabel),
		ClusterFilter:   stringFlagOrNil(cmd, "cluster-filter", f.clusterFilter),
		Namespaces:      stringFlagOrNil(cmd, "namespaces", f.namespaces),
		Period:          stringFlagOrNil(cmd, "period", f.period),
		Timezone:        stringFlagOrNil(cmd, "timezone", f.timezone),
		NodeAutoscaling: stringFlagOrNil(cmd, "node-autoscaling", f.nodeAutoscaling),
		MinSeverity:     stringFlagOrNil(cmd, "min-severity", f.minSeverity),
		ReportProfile:   stringFlagOrNil(cmd, "report-profile", f.reportProfile),
		Language:        stringFlagOrNil(cmd, "lang", f.lang),
		Insecure:        cmd.Flags().Changed("insecure") && f.insecure,
	}
}

// configSource builds the config.Source a collecting mode loads from.
func configSource(cmd *cobra.Command, f *reportFlags, env Env) config.Source {
	return config.Source{File: f.config, Getenv: env.Getenv, Flags: buildConfigFlags(cmd, f)}
}

// runReport is the whole of the report command's own logic: it resolves the
// mode, runs it, and writes exactly one line per success line to stdout, or
// one "error: <message>" line to stderr — both already wrapped by Run's
// Redactor, through cmd.OutOrStdout()/ErrOrStderr(), which return the very
// writers Run gave the root command.
func runReport(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, f *reportFlags) int {
	mode, err := determineMode(cmd)
	var lines []string
	if err == nil {
		switch mode {
		case modeCheck:
			lines, err = runCheck(ctx, env, redactor, cmd, f)
		case modeSnapshot:
			lines, err = runSnapshot(ctx, env, redactor, cmd, f)
		case modeFromSnapshot:
			lines, err = runFromSnapshot(ctx, env, cmd, f)
		default:
			lines, err = runPDF(ctx, env, redactor, cmd, f)
		}
	}

	if err != nil {
		c, msg := classify(ctx, err)
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", msg)
		return c
	}

	out := cmd.OutOrStdout()
	for _, line := range lines {
		_, _ = fmt.Fprintf(out, "%s\n", line)
	}
	return 0
}

// classify decides the exit code and message of a failed run. A cancelled
// context is checked first, whatever kind of error it produced along the
// way, so that, for instance, a metrics request abandoned mid-flight because
// the run was interrupted is reported as "interrupted", not as a metrics
// error.
func classify(ctx context.Context, err error) (int, string) {
	if ctx.Err() != nil {
		return 1, "interrupted"
	}

	var ee *exitError
	if errors.As(err, &ee) {
		return ee.code, ee.Error()
	}
	var pe *prom.Error
	if errors.As(err, &pe) {
		return 3, err.Error()
	}
	var se *discovery.ScopeError
	if errors.As(err, &se) {
		return 4, err.Error()
	}
	if errors.Is(err, discovery.ErrInsufficient) || errors.Is(err, collectpkg.ErrNoUsefulSections) {
		return 5, err.Error()
	}
	return 1, "internal failure: " + err.Error()
}

// runCheck runs the preflight and reports it; it builds no snapshot. It
// computes the window before the preflight, exactly like every mode that
// collects, so that an empty period stops a check the same way it stops a
// run, before any request; the window itself is otherwise unused here.
func runCheck(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, f *reportFlags) ([]string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, &exitError{2, err}
	}
	redactor.Add(loaded.Token)

	emittedAt := env.Now()
	window, err := collectpkg.NewWindow(loaded.Config.Analysis, emittedAt)
	if err != nil {
		return nil, &exitError{2, err}
	}

	c, err := discovery.RunPreflight(ctx, prom.Options{Metrics: loaded.Config.Metrics, Token: loaded.Token}, loaded.Config.Cluster, loaded.Config.Analysis, window.End)
	if err != nil {
		return nil, err
	}

	lines := preflightLines(c.Capabilities)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, c.Capabilities)...)
	}
	return lines, nil
}

// runSnapshot runs the preflight, builds the snapshot, and writes it. It
// needs no typst.
func runSnapshot(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, f *reportFlags) ([]string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, &exitError{2, err}
	}
	redactor.Add(loaded.Token)

	outputPath := f.snapshotPath
	if err := checkSnapshotOutputPath(outputPath); err != nil {
		return nil, &exitError{6, err}
	}

	emittedAt := env.Now()
	window, err := collectpkg.NewWindow(loaded.Config.Analysis, emittedAt)
	if err != nil {
		return nil, &exitError{2, err}
	}

	res, err := pipeline.Collect(ctx, pipeline.Run{
		Options:   prom.Options{Metrics: loaded.Config.Metrics, Token: loaded.Token},
		Config:    loaded.Config,
		Window:    window,
		EmittedAt: emittedAt,
		Now:       env.Now,
		NewID:     env.NewID,
		Version:   env.Version,
	})
	if err != nil {
		return nil, err
	}

	data, err := snapshot.Encode(res.Snapshot)
	if err != nil {
		return nil, err
	}
	if err := writeAtomicFile(outputPath, data); err != nil {
		return nil, &exitError{6, err}
	}

	lines := preflightLines(res.Capabilities)
	lines = append(lines, collectionLines(res.Collected)...)
	lines = append(lines, "snapshot: "+outputPath)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, res.Capabilities)...)
	}
	return lines, nil
}

// runPDF runs the preflight, builds the snapshot in memory, builds the
// document, and compiles it.
func runPDF(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, f *reportFlags) ([]string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, &exitError{2, err}
	}
	redactor.Add(loaded.Token)

	emittedAt := env.Now()
	window, err := collectpkg.NewWindow(loaded.Config.Analysis, emittedAt)
	if err != nil {
		return nil, &exitError{2, err}
	}

	loc, err := loaded.Config.Analysis.Location()
	if err != nil {
		return nil, &exitError{2, err}
	}

	outputPath := f.output
	if !cmd.Flags().Changed("output") {
		name := defaultOutputName(loaded.Config.Cluster.Name, loaded.Config.Report.Profile, loaded.Config.Report.MinSeverity, loaded.Config.Report.Language, emittedAt.In(loc))
		outputPath = filepath.Join(loaded.Config.Report.OutputDirectory, name)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return nil, &exitError{6, err}
	}
	if err := checkOutputPath(outputPath); err != nil {
		return nil, &exitError{6, err}
	}

	tc, err := typst.Locate(env.Getenv, env.LookPath)
	if err != nil {
		return nil, &exitError{6, err}
	}
	if err := tc.Probe(ctx); err != nil {
		return nil, &exitError{6, err}
	}

	res, err := pipeline.Collect(ctx, pipeline.Run{
		Options:   prom.Options{Metrics: loaded.Config.Metrics, Token: loaded.Token},
		Config:    loaded.Config,
		Window:    window,
		EmittedAt: emittedAt,
		Now:       env.Now,
		NewID:     env.NewID,
		Version:   env.Version,
	})
	if err != nil {
		return nil, err
	}

	lang, err := catalog.ParseLanguage(loaded.Config.Report.Language)
	if err != nil {
		return nil, err
	}
	data, err := render.Document(res.Snapshot, lang, document.Options{})
	if err != nil {
		return nil, err
	}

	if err := compilePDF(ctx, env, tc, data, outputPath); err != nil {
		return nil, err
	}

	lines := preflightLines(res.Capabilities)
	lines = append(lines, collectionLines(res.Collected)...)
	lines = append(lines, "report: "+outputPath)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, res.Capabilities)...)
	}
	return lines, nil
}

// runFromSnapshot reads a stored snapshot and renders it; it makes no
// request, reads no configuration file, and so has no token of its own to
// add to the redactor.
func runFromSnapshot(ctx context.Context, env Env, cmd *cobra.Command, f *reportFlags) ([]string, error) {
	opts, err := config.LoadRenderOptions(env.Getenv, buildConfigFlags(cmd, f))
	if err != nil {
		return nil, &exitError{2, err}
	}

	raw, err := os.ReadFile(f.fromSnapshot)
	if err != nil {
		cause := err.Error()
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			cause = pathErr.Err.Error()
		}
		return nil, &exitError{2, fmt.Errorf("snapshot cannot be read: %s: %s", f.fromSnapshot, cause)}
	}
	snap, err := snapshot.Decode(raw)
	if err != nil {
		return nil, &exitError{2, err}
	}

	profile := opts.Profile
	if profile == "" {
		profile = snap.Report.Profile
	}
	outputDir := opts.OutputDirectory
	if outputDir == "" {
		outputDir = "."
	}
	severity := opts.MinSeverity
	if severity == "" {
		severity = snap.Report.MinSeverity
	}
	emittedAt, err := snapshot.Time(snap.Report.EmittedAt)
	if err != nil {
		return nil, err
	}

	outputPath := f.output
	if !cmd.Flags().Changed("output") {
		name := defaultOutputName(snap.Scope.ClusterName, profile, severity, opts.Language, emittedAt)
		outputPath = filepath.Join(outputDir, name)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return nil, &exitError{6, err}
	}
	if err := checkOutputPath(outputPath); err != nil {
		return nil, &exitError{6, err}
	}

	tc, err := typst.Locate(env.Getenv, env.LookPath)
	if err != nil {
		return nil, &exitError{6, err}
	}
	if err := tc.Probe(ctx); err != nil {
		return nil, &exitError{6, err}
	}

	lang, err := catalog.ParseLanguage(opts.Language)
	if err != nil {
		return nil, err
	}
	data, err := render.Document(snap, lang, document.Options{Profile: opts.Profile, MinSeverity: opts.MinSeverity})
	if err != nil {
		return nil, err
	}

	if err := compilePDF(ctx, env, tc, data, outputPath); err != nil {
		return nil, err
	}

	lines := []string{"snapshot: loaded", "report: " + outputPath}
	if f.verbose {
		lines = append(lines, "generator: "+snap.Report.GeneratorVersion)
	}
	return lines, nil
}

// collectionLines names the state of each collection, in the order the
// collectors run, and the autoscaling status.
func collectionLines(c collectpkg.Collected) []string {
	return []string{
		"namespace collection: " + c.Namespaces.State,
		"workload collection: " + c.Workloads.State,
		"hpa collection: " + c.HPAs.State,
		"hpa coverage collection: " + c.Coverage.State,
		"node collection: " + c.Nodes.State,
		"autoscaling collection: " + c.Autoscaling.Status,
	}
}

// preflightLines are the four milestone lines every collecting mode prints
// on success, followed by one line per capability, in their fixed order.
func preflightLines(capabilities []discovery.Capability) []string {
	lines := []string{"config: loaded", "endpoint: available", "scope: resolved", "capability: discovered"}
	for _, c := range capabilities {
		lines = append(lines, c.Name+": "+c.State)
	}
	return lines
}

// verboseCollectingLines are the four lines --verbose adds for a check,
// snapshot, or PDF run.
func verboseCollectingLines(f *reportFlags, cfg config.Config, capabilities []discovery.Capability) []string {
	return []string{
		"config path: " + f.config,
		"endpoint path: " + endpointPath(cfg.Metrics.URL),
		"retries: 2 (3 attempts)",
		"strategies: " + strategiesLine(capabilities),
	}
}

// endpointPath returns the path component of rawURL, or "/" when it is
// empty. The host and scheme are never printed.
func endpointPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Path == "" {
		return "/"
	}
	return u.Path
}

// strategiesLine lists the strategies of capabilities other than "none",
// each once, sorted, joined by ", ".
func strategiesLine(capabilities []discovery.Capability) string {
	seen := make(map[string]bool, len(capabilities))
	var out []string
	for _, c := range capabilities {
		if c.Strategy == discovery.StrategyNone || seen[c.Strategy] {
			continue
		}
		seen[c.Strategy] = true
		out = append(out, c.Strategy)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// compilePDF compiles data and writes the PDF beside outputPath, renamed into
// place; every failure is exit code 6.
func compilePDF(ctx context.Context, env Env, tc typst.Toolchain, data []byte, outputPath string) error {
	pdf, err := render.Compile(ctx, tc, env.TempDir, data)
	if err != nil {
		return &exitError{6, err}
	}
	if err := writeAtomicFile(outputPath, pdf); err != nil {
		return &exitError{6, err}
	}
	return nil
}
