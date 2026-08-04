package handlers

// The patroller's search.
//
//   GET /api/v1/patrol/search?by=lecturer|unit&q=…
//
// WHY SEARCH AT ALL. The round used to open on a list of every timetabled session in
// the institution, each with a Taught/Not-taught pair of buttons. That is a screen
// which invites ticking without visiting: the lecturer's name, room and time are all
// there, and nothing distinguishes a slot the patroller stood in front of from one
// they scrolled past. A patrol tick is an accusation weighed against the
// coordinator's own record precisely because it comes from someone who was there.
//
// So the patroller now sees nothing until they look someone up. They arrive at a
// room, read the staff ID or the unit code off the door or the lecturer's badge,
// search it, and get back that person's sessions for today. The list is a
// consequence of a deliberate act rather than a menu.
//
// Both searches return the same shape, because the confirm screen after them is the
// same screen: unit, lecturer, room, time — then one green tick or one red cross.

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

// patrolSearchSQL is shared by both modes. Only the final predicate differs, so the
// two searches cannot drift into returning different fields for the same lecture.
//
// $1 tenant, $2 ISO weekday, $3 the normalised query.
const patrolSearchSQL = `
	SELECT ts.unit_id,
	       COALESCE(cu.name, ts.unit_id),
	       COALESCE(cu.course_id, ''),
	       COALESCE(lec.staff_id, ''),
	       COALESCE(lec.full_name, ''),
	       COALESCE(NULLIF(ts.room, ''), ts.venue_id, ''),
	       COALESCE(ts.venue_id, ''),
	       ts.day_of_week,
	       to_char(ts.start_time, 'HH24:MI'),
	       COALESCE(ts.duration_minutes, 60),
	       COALESCE(ts.offering_id::text, ''),
	       -- The cohort label, so two intakes running the same unit at the same hour
	       -- are distinguishable in the result list rather than looking like a
	       -- duplicate the patroller has to guess between.
	       COALESCE(NULLIF(CONCAT_WS(' · ', c.name, o.session_type,
	                                 'Yr' || o.study_year, 'Sem' || o.semester,
	                                 NULLIF(o.intake, '')), ''), '')
	FROM timetable_slots ts
	JOIN course_units cu ON cu.unit_id = ts.unit_id AND cu.tenant_id = ts.tenant_id
	LEFT JOIN course_offerings o ON o.offering_id = ts.offering_id AND o.tenant_id = ts.tenant_id
	LEFT JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
	-- The lecturer for this slot: its own if set, else the unit's assignment. Same
	-- resolution the manifest uses, so the name the patroller searches by is the name
	-- the coordinator's dashboard shows.
	LEFT JOIN LATERAL (
	    SELECT l.staff_id, l.full_name
	    FROM lecturers l
	    WHERE l.tenant_id = ts.tenant_id
	      AND ( l.lecturer_id = ts.lecturer_id
	         OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
	               SELECT la.lecturer_id FROM lecturer_assignments la
	               WHERE la.unit_id = ts.unit_id AND la.tenant_id = ts.tenant_id
	               ORDER BY la.academic_year DESC LIMIT 1) ) )
	    LIMIT 1
	) lec ON true
	WHERE ts.tenant_id = $1 AND ts.day_of_week = $2 `

// PatrolSearch finds today's sessions by lecturer staff id or by unit code.
func PatrolSearch(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		mode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("by")))
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if mode == "" {
			mode = "lecturer"
		}
		if mode != "lecturer" && mode != "unit" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "by must be 'lecturer' or 'unit'"))
			return
		}
		// An empty query returns nothing, deliberately. Falling back to "everything"
		// would reinstate exactly the browse-the-institution screen this replaces.
		if q == "" {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"mode": mode, "query": "", "results": []patrolSlot{},
			})
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		// Same handset rule as every other patrol route: a token alone is not enough
		// to file an accusation.
		if err := checkPatrolDevice(r, conn, tenantID, userID); err != nil {
			writeDeviceRefusal(w)
			return
		}

		// Prefix match, case- and whitespace-insensitive. A patroller types "kiu/044"
		// off a badge that reads "KIU/044", and types a partial code far more often
		// than a complete one.
		needle := strings.ToLower(strings.TrimSpace(q)) + "%"

		var where string
		switch mode {
		case "lecturer":
			// Staff id OR name: the patroller may be reading either off a door.
			where = ` AND (btrim(lower(COALESCE(lec.staff_id,''))) LIKE $3
			            OR btrim(lower(COALESCE(lec.full_name,''))) LIKE $3)`
		case "unit":
			where = ` AND (btrim(lower(ts.unit_id)) LIKE $3
			            OR btrim(lower(COALESCE(cu.name,''))) LIKE $3)`
		}

		rows, err := conn.Query(r.Context(),
			patrolSearchSQL+where+` ORDER BY ts.start_time LIMIT 50`,
			tenantID, clock.ISOWeekday(), needle)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := make([]patrolSlot, 0)
		for rows.Next() {
			var s patrolSlot
			if err := rows.Scan(&s.UnitID, &s.UnitName, &s.CourseCode, &s.LecturerStaffID,
				&s.LecturerName, &s.Room, &s.RoomCode, &s.DayOfWeek, &s.StartTime,
				&s.DurationMinutes, &s.OfferingID, &s.Cohort); err != nil {
				continue
			}
			out = append(out, s)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"mode":    mode,
			"query":   q,
			"date":    clock.Today(),
			"results": out,
		})
	}
}
