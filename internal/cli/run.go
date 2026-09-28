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

// Run builds the root command, executes it against args, and returns the
// process exit code. Output streams are passed explicitly rather than using
// os.Stdout/os.Stderr so callers (including tests) control where they go.
//
// Cobra's own error and usage printing stays silenced; a plain cobra error
// (an unknown command or flag) is printed once here, unredacted, since it
// carries nothing but the command's own definitions. A command that has
// already reported its own diagnostic, redacted, signals its exit code with
// exitStatus instead, so that message is never printed a second time.
func Run(ctx context.Context, env Env, args []string, stdout, stderr io.Writer) int {
	cmd := newRootCmd(ctx, env)
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.Execute(); err != nil {
		var status *exitStatus
		if errors.As(err, &status) {
			return status.code
		}
		// Discard the error from writing to stderr; if it fails, there is
		// nowhere left to report the failure.
		_, _ = fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

func newRootCmd(ctx context.Context, env Env) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "k8s-capacity-report",
		Short:   "Generate Kubernetes capacity reports from Prometheus metrics.",
		Version: kcr.Version(),
		// No positional argument belongs at the root; NoArgs turns one into
		// an "unknown command" error instead of silently accepting it.
		Args:          cobra.NoArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
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
	cmd.AddCommand(newReportCmd(ctx, env))
	return cmd
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
// mode, runs it, and writes exactly one line to stdout per success line, or
// one "error: <message>" line to stderr, through a redactor built from every
// secret the run has learned about by the time it prints.
func runReport(ctx context.Context, env Env, cmd *cobra.Command, f *reportFlags) (code int) {
	rawStdout := cmd.OutOrStdout()
	rawStderr := cmd.ErrOrStderr()

	var secrets []string
	if cmd.Flags().Changed("token") {
		secrets = append(secrets, f.token)
	}

	defer func() {
		if r := recover(); r != nil {
			red := redact.New(secrets...)
			_, _ = fmt.Fprintf(red.Writer(rawStderr), "error: internal failure: %v\n", r)
			code = 1
		}
	}()

	mode, err := determineMode(cmd)
	var lines []string
	if err == nil {
		switch mode {
		case modeCheck:
			lines, secrets, err = runCheck(ctx, env, cmd, f, secrets)
		case modeSnapshot:
			lines, secrets, err = runSnapshot(ctx, env, cmd, f, secrets)
		case modeFromSnapshot:
			lines, secrets, err = runFromSnapshot(ctx, env, cmd, f, secrets)
		default:
			lines, secrets, err = runPDF(ctx, env, cmd, f, secrets)
		}
	}

	red := redact.New(secrets...)
	if err != nil {
		c, msg := classify(ctx, err)
		_, _ = fmt.Fprintf(red.Writer(rawStderr), "error: %s\n", msg)
		return c
	}

	out := red.Writer(rawStdout)
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
	emittedAt             time.Time
	collectionCompletedAt time.Time
}

// collect runs the preflight every collecting mode shares: connect, resolve
// the scope, detect the capabilities, and check they are sufficient. The
// clock is read once before connecting (the emission and collection-start
// time) and once after the capabilities are known (the collection-completed
// time), matching every mode that collects, whether or not it goes on to
// build a snapshot.
func collect(ctx context.Context, env Env, cfg config.Config, token string) (collected, error) {
	emittedAt := env.Now()

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
		emittedAt:             emittedAt,
		collectionCompletedAt: env.Now(),
	}, nil
}

// runCheck runs the preflight and reports it; it builds no snapshot.
func runCheck(ctx context.Context, env Env, cmd *cobra.Command, f *reportFlags, secrets []string) ([]string, []string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, secrets, &exitError{2, err}
	}
	secrets = append(secrets, loaded.Token)

	c, err := collect(ctx, env, loaded.Config, loaded.Token)
	if err != nil {
		return nil, secrets, err
	}

	lines := preflightLines(c.capabilities)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, c.capabilities)...)
	}
	return lines, secrets, nil
}

// runSnapshot runs the preflight, builds the snapshot, and writes it. It
// needs no typst.
func runSnapshot(ctx context.Context, env Env, cmd *cobra.Command, f *reportFlags, secrets []string) ([]string, []string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, secrets, &exitError{2, err}
	}
	secrets = append(secrets, loaded.Token)

	outputPath := f.snapshotPath
	if err := checkSnapshotOutputPath(outputPath); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	c, err := collect(ctx, env, loaded.Config, loaded.Token)
	if err != nil {
		return nil, secrets, err
	}

	reportID, err := env.NewID()
	if err != nil {
		return nil, secrets, err
	}

	snap, err := snapshot.Build(snapshot.Input{
		ID:                  reportID,
		GeneratorVersion:    env.Version,
		EmittedAt:           c.emittedAt,
		Config:              loaded.Config,
		EndpointKind:        c.client.Kind(),
		Scope:               c.scope,
		Capabilities:        c.capabilities,
		CollectionStarted:   c.emittedAt,
		CollectionCompleted: c.collectionCompletedAt,
	})
	if err != nil {
		return nil, secrets, err
	}

	data, err := snapshot.Encode(snap)
	if err != nil {
		return nil, secrets, err
	}
	if err := writeAtomicFile(outputPath, data); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	lines := preflightLines(c.capabilities)
	lines = append(lines, "snapshot: "+outputPath)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, c.capabilities)...)
	}
	return lines, secrets, nil
}

// runPDF runs the preflight, builds the snapshot in memory, builds the
// document, and compiles it.
func runPDF(ctx context.Context, env Env, cmd *cobra.Command, f *reportFlags, secrets []string) ([]string, []string, error) {
	loaded, err := config.Load(configSource(cmd, f, env))
	if err != nil {
		return nil, secrets, &exitError{2, err}
	}
	secrets = append(secrets, loaded.Token)

	if loaded.Config.Report.Profile == "executive" {
		return nil, secrets, &exitError{2, executiveError(loaded.Config.Report.Profile)}
	}

	emittedAt := env.Now()
	loc, err := loaded.Config.Analysis.Location()
	if err != nil {
		return nil, secrets, &exitError{2, err}
	}

	outputPath := f.output
	if !cmd.Flags().Changed("output") {
		name := defaultOutputName(loaded.Config.Cluster.Name, loaded.Config.Report.MinSeverity, loaded.Config.Report.Language, emittedAt.In(loc))
		outputPath = filepath.Join(loaded.Config.Report.OutputDirectory, name)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return nil, secrets, &exitError{6, err}
	}
	if err := checkOutputPath(outputPath); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	tc, err := typst.Locate(env.Getenv, env.LookPath)
	if err != nil {
		return nil, secrets, &exitError{6, err}
	}
	if err := tc.Probe(ctx); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	client, err := prom.Connect(ctx, prom.Options{Metrics: loaded.Config.Metrics, Token: loaded.Token})
	if err != nil {
		return nil, secrets, err
	}
	scope, err := discovery.ResolveScope(ctx, client, loaded.Config.Cluster, emittedAt)
	if err != nil {
		return nil, secrets, err
	}
	capabilities, err := discovery.Detect(ctx, client, scope, loaded.Config.Analysis, emittedAt)
	if err != nil {
		return nil, secrets, err
	}
	if err := discovery.CheckSufficient(capabilities); err != nil {
		return nil, secrets, err
	}
	collectionCompleted := env.Now()

	reportID, err := env.NewID()
	if err != nil {
		return nil, secrets, err
	}

	snap, err := snapshot.Build(snapshot.Input{
		ID:                  reportID,
		GeneratorVersion:    env.Version,
		EmittedAt:           emittedAt,
		Config:              loaded.Config,
		EndpointKind:        client.Kind(),
		Scope:               scope,
		Capabilities:        capabilities,
		CollectionStarted:   emittedAt,
		CollectionCompleted: collectionCompleted,
	})
	if err != nil {
		return nil, secrets, err
	}

	lang, err := catalog.ParseLanguage(loaded.Config.Report.Language)
	if err != nil {
		return nil, secrets, err
	}
	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, secrets, err
	}
	doc, err := document.Build(snap, cat, document.Options{})
	if err != nil {
		return nil, secrets, err
	}
	data, err := document.Encode(doc)
	if err != nil {
		return nil, secrets, err
	}

	if err := compileToPath(ctx, env, tc, data, outputPath); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	lines := preflightLines(capabilities)
	lines = append(lines, "report: "+outputPath)
	if f.verbose {
		lines = append(lines, verboseCollectingLines(f, loaded.Config, capabilities)...)
	}
	return lines, secrets, nil
}

// runFromSnapshot reads a stored snapshot and renders it; it makes no
// request and reads no configuration file.
func runFromSnapshot(ctx context.Context, env Env, cmd *cobra.Command, f *reportFlags, secrets []string) ([]string, []string, error) {
	opts, err := config.LoadRenderOptions(env.Getenv, buildConfigFlags(cmd, f))
	if err != nil {
		return nil, secrets, &exitError{2, err}
	}

	raw, err := os.ReadFile(f.fromSnapshot)
	if err != nil {
		cause := err.Error()
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			cause = pathErr.Err.Error()
		}
		return nil, secrets, &exitError{2, fmt.Errorf("snapshot cannot be read: %s: %s", f.fromSnapshot, cause)}
	}
	snap, err := snapshot.Decode(raw)
	if err != nil {
		return nil, secrets, &exitError{2, err}
	}

	profile := opts.Profile
	if profile == "" {
		profile = snap.Report.Profile
	}
	if profile == "executive" {
		return nil, secrets, &exitError{2, executiveError(profile)}
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
		return nil, secrets, err
	}

	outputPath := f.output
	if !cmd.Flags().Changed("output") {
		name := defaultOutputName(snap.Scope.ClusterName, severity, opts.Language, emittedAt)
		outputPath = filepath.Join(outputDir, name)
	}
	outputPath, err = filepath.Abs(outputPath)
	if err != nil {
		return nil, secrets, &exitError{6, err}
	}
	if err := checkOutputPath(outputPath); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	tc, err := typst.Locate(env.Getenv, env.LookPath)
	if err != nil {
		return nil, secrets, &exitError{6, err}
	}
	if err := tc.Probe(ctx); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	lang, err := catalog.ParseLanguage(opts.Language)
	if err != nil {
		return nil, secrets, err
	}
	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, secrets, err
	}
	doc, err := document.Build(snap, cat, document.Options{Profile: opts.Profile, MinSeverity: opts.MinSeverity})
	if err != nil {
		return nil, secrets, err
	}
	data, err := document.Encode(doc)
	if err != nil {
		return nil, secrets, err
	}

	if err := compileToPath(ctx, env, tc, data, outputPath); err != nil {
		return nil, secrets, &exitError{6, err}
	}

	lines := []string{"snapshot: loaded", "report: " + outputPath}
	if f.verbose {
		lines = append(lines, "generator: "+snap.Report.GeneratorVersion)
	}
	return lines, secrets, nil
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

// compileToPath makes the work folder, unpacks the template, writes
// report.json, compiles to a temporary file beside outputPath, and renames
// it into place. The work folder is removed in every case.
func compileToPath(ctx context.Context, env Env, tc typst.Toolchain, data []byte, outputPath string) (err error) {
	parent := env.TempDir
	if parent == "" {
		parent = os.TempDir()
	}
	workDir, err := os.MkdirTemp(parent, "k8s-capacity-report-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	if err := template.Unpack(workDir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(workDir, "report.json"), data, 0o644); err != nil {
		return err
	}

	tmpPath, err := reserveTempFile(outputPath)
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
		Root:     workDir,
		FontPath: template.FontPath(workDir),
		Source:   template.Entry(workDir),
		Output:   tmpPath,
		Format:   typst.PDF,
		Inputs:   map[string]string{"data": "/report.json"},
	}
	if _, err := tc.Compile(ctx, job); err != nil {
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
}

// reserveTempFile creates an empty ".<name>.<random>.tmp" file beside
// finalPath and returns its path.
func reserveTempFile(finalPath string) (string, error) {
	dir := filepath.Dir(finalPath)
	base := filepath.Base(finalPath)
	f, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return "", err
	}
	name := f.Name()
	if err := f.Close(); err != nil {
		return "", err
	}
	return name, nil
}

// syncFile flushes path's content to disk.
func syncFile(path string) (err error) {
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	return f.Sync()
}

// writeAtomicFile writes data to a temporary file beside path, flushes it,
// and renames it into place. The temporary file is removed on any failure.
func writeAtomicFile(path string, data []byte) (err error) {
	tmpPath, err := reserveTempFile(path)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
