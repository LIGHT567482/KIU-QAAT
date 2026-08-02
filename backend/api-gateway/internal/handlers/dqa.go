package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

type thresholdConfig struct {
	AttendanceThreshold  int `json:"attendance_threshold"`
	CheckinWindowMinutes int `json:"checkin_window_minutes"`
	AutoKillMinutes      int `json:"auto_kill_minutes"`
}

// GET /api/v1/dashboard/dqa/thresholds
func GetThresholds(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var cfg thresholdConfig
		err := pool.QueryRow(r.Context(), `
			SELECT attendance_threshold, checkin_window_minutes,
			       auto_kill_minutes
			FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&cfg.AttendanceThreshold, &cfg.CheckinWindowMinutes,
				&cfg.AutoKillMinutes)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "TENANT_NOT_FOUND", "message": "tenant not found",
			})
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	}
}

// PUT /api/v1/dashboard/dqa/thresholds
func PutThresholds(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var cfg thresholdConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "INVALID_REQUEST", "message": "malformed body",
			})
			return
		}

		// Policy: the attendance threshold is FIXED at a 75% minimum — it can never go
		// below it. A lower value is clamped up to 75; cap at 100.
		if cfg.AttendanceThreshold < 75 {
			cfg.AttendanceThreshold = 75
		}
		if cfg.AttendanceThreshold > 100 {
			cfg.AttendanceThreshold = 100
		}

		_, err := pool.Exec(r.Context(), `
			UPDATE tenants
			SET attendance_threshold   = $1,
			    checkin_window_minutes = $2,
			    auto_kill_minutes      = $3
			WHERE tenant_id = $4`,
			cfg.AttendanceThreshold, cfg.CheckinWindowMinutes,
			cfg.AutoKillMinutes, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error": "INTERNAL_ERROR", "message": "update failed",
			})
			return
		}

		// Bust the Daily Manifest cache — next fetch will include new policy.
		pattern := fmt.Sprintf("manifest:%s:*", tenantID)
		keys, _ := rdb.Keys(r.Context(), pattern).Result()
		if len(keys) > 0 {
			rdb.Del(r.Context(), keys...) //nolint:errcheck
		}

		writeJSON(w, http.StatusOK, cfg)
	}
}

// GET /api/v1/dashboard/dqa/course-health — Role: DQA_DIRECTOR
// Returns per-course-unit aggregate attendance health for heatmap display.
func DQACourseHealth(pool *pgxpool.Pool) http.HandlerFunc {
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
			  cu.unit_id,
			  cu.name AS unit_name,
			  c.course_id,
			  c.name AS course_name,
			  COALESCE(c.department, '') AS department,
			  COUNT(DISTINCT sas.student_id) AS enrolled_count,
			  ROUND(COALESCE(AVG(sas.attendance_percentage), 0), 1) AS avg_attendance_pct,
			  COUNT(*) FILTER (WHERE sas.attendance_percentage < t.attendance_threshold) AS below_threshold_count,
			  t.attendance_threshold
			FROM course_units cu
			JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = $1
			LEFT JOIN student_attendance_summary sas
			  ON sas.unit_id = cu.unit_id AND sas.tenant_id = $1
			JOIN tenants t ON t.tenant_id = $1
			WHERE cu.tenant_id = $1
			GROUP BY cu.unit_id, cu.name, c.course_id, c.name, c.department, t.attendance_threshold
			ORDER BY avg_attendance_pct ASC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type unitHealth struct {
			UnitID              string  `json:"unit_id"`
			UnitName            string  `json:"unit_name"`
			CourseID            string  `json:"course_id"`
			CourseName          string  `json:"course_name"`
			Department          string  `json:"department"`
			EnrolledCount       int     `json:"enrolled_count"`
			AvgAttendancePct    float64 `json:"avg_attendance_pct"`
			BelowThresholdCount int     `json:"below_threshold_count"`
			Threshold           int     `json:"threshold"`
			RiskLevel           string  `json:"risk_level"`
		}

		var units []unitHealth
		for rows.Next() {
			var u unitHealth
			rows.Scan(&u.UnitID, &u.UnitName, &u.CourseID, &u.CourseName, &u.Department,
				&u.EnrolledCount, &u.AvgAttendancePct, &u.BelowThresholdCount, &u.Threshold) //nolint:errcheck
			switch {
			case u.AvgAttendancePct >= float64(u.Threshold):
				u.RiskLevel = "HEALTHY"
			case u.AvgAttendancePct >= float64(u.Threshold)*0.85:
				u.RiskLevel = "AT_RISK"
			default:
				u.RiskLevel = "CRITICAL"
			}
			units = append(units, u)
		}
		if units == nil {
			units = []unitHealth{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"units": units,
			"total": len(units),
		})
	}
}

// GET /api/v1/dashboard/dqa/trends — Role: DQA_DIRECTOR
// Returns week-over-week attendance trend for the last 12 weeks.
func DQATrends(pool *pgxpool.Pool) http.HandlerFunc {
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
			  date_trunc('week', s.session_date::timestamptz)::date AS week_start,
			  COUNT(DISTINCT s.session_id) AS sessions_held,
			  COUNT(DISTINCT al.student_id) AS unique_students,
			  COUNT(al.log_id) AS total_checkins,
			  ROUND(COALESCE(
			    100.0 * COUNT(DISTINCT al.student_id)
			    / NULLIF(COUNT(DISTINCT s.session_id), 0)
			  , 0), 1) AS avg_students_per_session
			FROM sessions s
			LEFT JOIN attendance_logs al ON al.session_id = s.session_id
			WHERE s.tenant_id = $1
			  AND s.session_date >= CURRENT_DATE - INTERVAL '12 weeks'
			  AND s.session_status IN ('ACTIVE', 'CLOSED', 'AUTO_CLOSED')
			GROUP BY week_start
			ORDER BY week_start`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type weekPoint struct {
			WeekStart             string  `json:"week_start"`
			SessionsHeld          int     `json:"sessions_held"`
			UniqueStudents        int     `json:"unique_students"`
			TotalCheckins         int     `json:"total_checkins"`
			AvgStudentsPerSession float64 `json:"avg_students_per_session"`
		}

		var trend []weekPoint
		for rows.Next() {
			var p weekPoint
			var weekTime time.Time
			rows.Scan(&weekTime, &p.SessionsHeld, &p.UniqueStudents,
				&p.TotalCheckins, &p.AvgStudentsPerSession) //nolint:errcheck
			p.WeekStart = weekTime.Format("2006-01-02")
			trend = append(trend, p)
		}
		if trend == nil {
			trend = []weekPoint{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"trend": trend,
		})
	}
}

// GET /api/v1/dashboard/dqa/punctuality — Role: DQA_DIRECTOR
// Shows lecturer gate-open punctuality (wait time between session setup and gate open).
func DQAPunctuality(pool *pgxpool.Pool) http.HandlerFunc {
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
			  COUNT(*) AS total_sessions,
			  COUNT(*) FILTER (WHERE s.gate_open_time IS NOT NULL) AS gate_opened_count,
			  COUNT(*) FILTER (WHERE s.gate_open_time IS NULL
			    AND s.session_status IN ('CLOSED','AUTO_CLOSED')) AS no_gate_open_count,
			  ROUND(COALESCE(AVG(
			    CASE WHEN s.gate_open_time IS NOT NULL
			    THEN EXTRACT(EPOCH FROM (s.gate_open_time - s.created_at)) / 60 END
			  ), 0), 1) AS avg_wait_minutes,
			  COUNT(*) FILTER (
			    WHERE s.gate_open_time IS NOT NULL
			    AND EXTRACT(EPOCH FROM (s.gate_open_time - s.created_at)) / 60 > 15
			  ) AS late_open_count
			FROM sessions s
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = $1
			WHERE s.tenant_id = $1
			  AND s.session_date >= CURRENT_DATE - INTERVAL '30 days'
			GROUP BY s.coordinator_id, u.full_name
			ORDER BY late_open_count DESC, avg_wait_minutes DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type record struct {
			CoordinatorID   string  `json:"coordinator_id"`
			CoordinatorName string  `json:"coordinator_name"`
			TotalSessions   int     `json:"total_sessions"`
			GateOpenedCount int     `json:"gate_opened_count"`
			NoGateOpenCount int     `json:"no_gate_open_count"`
			AvgWaitMinutes  float64 `json:"avg_wait_minutes"`
			LateOpenCount   int     `json:"late_open_count"`
		}

		var records []record
		for rows.Next() {
			var rec record
			rows.Scan(&rec.CoordinatorID, &rec.CoordinatorName, &rec.TotalSessions,
				&rec.GateOpenedCount, &rec.NoGateOpenCount, &rec.AvgWaitMinutes, &rec.LateOpenCount) //nolint:errcheck
			records = append(records, rec)
		}
		if records == nil {
			records = []record{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"period":  "last 30 days",
			"records": records,
		})
	}
}

// GET /api/v1/dashboard/dqa/ineligible — Role: DQA_DIRECTOR
// Returns all students currently below the attendance threshold with deficit details.
func DQABulkIneligible(pool *pgxpool.Pool) http.HandlerFunc {
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
			  sas.student_id,
			  se.full_name AS student_name,
			  se.email,
			  se.course_id,
			  c.name AS course_name,
			  sas.unit_id,
			  cu.name AS unit_name,
			  sas.sessions_held,
			  sas.sessions_attended,
			  sas.attendance_percentage,
			  t.attendance_threshold,
			  GREATEST(0, CEIL(t.attendance_threshold::numeric / 100.0 * sas.sessions_held) - sas.sessions_attended)::int AS deficit_sessions
			FROM student_attendance_summary sas
			JOIN students_extended se ON se.student_id = sas.student_id AND se.tenant_id = $1
			JOIN course_units cu ON cu.unit_id = sas.unit_id
			JOIN courses c ON c.course_id = se.course_id
			JOIN tenants t ON t.tenant_id = $1
			WHERE sas.tenant_id = $1
			  AND sas.attendance_percentage < t.attendance_threshold
			ORDER BY sas.student_id, sas.unit_id`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type record struct {
			StudentID        string  `json:"student_id"`
			StudentName      string  `json:"student_name"`
			Email            string  `json:"email"`
			CourseID         string  `json:"course_id"`
			CourseName       string  `json:"course_name"`
			UnitID           string  `json:"unit_id"`
			UnitName         string  `json:"unit_name"`
			SessionsHeld     int     `json:"sessions_held"`
			SessionsAttended int     `json:"sessions_attended"`
			AttendancePct    float64 `json:"attendance_percentage"`
			Threshold        int     `json:"threshold"`
			DeficitSessions  int     `json:"deficit_sessions"`
		}

		var records []record
		for rows.Next() {
			var rec record
			rows.Scan(&rec.StudentID, &rec.StudentName, &rec.Email,
				&rec.CourseID, &rec.CourseName, &rec.UnitID, &rec.UnitName,
				&rec.SessionsHeld, &rec.SessionsAttended, &rec.AttendancePct,
				&rec.Threshold, &rec.DeficitSessions) //nolint:errcheck
			records = append(records, rec)
		}
		if records == nil {
			records = []record{}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ineligible_count": len(records),
			"records":          records,
		})
	}
}

// DQAAllEligibility returns the eligibility record for EVERY student (eligible
// and ineligible alike), one row per student-unit, with the dimensions the DQA
// dashboard filters on (course, unit, year, semester, intake, academic year,
// status). Unlike DQABulkIneligible it does not filter by threshold, so the
// dashboard can show all students regardless of any search.
func DQAAllEligibility(pool *pgxpool.Pool) http.HandlerFunc {
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
			  sas.student_id,
			  se.full_name AS student_name,
			  se.email,
			  se.course_id,
			  c.name AS course_name,
			  sas.unit_id,
			  cu.name AS unit_name,
			  sas.sessions_held,
			  sas.sessions_attended,
			  sas.attendance_percentage,
			  t.attendance_threshold,
			  GREATEST(0, CEIL(t.attendance_threshold::numeric / 100.0 * sas.sessions_held) - sas.sessions_attended)::int AS deficit_sessions,
			  CASE WHEN sas.attendance_percentage >= t.attendance_threshold THEN 'ELIGIBLE' ELSE 'INELIGIBLE' END AS status,
			  COALESCE(se.current_year, 0),
			  COALESCE(se.semester, 0),
			  COALESCE(se.intake_session, ''),
			  COALESCE(se.academic_year, '')
			FROM student_attendance_summary sas
			JOIN students_extended se ON se.student_id = sas.student_id AND se.tenant_id = $1
			JOIN course_units cu ON cu.unit_id = sas.unit_id
			JOIN courses c ON c.course_id = se.course_id
			JOIN tenants t ON t.tenant_id = $1
			WHERE sas.tenant_id = $1
			ORDER BY se.full_name, sas.student_id, sas.unit_id`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type record struct {
			StudentID        string  `json:"student_id"`
			StudentName      string  `json:"student_name"`
			Email            string  `json:"email"`
			CourseID         string  `json:"course_id"`
			CourseName       string  `json:"course_name"`
			UnitID           string  `json:"unit_id"`
			UnitName         string  `json:"unit_name"`
			SessionsHeld     int     `json:"sessions_held"`
			SessionsAttended int     `json:"sessions_attended"`
			AttendancePct    float64 `json:"attendance_percentage"`
			Threshold        int     `json:"threshold"`
			DeficitSessions  int     `json:"deficit_sessions"`
			Status           string  `json:"status"`
			CurrentYear      int     `json:"current_year"`
			Semester         int     `json:"semester"`
			IntakeSession    string  `json:"intake_session"`
			AcademicYear     string  `json:"academic_year"`
		}

		records := []record{}
		var eligible, ineligible int
		for rows.Next() {
			var rec record
			rows.Scan(&rec.StudentID, &rec.StudentName, &rec.Email,
				&rec.CourseID, &rec.CourseName, &rec.UnitID, &rec.UnitName,
				&rec.SessionsHeld, &rec.SessionsAttended, &rec.AttendancePct,
				&rec.Threshold, &rec.DeficitSessions, &rec.Status,
				&rec.CurrentYear, &rec.Semester, &rec.IntakeSession, &rec.AcademicYear) //nolint:errcheck
			if rec.Status == "ELIGIBLE" {
				eligible++
			} else {
				ineligible++
			}
			records = append(records, rec)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"total_count":      len(records),
			"eligible_count":   eligible,
			"ineligible_count": ineligible,
			"records":          records,
		})
	}
}
