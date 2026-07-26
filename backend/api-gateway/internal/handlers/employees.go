package handlers

// Employee registry — general (non-teaching) staff whose attendance comes from an
// external check-in tablet. Deliberately SEPARATE from lecturers and students:
// its own registration flow + its own bulk import/export template.
// Columns: staff_id, title, full_name, department, job_title, email, phone.

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GET /api/v1/admin/tenants/{tenant_id}/employees
func ListEmployees(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT employee_pk::text, staff_id, COALESCE(title,''), full_name,
			       COALESCE(department,''), COALESCE(job_title,''), COALESCE(email,''),
			       COALESCE(phone,''), is_active
			FROM employees WHERE tenant_id = $1 ORDER BY full_name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type employee struct {
			EmployeePK string `json:"employee_pk"`
			StaffID    string `json:"staff_id"`
			Title      string `json:"title"`
			FullName   string `json:"full_name"`
			Department string `json:"department"`
			JobTitle   string `json:"job_title"`
			Email      string `json:"email"`
			Phone      string `json:"phone"`
			IsActive   bool   `json:"is_active"`
		}
		list := []employee{}
		for rows.Next() {
			var e employee
			if err := rows.Scan(&e.EmployeePK, &e.StaffID, &e.Title, &e.FullName,
				&e.Department, &e.JobTitle, &e.Email, &e.Phone, &e.IsActive); err != nil {
				continue
			}
			list = append(list, e)
		}
		writeJSON(w, http.StatusOK, list)
	}
}

type employeeReq struct {
	StaffID    string `json:"staff_id"`
	Title      string `json:"title"`
	FullName   string `json:"full_name"`
	Department string `json:"department"`
	JobTitle   string `json:"job_title"`
	Email      string `json:"email"`
	Phone      string `json:"phone"`
}

// POST /api/v1/admin/tenants/{tenant_id}/employees
func CreateEmployee(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var req employeeReq
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed JSON"))
			return
		}
		req.StaffID = strings.TrimSpace(req.StaffID)
		req.FullName = strings.TrimSpace(req.FullName)
		if req.StaffID == "" || req.FullName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "staff_id and full_name are required"))
			return
		}
		var pk string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO employees (tenant_id, staff_id, title, full_name, department, job_title, email, phone)
			VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''))
			ON CONFLICT (tenant_id, staff_id) DO UPDATE SET
			    title      = COALESCE(NULLIF(EXCLUDED.title,''), employees.title),
			    full_name  = EXCLUDED.full_name,
			    department = COALESCE(NULLIF(EXCLUDED.department,''), employees.department),
			    job_title  = COALESCE(NULLIF(EXCLUDED.job_title,''), employees.job_title),
			    email      = COALESCE(NULLIF(EXCLUDED.email,''), employees.email),
			    phone      = COALESCE(NULLIF(EXCLUDED.phone,''), employees.phone)
			RETURNING employee_pk::text`,
			tenantID, req.StaffID, req.Title, req.FullName, req.Department, req.JobTitle, req.Email, req.Phone).Scan(&pk)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"employee_pk": pk, "status": "SAVED"})
	}
}

// PATCH /api/v1/admin/employees/{id}
func UpdateEmployee(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		var req employeeReq
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed JSON"))
			return
		}
		ct, err := adminPool.Exec(r.Context(), `
			UPDATE employees SET
			    staff_id   = COALESCE(NULLIF($2,''), staff_id),
			    title      = NULLIF($3,''),
			    full_name  = COALESCE(NULLIF($4,''), full_name),
			    department = NULLIF($5,''),
			    job_title  = NULLIF($6,''),
			    email      = NULLIF($7,''),
			    phone      = NULLIF($8,'')
			WHERE employee_pk = $1`,
			id, strings.TrimSpace(req.StaffID), req.Title, strings.TrimSpace(req.FullName),
			req.Department, req.JobTitle, req.Email, req.Phone)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "employee not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "UPDATED"})
	}
}

// DELETE /api/v1/admin/employees/{id}
func DeleteEmployee(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		ct, err := adminPool.Exec(r.Context(), `DELETE FROM employees WHERE employee_pk = $1`, id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "employee not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED"})
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/employees/import  (multipart field "roster")
func ImportEmployees(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, _, err := r.FormFile("roster")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' not found"))
			return
		}
		defer file.Close()

		rows, idx, err := readTabular(file)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", err.Error()))
			return
		}
		if _, ok := idx["staff_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", "missing required column: staff_id"))
			return
		}
		if _, ok := idx["full_name"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", "missing required column: full_name"))
			return
		}

		res := &importResult{Errors: []string{}}
		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			staffID := cell(row, idx, "staff_id")
			fullName := cell(row, idx, "full_name")
			if staffID == "" || fullName == "" {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: staff_id and full_name required", ln))
				continue
			}
			var inserted bool
			err := adminPool.QueryRow(r.Context(), `
				INSERT INTO employees (tenant_id, staff_id, title, full_name, department, job_title, email, phone)
				VALUES ($1,$2,NULLIF($3,''),$4,NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''))
				ON CONFLICT (tenant_id, staff_id) DO UPDATE SET
				    title      = COALESCE(NULLIF(EXCLUDED.title,''), employees.title),
				    full_name  = EXCLUDED.full_name,
				    department = COALESCE(NULLIF(EXCLUDED.department,''), employees.department),
				    job_title  = COALESCE(NULLIF(EXCLUDED.job_title,''), employees.job_title),
				    email      = COALESCE(NULLIF(EXCLUDED.email,''), employees.email),
				    phone      = COALESCE(NULLIF(EXCLUDED.phone,''), employees.phone)
				RETURNING (xmax = 0)`,
				tenantID, staffID, cell(row, idx, "title"), fullName,
				cell(row, idx, "department"), cell(row, idx, "job_title"),
				cell(row, idx, "email"), cell(row, idx, "phone")).Scan(&inserted)
			if err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", ln, err.Error()))
				continue
			}
			if inserted {
				res.Inserted++
			} else {
				res.Updated++
			}
		}
		writeJSON(w, http.StatusOK, res)
	}
}

// GET /api/v1/admin/tenants/{tenant_id}/employees/export.xlsx
func ExportEmployeesXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT staff_id, COALESCE(title,''), full_name, COALESCE(department,''),
			       COALESCE(job_title,''), COALESCE(email,''), COALESCE(phone,'')
			FROM employees WHERE tenant_id = $1 ORDER BY full_name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		out := [][]string{{"staff_id", "title", "full_name", "department", "job_title", "email", "phone"}}
		for rows.Next() {
			var sid, title, name, dept, job, email, phone string
			rows.Scan(&sid, &title, &name, &dept, &job, &email, &phone) //nolint:errcheck
			out = append(out, []string{sid, title, name, dept, job, email, phone})
		}
		xlsx, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="employees.xlsx"`)
		_, _ = w.Write(xlsx)
	}
}
