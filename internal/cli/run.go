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
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/document"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/redact"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
	"github.com/marceloalvescf/k8s-capacity-report/internal/typst"
	"github.com/marceloalvescf/k8s-capacity-report/template"
)

// Env is everything the command takes from outside itself. Every field is
// injected so a test never touches the real process environment, PATH,
// clock, or random source.
type Env struct {
	Getenv   func(string) string
	LookPath func(string) (string, error)
	Now      func() time.Time
	NewID    func() (string, error)
	TempDir  string // parent of the work folder; empty means os.TempDir()
	Version  string // written to the snapshot as the generator version
}

// RealEnv returns the environment of the running process.
func RealEnv() Env {
	return Env{
		Getenv:   os.Getenv,
		LookPath: exec.LookPath,
		Now:      time.Now,
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
// One redact.Redactor is made here, on the very first line, from every
// --token value tokenValuesFromArgs can find in args, and both streams are
// wrapped by it before cobra ever sees them: nothing below this function
// writes to a raw stream. The same Redactor is handed down to the report
// command, which adds the configuration's own token to it, in place, the
// moment config.Load returns (see runCheck, runSnapshot, and runPDF); every
// writer already built from that Redactor sees the addition immediately,
// including one the panic recovery below reaches for after the fact — so a
// panic anywhere after the token is known is redacted, not only a panic
// that happens to occur before any mode function is even called.
//
// The recovery sits here, above cmd.Execute(), rather than inside the
// report command's own logic, so that it also catches a panic from
// anything cobra itself might do while resolving or parsing.
func Run(ctx context.Context, env Env, args []string, stdout, stderr io.Writer) (code int) {
	redactor := redact.New(tokenValuesFromArgs(args)...)
	wrappedOut := redactor.Writer(stdout)
	wrappedErr := redactor.Writer(stderr)

	defer func() {
		if r := recover(); r != nil {
			_, _ = fmt.Fprintf(wrappedErr, "error: internal failure: %v\n", r)
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
		// Every failure prints exactly one "error: <message>" line, cobra's
		// own errors included (an unknown flag, or report's own refusal of
		// a positional argument), each as a single Write through the
		// redactor already wrapping wrappedErr.
		_, _ = fmt.Fprintf(wrappedErr, "error: %s\n", err)
		return 2
	}
	return 0
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
		// This program has the commands its design names (report, and for
		// later phases serve, worker, and admin) and no other. Cobra's
		// default completion command is not one of them, and it carries
		// its own unmodified cobra.NoArgs, which repeats a stray argument
		// verbatim exactly like the root's own did before this task's
		// fix — the same leak, one command over. Removed outright, rather
		// than given its own Args, since the design never asked for it.
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
	cmd.AddCommand(newReportCmd(ctx, env, redactor))
	// Cobra only adds its default "help" command lazily, inside Execute();
	// registering it here instead means commandNames' one walk of
	// cmd.Commands() already sees it before Run's own pre-check
	// (unknownRootCommand) ever runs, with no need to name it by a literal
	// string. Calling this twice (cobra calls it again inside Execute) is
	// harmless: it does nothing once a help command already exists.
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
// bare --nope, with no command named at all) keeps its own message.
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
	if errors.Is(err, discovery.ErrInsufficient) {
		return 5, err.Error()
	}
	return 1, "internal failure: " + err.Error()
}

// collected is the result of connecting, resolving the scope, and detecting
// the capabilities: everything runCheck, runSnapshot, and runPDF share.
type collected struct {
	client                *prom.Client
	scope                 discovery.Scope
	capabilities          []discovery.Capability
	collectionCompletedAt time.Time
}

// collect runs the preflight every collecting mode shares: connect, resolve
// the scope, detect the capabilities, and check they are sufficient.
// emittedAt is the caller's own first clock read (the emission and
// collection-start time), taken as a parameter rather than read here, so
// that a caller that also needs it for something else (runPDF's default
// output name) reads the clock only once for both purposes; collect reads
// it a second time, once the capabilities are known, for the
// collection-completed time. This keeps the clock read exactly twice in
// every collecting mode, whatever else that mode does with the first
// reading.
func collect(ctx context.Context, env Env, cfg config.Config, token string, emittedAt time.Time) (collected, error) {
	client, err := prom.Connect(ctx, prom.Options{Metrics: cfg.Metrics, Token: token})
	if err != nil {
		return collected{}, err
	}
	scope, err := discovery.ResolveScope(ctx, client, cfg.Cluster, emittedAt)
	if err != nil {
		return collected{}, err
	}
	capabilities, err := discovery.Detect(ctx, client, scope, cfg.Analysis, emittedAt)
	if err != nil {
		return collected{}, err
	}
	if err := discovery.CheckSufficient(capabilities); err != nil {
		return collected{}, err
	}

	return collected{
		client:                client,
		scope:                 scope,
		capabilities:          capabilities,
		collectionCompletedAt: env.Now(),
	}, nil
}

// runCheck runs the preflight and reports it; it builds no snapshot.
func runCheck(ctx context.Context, env Env, redactor *redact.Redactor, cmd *cobra.Command, f *reportFlags) ([]string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, &exitError{2, err}
	}
	redactor.Add(loaded.Token)

	emittedAt := env.Now()
	c, err := collect(ctx, env, loaded.Config, loaded.Token, emittedAt)
	if err != nil {
		return nil, err
	}

	lines := preflightLines(c.capabilities)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, c.capabilities)...)
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
	c, err := collect(ctx, env, loaded.Config, loaded.Token, emittedAt)
	if err != nil {
		return nil, err
	}

	reportID, err := env.NewID()
	if err != nil {
		return nil, err
	}

	snap, err := snapshot.Build(snapshot.Input{
		ID:                  reportID,
		GeneratorVersion:    env.Version,
		EmittedAt:           emittedAt,
		Config:              loaded.Config,
		EndpointKind:        c.client.Kind(),
		Scope:               c.scope,
		Capabilities:        c.capabilities,
		CollectionStarted:   emittedAt,
		CollectionCompleted: c.collectionCompletedAt,
	})
	if err != nil {
		return nil, err
	}

	data, err := snapshot.Encode(snap)
	if err != nil {
		return nil, err
	}
	if err := writeAtomicFile(outputPath, data); err != nil {
		return nil, &exitError{6, err}
	}

	lines := preflightLines(c.capabilities)
	lines = append(lines, "snapshot: "+outputPath)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, c.capabilities)...)
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

	if loaded.Config.Report.Profile == "executive" {
		return nil, &exitError{2, executiveError(loaded.Config.Report.Profile)}
	}

	emittedAt := env.Now()
	loc, err := loaded.Config.Analysis.Location()
	if err != nil {
		return nil, &exitError{2, err}
	}

	outputPath := f.output
	if !cmd.Flags().Changed("output") {
		name := defaultOutputName(loaded.Config.Cluster.Name, loaded.Config.Report.MinSeverity, loaded.Config.Report.Language, emittedAt.In(loc))
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

	c, err := collect(ctx, env, loaded.Config, loaded.Token, emittedAt)
	if err != nil {
		return nil, err
	}

	reportID, err := env.NewID()
	if err != nil {
		return nil, err
	}

	snap, err := snapshot.Build(snapshot.Input{
		ID:                  reportID,
		GeneratorVersion:    env.Version,
		EmittedAt:           emittedAt,
		Config:              loaded.Config,
		EndpointKind:        c.client.Kind(),
		Scope:               c.scope,
		Capabilities:        c.capabilities,
		CollectionStarted:   emittedAt,
		CollectionCompleted: c.collectionCompletedAt,
	})
	if err != nil {
		return nil, err
	}

	lang, err := catalog.ParseLanguage(loaded.Config.Report.Language)
	if err != nil {
		return nil, err
	}
	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, err
	}
	doc, err := document.Build(snap, cat, document.Options{})
	if err != nil {
		return nil, err
	}
	data, err := document.Encode(doc)
	if err != nil {
		return nil, err
	}

	if err := compileToPath(ctx, env, tc, data, outputPath); err != nil {
		return nil, &exitError{6, err}
	}

	lines := preflightLines(c.capabilities)
	lines = append(lines, "report: "+outputPath)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, c.capabilities)...)
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
	if profile == "executive" {
		return nil, &exitError{2, executiveError(profile)}
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
		name := defaultOutputName(snap.Scope.ClusterName, severity, opts.Language, emittedAt)
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
	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, err
	}
	doc, err := document.Build(snap, cat, document.Options{Profile: opts.Profile, MinSeverity: opts.MinSeverity})
	if err != nil {
		return nil, err
	}
	data, err := document.Encode(doc)
	if err != nil {
		return nil, err
	}

	if err := compileToPath(ctx, env, tc, data, outputPath); err != nil {
		return nil, &exitError{6, err}
	}

	lines := []string{"snapshot: loaded", "report: " + outputPath}
	if f.verbose {
		lines = append(lines, "generator: "+snap.Report.GeneratorVersion)
	}
	return lines, nil
}

// executiveError is the one message a refused executive profile always
// prints, whether the profile came from the configuration or from a stored
// snapshot.
func executiveError(profile string) error {
	return fmt.Errorf("report profile %q is not available in this version", profile)
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

// withWorkFolder makes a fresh, empty folder under parent (or under
// os.TempDir() when parent is empty), runs work with its path, and removes
// the folder in every case — success, an error work returns, or a panic
// that unwinds through it — since the deferred removal is registered right
// after the folder is made and Go runs every such defer while a panic
// unwinds through its frame, whether or not anything above ever recovers
// it.
func withWorkFolder(parent string, work func(dir string) error) error {
	if parent == "" {
		parent = os.TempDir()
	}
	dir, err := os.MkdirTemp(parent, "k8s-capacity-report-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	return work(dir)
}

// settlePDFMode gives path the mode the compiler already left it with, less
// any bit outside 0o644: typst was measured (see the command's report) to
// leave its output at a mode derived from 0o666, not the 0o644 every other
// output file of this command is created with, so under a permissive umask
// the PDF would otherwise end up more open than 0o644 allows. This never
// makes the file more permissive than the compiler already left it, and
// never touches the umask itself, only the file already on disk.
func settlePDFMode(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.Chmod(path, info.Mode().Perm()&0o644)
}

// compileToPath unpacks the template into a fresh work folder, writes
// report.json, compiles to a temporary file beside outputPath, and renames
// it into place. The compiler is given a name that does not yet exist and
// creates the file itself.
func compileToPath(ctx context.Context, env Env, tc typst.Toolchain, data []byte, outputPath string) error {
	return withWorkFolder(env.TempDir, func(dir string) error {
		if err := template.Unpack(dir); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "report.json"), data, 0o644); err != nil {
			return err
		}

		tmpPath, err := tempSiblingName(outputPath)
		if err != nil {
			return err
		}
		succeeded := false
		defer func() {
			if !succeeded {
				_ = os.Remove(tmpPath)
			}
		}()

		job := typst.Job{
			Root:     dir,
			FontPath: template.FontPath(dir),
			Source:   template.Entry(dir),
			Output:   tmpPath,
			Format:   typst.PDF,
			Inputs:   map[string]string{"data": "/report.json"},
		}
		if _, err := tc.Compile(ctx, job); err != nil {
			return err
		}

		if err := settlePDFMode(tmpPath); err != nil {
			return err
		}
		if err := syncFile(tmpPath); err != nil {
			return err
		}
		if err := os.Rename(tmpPath, outputPath); err != nil {
			return err
		}
		succeeded = true
		return nil
	})
}
