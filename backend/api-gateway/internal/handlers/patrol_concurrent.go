package handlers

// THE OTHER UNITS IN THE ROOM.
//
// A monitor searches a lecturer, gets their lecture, and ticks it. But one hour in one room in
// front of one class routinely satisfies SEVERAL course units: the same taught content is required
// by several programmes, and each programme codes and names it differently — "Research Methods" is
// CSC 3103 to one cohort and BIT 3110 to another, usually owned by different departments.
//
// The search returned whichever single slot matched what was typed. So the monitor ticked one unit,
// and every student on the other codes had a lecture that quality assurance has no record of, while
// the lecturer was credited with one unit for an hour that delivered three. Neither of those is
// visible as an error afterwards — the record simply looks like a smaller lecture than it was.
//
// This attaches the rest to each result. They are not alternatives for the monitor to choose
// between: the monitor is standing in front of all of them simultaneously, and the record should
// say so.

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// attachConcurrentUnits fills AlsoHere on every slot in the list.
//
// SAME LECTURER, SAME WEEKDAY, SAME START, SAME ROOM — all four. Dropping any one of them would
// group lectures that are not the same lecture:
//
//   - without the lecturer, two different people teaching different cohorts at nine o'clock in
//     adjacent rooms would be merged
//   - without the room, a lecturer's two genuinely separate 09:00 classes (which the timetable
//     should not contain, but does when it is wrong) would be merged and the error hidden
//   - without the start time, a lecturer's whole day would collapse into one entry
//
// Rooms are compared on the resolved name, because one slot may carry a venue_id and another the
// free-text room for the same hall, and a monitor standing in it cannot tell those apart.
//
// One query for the whole result set rather than one per row: a lecturer's day is a handful of
// slots, and a per-row query would turn a corridor search into a dozen round trips on a phone
// that is about to lose signal.
//
// The weekday comes from each slot rather than from "today", because the offline manifest carries a
// whole WEEK: keying on today would give Monday's companions to Thursday's lecture, which is worse
// than none — it would name units that are not in the room.
func attachConcurrentUnits(ctx context.Context, conn *pgxpool.Conn, tenantID string, slots []patrolSlot) {
	if len(slots) == 0 {
		return
	}

	keys := make(map[string]bool, len(slots))
	staffIDs := make([]string, 0, len(slots))
	for i := range slots {
		if slots[i].AlsoHere == nil {
			slots[i].AlsoHere = []patrolSlotUnit{}
		}
		if s := strings.TrimSpace(slots[i].LecturerStaffID); s != "" && !keys[s] {
			keys[s] = true
			staffIDs = append(staffIDs, s)
		}
	}
	if len(staffIDs) == 0 {
		return
	}

	rows, err := conn.Query(ctx, `
		SELECT COALESCE(lec.staff_id,''), ts.day_of_week, to_char(ts.start_time,'HH24:MI'),
		       COALESCE(NULLIF(ts.room,''), v.name, ts.venue_id, ''),
		       ts.unit_id, COALESCE(cu.name, ts.unit_id), COALESCE(cu.course_id,''),
		       COALESCE(ts.offering_id::text,''),
		       COALESCE(NULLIF(CONCAT_WS(' · ', c.name, o.session_type,
		                'Yr' || o.study_year, 'Sem' || o.semester, NULLIF(o.intake,'')), ''), '')
		  FROM timetable_slots ts
		  JOIN course_units cu ON cu.unit_id = ts.unit_id
		  LEFT JOIN venues v ON v.venue_id = ts.venue_id
		  LEFT JOIN course_offerings o ON o.offering_id = ts.offering_id
		  LEFT JOIN courses c ON c.course_id = o.course_id
		  LEFT JOIN LATERAL (
		      SELECT l.staff_id FROM lecturers l
		       WHERE ( l.lecturer_id = ts.lecturer_id
		            OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
		                  SELECT la.lecturer_id FROM lecturer_assignments la
		                   WHERE la.unit_id = ts.unit_id
		                   ORDER BY la.academic_year DESC LIMIT 1) ) )
		       LIMIT 1
		  ) lec ON true
		 WHERE ts.tenant_id = $1
		   AND COALESCE(lec.staff_id,'') = ANY($2)
		   -- Distance cohorts have no room to stand in, so they are never part of what a monitor
		   -- is looking at (see PatrolManifest).
		   AND COALESCE(o.delivery_mode, 'IN_PERSON') <> 'ONLINE'`,
		tenantID, staffIDs)
	if err != nil {
		return // the search itself still works; it just carries no companions
	}
	defer rows.Close()

	type row struct {
		staff, start, room string
		dow                int
		u                  patrolSlotUnit
	}
	all := []row{}
	for rows.Next() {
		var x row
		if rows.Scan(&x.staff, &x.dow, &x.start, &x.room, &x.u.UnitID, &x.u.UnitName,
			&x.u.CourseCode, &x.u.OfferingID, &x.u.Cohort) == nil {
			x.u.Room = x.room
			all = append(all, x)
		}
	}

	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	for i := range slots {
		me := &slots[i]
		for _, x := range all {
			if norm(x.staff) != norm(me.LecturerStaffID) ||
				x.dow != me.DayOfWeek ||
				x.start != me.StartTime ||
				norm(x.room) != norm(me.Room) {
				continue
			}
			// The unit the monitor already has in front of them is not "also here". Matched on the
			// cohort as well as the code, because one unit taught to two cohorts in the same room
			// at the same hour is two real entries the monitor needs to see, not a duplicate.
			if norm(x.u.UnitID) == norm(me.UnitID) && x.u.OfferingID == me.OfferingID {
				continue
			}
			me.AlsoHere = append(me.AlsoHere, x.u)
		}
	}
}
