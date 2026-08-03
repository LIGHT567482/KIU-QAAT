// Package clock is the system clock, in the institution's timezone.
//
// Before this existed the gateway asked three different questions and called
// them all "now": `time.Now().UTC()` (sessions, manifest cache keys),
// naked `time.Now()` (the session window, no-shows, report defaults), and one
// hardcoded `time.LoadLocation("Africa/Kampala")` in the employee report. Under
// docker-compose the container was pinned to Kampala so the middle group happened
// to be right; under render.yaml no TZ is set, so the same code runs in UTC and
// every "today" is three hours out. manifest.go managed to use UTC for its cache
// key and local for its weekday inside one handler, which near midnight disagree
// about what day it is.
//
// So: one place decides. Every "what day is it" and "what time is it" question in
// the gateway resolves here, in the zone the institution actually operates in, no
// matter what the container's TZ happens to be.
//
// UTC is still correct for durations, token expiry and anything stored as an
// instant — this package is for *civil* time: which calendar day a session
// belongs to, which weekday's timetable to serve, whether the clock has passed
// the session window.
package clock

import (
	"log/slog"
	"os"
	"sync"
	"time"
)

// DefaultTZ is used when INSTITUTION_TZ is unset. Kampala because that is where
// the institution is; override per deployment rather than editing this.
const DefaultTZ = "Africa/Kampala"

var (
	once sync.Once
	loc  *time.Location
)

// Location returns the institution's timezone, resolved once.
//
// A missing zoneinfo database is a real possibility: the services build FROM
// alpine and a scratch/distroless base would carry no tzdata at all. Falling back
// to UTC keeps the process up rather than panicking on boot, but it is logged at
// WARN because every date in the system silently shifts when it happens.
func Location() *time.Location {
	once.Do(func() {
		name := os.Getenv("INSTITUTION_TZ")
		if name == "" {
			name = DefaultTZ
		}
		l, err := time.LoadLocation(name)
		if err != nil {
			slog.Warn("institution timezone unavailable, falling back to UTC — every date will shift",
				"requested", name, "error", err)
			loc = time.UTC
			return
		}
		loc = l
	})
	return loc
}

// Name is the resolved zone name, for logging and for `SET TIME ZONE`.
func Name() string { return Location().String() }

// Now is the current instant in institution time.
func Now() time.Time { return time.Now().In(Location()) }

// Today is the current civil date as "2006-01-02".
func Today() string { return Now().Format("2006-01-02") }

// HHMM is the current wall-clock time as "15:04".
func HHMM() string { return Now().Format("15:04") }

// ISOWeekday is today's ISO weekday: Monday=1 … Sunday=7.
//
// Go's time.Weekday puts Sunday at 0; the whole schema (timetable_slots.day_of_week,
// session_active_days, the Android DAYS array) uses ISO. Converting in one place
// stops the 0-vs-7 remap being rewritten at each call site.
func ISOWeekday() int {
	if d := int(Now().Weekday()); d == 0 {
		return 7
	} else {
		return d
	}
}

// MinutesSinceMidnight is now as minutes past 00:00, for comparing against a
// timetable slot's start time without parsing dates.
func MinutesSinceMidnight() int {
	n := Now()
	return n.Hour()*60 + n.Minute()
}

// ParseDate reads a "2006-01-02" date as midnight in institution time. Used when
// a caller supplies a date and it must be anchored to the same zone the rest of
// the system counts days in.
func ParseDate(s string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", s, Location())
}

// ParseHHMM reads a "15:04" wall-clock time onto the given civil date.
func ParseHHMM(date time.Time, hhmm string) (time.Time, error) {
	t, err := time.ParseInLocation("15:04", hhmm, Location())
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(date.Year(), date.Month(), date.Day(), t.Hour(), t.Minute(), 0, 0, Location()), nil
}
