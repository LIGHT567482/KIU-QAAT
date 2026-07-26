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
			       COALESCE(string_agg(DISTINCT l.full_name || CASE WHEN COALESCE(l.phone,'')<>'' THEN ' ('||l.phone||')' ELSE '' END, ', '), '')
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
			Phone     string `json:"lecturer_phone"`
		}
		slots := []slot{}
		srows, serr := conn.Query(r.Context(), `
			SELECT s.unit_id, COALESCE(cu.name, s.unit_id), s.day_of_week,
			       to_char(s.start_time,'HH24:MI'), s.duration_minutes,
			       COALESCE(s.room,''),
			       COALESCE(NULLIF(l.full_name,''), la_l.full_name, ''),
			       COALESCE(NULLIF(l.phone,''),     la_l.phone,     '')
			FROM timetable_slots s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id AND cu.tenant_id = s.tenant_id
			LEFT JOIN lecturers   l  ON l.lecturer_id = s.lecturer_id
			-- fall back to the unit's assignment when the slot has no lecturer set
			LEFT JOIN LATERAL (
			    SELECT l2.full_name, l2.phone FROM lecturer_assignments la
			    JOIN lecturers l2 ON l2.lecturer_id = la.lecturer_id AND l2.tenant_id = la.tenant_id
			    WHERE la.unit_id = s.unit_id AND la.tenant_id = s.tenant_id ORDER BY la.academic_year DESC LIMIT 1
			) la_l ON true
			WHERE s.offering_id = $1::uuid
			ORDER BY s.day_of_week, s.start_time`, offeringID)
		if serr == nil {
			for srows.Next() {
				var s slot
				srows.Scan(&s.UnitID, &s.UnitName, &s.DayOfWeek, &s.StartTime, &s.Duration, &s.Room, &s.Lecturer, &s.Phone) //nolint:errcheck
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

// GET /api/v1/coordinator/attendance — per-student, per-unit attendance summary for
// the coordinator's cohort (drives the chronic-absentee + trend views). Sorted by the
// lowest attendance first so at-risk students surface immediately.
func CoordinatorAttendance(pool *pgxpool.Pool) http.HandlerFunc {
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
			SELECT se.student_id, se.full_name, s.unit_id, s.unit_name,
			       s.sessions_held, s.sessions_attended, s.attendance_percentage,
			       COALESCE(t.attendance_threshold, 75)
			FROM course_offerings o
			JOIN students_extended se ON se.offering_id = o.offering_id
			JOIN student_attendance_summary s ON s.student_id = se.student_id AND s.tenant_id = se.tenant_id
			JOIN tenants t ON t.tenant_id = o.tenant_id
			WHERE o.coordinator_id = $1 AND o.tenant_id = $2
			ORDER BY s.attendance_percentage ASC, se.full_name`, coordID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type row struct {
			StudentID         string  `json:"student_id"`
			FullName          string  `json:"full_name"`
			UnitID            string  `json:"unit_id"`
			UnitName          string  `json:"unit_name"`
			SessionsHeld      int     `json:"sessions_held"`
			SessionsAttended  int     `json:"sessions_attended"`
			AttendancePercent float64 `json:"attendance_percentage"`
			Threshold         int     `json:"threshold"`
		}
		out := []row{}
		for rows.Next() {
			var x row
			rows.Scan(&x.StudentID, &x.FullName, &x.UnitID, &x.UnitName, //nolint:errcheck
				&x.SessionsHeld, &x.SessionsAttended, &x.AttendancePercent, &x.Threshold)
			out = append(out, x)
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

// GET /api/v1/coordinator/trends — weekly attendance-rate trend across the
// coordinator's cohort (mirrors the phone app's Trends screen). Each week's rate =
// (attendances that week) / (sessions that week × enrolled students).
func CoordinatorTrends(pool *pgxpool.Pool) http.HandlerFunc {
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

		threshold := 75
		_ = conn.QueryRow(r.Context(),
			`SELECT COALESCE(attendance_threshold,75) FROM tenants WHERE tenant_id=$1`, tenantID).Scan(&threshold)

		rows, err := conn.Query(r.Context(), `
			WITH sess AS (
			  SELECT s.session_id, date_trunc('week', s.session_date)::date AS wk,
			         (SELECT COUNT(DISTINCT al.student_id) FROM attendance_logs al WHERE al.session_id = s.session_id) AS attended
			  FROM sessions s
			  WHERE s.coordinator_id = $1 AND s.tenant_id = $2
			    AND s.session_status IN ('CLOSED','AUTO_CLOSED')
			),
			enrolled AS (
			  SELECT COUNT(*)::int AS n FROM students_extended se
			  JOIN course_offerings o ON o.offering_id = se.offering_id
			  WHERE o.coordinator_id = $1 AND o.tenant_id = $2 AND se.enrollment_status = 'ACTIVE'
			)
			SELECT to_char(wk,'YYYY-MM-DD'),
			       COALESCE(SUM(attended),0)::int,
			       (COUNT(*) * GREATEST((SELECT n FROM enrolled),1))::int
			FROM sess GROUP BY wk ORDER BY wk`, coordID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type week struct {
			Label   string  `json:"label"`
			RatePct float64 `json:"rate_pct"`
			Below   bool    `json:"below_threshold"`
		}
		weeks := []week{}
		totA, totP := 0, 0
		var bestI, worstI = -1, -1
		for rows.Next() {
			var label string
			var att, poss int
			rows.Scan(&label, &att, &poss) //nolint:errcheck
			rate := 0.0
			if poss > 0 {
				rate = float64(att) / float64(poss) * 100
			}
			rate = float64(int(rate*10+0.5)) / 10
			weeks = append(weeks, week{label, rate, rate < float64(threshold)})
			totA += att
			totP += poss
			i := len(weeks) - 1
			if bestI < 0 || rate > weeks[bestI].RatePct {
				bestI = i
			}
			if worstI < 0 || rate < weeks[worstI].RatePct {
				worstI = i
			}
		}

		avg := 0.0
		if totP > 0 {
			avg = float64(int(float64(totA)/float64(totP)*1000+0.5)) / 10
		}
		trend := "STABLE"
		if len(weeks) >= 2 {
			d := weeks[len(weeks)-1].RatePct - weeks[len(weeks)-2].RatePct
			if d > 1 {
				trend = "RISING"
			} else if d < -1 {
				trend = "FALLING"
			}
		}
		belowCount := 0
		for _, wk := range weeks {
			if wk.Below {
				belowCount++
			}
		}
		var best, worst *week
		if bestI >= 0 {
			best = &weeks[bestI]
		}
		if worstI >= 0 {
			worst = &weeks[worstI]
		}
		current := 0.0
		if len(weeks) > 0 {
			current = weeks[len(weeks)-1].RatePct
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"threshold":         threshold,
			"weeks":             weeks,
			"current_week_rate": current,
			"semester_average":  avg,
			"best_week":         best,
			"worst_week":        worst,
			"weeks_below":       belowCount,
			"trend":             trend,
		})
	}
}
