package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/dashboard/vc/overview — Role: VC
func VCOverview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		var totalSessions, totalStudents, ghostCount int
		var avgPct float64

		conn.QueryRow(r.Context(), `
			SELECT
			  COUNT(DISTINCT s.session_id),
			  COUNT(DISTINCT al.student_id),
			  COUNT(DISTINCT CASE WHEN 'GHOST_LECTURE_SUSPECTED' = ANY(s.audit_flags) THEN s.session_id END),
			  COALESCE(ROUND(AVG(unit_pct),2), 0)
			FROM sessions s
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			LEFT JOIN (
			  SELECT session_id,
			    CASE WHEN COUNT(student_id) = 0 THEN 0
			    ELSE 100 END AS unit_pct
			  FROM sessions
			  LEFT JOIN attendance_logs USING(session_id)
			  WHERE session_date = CURRENT_DATE AND tenant_id = $1
			  GROUP BY session_id
			) pcts ON pcts.session_id = s.session_id
			WHERE s.session_date = CURRENT_DATE AND s.tenant_id = $1`,
			tenantID).Scan(&totalSessions, &totalStudents, &ghostCount, &avgPct) //nolint:errcheck

		// Eligibility summary from materialized view.
		var eligible, ineligible int
		conn.QueryRow(r.Context(), `
			SELECT
			  COUNT(*) FILTER (WHERE attendance_percentage >= (SELECT attendance_threshold FROM tenants WHERE tenant_id = $1)),
			  COUNT(*) FILTER (WHERE attendance_percentage <  (SELECT attendance_threshold FROM tenants WHERE tenant_id = $1))
			FROM student_attendance_summary WHERE tenant_id = $1`, tenantID).
			Scan(&eligible, &ineligible) //nolint:errcheck

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"total_sessions_today":  totalSessions,
			"total_students_today":  totalStudents,
			"ghost_lecture_count":   ghostCount,
			"avg_attendance_pct":    avgPct,
			"eligibility_summary": map[string]int{
				"eligible":   eligible,
				"ineligible": ineligible,
				"pending":    0,
			},
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
			SELECT s.session_id, s.coordinator_id, s.unit_id, cu.name,
			       COALESCE(s.venue_id,''), s.session_status::text,
			       COUNT(al.log_id) AS student_count,
			       COALESCE(s.gate_open_time::text,''),
			       s.audit_flags
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			WHERE s.session_status = 'ACTIVE' AND s.tenant_id = $1
			GROUP BY s.session_id, cu.name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type session struct {
			SessionID     string   `json:"session_id"`
			CoordinatorID string   `json:"coordinator_id"`
			UnitID        string   `json:"unit_id"`
			UnitName      string   `json:"unit_name"`
			VenueID       string   `json:"venue_id"`
			SessionStatus string   `json:"session_status"`
			StudentCount  int      `json:"student_count"`
			GateOpenTime  string   `json:"gate_open_time"`
			AuditFlags    []string `json:"audit_flags"`
		}

		var sessions []session
		for rows.Next() {
			var s session
			rows.Scan(&s.SessionID, &s.CoordinatorID, &s.UnitID, &s.UnitName,
				&s.VenueID, &s.SessionStatus, &s.StudentCount, &s.GateOpenTime, &s.AuditFlags) //nolint:errcheck
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

// GET /api/v1/eligibility/:student_id — Roles: QA_OFFICER, DQA_DIRECTOR, VC
func GetEligibility(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		// Extract student_id from URL path — chi URLParam not available here;
		// parse from path directly.
		path := r.URL.Path
		studentID := ""
		const prefix = "/api/v1/eligibility/"
		if len(path) > len(prefix) {
			studentID = path[len(prefix):]
		}
		if studentID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "student_id required"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		var academicYear string
		var semester int
		conn.QueryRow(r.Context(),
			`SELECT academic_year, COALESCE(semester,1) FROM students_extended WHERE student_id = $1 AND tenant_id = $2`,
			studentID, tenantID).Scan(&academicYear, &semester) //nolint:errcheck

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
			UnitID               string  `json:"unit_id"`
			UnitName             string  `json:"unit_name"`
			SessionsHeld         int     `json:"sessions_held"`
			SessionsAttended     int     `json:"sessions_attended"`
			AttendancePct        float64 `json:"attendance_percentage"`
			Threshold            int     `json:"threshold"`
			Status               string  `json:"status"`
			DeficitSessions      *int    `json:"deficit_sessions,omitempty"`
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
				if needed < 0 { needed = 0 }
				u.DeficitSessions = &needed
			}
			units = append(units, u)
		}
		if units == nil {
			units = []unitResult{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"student_id":    studentID,
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

		var req struct {
			StudentID  string `json:"student_id"`
			OfficerID  string `json:"officer_id"`
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

		// Clear hardware fingerprint and increment rebind count.
		tag, err := conn.Exec(r.Context(), `
			UPDATE students_extended
			SET hardware_fingerprint = NULL,
			    rebind_count         = rebind_count + 1,
			    last_rebind_date     = now(),
			    updated_at           = now()
			WHERE student_id = $1 AND tenant_id = $2
			  AND rebind_count < 2`, req.StudentID, tenantID)
		if err != nil || tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusConflict, errBody("REBIND_LIMIT_REACHED",
				"student has reached the maximum of 2 device resets per academic year"))
			return
		}

		// Also clear hardware_vault entry.
		conn.Exec(r.Context(), `DELETE FROM hardware_vault WHERE student_id = $1`, req.StudentID) //nolint:errcheck

		writeJSON(w, http.StatusOK, map[string]string{
			"status":     "RESET",
			"student_id": req.StudentID,
			"reset_by":   req.OfficerID,
		})
	}
}

func errBody(code, msg string) map[string]string {
	return map[string]string{"error": code, "message": msg}
}
