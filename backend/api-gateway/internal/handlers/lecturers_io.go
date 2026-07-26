package handlers

// Lecturer bulk import/export — template-driven, mirrors the student SIS import
// (sis.go). Columns: staff_id, full_name, email, phone, department, title, gender.
// Keeps the manual "+ Add lecturer" path as the fallback.

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
				INSERT INTO lecturers (tenant_id, full_name, email, phone, department, staff_id, title, gender)
				VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,''))
				ON CONFLICT (tenant_id, staff_id) WHERE staff_id IS NOT NULL AND staff_id <> ''
				DO UPDATE SET
				    full_name  = EXCLUDED.full_name,
				    email      = COALESCE(EXCLUDED.email, lecturers.email),
				    phone      = COALESCE(EXCLUDED.phone, lecturers.phone),
				    department = COALESCE(EXCLUDED.department, lecturers.department),
				    title      = COALESCE(EXCLUDED.title, lecturers.title),
				    gender     = COALESCE(EXCLUDED.gender, lecturers.gender)
				RETURNING lecturer_id::text, (xmax = 0)`,
				tenantID, fullName, email, get("phone"), get("department"), staffID, get("title"), get("gender")).Scan(&lecturerID, &inserted)
		} else {
			// No staff ID → plain insert (cannot dedupe reliably without it).
			qErr = pool.QueryRow(ctx, `
				INSERT INTO lecturers (tenant_id, full_name, email, phone, department, title, gender)
				VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''))
				RETURNING lecturer_id::text`,
				tenantID, fullName, email, get("phone"), get("department"), get("title"), get("gender")).Scan(&lecturerID)
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
		// Email the permanent career QR when an address was supplied.
		if email != "" && lecturerID != "" && httpReq != nil {
			emailLecturerQR(httpReq.Header.Get("Authorization"),
				lecturerScanURL(httpReq, makeLecturerQRToken(lecturerID, tenantID)),
				email, fullName, staffID)
		}
	}
	return res, nil
}

// GET /api/v1/admin/tenants/{tenant_id}/lecturers/export.xlsx  (optional ?department=)
func ExportLecturersXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		where := "tenant_id = $1"
		args := []interface{}{tenantID}
		if dept := strings.TrimSpace(r.URL.Query().Get("department")); dept != "" {
			args = append(args, dept)
			where += fmt.Sprintf(" AND department = $%d", len(args))
		}
		rows, err := adminPool.Query(r.Context(), `
			SELECT COALESCE(staff_id,''), full_name, COALESCE(email,''), COALESCE(phone,''),
			       COALESCE(department,''), COALESCE(title,''), COALESCE(gender,'')
			FROM lecturers WHERE `+where+` ORDER BY full_name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		// email is optional and used only to dispatch the lecturer's career QR.
		out := [][]string{{"staff_id", "full_name", "email", "phone", "department", "title", "gender"}}
		for rows.Next() {
			var sid, name, email, phone, dept, title, gender string
			rows.Scan(&sid, &name, &email, &phone, &dept, &title, &gender) //nolint:errcheck
			out = append(out, []string{sid, name, email, phone, dept, title, gender})
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
