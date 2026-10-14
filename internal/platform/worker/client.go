package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"

	"github.com/marceloalvescf/capacity-room/internal/platform/health"
	"github.com/marceloalvescf/capacity-room/internal/platform/reports"
)

// queueMaintenance runs the periodic job alone.
const queueMaintenance = "maintenance"

// Queues are the queues of the worker: reports runs collections and renders
// with slots workers, and maintenance runs one job at a time.
func Queues(slots int) map[string]river.QueueConfig {
	return map[string]river.QueueConfig{
		reports.QueueReports: {MaxWorkers: slots},
		queueMaintenance:     {MaxWorkers: 1},
	}
}

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
		TestOnly:             c.Test,
		Logger:               slog.New(atLeast{d.Logger.Handler(), slog.LevelWarn}),
		Queues:               Queues(c.Slots),
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

// Lifecycle holds a started client and tells whether it is still taking
// jobs.
type Lifecycle struct {
	Client   *river.Client[pgx.Tx]
	stopping atomic.Bool
}

// Check is the health check named worker: the client started, has not
// stopped, and Run has not begun to stop it. Start must have returned before
// the check can run concurrently with it.
func (l *Lifecycle) Check() health.Check {
	return health.Check{Name: "worker", Run: func(context.Context) bool {
		if l.stopping.Load() {
			return false
		}
		stopped := l.Client.Stopped()
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

// Run holds the started client until ctx is done or the client stops. The
// moment ctx is done the check turns false; Run then stops taking jobs and
// waits drain for the running ones, then cancels what remains, which River
// returns to the queue with its attempt restored, and waits drain again. A
// handler that still has not returned is abandoned and Run returns
// ErrJobsDidNotStop; River's rescuer recovers its job later. A client that
// stopped on its own is an error.
func (l *Lifecycle) Run(ctx context.Context, drain time.Duration) error {
	client := l.Client
	select {
	case <-client.Stopped():
		return errors.New("the client stopped on its own")
	case <-ctx.Done():
	}
	l.stopping.Store(true)

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
