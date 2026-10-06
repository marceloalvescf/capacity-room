// Package pipeline runs one collection: the preflight, the collectors, the
// useful check, the analysis, and the snapshot.
package pipeline

import (
	"context"
	"time"

	"github.com/marceloalvescf/k8s-capacity-report/internal/analysis"
	"github.com/marceloalvescf/k8s-capacity-report/internal/collect"
	"github.com/marceloalvescf/k8s-capacity-report/internal/config"
	"github.com/marceloalvescf/k8s-capacity-report/internal/discovery"
	"github.com/marceloalvescf/k8s-capacity-report/internal/prom"
	"github.com/marceloalvescf/k8s-capacity-report/internal/snapshot"
)

// Run is one collection: the source, the configuration, the window judged
// before the first request, and the two things a run reads at its end.
type Run struct {
	Options   prom.Options
	Config    config.Config
	Window    collect.Window
	EmittedAt time.Time              // the first clock read, taken by the caller
	Now       func() time.Time       // read once, after the useful check
	NewID     func() (string, error) // called once, after that read
	Version   string
	Step      func(ctx context.Context, s Step) error // nil: no hook; an error stops the run and is returned
}

// Step names a point of a run that a hook may observe.
type Step string

// The steps of a run, in the order the hook sees them.
const (
	StepCollecting   Step = "collecting"   // after the preflight
	StepAnalyzing    Step = "analyzing"    // after the useful check
	StepSnapshotting Step = "snapshotting" // after analysis
)

// Result is what the command prints from and what the worker stores.
type Result struct {
	Snapshot     snapshot.Snapshot
	Capabilities []discovery.Capability
	Collected    collect.Collected
}

// Collect runs the preflight, the collectors in their order, the useful
// check, reads Now, takes the id, analyses, and builds the snapshot. It
// returns the errors of discovery, prom, and collect unchanged.
func Collect(ctx context.Context, r Run) (Result, error) {
	c, err := discovery.RunPreflight(ctx, r.Options, r.Config.Cluster, r.Config.Analysis, r.Window.End)
	if err != nil {
		return Result{}, err
	}
	if err := r.step(ctx, StepCollecting); err != nil {
		return Result{}, err
	}

	collected, err := collect.All(ctx, c.Client, collect.Input{
		Scope:        c.Scope,
		Cluster:      r.Config.Cluster,
		Analysis:     r.Config.Analysis,
		Capabilities: c.Capabilities,
		Window:       r.Window,
	})
	if err != nil {
		return Result{}, err
	}
	if err := collect.CheckUseful(collected); err != nil {
		return Result{}, err
	}

	completedAt := r.Now()
	id, err := r.NewID()
	if err != nil {
		return Result{}, err
	}

	if err := r.step(ctx, StepAnalyzing); err != nil {
		return Result{}, err
	}
	findings := analysis.All(collected, r.Config.Thresholds)

	if err := r.step(ctx, StepSnapshotting); err != nil {
		return Result{}, err
	}
	snap, err := snapshot.Build(snapshot.Input{
		ID:                  id,
		GeneratorVersion:    r.Version,
		EmittedAt:           r.EmittedAt,
		Config:              r.Config,
		EndpointKind:        c.Client.Kind(),
		Scope:               c.Scope,
		Capabilities:        c.Capabilities,
		CollectionStarted:   r.EmittedAt,
		CollectionCompleted: completedAt,
		Window:              r.Window,
		Collected:           collected,
		Findings:            findings,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Snapshot: snap, Capabilities: c.Capabilities, Collected: collected}, nil
}

// step calls the hook, when there is one.
func (r Run) step(ctx context.Context, s Step) error {
	if r.Step == nil {
		return nil
	}
	return r.Step(ctx, s)
}
