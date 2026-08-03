package scheduler

import (
	"testing"
	"time"
)

func at(hhmm string) time.Time {
	t, err := time.Parse("2006-01-02 15:04", "2026-08-03 "+hhmm)
	if err != nil {
		panic(err)
	}
	return t
}

// The whole reason this package exists: a gateway that slept must come back and
// do the work it missed, not pretend the window never happened.
func TestPlanCatchesUpAfterSleep(t *testing.T) {
	// Asleep from 09:00 to 09:05 on a one-minute job.
	got, skipped := Plan(at("09:00"), at("09:05"), time.Minute, time.Hour)
	if len(got) != 5 {
		t.Fatalf("want 5 windows to catch up, got %d", len(got))
	}
	if skipped != 0 {
		t.Fatalf("nothing was stale, want skipped=0, got %d", skipped)
	}
	if !got[0].From.Equal(at("09:00")) || !got[0].To.Equal(at("09:01")) {
		t.Errorf("first window = %v..%v, want 09:00..09:01", got[0].From, got[0].To)
	}
	if !got[4].To.Equal(at("09:05")) {
		t.Errorf("last window ends %v, want 09:05", got[4].To)
	}
}

// Windows must tile the interval exactly: no gaps (a missed reminder) and no
// overlaps (the same lecture considered twice).
func TestPlanWindowsAreContiguousAndNonOverlapping(t *testing.T) {
	got, _ := Plan(at("08:00"), at("08:10"), time.Minute, time.Hour)
	for i := 1; i < len(got); i++ {
		if !got[i].From.Equal(got[i-1].To) {
			t.Fatalf("window %d starts %v but previous ended %v — gap or overlap",
				i, got[i].From, got[i-1].To)
		}
	}
}

// Nothing has elapsed yet, so there is nothing to do. A scheduler that returned a
// window here would re-run the same work every tick.
func TestPlanNoCompleteWindowYet(t *testing.T) {
	got, _ := Plan(at("09:00"), at("09:00").Add(30*time.Second), time.Minute, time.Hour)
	if len(got) != 0 {
		t.Fatalf("want no windows before one has fully elapsed, got %d", len(got))
	}
}

// A week-long outage must not produce a week of stale alerts.
func TestPlanDropsWindowsOlderThanMaxCatchUp(t *testing.T) {
	last := at("09:00").Add(-24 * time.Hour)
	got, skipped := Plan(last, at("09:00"), time.Minute, 10*time.Minute)
	if len(got) != 10 {
		t.Fatalf("want only the last 10 minutes replayed, got %d windows", len(got))
	}
	if skipped == 0 {
		t.Error("want the dropped windows counted so an operator can see it happened")
	}
	if got[0].From.Before(at("08:50")) {
		t.Errorf("replay starts %v, want no earlier than 08:50", got[0].From)
	}
}

// A one-second Every after a long sleep must not spin forever holding a
// connection.
func TestPlanIsBoundedBySafetyValve(t *testing.T) {
	got, _ := Plan(at("00:00"), at("09:00"), time.Second, 24*time.Hour)
	if len(got) > maxWindowsPerSweep {
		t.Fatalf("want at most %d windows, got %d", maxWindowsPerSweep, len(got))
	}
}

func TestPlanRejectsZeroInterval(t *testing.T) {
	if got, _ := Plan(at("09:00"), at("10:00"), 0, time.Hour); got != nil {
		t.Fatalf("a zero interval would loop forever; want nil, got %d windows", len(got))
	}
}
