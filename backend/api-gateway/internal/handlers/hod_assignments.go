package handlers

// Lecturer→unit assignment, done by the head of department.
//
// WHY THIS EXISTS SEPARATELY. Assignment was ADMIN-only, under
// /api/v1/admin/tenants/{tenant_id}/lecturer-assignments, and an admin route is
// unscoped by construction: it takes the tenant from the URL and trusts it. A
// head of department must be able to do the same job for THEIR department and
// nothing else, which is a different authorisation shape, not a looser one.
//
// Reusing the admin handlers with an extra role on the guard would have been
// smaller and wrong: an HOD would have been able to assign any lecturer to any
// unit in the institution, including other departments' units, because nothing in
// those handlers ever asks who is calling.
//
// So the scope comes from the caller's own user row via resolveOrgScope — never
// from a request parameter — and every query and mutation is filtered by it. The
// department a unit belongs to is `courses.department`, reached through
// course_units, which is the same join every other org-scoped screen uses.
//
//   GET    /api/v1/hod/assignments            — this department's assignments
//   GET    /api/v1/hod/assignable             — units + lecturers they may pick from
//   POST   /api/v1/hod/assignments            — assign, refusing units outside scope
//   DELETE /api/v1/hod/assignments/{id}       — unassign, same check

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type hodAssignment struct {
	AssignmentID string `json:"assignment_id"`
	LecturerID   string `json:"lecturer_id"`
	LecturerName string `json:"lecturer_name"`
	StaffID      string `json:"staff_id"`
	UnitID       string `json:"unit_id"`
	UnitName     string `json:"unit_name"`
	CourseID     string `json:"course_id"`
	CourseName   string `json:"course_name"`
	Department   string `json:"department"`
	AcademicYear string `json:"academic_year"`
	Year         int    `json:"year"`
	Semester     int    `json:"semester"`
	Intake       string `json:"intake_session"`
}

// GET /api/v1/hod/assignments
func HODListAssignments(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		sc, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		if !ok {
			// A bounded role with no department set matches nothing. Say so, rather
			// than rendering an empty table that looks like an empty institution.
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"unset": true, "assignments": []hodAssignment{},
				"message": "No department is set on your account — ask your administrator to set one.",
			})
			return
		}

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

		args := []interface{}{tenantID}
		where := sc.whereScope(&args)
		rows, err := conn.Query(r.Context(), `
			SELECT la.assignment_id::text, la.lecturer_id::text, COALESCE(l.full_name,''),
			       COALESCE(l.staff_id,''), la.unit_id, COALESCE(cu.name,''),
			       la.course_id, COALESCE(c.name,''), COALESCE(c.department,''),
			       COALESCE(la.academic_year,''), COALESCE(la.year,1), COALESCE(la.semester,1),
			       COALESCE(la.intake_session::text,'')
			FROM lecturer_assignments la
			JOIN lecturers    l  ON l.lecturer_id = la.lecturer_id
			JOIN course_units cu ON cu.unit_id   = la.unit_id     
			JOIN courses      c  ON c.course_id  = cu.course_id   
			WHERE la.tenant_id = $1`+where+`
			ORDER BY l.full_name, cu.name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []hodAssignment{}
		for rows.Next() {
			var a hodAssignment
			if rows.Scan(&a.AssignmentID, &a.LecturerID, &a.LecturerName, &a.StaffID, &a.UnitID,
				&a.UnitName, &a.CourseID, &a.CourseName, &a.Department, &a.AcademicYear,
				&a.Year, &a.Semester, &a.Intake) == nil {
				out = append(out, a)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"scope": sc.Val, "assignments": out,
		})
	}
}

// GET /api/v1/hod/assignable — the units in this department and the lecturers who
// may be put on them.
//
// Lecturers are NOT filtered to the department: teaching across departments is
// normal and is exactly how a lecturer comes to belong to two. The units are the
// scoped half, because those are what this HOD owns.
func HODAssignable(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		sc, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		if !ok {
			writeJSON(w, http.StatusOK, map[string]interface{}{"unset": true, "units": []any{}, "lecturers": []any{}})
			return
		}
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

		type unitOpt struct {
			UnitID     string `json:"unit_id"`
			UnitName   string `json:"unit_name"`
			CourseID   string `json:"course_id"`
			CourseName string `json:"course_name"`
			Year       int    `json:"year"`
			Semester   int    `json:"semester"`
			Assigned   int    `json:"assigned"`
		}
		args := []interface{}{tenantID}
		where := sc.whereScope(&args)
		urows, err := conn.Query(r.Context(), `
			SELECT cu.unit_id, COALESCE(cu.name,''), c.course_id, COALESCE(c.name,''),
			       COALESCE(cu.year,1), COALESCE(cu.semester,1),
			       (SELECT COUNT(*) FROM lecturer_assignments la
			         WHERE la.unit_id = cu.unit_id)
			FROM course_units cu
			JOIN courses c ON c.course_id = cu.course_id
			WHERE cu.tenant_id = $1`+where+`
			ORDER BY c.name, cu.year, cu.semester, cu.name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		units := []unitOpt{}
		for urows.Next() {
			var u unitOpt
			if urows.Scan(&u.UnitID, &u.UnitName, &u.CourseID, &u.CourseName, &u.Year, &u.Semester, &u.Assigned) == nil {
				units = append(units, u)
			}
		}
		urows.Close()

		type lecOpt struct {
			LecturerID string `json:"lecturer_id"`
			FullName   string `json:"full_name"`
			StaffID    string `json:"staff_id"`
		}
		lrows, err := conn.Query(r.Context(), `
			SELECT lecturer_id::text, COALESCE(full_name,''), COALESCE(staff_id,'')
			FROM lecturers WHERE tenant_id = $1 ORDER BY full_name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer lrows.Close()
		lecs := []lecOpt{}
		for lrows.Next() {
			var l lecOpt
			if lrows.Scan(&l.LecturerID, &l.FullName, &l.StaffID) == nil {
				lecs = append(lecs, l)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"scope": sc.Val, "units": units, "lecturers": lecs})
	}
}

// POST /api/v1/hod/assignments
func HODCreateAssignment(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())

		var req struct {
			LecturerID   string `json:"lecturer_id"`
			UnitID       string `json:"unit_id"`
			AcademicYear string `json:"academic_year"`
			Year         int    `json:"year"`
			Semester     int    `json:"semester"`
			Intake       string `json:"intake_session"`
		}
		if err := decodeJSON(r, &req); err != nil || req.LecturerID == "" || req.UnitID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "lecturer_id and unit_id are required"))
			return
		}
		sc, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NO_SCOPE", "No department is set on your account."))
			return
		}

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

		// The unit must be inside the caller's scope. Checked against the caller's own
		// user row, never against anything they sent — otherwise an HOD could assign
		// into another department by naming its unit.
		args := []interface{}{tenantID, req.UnitID}
		where := sc.whereScope(&args)
		var courseID string
		err = conn.QueryRow(r.Context(), `
			SELECT c.course_id
			FROM course_units cu
			JOIN courses c ON c.course_id = cu.course_id
			WHERE cu.tenant_id = $1 AND cu.unit_id = $2`+where+` LIMIT 1`, args...).Scan(&courseID)
		if err != nil {
			writeJSON(w, http.StatusForbidden, errBody("OUT_OF_SCOPE",
				"That unit is not in your department, so you cannot assign a lecturer to it."))
			return
		}

		if req.Year <= 0 {
			req.Year = 1
		}
		if req.Semester <= 0 {
			req.Semester = 1
		}
		if req.Intake == "" {
			req.Intake = "Morning"
		}
		if req.AcademicYear == "" {
			_ = conn.QueryRow(r.Context(),
				`SELECT COALESCE(active_academic_year,'') FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&req.AcademicYear)
		}

		var id string
		err = conn.QueryRow(r.Context(), `
			INSERT INTO lecturer_assignments
			    (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
			VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8)
			RETURNING assignment_id::text`,
			tenantID, req.LecturerID, req.UnitID, courseID, req.AcademicYear, req.Year, req.Semester, req.Intake).Scan(&id)
		if err != nil {
			// NO ENUM CAST above, and the error is no longer assumed to be a duplicate.
			//
			// intake_session is a varchar and this institution's commonest intake is "Day" — a
			// value the old `::intake_session_enum` cast has never accepted (the type only knows
			// Morning/Evening/Weekend/Distance). So staffing a Day cohort failed on the cast, and
			// because every error here was reported as a duplicate, the head of department was
			// told the lecturer was ALREADY assigned to a unit nobody had ever assigned them to —
			// and went looking for a row that did not exist.
			//
			// A real duplicate is SQLSTATE 23505 and nothing else; anything else is a fault worth
			// naming rather than disguising.
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeJSON(w, http.StatusConflict, errBody("CONFLICT",
					"That lecturer is already assigned to this unit for the same year and intake."))
				return
			}
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		auditAdmin(r, pool, tenantID, userID, "LECTURER_ASSIGNED", "lecturer_assignments", id, "")
		writeJSON(w, http.StatusOK, map[string]string{"assignment_id": id})
	}
}

// DELETE /api/v1/hod/assignments/{assignment_id}
func HODDeleteAssignment(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		role := middleware.GetRole(r.Context())
		id := chi.URLParam(r, "assignment_id")

		sc, ok := resolveOrgScope(r, pool, tenantID, userID, role)
		if !ok {
			writeJSON(w, http.StatusForbidden, errBody("NO_SCOPE", "No department is set on your account."))
			return
		}
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

		// Scope is re-checked on delete, not just on create: an assignment id is
		// guessable, and "can read it" is not "may remove it".
		args := []interface{}{tenantID, id}
		where := sc.whereScope(&args)
		tag, err := conn.Exec(r.Context(), `
			DELETE FROM lecturer_assignments la
			 USING course_units cu, courses c
			 WHERE la.tenant_id = $1 AND la.assignment_id = $2::uuid
			   AND cu.unit_id = la.unit_id
			   AND c.course_id = cu.course_id`+where, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusForbidden, errBody("OUT_OF_SCOPE",
				"That assignment is not in your department."))
			return
		}
		auditAdmin(r, pool, tenantID, userID, "LECTURER_UNASSIGNED", "lecturer_assignments", id, "")
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
