package handlers

// Lecturer-attendance views for the oversight dashboards (QA Officer, VC, DQA
// Director) — same data as the admin's lecturer-attendance pages, but scoped to
// the CALLER's tenant (from the JWT) over the RLS pool, so no {tenant_id} path.

import (
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /api/v1/dashboard/lecturer-attendance — detailed session logs.
func LecturerAttendanceLogsForCaller(pool *pgxpool.Pool) http.HandlerFunc {
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
			SELECT lal.log_id::text, lal.lecturer_id,
			       COALESCE(l.full_name, lal.lecturer_id), COALESCE(l.department,''),
			       lal.unit_id, COALESCE(cu.name, lal.unit_id), lal.session_date,
			       lal.gate_open_time, lal.gate_close_time, COALESCE(lal.contact_hours,0),
			       COALESCE(s.session_status::text,'UNKNOWN')
			FROM lecturer_attendance_logs lal
			LEFT JOIN lecturers l ON l.lecturer_id::text = lal.lecturer_id AND l.tenant_id = lal.tenant_id
			LEFT JOIN course_units cu ON cu.unit_id = lal.unit_id
			LEFT JOIN sessions s ON s.session_id = lal.session_id
			WHERE lal.tenant_id = $1
			ORDER BY lal.session_date DESC, lal.gate_open_time DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type logRow struct {
			LogID         string  `json:"log_id"`
			LecturerID    string  `json:"lecturer_id"`
			LecturerName  string  `json:"lecturer_name"`
			Department    string  `json:"department"`
			UnitID        string  `json:"unit_id"`
			UnitName      string  `json:"unit_name"`
			SessionDate   string  `json:"session_date"`
			GateOpenTime  string  `json:"gate_open_time"`
			GateCloseTime string  `json:"gate_close_time"`
			ContactHours  float64 `json:"contact_hours"`
			SessionStatus string  `json:"session_status"`
		}
		out := []logRow{}
		for rows.Next() {
			var l logRow
			var sd time.Time
			var open, close *time.Time
			rows.Scan(&l.LogID, &l.LecturerID, &l.LecturerName, &l.Department, &l.UnitID, &l.UnitName, //nolint:errcheck
				&sd, &open, &close, &l.ContactHours, &l.SessionStatus)
			l.SessionDate = sd.Format("2006-01-02")
			if open != nil {
				l.GateOpenTime = open.Format(time.RFC3339)
			}
			if close != nil {
				l.GateCloseTime = close.Format(time.RFC3339)
			}
			out = append(out, l)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// GET /api/v1/dashboard/lecturer-attendance/summary — per-lecturer aggregates.
func LecturerAttendanceSummaryForCaller(pool *pgxpool.Pool) http.HandlerFunc {
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
			SELECT lal.lecturer_id, COALESCE(l.full_name, lal.lecturer_id), COALESCE(l.department,''),
			       COALESCE(l.email,''), COUNT(*), COALESCE(SUM(lal.contact_hours),0),
			       COALESCE(AVG(lal.contact_hours),0), MAX(lal.session_date)
			FROM lecturer_attendance_logs lal
			LEFT JOIN lecturers l ON l.lecturer_id::text = lal.lecturer_id AND l.tenant_id = lal.tenant_id
			WHERE lal.tenant_id = $1
			GROUP BY lal.lecturer_id, l.full_name, l.department, l.email
			ORDER BY COUNT(*) DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type summaryRow struct {
			LecturerID        string  `json:"lecturer_id"`
			LecturerName      string  `json:"lecturer_name"`
			Department        string  `json:"department"`
			Email             string  `json:"email"`
			TotalSessions     int     `json:"total_sessions"`
			TotalContactHours float64 `json:"total_contact_hours"`
			AvgContactHours   float64 `json:"avg_contact_hours"`
			LastSessionDate   string  `json:"last_session_date"`
		}
		out := []summaryRow{}
		for rows.Next() {
			var sr summaryRow
			var last *time.Time
			rows.Scan(&sr.LecturerID, &sr.LecturerName, &sr.Department, &sr.Email, //nolint:errcheck
				&sr.TotalSessions, &sr.TotalContactHours, &sr.AvgContactHours, &last)
			if last != nil {
				sr.LastSessionDate = last.Format("2006-01-02")
			}
			out = append(out, sr)
		}
		writeJSON(w, http.StatusOK, out)
	}
}
