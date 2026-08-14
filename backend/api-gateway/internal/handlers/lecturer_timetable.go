package handlers

// THE LECTURER'S OWN TIMETABLE.
//
//	GET /api/v1/lecturer/timetable
//
// The coordinator has a weekly grid of ONE cohort. The lecturer's week is the other shape
// entirely: the same person teaches CSE 2420 to the Day cohort on Tuesday morning, to the Weekend
// cohort on Saturday, and supervises the e-learning run on a Sunday evening. Until now their
// dashboard offered a month of counters — "6 taught, 1 missed" — which answers how they did but
// never answers the question they actually open the page with, which is "where am I meant to be".
//
// So this returns EVERY slot the lecturer teaches, across every cohort, course, session type and
// intake, with the cohort's identity attached to each. The grid is Monday to Sunday, not Monday to
// Friday: weekend cohorts are a real part of the load, and a Mon–Fri grid silently hides a
// lecturer's Saturday. Distance/e-learning slots come back too, marked ONLINE, because a lecture
// with no room is still a lecture the lecturer has to turn up for.
//
// WHOSE SLOT IS IT. A slot names its lecturer (timetable_slots.lecturer_id); when it does not, the
// unit's assignment stands in. The old calendar keyed only on lecturer_assignments, so on a unit
// taught by two people across different cohorts it showed each of them the other's classes. The
// rule here is the same one the QA monitor's round already uses: the slot's own lecturer wins, and
// the assignment is only a fallback for slots that never named one.
//
// Filtering and sorting are deliberately left to the client. This is one lecturer's teaching load —
// tens of rows, not thousands — so shipping the whole set once lets the grid and the sortable list
// re-filter instantly without a round trip, and means a filter combination can never disagree with
// what the grid is showing.

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type lecturerSlot struct {
	SlotID   string `json:"slot_id"`
	UnitID   string `json:"unit_id"`
	UnitName string `json:"unit_name"`

	// The cohort this slot is taught to — the axis the lecturer filters on, because one unit
	// legitimately appears several times a week against different cohorts.
	OfferingID  string `json:"offering_id"`
	CourseID    string `json:"course_id"`
	CourseName  string `json:"course_name"`
	SessionType string `json:"session_type"` // Day · Evening · Weekend · Distance …
	Level       string `json:"level"`
	Intake      string `json:"intake"`
	StudyYear   int    `json:"study_year"`
	Semester    int    `json:"semester"`
	Coordinator string `json:"coordinator_name"`

	DayOfWeek int    `json:"day_of_week"` // 1 = Monday … 7 = Sunday
	StartTime string `json:"start_time"`  // HH:MM
	Minutes   int    `json:"duration_minutes"`

	Room     string `json:"room"`
	VenueID  string `json:"venue_id"`
	Building string `json:"building"`

	// ONLINE means this cohort is a distance / e-learning run: there is no room to walk to, the
	// class is started by the lecturer from wherever they are, and the students check in with the
	// rotating code rather than by being on a hotspot. See migration 087.
	DeliveryMode string `json:"delivery_mode"`

	// Whether the timetable names this lecturer on the slot itself, or they are here because they
	// hold the unit's assignment. Shown because "someone else may be covering this" is exactly the
	// ambiguity a lecturer needs to see rather than discover in the room.
	NamedOnSlot bool `json:"named_on_slot"`

	Enrolled int `json:"enrolled"`
}

// GET /api/v1/lecturer/timetable
func LecturerTimetable(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			// Not "no timetable" but "we could not tell who you are" — the dashboard says so
			// rather than showing an empty week that looks like a free one.
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"slots": []any{}, "lecturer": nil, "unresolved": true,
			})
			return
		}

		var fullName, staffID string
		_ = adminPool.QueryRow(r.Context(),
			`SELECT COALESCE(full_name,''), COALESCE(staff_id,'') FROM lecturers
			  WHERE lecturer_id = $1::uuid AND tenant_id = $2`,
			lecturerID, tenantID).Scan(&fullName, &staffID)

		rows, err := adminPool.Query(r.Context(), `
			SELECT ts.slot_id::text, ts.unit_id, COALESCE(cu.name, ts.unit_id),
			       COALESCE(ts.offering_id::text, ''), COALESCE(o.course_id, ''), COALESCE(c.name, ''),
			       COALESCE(o.session_type, ''), COALESCE(o.level, ''), COALESCE(o.intake, ''),
			       COALESCE(o.study_year, 0), COALESCE(o.semester, 0),
			       COALESCE(u.full_name, ''),
			       ts.day_of_week, to_char(ts.start_time, 'HH24:MI'),
			       COALESCE(ts.duration_minutes, 60),
			       COALESCE(NULLIF(ts.room, ''), v.name, ts.venue_id, ''),
			       COALESCE(ts.venue_id, ''), COALESCE(v.building, ''),
			       COALESCE(o.delivery_mode, 'IN_PERSON'),
			       COALESCE(ts.lecturer_id = $2::uuid, false),
			       COALESCE(enr.n, 0)
			FROM timetable_slots ts
			JOIN course_units cu          ON cu.unit_id = ts.unit_id
			LEFT JOIN course_offerings o  ON o.offering_id = ts.offering_id
			LEFT JOIN courses c           ON c.course_id = o.course_id
			LEFT JOIN users u             ON u.user_id::text = o.coordinator_id
			LEFT JOIN venues v            ON v.venue_id = ts.venue_id
			LEFT JOIN LATERAL (
			    SELECT COUNT(*) AS n FROM students_extended se
			    WHERE se.offering_id = ts.offering_id
			      AND se.enrollment_status = 'ACTIVE'
			) enr ON true
			WHERE ts.tenant_id = $1
			  -- The slot's own lecturer wins; the unit assignment is only a fallback for slots
			  -- that never named one. Without the first branch a lecturer sees a colleague's
			  -- cohort; without the second, an imported timetable (which rarely fills the
			  -- lecturer column) shows them nothing at all.
			  AND ( ts.lecturer_id = $2::uuid
			     OR ( ts.lecturer_id IS NULL
			          AND EXISTS (SELECT 1 FROM lecturer_assignments la
			                      WHERE la.unit_id = ts.unit_id
			                        AND la.lecturer_id = $2::uuid) ) )
			ORDER BY ts.day_of_week, ts.start_time, cu.name`,
			tenantID, lecturerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		slots := make([]lecturerSlot, 0)
		for rows.Next() {
			var s lecturerSlot
			if err := rows.Scan(&s.SlotID, &s.UnitID, &s.UnitName,
				&s.OfferingID, &s.CourseID, &s.CourseName,
				&s.SessionType, &s.Level, &s.Intake, &s.StudyYear, &s.Semester, &s.Coordinator,
				&s.DayOfWeek, &s.StartTime, &s.Minutes,
				&s.Room, &s.VenueID, &s.Building, &s.DeliveryMode, &s.NamedOnSlot,
				&s.Enrolled); err != nil {
				continue
			}
			slots = append(slots, s)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lecturer": map[string]string{"full_name": fullName, "staff_id": staffID},
			"slots":    slots,
		})
	}
}
