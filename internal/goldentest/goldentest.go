// Package goldentest runs the whole chain — connect, resolve the scope,
// detect the capabilities, collect, analyze, build a snapshot, build a
// document — against one scenario of promtest.Scenarios, with one fixed
// input held constant across every run. It is not a test
// file: golden_test.go uses it to compare against the golden files, and
// cmd/update-testdata uses it to rewrite them. Neither the production binary
// nor any package outside this chain of tests may import it; internal/guard
// enforces that.
package goldentest

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/analysis"
	"github.com/marceloalvescf/capacity-room/internal/catalog"
	"github.com/marceloalvescf/capacity-room/internal/collect"
	"github.com/marceloalvescf/capacity-room/internal/config"
	"github.com/marceloalvescf/capacity-room/internal/discovery"
	"github.com/marceloalvescf/capacity-room/internal/document"
	"github.com/marceloalvescf/capacity-room/internal/prom"
	"github.com/marceloalvescf/capacity-room/internal/promtest"
	"github.com/marceloalvescf/capacity-room/internal/snapshot"
)

// DataFileScenarios names the scenarios that have golden data files. Of the
// other two, grafana-authenticated and thanos-multicluster give, in this
// part, the same document as direct-complete.
var DataFileScenarios = []string{
	"direct-complete", "partial-data", "raw-fallback", "value-edge-cases", "edge-values", "edge-scaling",
}

// ExecutiveDataFileScenarios names the scenarios that have golden data files
// of the executive profile too.
var ExecutiveDataFileScenarios = []string{"direct-complete", "edge-scaling"}

// The fixed input every scenario runs with, held constant so that only the
// scenario itself can change a byte of the result.
const (
	reportID         = "00000000-0000-4000-8000-000000000001"
	generatorVersion = "0.0.0-golden"
	clusterName      = "prod"
	organization     = "ACME"
	classification   = "Uso interno"
	author           = "Time SRE"
	collectionSpan   = 42 * time.Second
)

// namespaces and excludedContainers are the fixed list values of the input;
// they are declared separately because config.Cluster.Namespaces and
// config.Analysis.ExcludedContainers each take a slice.
var (
	namespaces         = []string{"payments"}
	excludedContainers = []string{"istio-proxy"}
)

// fixedEmittedAt returns 2026-07-15 14:30:00 in America/Sao_Paulo, the
// "emitted at" of the fixed input.
func fixedEmittedAt() (time.Time, error) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		return time.Time{}, fmt.Errorf("loading America/Sao_Paulo: %w", err)
	}
	return time.Date(2026, time.July, 15, 14, 30, 0, 0, loc), nil
}

// fixedConfig starts from config.Default and sets the fields of the fixed
// input, plus the ones the scenario itself carries. The metrics URL is the
// only field that ever holds the test server's address; snapshot.Build
// never reads it into the stored snapshot.
func fixedConfig(scenario promtest.Scenario, serverURL string) config.Config {
	cfg := config.Default()
	cfg.Metrics.URL = serverURL + scenario.APIRoot
	cfg.Metrics.EndpointType = scenario.EndpointType
	cfg.Cluster.Name = clusterName
	cfg.Cluster.Namespaces = namespaces
	cfg.Analysis.NodeAutoscaling = scenario.NodeAutoscaling
	if scenario.Start != "" {
		cfg.Analysis.Period, cfg.Analysis.Start, cfg.Analysis.End = "", scenario.Start, scenario.End
	}
	cfg.Analysis.ExcludedContainers = excludedContainers
	cfg.Report.Organization = organization
	cfg.Report.Classification = classification
	cfg.Report.Author = author
	return cfg
}

// runOutput is everything one call to run produces from a scenario: the
// snapshot the whole chain built, and what its two collectors returned
// before analysis turned them into findings and before the snapshot
// flattened namespace_usage and workload_usage's own state into the ten
// statistics of each row.
type runOutput struct {
	Snapshot  snapshot.Snapshot
	Collected collect.Collected
}

// run serves scenario's fixture in this process and carries it through the
// whole chain the command itself runs before writing a snapshot: build the
// window, connect, resolve the scope, detect the capabilities, check they
// are sufficient, collect, analyze, and build. It returns the built
// snapshot and what the collectors returned, together with the fake
// server that served it, so that Snapshot and Queries, run once each,
// always describe the very same run of the scenario, and so that a caller
// that needs to know whether every scripted answer was consumed can ask
// the server itself once the run is over.
func run(ctx context.Context, scenario promtest.Scenario) (runOutput, *promtest.Server, error) {
	path, err := promtest.Path(scenario.Name)
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: locating fixture: %w", scenario.Name, err)
	}
	fixture, err := promtest.Load(path)
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: loading fixture: %w", scenario.Name, err)
	}

	fake := promtest.NewServer(fixture)
	server := httptest.NewServer(fake)
	defer server.Close()

	emittedAt, err := fixedEmittedAt()
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: %w", scenario.Name, err)
	}
	cfg := fixedConfig(scenario, server.URL)

	window, err := collect.NewWindow(cfg.Analysis, emittedAt)
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: building the window: %w", scenario.Name, err)
	}

	client, err := prom.Connect(ctx, prom.Options{
		Metrics: cfg.Metrics,
		Token:   scenario.Token,
	})
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: connecting: %w", scenario.Name, err)
	}

	scope, err := discovery.ResolveScope(ctx, client, cfg.Cluster, window.End)
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: resolving the scope: %w", scenario.Name, err)
	}

	capabilities, err := discovery.Detect(ctx, client, scope, cfg.Analysis, window.End)
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: detecting the capabilities: %w", scenario.Name, err)
	}

	if err := discovery.CheckSufficient(capabilities); err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: checking the capabilities: %w", scenario.Name, err)
	}

	collected, err := collect.All(ctx, client, collect.Input{
		Scope:        scope,
		Cluster:      cfg.Cluster,
		Analysis:     cfg.Analysis,
		Capabilities: capabilities,
		Window:       window,
	})
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: collecting: %w", scenario.Name, err)
	}

	if err := collect.CheckUseful(collected); err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: checking the collection: %w", scenario.Name, err)
	}

	findings := analysis.All(collected, cfg.Thresholds)

	snap, err := snapshot.Build(snapshot.Input{
		ID:                  reportID,
		GeneratorVersion:    generatorVersion,
		EmittedAt:           emittedAt,
		Config:              cfg,
		EndpointKind:        client.Kind(),
		Scope:               scope,
		Capabilities:        capabilities,
		CollectionStarted:   emittedAt,
		CollectionCompleted: emittedAt.Add(collectionSpan),
		Window:              window,
		Collected:           collected,
		Findings:            findings,
	})
	if err != nil {
		return runOutput{}, nil, fmt.Errorf("%s: building the snapshot: %w", scenario.Name, err)
	}

	return runOutput{Snapshot: snap, Collected: collected}, fake, nil
}

// Snapshot serves the scenario in this process, runs the connection, the
// scope, and the capabilities against it with the fixed input, and returns
// the encoded snapshot.
func Snapshot(ctx context.Context, scenario promtest.Scenario) ([]byte, error) {
	out, _, err := run(ctx, scenario)
	if err != nil {
		return nil, err
	}
	data, err := snapshot.Encode(out.Snapshot)
	if err != nil {
		return nil, fmt.Errorf("%s: encoding the snapshot: %w", scenario.Name, err)
	}
	return data, nil
}

// Queries runs scenario the same way Snapshot does, and returns the exact
// queries the program sent, in the order it sent them: one line per
// request, the path of the request, one tab, and the text of the form
// field "query" as it was sent, with a line break inside a query written
// as the two characters \n rather than an actual line break, so that the
// golden file itself stays one line per request.
func Queries(ctx context.Context, scenario promtest.Scenario) ([]byte, error) {
	_, fake, err := run(ctx, scenario)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for _, s := range fake.Seen() {
		query := strings.ReplaceAll(s.Query, "\n", `\n`)
		fmt.Fprintf(&buf, "%s\t%s\n", s.Path, query)
	}
	return buf.Bytes(), nil
}

// DataFile decodes an encoded snapshot and returns the encoded document in
// one language and one profile, with the severity stored in the snapshot.
// An empty profile keeps the snapshot's.
func DataFile(snap []byte, lang catalog.Language, profile string) ([]byte, error) {
	decoded, err := snapshot.Decode(snap)
	if err != nil {
		return nil, fmt.Errorf("decoding the snapshot: %w", err)
	}

	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, fmt.Errorf("loading the %s catalog: %w", lang, err)
	}

	doc, err := document.Build(decoded, cat, document.Options{Profile: profile})
	if err != nil {
		return nil, fmt.Errorf("building the document: %w", err)
	}

	data, err := document.Encode(doc)
	if err != nil {
		return nil, fmt.Errorf("encoding the document: %w", err)
	}
	return data, nil
}

// SnapshotPath returns the place of a scenario's golden snapshot file, from
// the folder that holds go.mod.
func SnapshotPath(scenario string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", "snapshot", scenario+".json"), nil
}

// DataFilePath returns the place of a scenario's golden data file, in one
// language.
func DataFilePath(scenario string, lang catalog.Language) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "template", "tests", "capacity", "fixtures", "command", scenario+"."+string(lang)+".json"), nil
}

// ExecutiveDataFilePath returns the place of a scenario's golden data file
// of the executive profile, in one language.
func ExecutiveDataFilePath(scenario string, lang catalog.Language) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "template", "tests", "capacity", "fixtures", "command", scenario+".executive."+string(lang)+".json"), nil
}

// QueriesPath returns the place of a scenario's golden file of queries.
func QueriesPath(scenario string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "testdata", "queries", scenario+".txt"), nil
}

// moduleRoot walks up from the working directory to the folder that holds
// go.mod. It is duplicated from internal/promtest, whose own moduleRoot is
// unexported.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

// ViewScenarios names the scenarios that have golden view files.
var ViewScenarios = []string{"edge-scaling", "direct-complete"}

// ViewFile decodes an encoded snapshot and returns the encoded report view in
// one language.
func ViewFile(snap []byte, lang catalog.Language) ([]byte, error) {
	decoded, err := snapshot.Decode(snap)
	if err != nil {
		return nil, fmt.Errorf("decoding the snapshot: %w", err)
	}

	cat, err := catalog.Load(lang)
	if err != nil {
		return nil, fmt.Errorf("loading the %s catalog: %w", lang, err)
	}

	view, err := document.BuildView(decoded, cat)
	if err != nil {
		return nil, fmt.Errorf("building the view: %w", err)
	}

	data, err := document.EncodeView(view)
	if err != nil {
		return nil, fmt.Errorf("encoding the view: %w", err)
	}
	return data, nil
}

// ViewFilePath returns the place of a scenario's golden view file, in one
// language.
func ViewFilePath(scenario string, lang catalog.Language) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "web", "tests", "fixtures", "view", scenario+"."+string(lang)+".json"), nil
}
