package handlers

// Lecturer roster & analytics (LECTURER role). Everyone who studies his unit(s) and their
// attendance across every session he has ever had — ACROSS cohorts. Every row carries the
// sortable dimensions (cohort, course, unit, level, year, semester, intake) so the app can
// sort/filter by any of them client-side; the session endpoints cover "attended / did NOT
// attend a specific session".
//
//   GET /api/v1/lecturer/roster?scope=enrolled|attended&unit_id=
//   GET /api/v1/lecturer/sessions?unit_id=
//   GET /api/v1/lecturer/sessions/{session_id}/students?status=present|absent|all

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

func cohortLabel(year, sem int, level, intake string) string {
	parts := []string{}
	if year > 0 {
		parts = append(parts, fmt.Sprintf("Yr%d", year))
	}
	if sem > 0 {
		parts = append(parts, fmt.Sprintf("Sem%d", sem))
	}
	if level != "" {
		parts = append(parts, level)
	}
	label := strings.Join(parts, " ")
	if intake != "" {
		if label != "" {
			label += " · "
		}
		label += intake
	}
	return label
}

// LecturerRoster — students who study this lecturer's unit(s). scope=enrolled (all who
// should study), scope=attended (only those who have attended ≥1 of his sessions).
func LecturerRoster(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		scope := r.URL.Query().Get("scope")
		unitFilter := strings.TrimSpace(r.URL.Query().Get("unit_id"))

		args := []interface{}{tenantID, lecturerID}
		unitWhere := ""
		if unitFilter != "" {
			args = append(args, unitFilter)
			unitWhere = fmt.Sprintf(" AND cu.unit_id = $%d", len(args))
		}
		attendedWhere := ""
		if scope == "attended" {
			attendedWhere = ` AND EXISTS (
				SELECT 1 FROM attendance_logs al JOIN sessions ses ON ses.session_id = al.session_id
				WHERE al.tenant_id = $1 AND al.student_id = s.student_id AND ses.unit_id = cu.unit_id)`
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT s.student_id, s.full_name,
			       cu.unit_id, cu.name, COALESCE(cu.level,''), COALESCE(cu.year,0), COALESCE(cu.semester,0),
			       o.course_id, COALESCE(c.name,''), o.offering_id::text,
			       COALESCE(o.study_year,0), COALESCE(o.semester,0), COALESCE(o.level,''), COALESCE(o.intake,''),
			       (SELECT count(*) FROM attendance_logs al JOIN sessions ses ON ses.session_id = al.session_id
			         WHERE al.tenant_id = $1 AND al.student_id = s.student_id AND ses.unit_id = cu.unit_id) AS attended_count
			FROM lecturer_assignments la
			JOIN course_units cu     ON cu.unit_id   = la.unit_id  AND cu.tenant_id = la.tenant_id
			JOIN course_offerings o  ON o.course_id  = cu.course_id AND o.tenant_id = cu.tenant_id
			LEFT JOIN courses c      ON c.course_id  = o.course_id  AND c.tenant_id = o.tenant_id
			JOIN students_extended s ON s.offering_id = o.offering_id AND s.tenant_id = o.tenant_id AND s.enrollment_status = 'ACTIVE'
			WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid`+unitWhere+attendedWhere+`
			ORDER BY cu.name, s.full_name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type row struct {
			StudentID     string `json:"student_id"`
			FullName      string `json:"full_name"`
			UnitID        string `json:"unit_id"`
			UnitName      string `json:"unit_name"`
			UnitLevel     string `json:"unit_level"`
			UnitYear      int    `json:"unit_year"`
			UnitSemester  int    `json:"unit_semester"`
			CourseID      string `json:"course_id"`
			CourseName    string `json:"course_name"`
			OfferingID    string `json:"offering_id"`
			Cohort        string `json:"cohort"`
			CohortYear    int    `json:"cohort_year"`
			CohortSem     int    `json:"cohort_semester"`
			CohortLevel   string `json:"cohort_level"`
			Intake        string `json:"intake"`
			AttendedCount int    `json:"attended_count"`
		}
		out := []row{}
		for rows.Next() {
			var x row
			if rows.Scan(&x.StudentID, &x.FullName, &x.UnitID, &x.UnitName, &x.UnitLevel, &x.UnitYear, &x.UnitSemester,
				&x.CourseID, &x.CourseName, &x.OfferingID, &x.CohortYear, &x.CohortSem, &x.CohortLevel, &x.Intake, &x.AttendedCount) != nil {
				continue
			}
			x.Cohort = cohortLabel(x.CohortYear, x.CohortSem, x.CohortLevel, x.Intake)
			out = append(out, x)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// LecturerSessions — every session for this lecturer's units (across cohorts), with the
// cohort, date, day-of-week, present count and enrolled count.
func LecturerSessions(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		unitFilter := strings.TrimSpace(r.URL.Query().Get("unit_id"))
		args := []interface{}{tenantID, lecturerID}
		unitWhere := ""
		if unitFilter != "" {
			args = append(args, unitFilter)
			unitWhere = fmt.Sprintf(" AND ses.unit_id = $%d", len(args))
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT ses.session_id::text, ses.unit_id, COALESCE(cu.name,''), ses.session_date::text,
			       COALESCE(EXTRACT(ISODOW FROM ses.session_date)::int, 0),
			       COALESCE(o.study_year,0), COALESCE(o.semester,0), COALESCE(o.level,''), COALESCE(o.intake,''),
			       ses.session_status::text,
			       (SELECT count(*) FROM attendance_logs al WHERE al.session_id = ses.session_id) AS present,
			       (SELECT count(*) FROM students_extended s WHERE s.offering_id = ses.offering_id AND s.enrollment_status='ACTIVE') AS enrolled
			FROM sessions ses
			JOIN lecturer_assignments la ON la.unit_id = ses.unit_id AND la.tenant_id = ses.tenant_id AND la.lecturer_id = $2::uuid
			LEFT JOIN course_units cu     ON cu.unit_id = ses.unit_id AND cu.tenant_id = ses.tenant_id
			LEFT JOIN course_offerings o  ON o.offering_id = ses.offering_id AND o.tenant_id = ses.tenant_id
			WHERE ses.tenant_id = $1`+unitWhere+`
			ORDER BY ses.session_date DESC, ses.gate_open_time DESC`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type sess struct {
			SessionID string `json:"session_id"`
			UnitID    string `json:"unit_id"`
			UnitName  string `json:"unit_name"`
			Date      string `json:"session_date"`
			DayOfWeek int    `json:"day_of_week"`
			Cohort    string `json:"cohort"`
			Status    string `json:"status"`
			Present   int    `json:"present_count"`
			Enrolled  int    `json:"enrolled_count"`
		}
		out := []sess{}
		for rows.Next() {
			var x sess
			var yr, sm int
			var lvl, intake string
			if rows.Scan(&x.SessionID, &x.UnitID, &x.UnitName, &x.Date, &x.DayOfWeek,
				&yr, &sm, &lvl, &intake, &x.Status, &x.Present, &x.Enrolled) != nil {
				continue
			}
			x.Cohort = cohortLabel(yr, sm, lvl, intake)
			out = append(out, x)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// LecturerSessionStudents — for ONE session: who was present, who was absent, or all
// enrolled with a present flag. status = present | absent | all (default all).
func LecturerSessionStudents(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		sessionID := extractPathID(r.URL.Path, "/api/v1/lecturer/sessions/", "/students")
		if sessionID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "missing session id"))
			return
		}
		status := r.URL.Query().Get("status")
		if status == "" {
			status = "all"
		}

		// Authorise: the session must be for a unit this lecturer is assigned to.
		var offeringID, unitID string
		err := adminPool.QueryRow(r.Context(), `
			SELECT ses.offering_id::text, ses.unit_id
			FROM sessions ses
			JOIN lecturer_assignments la ON la.unit_id = ses.unit_id AND la.tenant_id = ses.tenant_id AND la.lecturer_id = $2::uuid
			WHERE ses.tenant_id = $1 AND ses.session_id = $3::uuid LIMIT 1`,
			tenantID, lecturerID, sessionID).Scan(&offeringID, &unitID)
		if err != nil {
			writeJSON(w, http.StatusForbidden, errBody("NOT_ASSIGNED", "not your session"))
			return
		}

		// Enrolled students of the session's cohort, LEFT JOINed to their attendance row for
		// THIS session — so present = matched, absent = NULL.
		rows, err := adminPool.Query(r.Context(), `
			SELECT s.student_id, s.full_name, (al.log_id IS NOT NULL) AS present,
			       COALESCE(to_char(al.checkin_timestamp, 'YYYY-MM-DD"T"HH24:MI:SSZ'), '')
			FROM students_extended s
			LEFT JOIN attendance_logs al ON al.session_id = $2::uuid AND al.student_id = s.student_id AND al.tenant_id = $1
			WHERE s.tenant_id = $1 AND s.offering_id = $3::uuid AND s.enrollment_status = 'ACTIVE'
			ORDER BY s.full_name`, tenantID, sessionID, offeringID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type stu struct {
			StudentID string `json:"student_id"`
			FullName  string `json:"full_name"`
			Present   bool   `json:"present"`
			CheckinAt string `json:"checkin_at"`
		}
		out := []stu{}
		for rows.Next() {
			var x stu
			if rows.Scan(&x.StudentID, &x.FullName, &x.Present, &x.CheckinAt) != nil {
				continue
			}
			if status == "present" && !x.Present {
				continue
			}
			if status == "absent" && x.Present {
				continue
			}
			out = append(out, x)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"unit_id": unitID, "students": out})
	}
}
