package handlers

// Lecturer bulk import/export — template-driven, mirrors the student SIS import
// (sis.go). Columns: staff_id, full_name, email, phone, title, gender.
// Keeps the manual "+ Add lecturer" path as the fallback.
//
// NO department column on import. A lecturer has none of their own: they can teach across several
// colleges, and the units they are assigned to are what carry the department and school. A
// `department` cell in an uploaded file is therefore ignored rather than written — the export emits
// a DERIVED `departments` column for reading, which does not round-trip back in.

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// POST /api/v1/admin/tenants/{tenant_id}/lecturers/import  (multipart field "roster")
func ImportLecturers(adminPool *pgxpool.Pool) http.HandlerFunc {
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

		res, perr := processLecturerCSV(r.Context(), adminPool, tenantID, file, r)
		if perr != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", perr.Error()))
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

func processLecturerCSV(ctx context.Context, pool *pgxpool.Pool, tenantID string, src io.Reader, httpReq *http.Request) (*importResult, error) {
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	var rows [][]string
	if looksXLSX(data) {
		if rows, err = parseXLSX(data); err != nil {
			return nil, err
		}
	} else {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.TrimLeadingSpace = true
		cr.FieldsPerRecord = -1
		if rows, err = cr.ReadAll(); err != nil {
			return nil, fmt.Errorf("could not parse CSV: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("file has no rows")
	}

	colIdx := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	if _, ok := colIdx["full_name"]; !ok {
		return nil, fmt.Errorf("missing required column: full_name")
	}

	res := &importResult{Errors: []string{}}
	for ln := 1; ln < len(rows); ln++ {
		row := rows[ln]
		get := func(c string) string {
			i, ok := colIdx[c]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		fullName := get("full_name")
		if fullName == "" {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: full_name required", ln))
			continue
		}
		staffID := get("staff_id")
		// email is OPTIONAL — used only to dispatch the lecturer's career QR, never
		// for login. A blank email simply means no QR is emailed for this row.
		email := get("email")

		var inserted bool
		var lecturerID string
		var qErr error
		if staffID != "" {
			// Upsert by (tenant, staff_id) — matches the partial unique index
			// ux_lecturers_tenant_staffid (predicate must be repeated).
			qErr = pool.QueryRow(ctx, `
				INSERT INTO lecturers (tenant_id, full_name, email, phone, staff_id, title, gender)
				VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''))
				ON CONFLICT (tenant_id, staff_id) WHERE staff_id IS NOT NULL AND staff_id <> ''
				DO UPDATE SET
				    full_name  = EXCLUDED.full_name,
				    email      = COALESCE(EXCLUDED.email, lecturers.email),
				    phone      = COALESCE(EXCLUDED.phone, lecturers.phone),
				    title      = COALESCE(EXCLUDED.title, lecturers.title),
				    gender     = COALESCE(EXCLUDED.gender, lecturers.gender)
				RETURNING lecturer_id::text, (xmax = 0)`,
				tenantID, fullName, email, get("phone"), staffID, get("title"), get("gender")).Scan(&lecturerID, &inserted)
		} else {
			// No staff ID → plain insert (cannot dedupe reliably without it).
			qErr = pool.QueryRow(ctx, `
				INSERT INTO lecturers (tenant_id, full_name, email, phone, title, gender)
				VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''))
				RETURNING lecturer_id::text`,
				tenantID, fullName, email, get("phone"), get("title"), get("gender")).Scan(&lecturerID)
			inserted = true
		}
		if qErr != nil {
			res.Skipped++
			res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", ln, qErr.Error()))
			continue
		}
		if inserted {
			res.Inserted++
		} else {
			res.Updated++
		}
	}
	return res, nil
}

// GET /api/v1/admin/tenants/{tenant_id}/lecturers/export.xlsx  (optional ?department=)
//
// The exported `departments` column is DERIVED from the units the lecturer is assigned to, and so
// is the optional filter: a lecturer has no department of their own, and one who teaches in two
// colleges appears under both. It is informational — re-importing this file will not write it back,
// because assignment is what places a lecturer under a department.
func ExportLecturersXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		args := []interface{}{tenantID}
		having := ""
		if dept := strings.TrimSpace(r.URL.Query().Get("department")); dept != "" {
			args = append(args, dept)
			having = fmt.Sprintf(
				" HAVING BOOL_OR(btrim(lower(c.department)) = btrim(lower($%d)))", len(args))
		}
		rows, err := adminPool.Query(r.Context(), `
			SELECT COALESCE(l.staff_id,''), l.full_name, COALESCE(l.email,''), COALESCE(l.phone,''),
			       COALESCE(ARRAY_AGG(DISTINCT c.department) FILTER (WHERE COALESCE(c.department,'') <> ''), '{}'),
			       COALESCE(l.title,''), COALESCE(l.gender,'')
			FROM lecturers l
			LEFT JOIN lecturer_assignments la ON la.lecturer_id = l.lecturer_id AND la.tenant_id = l.tenant_id
			LEFT JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
			LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			WHERE l.tenant_id = $1
			GROUP BY l.lecturer_id, l.staff_id, l.full_name, l.email, l.phone, l.title, l.gender`+having+`
			ORDER BY l.full_name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		// email is optional and used only for correspondence.
		out := [][]string{{"staff_id", "full_name", "email", "phone", "departments", "title", "gender"}}
		for rows.Next() {
			var sid, name, email, phone, title, gender string
			var depts []string
			rows.Scan(&sid, &name, &email, &phone, &depts, &title, &gender) //nolint:errcheck
			out = append(out, []string{sid, name, email, phone, strings.Join(depts, ", "), title, gender})
		}

		xlsx, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="lecturers.xlsx"`)
		_, _ = w.Write(xlsx)
	}
}
