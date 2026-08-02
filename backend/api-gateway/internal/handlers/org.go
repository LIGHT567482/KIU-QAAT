package handlers

// Schools/colleges + departments management (Phase 1 org foundation). Admin-only, tenant-scoped
// via the explicit tenant_id URL param (adminPool bypasses RLS like the other admin handlers).
//
//   GET/POST    /api/v1/admin/tenants/{tenant_id}/schools
//   DELETE      /api/v1/admin/tenants/{tenant_id}/schools/{school_id}
//   GET/POST    /api/v1/admin/tenants/{tenant_id}/departments        (GET accepts ?school_id=)
//   DELETE      /api/v1/admin/tenants/{tenant_id}/departments/{department_id}

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ListSchools(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT s.school_id::text, s.name, COALESCE(s.abbreviation,''),
			       (SELECT COUNT(*) FROM departments d WHERE d.school_id = s.school_id) AS dept_count
			FROM schools s WHERE s.tenant_id = $1 ORDER BY s.name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type school struct {
			SchoolID string `json:"school_id"`
			Name     string `json:"name"`
			// The short form everyone actually says — "SOMAC" for "School of Mathematics and
			// Computing". Also an alias the org-scope matcher accepts; see school_alias.go.
			Abbreviation string `json:"abbreviation"`
			DeptCount    int    `json:"dept_count"`
		}
		out := []school{}
		for rows.Next() {
			var s school
			if rows.Scan(&s.SchoolID, &s.Name, &s.Abbreviation, &s.DeptCount) == nil {
				out = append(out, s)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func CreateSchool(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var req struct {
			Name string `json:"name"`
			// The short form. Separate from the name on purpose: "School of Mathematics and
			// Computing" belongs on a report, "SOMAC" is what fits in a column and what people
			// say. Institutions without both used to enter the abbreviation AS the name, which
			// left the full title recorded nowhere at all.
			Abbreviation string `json:"abbreviation"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Abbreviation = strings.TrimSpace(req.Abbreviation)
		if req.Name == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "the school's full name is required"))
			return
		}
		if req.Abbreviation == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"the school's short form is required — e.g. SOMAC for School of Mathematics and Computing"))
			return
		}
		if len(req.Abbreviation) > 32 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "the short form must be 32 characters or fewer"))
			return
		}
		var id string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO schools (tenant_id, name, abbreviation) VALUES ($1, $2, $3)
			RETURNING school_id::text`,
			tenantID, req.Name, req.Abbreviation).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT",
				"a school with that name or short form already exists: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"school_id": id, "status": "CREATED"})
	}
}

// UpdateSchool — PATCH /api/v1/admin/tenants/{tenant_id}/schools/{school_id}
//
// Exists so an institution that entered the ABBREVIATION as the name (which is what everyone did
// before there was a field for it) can put the full title in without losing anything: the old value
// moves to `abbreviation`, and because the matcher accepts both forms, every course, account and
// dashboard row still written against the short form keeps resolving to this school.
func UpdateSchool(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		id := chi.URLParam(r, "school_id")
		var req struct {
			Name         string `json:"name"`
			Abbreviation string `json:"abbreviation"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		req.Abbreviation = strings.TrimSpace(req.Abbreviation)
		if req.Name == "" || req.Abbreviation == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"both the full name and the short form are required"))
			return
		}
		ct, err := adminPool.Exec(r.Context(), `
			UPDATE schools SET name = $1, abbreviation = $2
			 WHERE school_id = $3::uuid AND tenant_id = $4`,
			req.Name, req.Abbreviation, id, tenantID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT",
				"a school with that name or short form already exists: "+err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such school"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "UPDATED"})
	}
}

func DeleteSchool(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		id := chi.URLParam(r, "school_id")
		_, err := adminPool.Exec(r.Context(),
			`DELETE FROM schools WHERE school_id = $1::uuid AND tenant_id = $2`, id, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED"})
	}
}

func ListDepartments(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		schoolID := r.URL.Query().Get("school_id")
		// school_id is NULL for a standalone SUPPORT department (Finance, ICT, Library…),
		// which belongs to no school — COALESCE so it scans, and reaches the UI as "".
		q := `SELECT d.department_id::text, COALESCE(d.school_id::text,''), d.name, d.kind
		      FROM departments d WHERE d.tenant_id = $1`
		args := []interface{}{tenantID}
		if schoolID != "" {
			q += ` AND d.school_id = $2::uuid`
			args = append(args, schoolID)
		}
		q += ` ORDER BY d.name`
		rows, err := adminPool.Query(r.Context(), q, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type dept struct {
			DepartmentID string `json:"department_id"`
			SchoolID     string `json:"school_id"`
			Name         string `json:"name"`
			Kind         string `json:"kind"`
		}
		out := []dept{}
		for rows.Next() {
			var d dept
			if rows.Scan(&d.DepartmentID, &d.SchoolID, &d.Name, &d.Kind) == nil {
				out = append(out, d)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func CreateDepartment(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var req struct {
			SchoolID string `json:"school_id"`
			Name     string `json:"name"`
			Kind     string `json:"kind"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Name) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "name is required"))
			return
		}
		req.Name = strings.TrimSpace(req.Name)
		if req.Kind == "" {
			req.Kind = "ACADEMIC"
		}
		// A SUPPORT department (Finance, Admissions, Bursary, Library, ICT…) is
		// institution-wide and belongs to no school, so school_id may be omitted.
		// An ACADEMIC one still has to name its school.
		req.SchoolID = strings.TrimSpace(req.SchoolID)
		if req.SchoolID == "" && req.Kind != "SUPPORT" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"school_id is required for an academic department"))
			return
		}
		var id string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO departments (tenant_id, school_id, name, kind)
			VALUES ($1, NULLIF($2,'')::uuid, $3, $4) RETURNING department_id::text`,
			tenantID, req.SchoolID, req.Name, req.Kind).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", "department already exists: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"department_id": id, "status": "CREATED"})
	}
}

func DeleteDepartment(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		id := chi.URLParam(r, "department_id")
		_, err := adminPool.Exec(r.Context(),
			`DELETE FROM departments WHERE department_id = $1::uuid AND tenant_id = $2`, id, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED"})
	}
}
