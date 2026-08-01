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

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ListSchools(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT s.school_id::text, s.name,
			       (SELECT COUNT(*) FROM departments d WHERE d.school_id = s.school_id) AS dept_count
			FROM schools s WHERE s.tenant_id = $1 ORDER BY s.name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type school struct {
			SchoolID  string `json:"school_id"`
			Name      string `json:"name"`
			DeptCount int    `json:"dept_count"`
		}
		out := []school{}
		for rows.Next() {
			var s school
			if rows.Scan(&s.SchoolID, &s.Name, &s.DeptCount) == nil {
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
		}
		if err := decodeJSON(r, &req); err != nil || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "name is required"))
			return
		}
		var id string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO schools (tenant_id, name) VALUES ($1, $2) RETURNING school_id::text`,
			tenantID, req.Name).Scan(&id)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", "school already exists: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"school_id": id, "status": "CREATED"})
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
		q := `SELECT d.department_id::text, d.school_id::text, d.name, d.kind
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
		if err := decodeJSON(r, &req); err != nil || req.Name == "" || req.SchoolID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "school_id and name are required"))
			return
		}
		if req.Kind == "" {
			req.Kind = "ACADEMIC"
		}
		var id string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO departments (tenant_id, school_id, name, kind)
			VALUES ($1, $2::uuid, $3, $4) RETURNING department_id::text`,
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
