// Package jobs holds the scheduled work the gateway performs on its own.
//
// Everything here runs on internal/scheduler, which hands each job a window of
// civil time and guarantees that every elapsed window is processed exactly once —
// including the ones that elapsed while a free-tier service was asleep. Jobs are
// therefore written against a window, not against "now", and must be safe to
// re-run: idempotency comes from notification_log's
// UNIQUE (tenant_id, kind, subject_key, subject_date), claimed BEFORE sending.
package jobs

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/scheduler"
)

// Notification kinds. These are the subject_key namespaces in notification_log, so
// renaming one re-sends its whole history — don't.
const (
	KindLectureReminder   = "LECTURE_REMINDER"
	KindAttendanceMissing = "ATTENDANCE_MISSING"
	KindQAEscalation      = "QA_ESCALATION"
)

// Timings, in minutes relative to a slot's scheduled start.
const (
	remindBeforeMinutes  = 10 // "your lecture starts in 10 minutes"
	chaseAfterMinutes    = 10 // nobody has recorded you yet
	escalateAfterMinutes = 20 // still nobody: go and see QA
	qaVisitWithinMinutes = 20 // how long they have to present themselves
)

// dueSlot is one timetabled lecture that a job has decided to act on.
type dueSlot struct {
	TenantID     string
	SlotID       string
	UnitID       string
	UnitName     string
	CourseCode   string
	Room         string
	StartTime    string // HH:MM
	LecturerUser string // users.user_id — the person to tell
	LecturerName string
	StaffID      string
}

// Register wires every lecture-related job into the scheduler.
//
// All three run minute-by-minute because they fire on a precise offset from a
// lecture's start; a coarser window would make "10 minutes before" mean anywhere
// in a five-minute band. MaxCatchUp is deliberately short: a gateway that was down
// all morning must not wake at noon and tell forty lecturers their 9am lecture is
// about to start.
func Register(s *scheduler.Scheduler, pool *pgxpool.Pool) {
	s.Register(scheduler.Job{
		Name: "lecture_reminder", Every: minute, MaxCatchUp: 30 * minute,
		Run: func(ctx context.Context, w scheduler.Window) error { return lectureReminder(ctx, pool, w) },
	})
	s.Register(scheduler.Job{
		Name: "attendance_missing", Every: minute, MaxCatchUp: 60 * minute,
		Run: func(ctx context.Context, w scheduler.Window) error { return attendanceMissing(ctx, pool, w) },
	})
	s.Register(scheduler.Job{
		Name: "qa_escalation", Every: minute, MaxCatchUp: 60 * minute,
		Run: func(ctx context.Context, w scheduler.Window) error { return qaEscalation(ctx, pool, w) },
	})
}

// lectureReminder: 10 minutes before a lecture, tell the lecturer it is about to
// start. Purely a courtesy, and the only one of the three that is not an
// accusation — which is why it is the one with the shortest catch-up.
func lectureReminder(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window) error {
	slots, err := slotsStartingIn(ctx, pool, w, remindBeforeMinutes)
	if err != nil {
		return err
	}
	for _, s := range slots {
		subject := fmt.Sprintf("Starts in %d minutes: %s", remindBeforeMinutes, s.UnitName)
		body := fmt.Sprintf("%s%s begins at %s%s.",
			s.UnitName,
			paren(s.CourseCode),
			s.StartTime,
			ifNotBlank(s.Room, " in "))
		if err := notifyLecturer(ctx, pool, s, KindLectureReminder, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// attendanceMissing: 10 minutes after a lecture should have started, nobody —
// neither the coordinator's session nor a QA patroller — has recorded it.
//
// The lecturer is told rather than reported, because at this point the likeliest
// explanations are that the coordinator has not opened the session yet or that the
// patroller has not reached the room. Being told gives them a chance to fix it
// before it becomes a mark against them.
func attendanceMissing(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window) error {
	slots, err := slotsStartedAgo(ctx, pool, w, chaseAfterMinutes)
	if err != nil {
		return err
	}
	for _, s := range slots {
		recorded, err := attendanceRecorded(ctx, pool, s)
		if err != nil {
			return err
		}
		if recorded {
			continue
		}
		// The start time in the subject, like the patrol alerts: a lecturer with two sittings of
		// the same unit on one day otherwise gets two alerts titled identically.
		subject := fmt.Sprintf("Attendance not recorded: %s at %s", s.UnitName, s.StartTime)
		body := fmt.Sprintf(
			"Your %s lecture%s was due to start at %s%s, and neither the coordinator nor a QA patroller has recorded it.\n\n"+
				"If you are teaching, ask the coordinator to open the session.",
			s.UnitName, paren(s.CourseCode), s.StartTime, ifNotBlank(s.Room, " in "))
		if err := notifyLecturer(ctx, pool, s, KindAttendanceMissing, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// qaEscalation: 20 minutes in and still no record from anybody.
//
// A lecture that was taught but never tracked is the case this exists for: the
// teaching happened, no evidence of it exists, and the lecturer will otherwise be
// counted absent by every report downstream. They are asked to present themselves
// to the QA office within 20 minutes so the record can be corrected while it is
// still checkable.
func qaEscalation(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window) error {
	slots, err := slotsStartedAgo(ctx, pool, w, escalateAfterMinutes)
	if err != nil {
		return err
	}
	for _, s := range slots {
		recorded, err := attendanceRecorded(ctx, pool, s)
		if err != nil {
			return err
		}
		if recorded {
			continue
		}
		subject := fmt.Sprintf("Please visit the QA office: %s at %s", s.UnitName, s.StartTime)
		body := fmt.Sprintf(
			"Your %s lecture%s at %s%s has no attendance record — neither the coordinator nor a QA patroller captured it.\n\n"+
				"If you taught this lecture, please go to the Quality Assurance office within %d minutes so the record can be corrected. "+
				"Left as it is, this counts as a lecture that did not happen.",
			s.UnitName, paren(s.CourseCode), s.StartTime, ifNotBlank(s.Room, " in "), qaVisitWithinMinutes)
		if err := notifyLecturer(ctx, pool, s, KindQAEscalation, subject, body); err != nil {
			return err
		}
	}
	return nil
}

// ── the queries ──────────────────────────────────────────────────────────────

// slotsStartingIn finds lectures whose start falls `offset` minutes AFTER this
// window — i.e. the ones to warn about now.
func slotsStartingIn(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window, offset int) ([]dueSlot, error) {
	return querySlots(ctx, pool, w, offset, true)
}

// slotsStartedAgo finds lectures whose start was `offset` minutes BEFORE this window.
func slotsStartedAgo(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window, offset int) ([]dueSlot, error) {
	return querySlots(ctx, pool, w, offset, false)
}

// querySlots is the shared lookup. The window is half-open, so a lecture is
// returned by exactly one window and cannot be both missed and duplicated.
//
// Only slots with a resolvable lecturer ACCOUNT come back: there is no point
// deciding to notify somebody the system cannot reach, and a lecturer with no
// user_id has never signed in.
func querySlots(ctx context.Context, pool *pgxpool.Pool, w scheduler.Window, offset int, ahead bool) ([]dueSlot, error) {
	// The target start-time band, as minutes since midnight.
	fromMin := minutesSinceMidnight(w.From)
	toMin := minutesSinceMidnight(w.To)
	if ahead {
		fromMin += offset
		toMin += offset
	} else {
		fromMin -= offset
		toMin -= offset
	}
	// A window that has walked off the end of the day has nothing in it.
	if toMin < 0 || fromMin > 24*60 {
		return nil, nil
	}

	rows, err := pool.Query(ctx, `
		SELECT ts.tenant_id::text, ts.slot_id::text, ts.unit_id,
		       COALESCE(cu.name, ts.unit_id), COALESCE(cu.course_id, ''),
		       COALESCE(NULLIF(ts.room, ''), ts.venue_id, ''),
		       to_char(ts.start_time, 'HH24:MI'),
		       lec.user_id::text, COALESCE(lec.full_name, ''), COALESCE(lec.staff_id, '')
		  FROM timetable_slots ts
		  JOIN course_units cu ON cu.unit_id = ts.unit_id
		  JOIN LATERAL (
		      SELECT l.user_id, l.full_name, l.staff_id
		        FROM lecturers l
		       WHERE l.user_id IS NOT NULL
		         AND ( l.lecturer_id = ts.lecturer_id
		            OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
		                  SELECT la.lecturer_id FROM lecturer_assignments la
		                   WHERE la.unit_id = ts.unit_id
		                   ORDER BY la.academic_year DESC LIMIT 1) ) )
		       LIMIT 1
		  ) lec ON true
		 WHERE ts.day_of_week = $1
		   AND EXTRACT(HOUR FROM ts.start_time) * 60 + EXTRACT(MINUTE FROM ts.start_time) >= $2
		   AND EXTRACT(HOUR FROM ts.start_time) * 60 + EXTRACT(MINUTE FROM ts.start_time) <  $3`,
		isoWeekday(w.From), fromMin, toMin)
	if err != nil {
		return nil, fmt.Errorf("due slots: %w", err)
	}
	defer rows.Close()

	var out []dueSlot
	for rows.Next() {
		var s dueSlot
		if rows.Scan(&s.TenantID, &s.SlotID, &s.UnitID, &s.UnitName, &s.CourseCode,
			&s.Room, &s.StartTime, &s.LecturerUser, &s.LecturerName, &s.StaffID) == nil {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

// attendanceRecorded asks whether ANYBODY has evidence of this lecture today —
// the coordinator's session or a patrol tick. Either is enough; the two records
// exist to be compared, not to both be required.
func attendanceRecorded(ctx context.Context, pool *pgxpool.Pool, s dueSlot) (bool, error) {
	var found bool
	err := pool.QueryRow(ctx, `
		SELECT EXISTS (
		    SELECT 1 FROM sessions se
		     WHERE se.tenant_id = $1 AND se.unit_id = $2 AND se.session_date = $3::date
		       AND se.session_status IN ('ACTIVE','PENDING_LECTURER','CLOSED','AUTO_CLOSED')
		) OR EXISTS (
		    SELECT 1 FROM lecturer_patrol_logs p
		     WHERE p.tenant_id = $1 AND p.unit_id = $2 AND p.session_date = $3::date
		)`, s.TenantID, s.UnitID, clock.Today()).Scan(&found)
	return found, err
}

// notifyLecturer claims the notification and, only if the claim succeeded, writes
// it into the lecturer's inbox.
//
// Claim-then-send: a crash between the two leaves a notification recorded but not
// delivered, never one delivered twice. A missed nudge the person can recover from;
// duplicate 3am alerts destroy trust in every alert after them.
func notifyLecturer(ctx context.Context, pool *pgxpool.Pool, s dueSlot, kind, subject, body string) error {
	day, err := clock.ParseDate(clock.Today())
	if err != nil {
		return err
	}
	already, err := scheduler.AlreadySent(ctx, pool, s.TenantID, kind, s.SlotID, day, s.LecturerUser, "APP")
	if err != nil {
		return fmt.Errorf("claim %s for slot %s: %w", kind, s.SlotID, err)
	}
	if already {
		return nil
	}

	var nid string
	if err := pool.QueryRow(ctx, `
		INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, unit_id, subject, body)
		VALUES ($1, NULL, 'QAAT', 'SYSTEM', 'DIRECT', $2, $3, $4)
		RETURNING notification_id::text`,
		s.TenantID, s.UnitID, subject, body).Scan(&nid); err != nil {
		return fmt.Errorf("write notification: %w", err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
		VALUES ($1, $2, $3::uuid) ON CONFLICT DO NOTHING`, nid, s.TenantID, s.LecturerUser)
	return err
}

// ── small helpers ────────────────────────────────────────────────────────────

const minute = time.Minute

// minutesSinceMidnight converts a wall-clock instant to the same units
// timetable_slots.start_time is compared in.
func minutesSinceMidnight(t time.Time) int { return t.Hour()*60 + t.Minute() }

// isoWeekday is Monday=1 … Sunday=7, matching timetable_slots.day_of_week.
// Go puts Sunday at 0, and every place that forgets to remap it silently looks up
// the wrong day's timetable.
func isoWeekday(t time.Time) int {
	if d := int(t.Weekday()); d == 0 {
		return 7
	} else {
		return d
	}
}

func paren(s string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return " (" + s + ")"
}

func ifNotBlank(s, prefix string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return prefix + s
}
