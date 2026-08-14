package handlers

// Telling people a lecture moved.
//
// A lecturer changing room, time or day without telling anyone is the ordinary
// case, not the exception — a double-booked hall, a broken projector, a clash
// nobody spotted. The patroller is usually the first person in the institution to
// find out, because they went looking for a lecture and found the room empty.
//
// Until now that discovery had nowhere to go. The tick recorded taught/not-taught
// against the timetabled slot and the move itself vanished, so the timetable stayed
// wrong, the next patroller repeated the search, and students reading the published
// week were still sent to the old room.
//
// So a tick carrying a venue/time/date change fans out: to the lecturer, because it
// is their lecture and the change was informal; and to the people accountable for
// that unit — its head of department, the dean and QA handler of its school, and the
// DQA — because an informal move is exactly the kind of thing quality assurance is
// meant to see rather than hear about later.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// patrolSenderName is the name every patrol alert arrives under.
//
// It used to be the patroller's own full name, on the notification row AND in the sentence — so
// a lecturer opening their inbox read "From Jane Doe" above "you were recorded as NOT TAUGHT".
// That turns an institutional observation into one colleague accusing another, which is the fast
// way to make patrollers unwilling to record what they saw and lecturers argue with the person
// rather than the record. The observation belongs to quality assurance; the tick is the same tick
// whoever walked the corridor.
//
// Nothing is lost to accountability: lecturer_patrol_logs still stores patroller_id,
// patroller_name and patroller_staff_id on every row, and the QA patrol reports read them. The
// name is simply not what a lecturer is shown.
const patrolSenderName = "QA Monitor"

// lectureWhen renders a slot the way the timetable says it — "Wed 6 Aug at 14:00" — from the
// date and start time carried on the tick.
//
// Every patrol alert now states it. A lecturer who teaches the same unit to three cohorts got
// "You were recorded as NOT TAUGHT for Data Structures" and had no way to tell WHICH sitting was
// meant, so the one question the alert had to answer — was I supposed to be somewhere? — was the
// one it left open. Ticks also sync late from a phone that was offline, so "today" is not a safe
// assumption for the reader either.
//
// Degrades rather than lies: an unparseable or missing date falls back to whatever it was given,
// and a slot with neither date nor time renders as "" so the caller can leave the clause out.
func lectureWhen(date, startTime string) string {
	date = strings.TrimSpace(date)
	startTime = strings.TrimSpace(startTime)
	day := date
	if t, err := time.Parse("2006-01-02", date); err == nil {
		day = t.Format("Mon 2 Jan 2006")
	}
	switch {
	case day != "" && startTime != "":
		return day + " at " + startTime
	case startTime != "":
		return "at " + startTime
	default:
		return day
	}
}

// venueChangeRecipients resolves who should hear about a lecture that moved.
//
// Returns user ids. Deliberately best-effort: a missing HOD or an unset school is
// normal in a half-configured institution, and must not stop the lecturer being
// told. The one recipient that matters most is the cheapest to find.
func venueChangeRecipients(ctx context.Context, conn interface {
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
}, tenantID, unitID, lecturerStaffID string) []string {
	seen := map[string]bool{}
	out := []string{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}

	// The lecturer named on the record, plus everyone whose remit covers the unit's
	// department or school. One query, because these are all `users` rows selected by
	// different criteria and three round trips would be three chances to half-fail.
	rows, err := conn.Query(ctx, `
		WITH unit_org AS (
		    SELECT COALESCE(c.department, '') AS department,
		           COALESCE(c.school, '')     AS school
		      FROM course_units cu
		      JOIN courses c ON c.course_id = cu.course_id
		     WHERE cu.unit_id = $2 AND cu.tenant_id = $1
		     LIMIT 1
		)
		-- the lecturer themselves
		SELECT l.user_id::text
		  FROM lecturers l
		 WHERE l.tenant_id = $1
		   AND btrim(lower(l.staff_id)) = btrim(lower($3))
		   AND l.user_id IS NOT NULL
		UNION
		-- the head of that department, and the dean / QA handler of that school
		SELECT u.user_id::text
		  FROM users u, unit_org o
		 WHERE u.tenant_id = $1
		   AND (
		        (u.role IN ('HOD', 'QA_DEPT_REP')
		         AND o.department <> '' AND btrim(lower(u.department)) = btrim(lower(o.department)))
		     OR (u.role IN ('DEAN', 'QA_SCHOOL_HANDLER')
		         AND o.school <> '' AND btrim(lower(u.school)) = btrim(lower(o.school)))
		     -- a QA handler covering several colleges (migration 075)
		     OR (u.role = 'QA_SCHOOL_HANDLER' AND o.school <> '' AND EXISTS (
		           SELECT 1 FROM user_schools us
		             JOIN schools s ON s.school_id = us.school_id
		            WHERE us.user_id = u.user_id
		              AND btrim(lower(s.name)) = btrim(lower(o.school))))
		     -- quality assurance sees every informal change, institution-wide
		     OR u.role IN ('DQA_DIRECTOR', 'QA_OFFICER')
		   )`, tenantID, unitID, lecturerStaffID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if rows.Scan(&id) == nil {
			add(id)
		}
	}
	return out
}

// nonBlank drops the empty parts so a joined clause never renders a stray separator — a slot with
// no room must read "(at 16:00)", not "(at 16:00, )".
func nonBlank(parts ...string) []string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// venueChangeMessage describes the move in the words someone reading their inbox
// needs: which slot on the timetable, what was scheduled, and what actually happened.
//
// Not who observed it — see patrolSenderName.
func venueChangeMessage(l patrolLogIn) (subject, body string) {
	when := lectureWhen(l.SessionDate, l.ScheduledTime)

	subject = fmt.Sprintf("Lecture moved: %s", strings.TrimSpace(l.UnitName))
	if l.CourseCode != "" {
		subject = fmt.Sprintf("Lecture moved: %s (%s)", strings.TrimSpace(l.UnitName), l.CourseCode)
	}
	// The time in the SUBJECT, not only the body: a lecturer with two sittings of the same unit
	// otherwise gets two alerts with identical titles and has to open both to tell them apart.
	if when != "" {
		subject += " — " + when
	}

	var changes []string
	if v := strings.TrimSpace(l.FoundVenue); v != "" && !strings.EqualFold(v, strings.TrimSpace(l.Room)) {
		from := strings.TrimSpace(l.Room)
		if from == "" {
			from = "no room on the timetable"
		}
		changes = append(changes, fmt.Sprintf("room: %s → %s", from, v))
	}
	if t := strings.TrimSpace(l.FoundStartTime); t != "" && t != strings.TrimSpace(l.ScheduledTime) {
		changes = append(changes, fmt.Sprintf("time: %s → %s", l.ScheduledTime, t))
	}
	if d := strings.TrimSpace(l.FoundDate); d != "" && d != strings.TrimSpace(l.SessionDate) {
		changes = append(changes, fmt.Sprintf("date: %s → %s", l.SessionDate, d))
	}
	if len(changes) == 0 {
		changes = append(changes, "details differ from the published timetable")
	}

	// "its published slot (Thu 6 Aug 2026 at 16:00, LR3)" — the timetable's own answer to where
	// the reader was expected, stated before the diff that says where they were found.
	published := "its published slot"
	if detail := strings.TrimSpace(strings.Join(nonBlank(when, l.Room), ", ")); detail != "" {
		published += " (" + detail + ")"
	}

	body = fmt.Sprintf(
		"The QA monitor found this lecture away from %s — %s.\n\n"+
			"This change was not recorded on the timetable. If it is permanent, ask the "+
			"Teaching & Learning Centre to update the published week; students are still "+
			"being sent to the timetabled room.",
		published, strings.Join(changes, "; "))
	if r := strings.TrimSpace(l.Remarks); r != "" {
		// "Note recorded by the QA monitor", not "QA monitor's note" — the possessive invited the reader
		// to wonder whose, which is the question these alerts deliberately stop answering.
		body += "\n\nNote recorded by the QA monitor: " + r
	}
	return subject, body
}
