// Package policy holds the attendance rules that are no longer negotiable.
//
// These were per-tenant columns (tenants.attendance_threshold,
// tenants.auto_kill_minutes) with two editors — the admin's Settings page and the
// DQA's Thresholds page, both writing the same handler. The institution has since
// fixed the numbers, so the configurability is now a way to get them wrong rather
// than a feature: a threshold that can be raised to 90 in a dashboard is a
// threshold that will eventually be raised to 90 by accident, and every
// eligibility answer in the system moves with it.
//
// Fixing them here also removes a real hazard. session-manager's clearance.go read
// the threshold into `threshold := 0` and IGNORED the query error, so a database
// hiccup meant a threshold of zero — and a zero threshold clears every student for
// exams. A constant cannot fail to load.
//
// The columns stay in the database and the UI stays in the tree, commented out, so
// restoring per-tenant policy is an edit rather than an archaeology exercise.
package policy

// AttendanceThresholdPercent is the minimum attendance for exam eligibility.
//
// Fixed at 75. This was already the DEFAULT and already had a 75 floor clamped in
// both editors ("can be raised, never set below 75") — so fixing it removes the
// upward freedom, which is the part that was never used deliberately.
const AttendanceThresholdPercent = 75

// AutoKillGraceMinutes is how long after a lecture's timetabled END a session stays
// open before it closes itself.
//
// ANCHORED TO THE END, NOT THE START. "15 minutes after the timetabled time" reads
// either way, and the difference is not subtle: measured from the start, a two-hour
// lecture would auto-close at 08:15 and take attendance with it. Measured from the
// end, the session covers the lecture and then closes promptly, which is what an
// auto-kill is for. Change AutoKillAnchor below if the other reading was meant.
const AutoKillGraceMinutes = 15

// AutoKillFromScheduledEnd selects the anchor described above. Flipping it to false
// measures the grace from the scheduled START instead.
const AutoKillFromScheduledEnd = true

// SessionAutoCloseMinutes returns how many minutes after a lecture's scheduled
// start the session should close, given the lecture's length.
//
// durationMin of 0 means the unit has no timetabled length — fall back to a
// two-hour lecture, which is what the Android session controller has always
// assumed, rather than closing immediately.
func SessionAutoCloseMinutes(durationMin int) int {
	if !AutoKillFromScheduledEnd {
		return AutoKillGraceMinutes
	}
	if durationMin <= 0 {
		durationMin = 120
	}
	return durationMin + AutoKillGraceMinutes
}
