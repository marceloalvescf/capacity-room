package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/marceloalvescf/k8s-capacity-report/internal/platform/health"
)

// The queues of the worker: reports runs collections and renders, and
// maintenance runs the periodic job alone.
const (
	queueReports     = "reports"
	queueMaintenance = "maintenance"
)

// maintenanceEvery is the period of the maintenance job.
const maintenanceEvery = time.Minute

// Config is what NewClient builds the River client from.
type Config struct {
	Deps  Deps
	Slots int  // workers of the queue reports
	Test  bool // River's TestOnly: no startup stagger

	hooks collectHooks // lets a test hold a collect attempt
}

// ErrJobsDidNotStop is returned by Run when a handler has not returned after
// the drain and a second wait of the same length.
var ErrJobsDidNotStop = errors.New("jobs did not stop")

// NewClient returns the River client of the worker: the queue reports with
// Slots workers, the queue maintenance with one, the collect, render, and
// maintenance workers, the maintenance job every minute, the timeouts of the
// settings, the rescue after the larger timeout plus one minute, the
// reindexer off, and a logger that writes warnings and errors of River to
// the logger of Deps.
func NewClient(c Config) (*river.Client[pgx.Tx], error) {
	d := c.Deps
	workers := river.NewWorkers()
	river.AddWorker(workers, &CollectWorker{Deps: d, hooks: c.hooks})
	river.AddWorker(workers, &RenderWorker{Deps: d})
	river.AddWorker(workers, &MaintenanceWorker{Deps: d})

	longest := max(d.Settings.CollectTimeout, d.Settings.RenderTimeout)
	client, err := river.NewClient(riverpgxv5.New(d.Pool), &river.Config{
		TestOnly: c.Test,
		Logger:   slog.New(atLeast{d.Logger.Handler(), slog.LevelWarn}),
		Queues: map[string]river.QueueConfig{
			queueReports:     {MaxWorkers: c.Slots},
			queueMaintenance: {MaxWorkers: 1},
		},
		Workers:              workers,
		MaxAttempts:          d.Settings.JobMaxAttempts,
		JobTimeout:           longest,
		RescueStuckJobsAfter: longest + time.Minute,
		ReindexerSchedule:    river.NeverSchedule(),
		PeriodicJobs: []*river.PeriodicJob{
			river.NewPeriodicJob(
				river.PeriodicInterval(maintenanceEvery),
				func() (river.JobArgs, *river.InsertOpts) {
					return MaintenanceArgs{}, &river.InsertOpts{Queue: queueMaintenance, MaxAttempts: d.Settings.JobMaxAttempts}
				},
				&river.PeriodicJobOpts{RunOnStart: true},
			),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create worker client: %w", err)
	}
	return client, nil
}

// Running is a health check named worker: the client started and has not
// stopped. Start must have returned before the check can run concurrently
// with it.
func Running(client *river.Client[pgx.Tx]) health.Check {
	return health.Check{Name: "worker", Run: func(context.Context) bool {
		stopped := client.Stopped()
		if stopped == nil {
			return false
		}
		select {
		case <-stopped:
			return false
		default:
			return true
		}
	}}
}

// Run holds a started client until ctx is done or the client stops. After
// ctx is done it stops taking jobs and waits drain for the running ones,
// then cancels what remains, which River returns to the queue with its
// attempt restored, and waits drain again. A handler that still has not
// returned is abandoned and Run returns ErrJobsDidNotStop; River's rescuer
// recovers its job later. A client that stopped on its own is an error.
func Run(ctx context.Context, client *river.Client[pgx.Tx], drain time.Duration) error {
	select {
	case <-client.Stopped():
		return errors.New("the client stopped on its own")
	case <-ctx.Done():
	}

	soft, cancelSoft := context.WithTimeout(context.Background(), drain)
	defer cancelSoft()
	if err := client.Stop(soft); err == nil {
		return nil
	}
	hard, cancelHard := context.WithTimeout(context.Background(), drain)
	defer cancelHard()
	if err := client.StopAndCancel(hard); err != nil {
		return ErrJobsDidNotStop
	}
	return nil
}

// atLeast passes to its handler only the records of its level or above.
type atLeast struct {
	slog.Handler
	min slog.Level
}

func (h atLeast) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.min && h.Handler.Enabled(ctx, level)
}

func (h atLeast) WithAttrs(attrs []slog.Attr) slog.Handler {
	return atLeast{h.Handler.WithAttrs(attrs), h.min}
}

func (h atLeast) WithGroup(name string) slog.Handler {
	return atLeast{h.Handler.WithGroup(name), h.min}
}
