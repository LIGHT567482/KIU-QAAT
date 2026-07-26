package handlers

// Lecturer dashboard — a logged-in lecturer (role LECTURER) sees the units they're
// assigned to and, per unit, the student attendance MATRIX (students × session
// dates, ✓ = present) along with the coordinator who ran each session. Read-only.
//
//   POST /api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}/create-login (ADMIN)
//   GET  /api/v1/lecturer/overview              (LECTURER)
//   GET  /api/v1/lecturer/attendance?unit_id=   (LECTURER)

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// randPassword generates a short strong password (shared: used by admin user/lecturer
// creation flows elsewhere). Lecturers themselves no longer log in with a password.
func randPassword() string {
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	return "Lec" + base64.RawURLEncoding.EncodeToString(b) // ~12 chars, mixed
}

// resolveLecturerID maps the logged-in LECTURER user to their lecturers.lecturer_id
// (by the user_id link, with an email fallback for accounts made via Users).
func resolveLecturerID(adminPool *pgxpool.Pool, r *http.Request, tenantID, userID string) (string, bool) {
	var id string
	err := adminPool.QueryRow(r.Context(), `
		SELECT l.lecturer_id::text FROM lecturers l
		WHERE l.tenant_id = $1
		  AND ( l.user_id::text = $2
		        OR (l.user_id IS NULL AND l.email = (SELECT u.email FROM users u WHERE u.user_id::text = $2)) )
		LIMIT 1`, tenantID, userID).Scan(&id)
	return id, err == nil && id != ""
}

// GET /api/v1/lecturer/overview
func LecturerOverview(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{"lecturer": nil, "units": []any{}})
			return
		}
		var name string
		_ = adminPool.QueryRow(r.Context(), `SELECT full_name FROM lecturers WHERE lecturer_id=$1::uuid`, lecturerID).Scan(&name)

		rows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT cu.unit_id, cu.name, COALESCE(cu.year,1), COALESCE(cu.semester,1),
			       COALESCE(c.name,'')
			FROM lecturer_assignments la
			JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
			LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			WHERE la.tenant_id = $1 AND la.lecturer_id = $2::uuid
			ORDER BY COALESCE(c.name,''), COALESCE(cu.year,1), COALESCE(cu.semester,1), cu.name`, tenantID, lecturerID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type unit struct {
			UnitID     string `json:"unit_id"`
			Name       string `json:"name"`
			Year       int    `json:"year"`
			Semester   int    `json:"semester"`
			CourseName string `json:"course_name"`
		}
		units := []unit{}
		for rows.Next() {
			var u unit
			rows.Scan(&u.UnitID, &u.Name, &u.Year, &u.Semester, &u.CourseName) //nolint:errcheck
			units = append(units, u)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lecturer": map[string]string{"lecturer_id": lecturerID, "full_name": name},
			"units":    units,
		})
	}
}

// GET /api/v1/lecturer/attendance?unit_id=
// The attendance matrix for one of the lecturer's units: students (rows) × session
// dates (columns) with present/absent, plus the coordinator who ran each session.
func LecturerAttendance(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		unitID := strings.TrimSpace(r.URL.Query().Get("unit_id"))
		if unitID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "unit_id is required"))
			return
		}
		lecturerID, ok := resolveLecturerID(adminPool, r, tenantID, userID)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NOT_A_LECTURER", "no lecturer profile for this account"))
			return
		}
		// Authorisation: the unit must be one the lecturer is assigned to.
		var assigned bool
		_ = adminPool.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM lecturer_assignments WHERE tenant_id=$1 AND lecturer_id=$2::uuid AND unit_id=$3)`,
			tenantID, lecturerID, unitID).Scan(&assigned)
		if !assigned {
			writeJSON(w, http.StatusForbidden, errBody("NOT_ASSIGNED", "you are not assigned to this unit"))
			return
		}

		// A unit can be SHARED across coordinators/cohorts — let the lecturer filter
		// by the coordinator who ran the sessions.
		coordFilter := strings.TrimSpace(r.URL.Query().Get("coordinator"))

		// Coordinators who have ever run this unit (the filter's options).
		type coordOpt struct {
			CoordinatorID   string `json:"coordinator_id"`
			CoordinatorName string `json:"coordinator_name"`
		}
		coordinators := []coordOpt{}
		cRows, _ := adminPool.Query(r.Context(), `
			SELECT DISTINCT COALESCE(s.coordinator_id,''), COALESCE(u.full_name,'')
			FROM sessions s
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = s.tenant_id
			WHERE s.tenant_id = $1 AND s.unit_id = $2 AND COALESCE(s.coordinator_id,'') <> ''
			ORDER BY 2`, tenantID, unitID)
		if cRows != nil {
			for cRows.Next() {
				var c coordOpt
				cRows.Scan(&c.CoordinatorID, &c.CoordinatorName) //nolint:errcheck
				coordinators = append(coordinators, c)
			}
			cRows.Close()
		}

		type sess struct {
			SessionID   string `json:"session_id"`
			Date        string `json:"session_date"`
			Coordinator string `json:"coordinator_name"`
		}
		sessArgs := []interface{}{tenantID, unitID}
		sessWhere := ""
		if coordFilter != "" {
			sessArgs = append(sessArgs, coordFilter)
			sessWhere = fmt.Sprintf(" AND s.coordinator_id = $%d", len(sessArgs))
		}
		sRows, err := adminPool.Query(r.Context(), `
			SELECT s.session_id::text, s.session_date::text, COALESCE(u.full_name, '')
			FROM sessions s
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = s.tenant_id
			WHERE s.tenant_id = $1 AND s.unit_id = $2`+sessWhere+`
			ORDER BY s.session_date, s.gate_open_time`, sessArgs...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		sessions := []sess{}
		sessIndex := map[string]int{}
		sessionIDs := []string{}
		for sRows.Next() {
			var s sess
			sRows.Scan(&s.SessionID, &s.Date, &s.Coordinator) //nolint:errcheck
			sessIndex[s.SessionID] = len(sessions)
			sessionIDs = append(sessionIDs, s.SessionID)
			sessions = append(sessions, s)
		}
		sRows.Close()

		// Roster = students enrolled in this unit's (course, year, semester) UNION
		// anyone who actually attended these sessions — so students of OTHER courses
		// sharing the unit still appear.
		type student struct {
			StudentID string `json:"student_id"`
			FullName  string `json:"full_name"`
			Present   []bool `json:"present"`
		}
		students := []student{}
		stIndex := map[string]int{}
		addStudent := func(id, name string) {
			if _, seen := stIndex[id]; seen || id == "" {
				return
			}
			stIndex[id] = len(students)
			students = append(students, student{StudentID: id, FullName: name, Present: make([]bool, len(sessions))})
		}
		stRows, err := adminPool.Query(r.Context(), `
			SELECT se.student_id, se.full_name
			FROM students_extended se
			WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'
			  AND se.course_id    = (SELECT course_id FROM course_units WHERE unit_id=$2 AND tenant_id=$1)
			  AND se.current_year = (SELECT year      FROM course_units WHERE unit_id=$2 AND tenant_id=$1)
			  AND se.semester     = (SELECT semester  FROM course_units WHERE unit_id=$2 AND tenant_id=$1)
			ORDER BY se.full_name`, tenantID, unitID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for stRows.Next() {
			var id, name string
			stRows.Scan(&id, &name) //nolint:errcheck
			addStudent(id, name)
		}
		stRows.Close()

		// Fill the matrix from attendance_logs for the (filtered) sessions, adding any
		// attendee not already on the enrolled roster (shared-unit / cross-course).
		if len(sessionIDs) > 0 {
			aRows, err := adminPool.Query(r.Context(), `
				SELECT al.session_id::text, al.student_id, COALESCE(se.full_name, al.student_id)
				FROM attendance_logs al
				LEFT JOIN students_extended se ON se.student_id = al.student_id AND se.tenant_id = $1
				WHERE al.session_id = ANY($2)`, tenantID, sessionIDs)
			if err == nil {
				type hit struct{ sid, stid string }
				hits := []hit{}
				for aRows.Next() {
					var sid, stid, name string
					aRows.Scan(&sid, &stid, &name) //nolint:errcheck
					addStudent(stid, name) // ensure a row exists (may grow students)
					hits = append(hits, hit{sid, stid})
				}
				aRows.Close()
				// Present[] slices were sized before late-added attendees; normalise.
				for i := range students {
					if len(students[i].Present) != len(sessions) {
						students[i].Present = make([]bool, len(sessions))
					}
				}
				for _, h := range hits {
					if si, sok := sessIndex[h.sid]; sok {
						if ti, tok := stIndex[h.stid]; tok {
							students[ti].Present[si] = true
						}
					}
				}
			}
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"unit_id":      unitID,
			"sessions":     sessions,
			"students":     students,
			"coordinators": coordinators, // filter options (a unit may be shared)
		})
	}
}
