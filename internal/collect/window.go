// Package collect turns range-query answers into the numbers the report
// shows: the query window collectors ask about, and the reduction of a
// series to an average, a minimum, a peak, a current value, and a ratio.
package collect

import (
	"errors"
	"fmt"
	"time"

	"github.com/marceloalvescf/capacity-room/internal/config"
)

// Window is the span of time the collectors ask about.
type Window struct {
	Start time.Time
	End   time.Time
	Step  string
}

// ErrEmptyPeriod is the error of a period whose start is not before its end.
var ErrEmptyPeriod = errors.New("the analysis period is empty")

// ErrRangeAfterRun is the error of a range that ends after the run started.
var ErrRangeAfterRun = errors.New("the analysis range ends after the run")

// rangeAfterRunError names the end of a refused range, the emission time, and
// the zone, and wraps ErrRangeAfterRun.
type rangeAfterRunError struct {
	end     string
	emitted string
	zone    string
}

func (e *rangeAfterRunError) Error() string {
	return fmt.Sprintf("analysis.end: the range ends at %s, after the run started at %s in %s", e.end, e.emitted, e.zone)
}

func (e *rangeAfterRunError) Unwrap() error { return ErrRangeAfterRun }

// emptyPeriodError names the period, the end, and the zone of a refused
// window, and wraps ErrEmptyPeriod.
type emptyPeriodError struct {
	period string
	end    string
	zone   string
}

func (e *emptyPeriodError) Error() string {
	return fmt.Sprintf("analysis.period: the period %s is empty at %s in %s, because the offset of the zone changes inside it; choose a longer period", e.period, e.end, e.zone)
}

func (e *emptyPeriodError) Unwrap() error { return ErrEmptyPeriod }

// NewWindow computes the query window of analysis at emittedAt. End is
// emittedAt cut to whole seconds; Start is the wall clock of End in the
// zone of analysis, less the period, normalised by the zone; Step comes
// from Step. When the zone's offset changes inside the period, Start can
// end up not before End; that window is refused. When analysis holds a
// range, Start and End are its two instants, and an End after emittedAt cut
// to whole seconds is refused.
func NewWindow(analysis config.Analysis, emittedAt time.Time) (Window, error) {
	loc, err := analysis.Location()
	if err != nil {
		return Window{}, err
	}
	if analysis.Start != "" {
		return rangeWindow(analysis, loc, emittedAt)
	}
	period, err := analysis.Duration()
	if err != nil {
		return Window{}, err
	}

	end := emittedAt.Truncate(time.Second)
	start := periodStart(end, loc, period)

	if !start.Before(end) {
		return Window{}, &emptyPeriodError{
			period: analysis.Period,
			end:    end.In(loc).Format(time.RFC3339),
			zone:   analysis.Timezone,
		}
	}

	return Window{Start: start, End: end, Step: Step(period)}, nil
}

// rangeWindow builds the window of the range of analysis.
func rangeWindow(analysis config.Analysis, loc *time.Location, emittedAt time.Time) (Window, error) {
	start, end, err := analysis.Range()
	if err != nil {
		return Window{}, err
	}
	run := emittedAt.Truncate(time.Second)
	if end.After(run) {
		return Window{}, &rangeAfterRunError{
			end:     end.In(loc).Format(config.WallTimeLayout),
			emitted: run.In(loc).Format(config.WallTimeLayout),
			zone:    analysis.Timezone,
		}
	}
	return Window{Start: start, End: end, Step: Step(end.Sub(start))}, nil
}

// periodStart computes the wall clock of emittedAt in loc, less period, and
// lets time.Date normalise the result in loc. This is deliberately not
// emittedAt.Add(-period): the two disagree exactly on a day that crosses a
// change of the zone's offset, and the original counts wall-clock time, not
// elapsed duration.
func periodStart(emittedAt time.Time, loc *time.Location, period time.Duration) time.Time {
	local := emittedAt.In(loc)
	return time.Date(
		local.Year(), local.Month(), local.Day(),
		local.Hour(), local.Minute(), local.Second()-int(period.Seconds()),
		0, loc,
	)
}

// Step returns the query resolution for period: up to 30 minutes 30s; up
// to 6 hours 2m; up to 24 hours 5m; up to 7 days 15m; otherwise 1h. Each
// limit is inclusive.
func Step(period time.Duration) string {
	switch {
	case period <= 30*time.Minute:
		return "30s"
	case period <= 6*time.Hour:
		return "2m"
	case period <= 24*time.Hour:
		return "5m"
	case period <= 7*24*time.Hour:
		return "15m"
	default:
		return "1h"
	}
}
