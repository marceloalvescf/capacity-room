//go:build postgres

package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/settings"
)

// newClient builds the client of the worker over the environment's pool.
func (e *env) newClient(hooks collectHooks) *river.Client[pgx.Tx] {
	e.t.Helper()
	client, err := NewClient(Config{Deps: e.deps, Slots: e.settings.WorkerSlots, Test: true, hooks: hooks})
	if err != nil {
		e.t.Fatal(err)
	}
	e.client = client
	return client
}

func clientSettings(s *settings.Settings) {
	s.WorkerSlots = 1
	s.WorkerDrain = time.Second
	s.RenderTimeout = 5 * time.Minute
	s.CollectTimeout = 15 * time.Minute
}

func TestRunning(t *testing.T) {
	e := newEnv(t, clientSettings)
	client := e.newClient(collectHooks{})
	lc := &Lifecycle{Client: client}
	check := lc.Check()
	if check.Name != "worker" {
		t.Fatalf("name %q", check.Name)
	}
	ctx := context.Background()
	if check.Run(ctx) {
		t.Fatal("running before the client started")
	}
	if err := client.Start(ctx); err != nil {
		t.Fatal(err)
	}
	if !check.Run(ctx) {
		t.Fatal("not running after the client started")
	}
	stopCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if check.Run(ctx) {
		t.Fatal("running after the client stopped")
	}
}

func TestRunStopsAnIdleClient(t *testing.T) {
	e := newEnv(t, clientSettings)
	client := e.newClient(collectHooks{})
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- (&Lifecycle{Client: client}).Run(ctx, time.Second) }()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestRunReturnsWhenTheClientStops(t *testing.T) {
	e := newEnv(t, clientSettings)
	client := e.newClient(collectHooks{})
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- (&Lifecycle{Client: client}).Run(context.Background(), time.Second) }()
	stopCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Run returned nil for a client that stopped on its own")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
	}
}

func TestWorkerStopAbandonsAHungJob(t *testing.T) {
	e, _, _ := scenarioEnv(t, clientSettings)
	reached := make(chan struct{})
	release := make(chan struct{})
	client := e.newClient(collectHooks{beforeClaim: func(context.Context, int) {
		close(reached)
		<-release // ignores the context of the job
	}})
	if err := client.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		close(release)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = client.StopAndCancel(ctx)
	})
	e.request()
	select {
	case <-reached:
	case <-time.After(waitLimit):
		t.Fatal("the attempt never started")
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	lc := &Lifecycle{Client: client}
	check := lc.Check()
	if !check.Run(ctx) {
		t.Fatal("not ready before the stop")
	}
	go func() { done <- lc.Run(ctx, time.Second) }()
	start := time.Now()
	cancel()
	// Readiness turns false when the stop begins, and the stop holds for the
	// whole drain, so the flag is polled until it falls or the limit ends.
	notReady := time.Now().Add(waitLimit)
	for check.Run(context.Background()) {
		if time.Now().After(notReady) {
			t.Fatal("still ready while the stop drains")
		}
		time.Sleep(5 * time.Millisecond)
	}
	if time.Since(start) >= time.Second {
		t.Fatal("readiness fell only after the first drain")
	}
	select {
	case err := <-done:
		if !errors.Is(err, ErrJobsDidNotStop) || err.Error() != "jobs did not stop" {
			t.Fatalf("err = %v", err)
		}
		if took := time.Since(start); took < 2*time.Second {
			t.Fatalf("gave up after %v, before both waits of the drain", took)
		}
		if check.Run(context.Background()) {
			t.Fatal("ready after the stop was abandoned")
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Run did not give up within twice the drain and a margin")
	}
}

func TestQueues(t *testing.T) {
	q := Queues(3)
	if len(q) != 2 || q["reports"].MaxWorkers != 3 || q["maintenance"].MaxWorkers != 1 {
		t.Fatalf("queues %+v", q)
	}
}
