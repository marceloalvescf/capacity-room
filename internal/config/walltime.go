package config

import (
	"errors"
	"regexp"
	"strconv"
	"time"
)

// WallTimeLayout is the layout of a wall time of spec 5.2.
const WallTimeLayout = "2006-01-02 15:04"

// The refusals of ParseWallTime.
var (
	ErrWallTimeShape  = errors.New("a wall time must look like 2026-09-01 00:00")
	ErrWallTimeAbsent = errors.New("the wall time does not exist in the zone")
	ErrWallTimeTwice  = errors.New("the wall time occurs twice in the zone")
)

var wallTimeRe = regexp.MustCompile(`^([0-9]{4})-([0-9]{2})-([0-9]{2}) ([0-9]{2}):([0-9]{2})$`)

// ParseWallTime reads text as a wall time in loc and returns its one instant.
// A wall time that falls in a gap or an overlap of the zone is refused.
func ParseWallTime(text string, loc *time.Location) (time.Time, error) {
	m := wallTimeRe.FindStringSubmatch(text)
	if m == nil {
		return time.Time{}, ErrWallTimeShape
	}
	var f [5]int
	for i := range f {
		f[i], _ = strconv.Atoi(m[i+1]) // the pattern admits digits only
	}
	year, month, day, hour, minute := f[0], time.Month(f[1]), f[2], f[3], f[4]

	wall := time.Date(year, month, day, hour, minute, 0, 0, time.UTC)
	if wall.Year() != year || wall.Month() != month || wall.Day() != day ||
		wall.Hour() != hour || wall.Minute() != minute {
		return time.Time{}, ErrWallTimeShape
	}

	candidate := time.Date(year, month, day, hour, minute, 0, 0, loc)
	offsets := map[int]bool{}
	_, offsets0 := candidate.Zone()
	offsets[offsets0] = true
	zoneStart, zoneEnd := candidate.ZoneBounds()
	if !zoneStart.IsZero() {
		_, off := zoneStart.Add(-time.Second).In(loc).Zone()
		offsets[off] = true
	}
	if !zoneEnd.IsZero() {
		_, off := zoneEnd.In(loc).Zone()
		offsets[off] = true
	}

	var found []time.Time
	for off := range offsets {
		instant := wall.Add(-time.Duration(off) * time.Second)
		if instant.In(loc).Format(WallTimeLayout) != text {
			continue
		}
		duplicate := false
		for _, other := range found {
			if other.Equal(instant) {
				duplicate = true
			}
		}
		if !duplicate {
			found = append(found, instant)
		}
	}
	switch len(found) {
	case 0:
		return time.Time{}, ErrWallTimeAbsent
	case 1:
		return found[0], nil
	default:
		return time.Time{}, ErrWallTimeTwice
	}
}
