package handlers

// Lecturer bulk import/export — template-driven, mirrors the student SIS import
// (sis.go). Columns: staff_id, full_name, email, phone, title, gender, school.
// Keeps the manual "+ Add lecturer" path as the fallback.
//
// SCHOOL IS ON THE IMPORT; DEPARTMENT IS NOT, and the distinction is the whole org model.
//
// A lecturer's HOME college is stored on the person: the institution's rule is that everyone sits
// under one, including a new lecturer who has not been given a single unit yet. That is exactly
// what a derived value cannot express — derive it from their assignments and an unassigned
// lecturer belongs nowhere and is invisible to every org role.
//
// Their DEPARTMENT is not stored, because a lecturer can teach across several at once and one
// field on the person could only ever be wrong for the rest. It comes from the units they are
// assigned to. A `department` cell in an uploaded file is therefore ignored rather than written.

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
		// The home college, by short form or full name — "SOMAC" and "School of Mathematics and
		// Computing" are the same place, and an institution's own spreadsheet will use whichever
		// its author thinks in. An unrecognised value does NOT skip the row: the lecturer is
		// still a real person worth recording, so they are imported without a college and the
		// line is reported so it can be fixed rather than silently lost.
		schoolID := ""
		if sc := firstNonEmpty(get("school"), get("college"), get("school/college"), get("school_college")); sc != "" {
			if err := pool.QueryRow(ctx, `
				SELECT school_id::text FROM schools
				WHERE tenant_id = $1
				  AND (btrim(lower(name)) = btrim(lower($2)) OR btrim(lower(COALESCE(abbreviation,''))) = btrim(lower($2)))
				LIMIT 1`, tenantID, sc).Scan(&schoolID); err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf(
					"line %d: no school or college called %q — lecturer imported without one", ln, sc))
			}
		}
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
				INSERT INTO lecturers (tenant_id, full_name, email, phone, staff_id, title, gender, school_id)
				VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),NULLIF($8,'')::uuid)
				ON CONFLICT (tenant_id, staff_id) WHERE staff_id IS NOT NULL AND staff_id <> ''
				DO UPDATE SET
				    full_name  = EXCLUDED.full_name,
				    email      = COALESCE(EXCLUDED.email, lecturers.email),
				    phone      = COALESCE(EXCLUDED.phone, lecturers.phone),
				    title      = COALESCE(EXCLUDED.title, lecturers.title),
				    gender     = COALESCE(EXCLUDED.gender, lecturers.gender),
				    -- COALESCE, so a re-import with the school column left blank does not
				    -- unfile every lecturer it touches.
				    school_id  = COALESCE(EXCLUDED.school_id, lecturers.school_id)
				RETURNING lecturer_id::text, (xmax = 0)`,
				tenantID, fullName, email, get("phone"), staffID, get("title"), get("gender"), schoolID).Scan(&lecturerID, &inserted)
		} else {
			// No staff ID → plain insert (cannot dedupe reliably without it).
			qErr = pool.QueryRow(ctx, `
				INSERT INTO lecturers (tenant_id, full_name, email, phone, title, gender, school_id)
				VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,'')::uuid)
				RETURNING lecturer_id::text`,
				tenantID, fullName, email, get("phone"), get("title"), get("gender"), schoolID).Scan(&lecturerID)
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
			       COALESCE(l.title,''), COALESCE(l.gender,''),
			       COALESCE(NULLIF(hs.abbreviation,''), hs.name, '')
			FROM lecturers l
			LEFT JOIN schools hs ON hs.school_id = l.school_id AND hs.tenant_id = l.tenant_id
			LEFT JOIN lecturer_assignments la ON la.lecturer_id = l.lecturer_id AND la.tenant_id = l.tenant_id
			LEFT JOIN course_units cu ON cu.unit_id = la.unit_id AND cu.tenant_id = la.tenant_id
			LEFT JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
			WHERE l.tenant_id = $1
			GROUP BY l.lecturer_id, l.staff_id, l.full_name, l.email, l.phone, l.title, l.gender,
			         hs.abbreviation, hs.name`+having+`
			ORDER BY l.full_name`, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		// email is optional and used only for correspondence.
		// `school` round-trips (it is stored on the lecturer); `departments` does not (it is
		// derived from their assignments) — hence the two sitting side by side.
		out := [][]string{{"staff_id", "full_name", "email", "phone", "title", "gender", "school", "departments"}}
		for rows.Next() {
			var sid, name, email, phone, title, gender, school string
			var depts []string
			rows.Scan(&sid, &name, &email, &phone, &depts, &title, &gender, &school) //nolint:errcheck
			out = append(out, []string{sid, name, email, phone, title, gender, school, strings.Join(depts, ", ")})
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

// ─── Removing a lecturer ─────────────────────────────────────────────────────
//
// WHAT A DELETE TAKES WITH IT, decided by the foreign keys and worth stating because it is not
// obvious from the button:
//
//	lecturer_assignments           CASCADE   — their unit assignments go
//	lecturer_biometric_templates   CASCADE   — their enrolled fingerprint goes
//	lecturer_webauthn_credentials  CASCADE   — their passkey goes
//	timetable_slots.lecturer_id    SET NULL  — the LECTURES SURVIVE, unattributed
//
// WHAT IT DELIBERATELY DOES NOT TOUCH is the teaching record: lecturer_attendance_logs,
// lecturer_patrol_logs and lecturer_presence_claims key on the staff id as text, not by foreign
// key, so every hour a lecturer taught and every patrol tick filed against them survives. That is
// the point. A system whose attendance history can be erased by removing a person from a list is
// not an attendance system, and the one case where someone most wants the record gone is the case
// where it matters most that it is not.
//
// The linked login is DEACTIVATED rather than deleted. Deleting the users row would take the audit
// trail with it (audit_log and notifications reference user_id), while leaving it active would let
// a removed lecturer keep signing in to a dashboard that now resolves to nothing.

// deleteLecturers removes a set of lecturers in one transaction and reports what it did.
// Shared by the single and bulk endpoints so the two can never drift apart.
func deleteLecturers(ctx context.Context, pool *pgxpool.Pool, tenantID string, ids []string) (map[string]interface{}, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// Counted BEFORE the delete: afterwards the assignment rows are gone and the slots are
	// already NULL, so there would be nothing left to count. An administrator pressing this
	// deserves to be told what it cost.
	var assignments, slots int
	_ = tx.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM lecturer_assignments
		         WHERE tenant_id = $1 AND lecturer_id = ANY($2::uuid[])),
		       (SELECT COUNT(*) FROM timetable_slots
		         WHERE tenant_id = $1 AND lecturer_id = ANY($2::uuid[]))`,
		tenantID, ids).Scan(&assignments, &slots)

	// Deactivate the sign-in first: if anything below fails the whole transaction rolls back,
	// so the two can never end up half-applied.
	var deactivated int64
	if tag, e := tx.Exec(ctx, `
		UPDATE users SET is_active = false
		WHERE tenant_id = $1
		  AND user_id IN (SELECT user_id FROM lecturers
		                   WHERE tenant_id = $1 AND lecturer_id = ANY($2::uuid[]) AND user_id IS NOT NULL)`,
		tenantID, ids); e == nil {
		deactivated = tag.RowsAffected()
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM lecturers WHERE tenant_id = $1 AND lecturer_id = ANY($2::uuid[])`, tenantID, ids)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"deleted":              tag.RowsAffected(),
		"assignments_removed":  assignments,
		"timetable_slots_kept": slots,
		"logins_deactivated":   deactivated,
	}, nil
}

// DELETE /api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}
func DeleteLecturer(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		id := strings.TrimSpace(chi.URLParam(r, "lecturer_id"))
		if id == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "lecturer_id is required"))
			return
		}
		res, err := deleteLecturers(r.Context(), adminPool, tenantID, []string{id})
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if res["deleted"].(int64) == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "no such lecturer in this institution"))
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/lecturers/bulk-delete   {"lecturer_ids": [...]}
//
// A POST rather than a DELETE with a body: request bodies on DELETE are permitted by the spec but
// dropped by enough proxies and client libraries that a list sent that way arrives empty, and an
// empty list deletes nothing while reporting success — the worst possible failure for this button.
func BulkDeleteLecturers(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var req struct {
			LecturerIDs []string `json:"lecturer_ids"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		ids := make([]string, 0, len(req.LecturerIDs))
		for _, id := range req.LecturerIDs {
			if id = strings.TrimSpace(id); id != "" {
				ids = append(ids, id)
			}
		}
		// Refused, not treated as a no-op success. "Deleted 0" after selecting rows reads as
		// "they were already gone", and the operator moves on believing it worked.
		if len(ids) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "select at least one lecturer"))
			return
		}
		res, err := deleteLecturers(r.Context(), adminPool, tenantID, ids)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
