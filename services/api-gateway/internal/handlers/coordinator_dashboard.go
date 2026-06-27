package handlers

// Coordinator dashboard (next.txt #1): after closing a session the coordinator is
// taken to a dashboard scoped to THEIR offering (session cohort) — the program's
// units with codes, the assigned lecturers and the day/time they're studied, the
// students in the cohort, and the roster of the last closed (synced) session.
// Everything is scoped through course_offerings.coordinator_id, so one coordinator
// can never see another session's data.

import (
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/coordinator/overview
func CoordinatorOverview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())

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

		var offeringID, courseID, courseName, level, intake, sessionType string
		var studyYear, semester int
		err = conn.QueryRow(r.Context(), `
			SELECT o.offering_id::text, o.course_id, c.name,
			       o.study_year, o.semester, COALESCE(o.level,''), COALESCE(o.intake,''), o.session_type
			FROM course_offerings o
			JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
			WHERE o.coordinator_id = $1 AND o.tenant_id = $2`,
			coordID, tenantID).Scan(&offeringID, &courseID, &courseName, &studyYear, &semester, &level, &intake, &sessionType)
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusOK, map[string]interface{}{"offering": nil, "units": []any{}})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		// Units are scoped to THIS cohort's year + semester.
		rows, err := conn.Query(r.Context(), `
			SELECT cu.unit_id, cu.name, COALESCE(cu.year,1), COALESCE(cu.semester,1),
			       COALESCE(ous.day_of_week, 0),
			       COALESCE(to_char(ous.session_start, 'HH24:MI'), ''),
			       COALESCE(ous.session_duration_minutes, 0),
			       COALESCE(string_agg(DISTINCT l.full_name, ', '), '')
			FROM course_offerings o
			JOIN course_units cu ON cu.course_id = o.course_id AND cu.tenant_id = o.tenant_id
			       AND cu.year = o.study_year AND cu.semester = o.semester AND cu.level = o.level
			LEFT JOIN offering_unit_schedules ous ON ous.offering_id = o.offering_id AND ous.unit_id = cu.unit_id
			LEFT JOIN lecturer_assignments la ON la.unit_id = cu.unit_id AND la.tenant_id = o.tenant_id
			LEFT JOIN lecturers l ON l.lecturer_id = la.lecturer_id
			WHERE o.offering_id = $1::uuid
			GROUP BY cu.unit_id, cu.name, cu.year, cu.semester, ous.day_of_week, ous.session_start, ous.session_duration_minutes
			ORDER BY cu.year, cu.semester, cu.name`, offeringID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type unit struct {
			UnitID          string `json:"unit_id"`
			Name            string `json:"name"`
			Year            int    `json:"year"`
			Semester        int    `json:"semester"`
			DayOfWeek       int    `json:"day_of_week"`
			SessionStart    string `json:"session_start"`
			DurationMinutes int    `json:"session_duration_minutes"`
			Lecturers       string `json:"lecturers"`
		}
		units := []unit{}
		for rows.Next() {
			var u unit
			rows.Scan(&u.UnitID, &u.Name, &u.Year, &u.Semester, &u.DayOfWeek, //nolint:errcheck
				&u.SessionStart, &u.DurationMinutes, &u.Lecturers)
			units = append(units, u)
		}

		// The full multi-day weekly grid (timetable_slots) for THIS offering — what
		// the coordinator's dashboard timetable renders. One row per day a unit runs.
		type slot struct {
			UnitID    string `json:"unit_id"`
			UnitName  string `json:"unit_name"`
			DayOfWeek int    `json:"day_of_week"`
			StartTime string `json:"start_time"`
			Duration  int    `json:"duration_minutes"`
			Room      string `json:"room"`
			Lecturer  string `json:"lecturer_name"`
		}
		slots := []slot{}
		srows, serr := conn.Query(r.Context(), `
			SELECT s.unit_id, COALESCE(cu.name, s.unit_id), s.day_of_week,
			       to_char(s.start_time,'HH24:MI'), s.duration_minutes,
			       COALESCE(s.room,''), COALESCE(l.full_name,'')
			FROM timetable_slots s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id AND cu.tenant_id = s.tenant_id
			LEFT JOIN lecturers   l  ON l.lecturer_id = s.lecturer_id
			WHERE s.offering_id = $1::uuid
			ORDER BY s.day_of_week, s.start_time`, offeringID)
		if serr == nil {
			for srows.Next() {
				var s slot
				srows.Scan(&s.UnitID, &s.UnitName, &s.DayOfWeek, &s.StartTime, &s.Duration, &s.Room, &s.Lecturer) //nolint:errcheck
				slots = append(slots, s)
			}
			srows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"offering": map[string]interface{}{
				"offering_id": offeringID, "course_id": courseID, "course_name": courseName,
				"session_type": sessionType, "study_year": studyYear, "semester": semester,
				"level": level, "intake": intake,
			},
			"units": units,
			"slots": slots,
		})
	}
}

// GET /api/v1/coordinator/students — students in the coordinator's offering.
func CoordinatorStudents(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())

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

		rows, err := conn.Query(r.Context(), `
			SELECT se.student_id, se.full_name, se.email,
			       COALESCE(se.current_year,1), COALESCE(se.semester,1),
			       COALESCE(se.academic_year,''), COALESCE(se.intake_session,''),
			       se.enrollment_status::text
			FROM course_offerings o
			JOIN students_extended se ON se.offering_id = o.offering_id
			WHERE o.coordinator_id = $1 AND o.tenant_id = $2
			ORDER BY se.full_name`, coordID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type student struct {
			StudentID     string `json:"student_id"`
			FullName      string `json:"full_name"`
			Email         string `json:"email"`
			CurrentYear   int    `json:"current_year"`
			Semester      int    `json:"semester"`
			AcademicYear  string `json:"academic_year"`
			IntakeSession string `json:"intake_session"`
			Status        string `json:"enrollment_status"`
		}
		out := []student{}
		for rows.Next() {
			var s student
			rows.Scan(&s.StudentID, &s.FullName, &s.Email, &s.CurrentYear, &s.Semester, //nolint:errcheck
				&s.AcademicYear, &s.IntakeSession, &s.Status)
			out = append(out, s)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GET /api/v1/coordinator/last-roster — the most recent CLOSED session's roster.
func CoordinatorLastRoster(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())

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

		var sessionID, unitName, sessionDate, closedAt, offeringID string
		err = conn.QueryRow(r.Context(), `
			SELECT s.session_id::text, COALESCE(cu.name, s.unit_id), s.session_date::text,
			       COALESCE(s.gate_close_time::text, ''), COALESCE(s.offering_id::text, '')
			FROM sessions s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
			WHERE s.coordinator_id = $1 AND s.tenant_id = $2
			  AND s.session_status IN ('CLOSED', 'AUTO_CLOSED')
			ORDER BY s.gate_close_time DESC NULLS LAST, s.session_date DESC
			LIMIT 1`, coordID, tenantID).Scan(&sessionID, &unitName, &sessionDate, &closedAt, &offeringID)
		if err == pgx.ErrNoRows {
			writeJSON(w, http.StatusOK, map[string]interface{}{"session": nil, "roster": []any{}})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		// Roster scoped to the session's offering cohort (fall back to program for
		// legacy sessions without an offering_id).
		rows, err := conn.Query(r.Context(), `
			SELECT se.student_id, se.full_name,
			       CASE WHEN al.log_id IS NOT NULL THEN 'PRESENT' ELSE 'ABSENT' END,
			       COALESCE(al.checkin_timestamp::text, '')
			FROM students_extended se
			LEFT JOIN attendance_logs al ON al.student_id = se.student_id AND al.session_id = $1
			WHERE se.tenant_id = $2 AND se.enrollment_status = 'ACTIVE'
			  AND ( ($3 <> '' AND se.offering_id::text = $3)
			        OR ($3 = '' AND se.course_id = (
			              SELECT cu.course_id FROM course_units cu
			              JOIN sessions s ON s.unit_id = cu.unit_id WHERE s.session_id = $1)) )
			ORDER BY CASE WHEN al.log_id IS NOT NULL THEN 0 ELSE 1 END, se.full_name`,
			sessionID, tenantID, offeringID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type row struct {
			StudentID   string `json:"student_id"`
			FullName    string `json:"full_name"`
			Status      string `json:"status"`
			CheckinTime string `json:"checkin_time,omitempty"`
		}
		roster := []row{}
		present := 0
		for rows.Next() {
			var rr row
			rows.Scan(&rr.StudentID, &rr.FullName, &rr.Status, &rr.CheckinTime) //nolint:errcheck
			if rr.Status == "PRESENT" {
				present++
			}
			roster = append(roster, rr)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session": map[string]interface{}{
				"session_id": sessionID, "unit_name": unitName, "session_date": sessionDate,
				"closed_at": closedAt, "present": present, "total": len(roster),
			},
			"roster": roster,
		})
	}
}

// GET /api/v1/coordinator/active-sessions
// Lists the coordinator's currently live sessions (ACTIVE or awaiting the
// lecturer gate) so they can see and close one straight from the dashboard —
// the same sessions that block opening a new one with SESSION_ALREADY_OPEN.
func CoordinatorActiveSessions(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())

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

		rows, err := conn.Query(r.Context(), `
			SELECT s.session_id::text, COALESCE(cu.name, s.unit_id), s.session_status::text,
			       COALESCE(s.gate_open_time::text, ''), COALESCE(s.checkin_window_end::text, ''),
			       (SELECT COUNT(*) FROM attendance_logs WHERE session_id = s.session_id)
			FROM sessions s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
			WHERE s.coordinator_id = $1 AND s.tenant_id = $2
			  AND s.session_status IN ('ACTIVE', 'PENDING_LECTURER')
			ORDER BY s.gate_open_time DESC NULLS LAST`,
			coordID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type sess struct {
			SessionID    string `json:"session_id"`
			UnitName     string `json:"unit_name"`
			Status       string `json:"status"`
			GateOpenTime string `json:"gate_open_time,omitempty"`
			WindowEnd    string `json:"checkin_window_end,omitempty"`
			PresentCount int    `json:"present_count"`
		}
		sessions := []sess{}
		for rows.Next() {
			var s sess
			rows.Scan(&s.SessionID, &s.UnitName, &s.Status, &s.GateOpenTime, &s.WindowEnd, &s.PresentCount) //nolint:errcheck
			sessions = append(sessions, s)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"sessions": sessions})
	}
}
