package handlers

// Lecturer PORTAL — passwordless, READ-ONLY. A lecturer opens the portal, enters
// their institution + staff ID, and searches the attendance logs of the units they
// teach (filterable by course, cohort/coordinator and unit; day is filtered on the
// client from the session dates). No login, no password — the staff ID resolves the
// lecturer within their tenant (the same passwordless model as the student portal).
//
//   GET /api/v1/lecturer-portal/overview?staff_id=&org=
//   GET /api/v1/lecturer-portal/attendance?staff_id=&org=&unit_id=&coordinator=

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// portalResolveTenant returns THE single institution's tenant (the `org` arg is ignored now —
// single-institution deployment). Kept as a function so callers are unchanged.
func portalResolveTenant(ctx context.Context, pool *pgxpool.Pool, org string) (string, bool) {
	tid := singleTenantID(ctx, pool)
	return tid, tid != ""
}

// portalResolveLecturer resolves (tenant, staff_id) → lecturer.
func portalResolveLecturer(ctx context.Context, pool *pgxpool.Pool, tenantID, staffID string) (id, name string, ok bool) {
	// Case-insensitive + whitespace-tolerant match: staff IDs imported from spreadsheets often
	// differ in case/trailing spaces from what a lecturer types, and an exact match wrongly
	// reported "no lecturer with that staff ID" even though the ID exists.
	err := pool.QueryRow(ctx,
		`SELECT lecturer_id::text, full_name FROM lecturers WHERE tenant_id = $1 AND lower(btrim(staff_id)) = lower($2) LIMIT 1`,
		tenantID, strings.TrimSpace(staffID)).Scan(&id, &name)
	return id, name, err == nil && id != ""
}

// GET /api/v1/lecturer-portal/overview?staff_id=&org=
func LecturerPortalOverview(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
		org := r.URL.Query().Get("org")
		if staffID == "" || strings.TrimSpace(org) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "staff_id and org are required"))
			return
		}
		tenantID, ok := portalResolveTenant(r.Context(), adminPool, org)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "institution not found"))
			return
		}
		lecturerID, name, ok := portalResolveLecturer(r.Context(), adminPool, tenantID, staffID)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no lecturer with that staff ID at this institution"))
			return
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT DISTINCT cu.unit_id, cu.name, COALESCE(cu.year,1), COALESCE(cu.semester,1),
			       COALESCE(c.course_id,''), COALESCE(c.name,'')
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
			CourseID   string `json:"course_id"`
			CourseName string `json:"course_name"`
		}
		units := []unit{}
		for rows.Next() {
			var u unit
			rows.Scan(&u.UnitID, &u.Name, &u.Year, &u.Semester, &u.CourseID, &u.CourseName) //nolint:errcheck
			units = append(units, u)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"lecturer": map[string]string{"staff_id": staffID, "full_name": name},
			"units":    units,
		})
	}
}

// GET /api/v1/lecturer-portal/attendance?staff_id=&org=&unit_id=&coordinator=
func LecturerPortalAttendance(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		staffID := strings.TrimSpace(r.URL.Query().Get("staff_id"))
		org := r.URL.Query().Get("org")
		unitID := strings.TrimSpace(r.URL.Query().Get("unit_id"))
		if staffID == "" || strings.TrimSpace(org) == "" || unitID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "staff_id, org and unit_id are required"))
			return
		}
		tenantID, ok := portalResolveTenant(r.Context(), adminPool, org)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "institution not found"))
			return
		}
		lecturerID, _, ok := portalResolveLecturer(r.Context(), adminPool, tenantID, staffID)
		if !ok {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no lecturer with that staff ID at this institution"))
			return
		}
		// The unit must be one this lecturer is assigned to (read-only, own units only).
		var assigned bool
		_ = adminPool.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM lecturer_assignments WHERE tenant_id=$1 AND lecturer_id=$2::uuid AND unit_id=$3)`,
			tenantID, lecturerID, unitID).Scan(&assigned)
		if !assigned {
			writeJSON(w, http.StatusForbidden, errBody("NOT_ASSIGNED", "you are not assigned to this unit"))
			return
		}

		payload, status, err := buildUnitAttendance(r.Context(), adminPool, tenantID, unitID,
			strings.TrimSpace(r.URL.Query().Get("coordinator")))
		if err != nil {
			writeJSON(w, status, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, payload)
	}
}

// buildUnitAttendance builds the read-only attendance matrix for one unit: sessions
// (columns, with date + the coordinator who ran each) × students (rows, present/absent),
// plus the list of coordinators who have run the unit (the cohort filter's options).
// Shared by the JWT lecturer dashboard and the passwordless lecturer portal.
func buildUnitAttendance(ctx context.Context, pool *pgxpool.Pool, tenantID, unitID, coordFilter string) (map[string]interface{}, int, error) {
	type coordOpt struct {
		CoordinatorID   string `json:"coordinator_id"`
		CoordinatorName string `json:"coordinator_name"`
	}
	coordinators := []coordOpt{}
	cRows, _ := pool.Query(ctx, `
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
	sRows, err := pool.Query(ctx, `
		SELECT s.session_id::text, s.session_date::text, COALESCE(u.full_name, ''),
		       COALESCE(s.offering_id::text,'')
		FROM sessions s
		LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = s.tenant_id
		WHERE s.tenant_id = $1 AND s.unit_id = $2`+sessWhere+`
		ORDER BY s.session_date, s.gate_open_time`, sessArgs...)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	sessions := []sess{}
	sessIndex := map[string]int{}
	sessionIDs := []string{}
	// The cohorts these sessions belong to — the roster below is scoped to them.
	seenOfferings := map[string]bool{}
	for sRows.Next() {
		var s sess
		var offID string
		sRows.Scan(&s.SessionID, &s.Date, &s.Coordinator, &offID) //nolint:errcheck
		sessIndex[s.SessionID] = len(sessions)
		sessionIDs = append(sessionIDs, s.SessionID)
		sessions = append(sessions, s)
		seenOfferings[offID] = true
	}
	sRows.Close()
	// nil (not an empty slice) means "no session held yet" — show the whole
	// programme roster rather than nobody, so the lecturer still sees their class.
	var shownOfferings []string
	if len(seenOfferings) > 0 {
		shownOfferings = make([]string, 0, len(seenOfferings))
		for id := range seenOfferings {
			shownOfferings = append(shownOfferings, id)
		}
	}

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
	// COHORT ISOLATION: the roster is only the students of the study sessions whose
	// logs are actually on screen. Matching on (course, year, semester) alone pulled
	// in every cohort of the programme, so a Weekend student surfaced in the Day
	// cohort's logs. Anyone who genuinely checked in is still added below.
	stRows, err := pool.Query(ctx, `
		SELECT se.student_id, se.full_name
		FROM students_extended se
		WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'
		  AND se.course_id    = (SELECT course_id FROM course_units WHERE unit_id=$2 AND tenant_id=$1)
		  AND se.current_year = (SELECT year      FROM course_units WHERE unit_id=$2 AND tenant_id=$1)
		  AND se.semester     = (SELECT semester  FROM course_units WHERE unit_id=$2 AND tenant_id=$1)
		  AND (
		        $3::text[] IS NULL
		        OR se.offering_id::text = ANY($3::text[])
		        OR (se.offering_id IS NULL AND '' = ANY($3::text[]))
		      )
		ORDER BY se.full_name`, tenantID, unitID, shownOfferings)
	if err != nil {
		return nil, http.StatusInternalServerError, err
	}
	for stRows.Next() {
		var id, name string
		stRows.Scan(&id, &name) //nolint:errcheck
		addStudent(id, name)
	}
	stRows.Close()

	if len(sessionIDs) > 0 {
		aRows, err := pool.Query(ctx, `
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
				addStudent(stid, name)
				hits = append(hits, hit{sid, stid})
			}
			aRows.Close()
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

	return map[string]interface{}{
		"unit_id":      unitID,
		"sessions":     sessions,
		"students":     students,
		"coordinators": coordinators,
	}, http.StatusOK, nil
}
