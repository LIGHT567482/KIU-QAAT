package handlers

// StudentLiveSessions handles GET /api/v1/student/live-sessions (STUDENT role).
//
// It lists the ACTIVE sessions a student may attend right now — every live
// session whose unit belongs to the course the student is enrolled in. The
// student-portal polls this so learners "see the sessions live and active"
// and can jump straight into check-in. Each row reports the planned schedule
// (#2) and whether this student has already checked in.

import (
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

func StudentLiveSessions(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

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

		// Resolve the student, their course (program) and their offering (session).
		var studentID, courseID, offeringID, enrollment string
		err = conn.QueryRow(r.Context(), `
			SELECT se.student_id, COALESCE(se.course_id,''), COALESCE(se.offering_id::text,''), se.enrollment_status::text
			FROM users u
			JOIN students_extended se ON se.email = u.email AND se.tenant_id = u.tenant_id
			WHERE u.user_id = $1 AND u.tenant_id = $2`,
			userID, tenantID).Scan(&studentID, &courseID, &offeringID, &enrollment)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusForbidden, errBody("NOT_ON_ROSTER", "your account is not linked to a student record"))
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "lookup failed"))
			return
		}

		rows, err := conn.Query(r.Context(), `
			SELECT s.session_id, s.unit_id, cu.name,
			       COALESCE(v.name, '') AS venue_name,
			       COALESCE(u.full_name, '') AS coordinator_name,
			       COALESCE(to_char(s.planned_start, 'HH24:MI'), '') AS planned_start,
			       COALESCE(s.planned_duration_minutes, 0) AS planned_duration,
			       COALESCE(s.gate_open_time::text, '') AS gate_open_time,
			       EXISTS (
			         SELECT 1 FROM attendance_logs al
			         WHERE al.session_id = s.session_id AND al.student_id = $3
			       ) AS already_checked_in
			FROM sessions s
			JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN venues v ON v.venue_id = s.venue_id AND v.tenant_id = $1
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = $1
			WHERE s.tenant_id = $1
			  AND s.session_status = 'ACTIVE'
			  -- Scope to the student's own session cohort: prefer offering, else fall
			  -- back to the program (legacy students without an offering).
			  AND ( ($4 <> '' AND s.offering_id::text = $4)
			        OR ($4 = '' AND cu.course_id = $2) )
			ORDER BY s.gate_open_time DESC NULLS LAST`,
			tenantID, courseID, studentID, offeringID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type liveSession struct {
			SessionID        string `json:"session_id"`
			UnitID           string `json:"unit_id"`
			UnitName         string `json:"unit_name"`
			VenueName        string `json:"venue_name"`
			CoordinatorName  string `json:"coordinator_name"`
			PlannedStart     string `json:"planned_start"`
			PlannedDuration  int    `json:"planned_duration_minutes"`
			GateOpenTime     string `json:"gate_open_time"`
			AlreadyCheckedIn bool   `json:"already_checked_in"`
		}

		out := []liveSession{}
		for rows.Next() {
			var s liveSession
			rows.Scan(&s.SessionID, &s.UnitID, &s.UnitName, &s.VenueName, //nolint:errcheck
				&s.CoordinatorName, &s.PlannedStart, &s.PlannedDuration,
				&s.GateOpenTime, &s.AlreadyCheckedIn)
			out = append(out, s)
		}
		writeJSON(w, http.StatusOK, out)
	}
}
