package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/dashboard/vc/overview
// Query params: date_from, date_to, course_id, unit_id, department
func VCOverview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		q := r.URL.Query()
		dateFrom := q.Get("date_from")
		dateTo := q.Get("date_to")
		courseID := q.Get("course_id")
		unitID := q.Get("unit_id")
		department := q.Get("department")

		today := time.Now().Format("2006-01-02")
		if dateFrom == "" {
			dateFrom = today
		}
		if dateTo == "" {
			dateTo = today
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// Build dynamic WHERE clauses for filter params.
		args := []interface{}{tenantID, dateFrom, dateTo}
		wheres := []string{"s.tenant_id = $1", "s.session_date BETWEEN $2 AND $3"}
		argIdx := 4

		if courseID != "" {
			wheres = append(wheres, fmt.Sprintf("cu.course_id = $%d", argIdx))
			args = append(args, courseID)
			argIdx++
		}
		if unitID != "" {
			wheres = append(wheres, fmt.Sprintf("s.unit_id = $%d", argIdx))
			args = append(args, unitID)
			argIdx++
		}
		if department != "" {
			wheres = append(wheres, fmt.Sprintf("c.department = $%d", argIdx))
			args = append(args, department)
			argIdx++
		}
		_ = argIdx

		whereClause := strings.Join(wheres, " AND ")

		var totalScheduled, totalActual, totalStudents, ghostCount int
		var avgPct float64

		conn.QueryRow(r.Context(), fmt.Sprintf(`
			SELECT
			  COUNT(DISTINCT s.session_id) AS total_scheduled,
			  COUNT(DISTINCT CASE WHEN s.session_status IN ('ACTIVE','CLOSED','AUTO_CLOSED') THEN s.session_id END) AS total_actual,
			  COUNT(DISTINCT al.student_id),
			  COUNT(DISTINCT CASE WHEN 'GHOST_LECTURE_SUSPECTED' = ANY(s.audit_flags) THEN s.session_id END),
			  COALESCE(ROUND(AVG(pcts.unit_pct),2), 0)
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			JOIN courses c ON c.course_id = cu.course_id
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			LEFT JOIN (
			  SELECT sub.session_id,
			    CASE WHEN COUNT(al2.student_id) = 0 THEN 0 ELSE
			      ROUND(100.0 * COUNT(al2.student_id) / NULLIF(
			        (SELECT COUNT(*) FROM students_extended se2 WHERE se2.course_id = sub.course_id AND se2.tenant_id = sub.tenant_id), 0
			      ), 1)
			    END AS unit_pct,
			    sub.course_id,
			    sub.tenant_id
			  FROM (
			    SELECT s2.session_id, cu2.course_id, s2.tenant_id
			    FROM sessions s2
			    JOIN course_units cu2 ON cu2.unit_id = s2.unit_id
			    WHERE s2.session_date BETWEEN $2 AND $3 AND s2.tenant_id = $1
			  ) sub
			  LEFT JOIN attendance_logs al2 ON al2.session_id = sub.session_id
			  GROUP BY sub.session_id, sub.course_id, sub.tenant_id
			) pcts ON pcts.session_id = s.session_id
			WHERE %s`, whereClause), args...).
			Scan(&totalScheduled, &totalActual, &totalStudents, &ghostCount, &avgPct) //nolint:errcheck

		// IER: (actual sessions / scheduled sessions) × avg attendance %.
		var ier float64
		if totalScheduled > 0 {
			ier = float64(totalActual) / float64(totalScheduled) * avgPct
		}

		// Eligibility summary.
		var eligible, ineligible int
		conn.QueryRow(r.Context(), `
			SELECT
			  COUNT(*) FILTER (WHERE attendance_percentage >= (SELECT attendance_threshold FROM tenants WHERE tenant_id = $1)),
			  COUNT(*) FILTER (WHERE attendance_percentage <  (SELECT attendance_threshold FROM tenants WHERE tenant_id = $1))
			FROM student_attendance_summary WHERE tenant_id = $1`, tenantID).
			Scan(&eligible, &ineligible) //nolint:errcheck

		// Ghost lecture list.
		type ghostSession struct {
			SessionID    string `json:"session_id"`
			UnitID       string `json:"unit_id"`
			UnitName     string `json:"unit_name"`
			SessionDate  string `json:"session_date"`
			StudentCount int    `json:"student_count"`
		}
		ghostRows, err := conn.Query(r.Context(), `
			SELECT s.session_id, s.unit_id, cu.name, s.session_date::text,
			       COUNT(al.log_id)
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			WHERE s.tenant_id = $1 AND 'GHOST_LECTURE_SUSPECTED' = ANY(s.audit_flags)
			  AND s.session_date BETWEEN $2 AND $3
			GROUP BY s.session_id, cu.name
			ORDER BY s.session_date DESC
			LIMIT 50`, tenantID, dateFrom, dateTo)
		var ghostSessions []ghostSession
		if err == nil {
			defer ghostRows.Close()
			for ghostRows.Next() {
				var gs ghostSession
				ghostRows.Scan(&gs.SessionID, &gs.UnitID, &gs.UnitName, &gs.SessionDate, &gs.StudentCount) //nolint:errcheck
				ghostSessions = append(ghostSessions, gs)
			}
		}
		if ghostSessions == nil {
			ghostSessions = []ghostSession{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"total_sessions_scheduled": totalScheduled,
			"total_sessions_actual":    totalActual,
			"total_students_today":     totalStudents,
			"ghost_lecture_count":      ghostCount,
			"avg_attendance_pct":       avgPct,
			"ier":                      fmt.Sprintf("%.1f", ier),
			"eligibility_summary": map[string]int{
				"eligible":   eligible,
				"ineligible": ineligible,
				"pending":    0,
			},
			"filters": map[string]string{
				"date_from":  dateFrom,
				"date_to":    dateTo,
				"course_id":  courseID,
				"unit_id":    unitID,
				"department": department,
			},
			"ghost_sessions": ghostSessions,
		})
	}
}

// GET /api/v1/dashboard/vc/lecturer-workload — Role: VC
func VCLecturerWorkload(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		q := r.URL.Query()
		dateFrom := q.Get("date_from")
		dateTo := q.Get("date_to")
		if dateFrom == "" {
			dateFrom = time.Now().AddDate(0, -1, 0).Format("2006-01-02")
		}
		if dateTo == "" {
			dateTo = time.Now().Format("2006-01-02")
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT
			  s.coordinator_id,
			  COALESCE(u.full_name, s.coordinator_id) AS coordinator_name,
			  COUNT(DISTINCT s.session_id) AS scheduled_sessions,
			  COUNT(DISTINCT CASE WHEN s.session_status IN ('ACTIVE','CLOSED','AUTO_CLOSED')
			    THEN s.session_id END) AS actual_sessions,
			  ROUND(COALESCE(SUM(
			    CASE WHEN s.gate_close_time IS NOT NULL AND s.gate_open_time IS NOT NULL
			    THEN EXTRACT(EPOCH FROM (s.gate_close_time - s.gate_open_time)) / 3600
			    END
			  ), 0), 2) AS total_contact_hours_actual,
			  COUNT(DISTINCT cu.unit_id) AS distinct_units
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = $1
			WHERE s.tenant_id = $1 AND s.session_date BETWEEN $2 AND $3
			GROUP BY s.coordinator_id, u.full_name
			ORDER BY scheduled_sessions DESC`, tenantID, dateFrom, dateTo)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type record struct {
			CoordinatorID     string  `json:"coordinator_id"`
			CoordinatorName   string  `json:"coordinator_name"`
			ScheduledSessions int     `json:"scheduled_sessions"`
			ActualSessions    int     `json:"actual_sessions"`
			TotalContactHours float64 `json:"total_contact_hours_actual"`
			DistinctUnits     int     `json:"distinct_units"`
			AttendanceRate    float64 `json:"attendance_rate_pct"`
		}

		var records []record
		for rows.Next() {
			var rec record
			rows.Scan(&rec.CoordinatorID, &rec.CoordinatorName, &rec.ScheduledSessions,
				&rec.ActualSessions, &rec.TotalContactHours, &rec.DistinctUnits) //nolint:errcheck
			if rec.ScheduledSessions > 0 {
				rec.AttendanceRate = float64(rec.ActualSessions) / float64(rec.ScheduledSessions) * 100
			}
			records = append(records, rec)
		}
		if records == nil {
			records = []record{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"date_from": dateFrom,
			"date_to":   dateTo,
			"workload":  records,
		})
	}
}

// GET /api/v1/dashboard/qa/live-sessions — Role: QA_OFFICER
func QALiveSessions(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT s.session_id, s.coordinator_id,
			       COALESCE(u.full_name, s.coordinator_id) AS coordinator_name,
			       s.unit_id, cu.name,
			       COALESCE(s.venue_id,''), s.session_status::text,
			       COUNT(al.log_id) AS student_count,
			       COALESCE(s.gate_open_time::text,''),
			       s.audit_flags,
			       enrolled.enrolled_count
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = $1
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			LEFT JOIN (
			  SELECT cu2.course_id,
			    COUNT(se.student_id) AS enrolled_count
			  FROM course_units cu2
			  JOIN students_extended se ON se.course_id = cu2.course_id AND se.tenant_id = $1
			  WHERE se.enrollment_status = 'ACTIVE'
			  GROUP BY cu2.course_id
			) enrolled ON enrolled.course_id = cu.course_id
			WHERE s.session_status = 'ACTIVE' AND s.tenant_id = $1
			GROUP BY s.session_id, cu.name, u.full_name, enrolled.enrolled_count`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type session struct {
			SessionID       string   `json:"session_id"`
			CoordinatorID   string   `json:"coordinator_id"`
			CoordinatorName string   `json:"coordinator_name"`
			UnitID          string   `json:"unit_id"`
			UnitName        string   `json:"unit_name"`
			VenueID         string   `json:"venue_id"`
			SessionStatus   string   `json:"session_status"`
			StudentCount    int      `json:"student_count"`
			EnrolledCount   int      `json:"enrolled_count"`
			GateOpenTime    string   `json:"gate_open_time"`
			AuditFlags      []string `json:"audit_flags"`
		}

		var sessions []session
		for rows.Next() {
			var s session
			rows.Scan(&s.SessionID, &s.CoordinatorID, &s.CoordinatorName,
				&s.UnitID, &s.UnitName, &s.VenueID, &s.SessionStatus,
				&s.StudentCount, &s.GateOpenTime, &s.AuditFlags, &s.EnrolledCount) //nolint:errcheck
			if s.AuditFlags == nil {
				s.AuditFlags = []string{}
			}
			sessions = append(sessions, s)
		}
		if sessions == nil {
			sessions = []session{}
		}
		writeJSON(w, http.StatusOK, sessions)
	}
}

// GET /api/v1/dashboard/qa/coordinator-health — Role: QA_OFFICER
func QACoordinatorHealth(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT
			  s.coordinator_id,
			  COALESCE(u.full_name, s.coordinator_id) AS coordinator_name,
			  COUNT(DISTINCT s.session_id) AS total_sessions,
			  COUNT(DISTINCT s.session_id) FILTER (WHERE s.sync_status = 'SYNCED')   AS synced_sessions,
			  COUNT(DISTINCT s.session_id) FILTER (WHERE s.sync_status = 'FAILED')   AS failed_sessions,
			  COUNT(DISTINCT s.session_id) FILTER (WHERE s.sync_status = 'PENDING')  AS pending_sessions,
			  ROUND(100.0 * COUNT(DISTINCT s.session_id) FILTER (WHERE s.sync_status = 'SYNCED')
			    / NULLIF(COUNT(DISTINCT s.session_id), 0), 1) AS sync_success_rate,
			  COUNT(al.log_id) FILTER (WHERE al.entry_method = 'MANUAL_OVERRIDE') AS manual_overrides,
			  COUNT(al.log_id) AS total_checkins,
			  ROUND(100.0 * COUNT(al.log_id) FILTER (WHERE al.entry_method = 'MANUAL_OVERRIDE')
			    / NULLIF(COUNT(al.log_id), 0), 1) AS manual_override_ratio
			FROM sessions s
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = $1
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			WHERE s.tenant_id = $1
			  AND s.session_date >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY s.coordinator_id, u.full_name
			ORDER BY total_sessions DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type record struct {
			CoordinatorID       string  `json:"coordinator_id"`
			CoordinatorName     string  `json:"coordinator_name"`
			TotalSessions       int     `json:"total_sessions"`
			SyncedSessions      int     `json:"synced_sessions"`
			FailedSessions      int     `json:"failed_sessions"`
			PendingSessions     int     `json:"pending_sessions"`
			SyncSuccessRate     float64 `json:"sync_success_rate"`
			ManualOverrides     int     `json:"manual_overrides"`
			TotalCheckins       int     `json:"total_checkins"`
			ManualOverrideRatio float64 `json:"manual_override_ratio"`
		}

		var records []record
		for rows.Next() {
			var rec record
			rows.Scan(&rec.CoordinatorID, &rec.CoordinatorName,
				&rec.TotalSessions, &rec.SyncedSessions, &rec.FailedSessions, &rec.PendingSessions,
				&rec.SyncSuccessRate, &rec.ManualOverrides, &rec.TotalCheckins, &rec.ManualOverrideRatio) //nolint:errcheck
			records = append(records, rec)
		}
		if records == nil {
			records = []record{}
		}
		writeJSON(w, http.StatusOK, records)
	}
}

// POST /api/v1/dashboard/qa/attendance-correction — Role: QA_OFFICER
// Inserts a MANUAL_OVERRIDE attendance entry with officer audit trail.
func QAManualCorrection(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		officerID := middleware.GetUserID(r.Context())

		var req struct {
			SessionID string `json:"session_id"`
			StudentID string `json:"student_id"`
			Reason    string `json:"reason"`
		}
		if err := decodeJSON(r, &req); err != nil || req.SessionID == "" || req.StudentID == "" || req.Reason == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "session_id, student_id and reason are required"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// Prevent duplicate manual corrections for same student+session.
		var existingCount int
		conn.QueryRow(r.Context(),
			`SELECT COUNT(*) FROM attendance_logs WHERE session_id = $1 AND student_id = $2`,
			req.SessionID, req.StudentID).Scan(&existingCount) //nolint:errcheck
		if existingCount > 0 {
			writeJSON(w, http.StatusConflict, errBody("ALREADY_PRESENT",
				"student already has an attendance record for this session"))
			return
		}

		// Get next sequence number.
		var nextSeq int
		conn.QueryRow(r.Context(),
			`SELECT COALESCE(MAX(sequence_number), 0) + 1 FROM attendance_logs WHERE session_id = $1`,
			req.SessionID).Scan(&nextSeq) //nolint:errcheck

		var logID string
		err = conn.QueryRow(r.Context(), `
			INSERT INTO attendance_logs
			  (tenant_id, session_id, student_id, checkin_timestamp, sequence_number,
			   entry_method, override_officer_id, override_reason)
			VALUES ($1, $2, $3, now(), $4, 'MANUAL_OVERRIDE', $5, $6)
			RETURNING log_id::text`,
			tenantID, req.SessionID, req.StudentID, nextSeq, officerID, req.Reason).Scan(&logID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "insert failed: "+err.Error()))
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status":     "RECORDED",
			"log_id":     logID,
			"student_id": req.StudentID,
			"session_id": req.SessionID,
			"officer_id": officerID,
		})
	}
}

// GET /api/v1/sessions/{session_id}/roster — Role: COORDINATOR
// Returns the list of students enrolled in the session's course unit
// together with their real-time check-in status for this session.
func SessionRoster(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		sessionID := chi.URLParam(r, "session_id")
		if sessionID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "session_id required"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT
			  se.student_id,
			  se.full_name,
			  CASE WHEN al.log_id IS NOT NULL THEN 'PRESENT' ELSE 'ABSENT' END AS status,
			  COALESCE(al.checkin_timestamp::text, '') AS checkin_time,
			  COALESCE(al.entry_method::text, '') AS entry_method
			FROM students_extended se
			LEFT JOIN attendance_logs al
			  ON al.student_id = se.student_id AND al.session_id = $1
			WHERE se.course_id = (
			    SELECT cu.course_id FROM course_units cu
			    JOIN sessions s ON s.unit_id = cu.unit_id
			    WHERE s.session_id = $1 AND s.tenant_id = $2
			)
			  AND se.tenant_id = $2
			  AND se.enrollment_status = 'ACTIVE'
			ORDER BY
			  CASE WHEN al.log_id IS NOT NULL THEN 0 ELSE 1 END,
			  se.full_name`, sessionID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type student struct {
			StudentID   string `json:"student_id"`
			FullName    string `json:"full_name"`
			Status      string `json:"status"`
			CheckinTime string `json:"checkin_time,omitempty"`
			EntryMethod string `json:"entry_method,omitempty"`
		}

		var roster []student
		for rows.Next() {
			var s student
			rows.Scan(&s.StudentID, &s.FullName, &s.Status, &s.CheckinTime, &s.EntryMethod) //nolint:errcheck
			roster = append(roster, s)
		}
		if roster == nil {
			roster = []student{}
		}

		present := 0
		for _, s := range roster {
			if s.Status == "PRESENT" {
				present++
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id":    sessionID,
			"total":         len(roster),
			"present_count": present,
			"absent_count":  len(roster) - present,
			"roster":        roster,
		})
	}
}

// GET /api/v1/eligibility/:student_id — Roles: QA_OFFICER, DQA_DIRECTOR, VC, STUDENT
// Accepts either the student registration number (e.g. NUT/CS/2024/001) OR the
// user UUID (returned by login). The UUID path is used by the student portal.
func GetEligibility(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		path := r.URL.Path
		rawID := ""
		const prefix = "/api/v1/eligibility/"
		if len(path) > len(prefix) {
			rawID = path[len(prefix):]
		}
		if rawID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "student_id required"))
			return
		}

		// IDOR guard: a STUDENT may only read their OWN eligibility. We ignore the
		// path id and force the lookup to the caller's JWT identity, so a student
		// cannot enumerate classmates by passing another student_id/user UUID. Staff
		// roles (QA/DQA/VC) legitimately read any student in their tenant.
		if middleware.GetRole(r.Context()) == middleware.RoleStudent {
			rawID = middleware.GetUserID(r.Context())
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// Resolve the student_id (reg number): try direct match first, then resolve
		// from user UUID via email join (used by the student portal after login).
		studentID := rawID
		var tryEmail string
		err2 := conn.QueryRow(r.Context(),
			`SELECT se.student_id FROM students_extended se WHERE se.student_id = $1 AND se.tenant_id = $2 LIMIT 1`,
			rawID, tenantID).Scan(&studentID)
		if err2 != nil {
			// rawID is not a registration number — try resolving as a user UUID.
			conn.QueryRow(r.Context(),
				`SELECT email FROM users WHERE user_id::text = $1 AND tenant_id = $2`,
				rawID, tenantID).Scan(&tryEmail) //nolint:errcheck
			if tryEmail != "" {
				conn.QueryRow(r.Context(),
					`SELECT student_id FROM students_extended WHERE email = $1 AND tenant_id = $2`,
					tryEmail, tenantID).Scan(&studentID) //nolint:errcheck
			}
		}

		var academicYear string
		var semester int
		var fullName string
		conn.QueryRow(r.Context(),
			`SELECT academic_year, COALESCE(semester,1), full_name FROM students_extended WHERE student_id = $1 AND tenant_id = $2`,
			studentID, tenantID).Scan(&academicYear, &semester, &fullName) //nolint:errcheck

		rows, err := conn.Query(r.Context(), `
			SELECT s.unit_id, s.unit_name,
			       s.sessions_held, s.sessions_attended, s.attendance_percentage,
			       t.attendance_threshold
			FROM student_attendance_summary s
			JOIN tenants t ON t.tenant_id = s.tenant_id
			WHERE s.student_id = $1 AND s.tenant_id = $2`, studentID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type unitResult struct {
			UnitID           string  `json:"unit_id"`
			UnitName         string  `json:"unit_name"`
			SessionsHeld     int     `json:"sessions_held"`
			SessionsAttended int     `json:"sessions_attended"`
			AttendancePct    float64 `json:"attendance_percentage"`
			Threshold        int     `json:"threshold"`
			Status           string  `json:"status"`
			DeficitSessions  *int    `json:"deficit_sessions,omitempty"`
		}

		var units []unitResult
		for rows.Next() {
			var u unitResult
			rows.Scan(&u.UnitID, &u.UnitName, &u.SessionsHeld, &u.SessionsAttended,
				&u.AttendancePct, &u.Threshold) //nolint:errcheck
			if u.AttendancePct >= float64(u.Threshold) {
				u.Status = "ELIGIBLE"
			} else {
				u.Status = "EXAM_INELIGIBLE"
				needed := int(float64(u.Threshold)/100*float64(u.SessionsHeld)+0.999) - u.SessionsAttended
				if needed < 0 {
					needed = 0
				}
				u.DeficitSessions = &needed
			}
			units = append(units, u)
		}
		if units == nil {
			units = []unitResult{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"student_id":    studentID,
			"full_name":     fullName,
			"academic_year": academicYear,
			"semester":      semester,
			"units":         units,
		})
	}
}

// GET /api/v1/student/progress?reg=<reg-no>&org=<tenant-domain> — PUBLIC, no auth.
// The single passwordless student portal: a student types their registration
// number and sees their own attendance/eligibility. The portal link carries the
// tenant (its domain), so a reg-no only ever resolves WITHIN that institution — a
// student of tenant A can never view a student of tenant B. reg-no stays the only
// credential entered. Note: still unauthenticated by design.
func StudentProgressByReg(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reg := strings.TrimSpace(r.URL.Query().Get("reg"))
		org := strings.TrimSpace(r.URL.Query().Get("org")) // tenant domain (or institution id) from the link
		if reg == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "reg (registration number) is required"))
			return
		}
		if org == "" {
			writeJSON(w, http.StatusBadRequest, errBody("ORG_REQUIRED", "open your institution's portal link to view your attendance"))
			return
		}
		// Resolve the institution from the link; the reg-no is then looked up ONLY
		// inside this tenant (cross-tenant isolation).
		var scopeTenant string
		if e := adminPool.QueryRow(r.Context(),
			`SELECT tenant_id::text FROM tenants WHERE lower(domain) = lower($1) OR lower(institution_id) = lower($1) LIMIT 1`,
			org).Scan(&scopeTenant); e != nil {
			writeJSON(w, http.StatusNotFound, errBody("UNKNOWN_INSTITUTION", "unknown institution link"))
			return
		}

		var tenantID, fullName, academicYear, institution string
		var semester int
		err := adminPool.QueryRow(r.Context(), `
			SELECT se.tenant_id::text, se.full_name, COALESCE(se.academic_year,''),
			       COALESCE(se.semester,1), COALESCE(t.name,'')
			FROM students_extended se JOIN tenants t ON t.tenant_id = se.tenant_id
			WHERE se.student_id = $1 AND se.tenant_id = $2 LIMIT 1`,
			reg, scopeTenant).Scan(&tenantID, &fullName, &academicYear, &semester, &institution)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no student with that registration number at this institution"))
			return
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT s.unit_id, s.unit_name, s.sessions_held, s.sessions_attended,
			       s.attendance_percentage, t.attendance_threshold
			FROM student_attendance_summary s
			JOIN tenants t ON t.tenant_id = s.tenant_id
			WHERE s.student_id = $1 AND s.tenant_id = $2`, reg, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type unitResult struct {
			UnitID           string  `json:"unit_id"`
			UnitName         string  `json:"unit_name"`
			SessionsHeld     int     `json:"sessions_held"`
			SessionsAttended int     `json:"sessions_attended"`
			AttendancePct    float64 `json:"attendance_percentage"`
			Threshold        int     `json:"threshold"`
			Status           string  `json:"status"`
			DeficitSessions  *int    `json:"deficit_sessions,omitempty"`
		}
		var units []unitResult
		for rows.Next() {
			var u unitResult
			rows.Scan(&u.UnitID, &u.UnitName, &u.SessionsHeld, &u.SessionsAttended, &u.AttendancePct, &u.Threshold) //nolint:errcheck
			if u.AttendancePct >= float64(u.Threshold) {
				u.Status = "ELIGIBLE"
			} else {
				u.Status = "EXAM_INELIGIBLE"
				needed := int(float64(u.Threshold)/100*float64(u.SessionsHeld)+0.999) - u.SessionsAttended
				if needed < 0 {
					needed = 0
				}
				u.DeficitSessions = &needed
			}
			units = append(units, u)
		}
		if units == nil {
			units = []unitResult{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"student_id":    reg,
			"full_name":     fullName,
			"institution":   institution,
			"academic_year": academicYear,
			"semester":      semester,
			"units":         units,
		})
	}
}

// POST /api/v1/dashboard/qa/device-reset — Role: QA_OFFICER
func QADeviceReset(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		officerID := middleware.GetUserID(r.Context())

		var req struct {
			StudentID  string `json:"student_id"`
			ReasonCode string `json:"reason_code"`
			ReasonText string `json:"reason_text"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		// Admin override: a FULL reset (rebind_count -> 0). This both clears the hardware/QR-card
		// binding AND unblocks the student app's self-rebind limit — so a student who used up their
		// 2 self-rebinds (REBIND_LIMIT) can register a fresh phone. The stale student_device_bindings
		// row is harmlessly overwritten on their next register-device call (which will itself apply
		// the standard 12h cooldown as a rebind).
		tag, err := conn.Exec(r.Context(), `
			UPDATE students_extended
			SET hardware_fingerprint = NULL,
			    rebind_count         = 0,
			    last_rebind_date     = now(),
			    updated_at           = now()
			WHERE student_id = $1 AND tenant_id = $2`, req.StudentID, tenantID)
		if err != nil || tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such student in this institution"))
			return
		}

		conn.Exec(r.Context(), `DELETE FROM hardware_vault WHERE student_id = $1`, req.StudentID) //nolint:errcheck

		writeJSON(w, http.StatusOK, map[string]string{
			"status":     "RESET",
			"student_id": req.StudentID,
			"reset_by":   officerID,
		})
	}
}

func errBody(code, msg string) map[string]string {
	return map[string]string{"error": code, "message": msg}
}
