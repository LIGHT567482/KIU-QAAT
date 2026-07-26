package handlers

// Curriculum bulk import/export — lets a university load its existing catalogue
// instead of typing ~260 courses × 6+ units by hand. Three separate files, each
// CSV or XLSX, each round-trips with its matching export (export == the template):
//
//   courses              : course_id, name, department, school
//   course-units (roadmap): unit_id, course_id, name, year, semester, level
//   lecturer-assignments  : unit_id, lecturer_staff_id, lecturer_name, academic_year, intake_session, year, semester
//
// The assignments importer AUTO-CREATES a lecturer it can't find (by staff_id, else
// by name) and maps them to the unit, so imported units arrive with their dedicated
// lecturers — no fresh manual attachment. Mirrors lecturers_io.go / sis.go.

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// readTabular reads a CSV or XLSX upload into rows + a lower-cased header→index map.
func readTabular(r io.Reader) ([][]string, map[string]int, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, nil, fmt.Errorf("could not read file: %w", err)
	}
	var rows [][]string
	if looksXLSX(data) {
		if rows, err = parseXLSX(data); err != nil {
			return nil, nil, err
		}
	} else {
		cr := csv.NewReader(bytes.NewReader(data))
		cr.TrimLeadingSpace = true
		cr.FieldsPerRecord = -1
		if rows, err = cr.ReadAll(); err != nil {
			return nil, nil, fmt.Errorf("could not parse CSV: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil, nil, fmt.Errorf("file has no rows")
	}
	idx := make(map[string]int, len(rows[0]))
	for i, h := range rows[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return rows, idx, nil
}

func cell(row []string, idx map[string]int, col string) string {
	i, ok := idx[col]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

func uploadFile(w http.ResponseWriter, r *http.Request) (io.ReadCloser, bool) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
		return nil, false
	}
	file, _, err := r.FormFile("roster")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' (the file) not found"))
		return nil, false
	}
	return file, true
}

func writeXLSX(w http.ResponseWriter, name string, out [][]string) {
	xlsx, err := buildXLSX(out)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, name))
	_, _ = w.Write(xlsx)
}

// ─── Courses ─────────────────────────────────────────────────────────────────

func ImportCourses(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		file, ok := uploadFile(w, r)
		if !ok {
			return
		}
		defer file.Close()
		rows, idx, err := readTabular(file)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", err.Error()))
			return
		}
		if _, ok := idx["course_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", "missing required column: course_id"))
			return
		}
		res := &importResult{Errors: []string{}}
		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			cid := cell(row, idx, "course_id")
			name := cell(row, idx, "name")
			if cid == "" || name == "" {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: course_id and name are required", ln))
				continue
			}
			// total_years + level are NOT course properties — years vary by level, and a
			// course is added independently of level (levels are added within it later).
			var inserted bool
			err := adminPool.QueryRow(r.Context(), `
				INSERT INTO courses (course_id, tenant_id, name, department, school)
				VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''))
				ON CONFLICT (course_id) DO UPDATE SET
				    name = EXCLUDED.name,
				    department = COALESCE(EXCLUDED.department, courses.department),
				    school = COALESCE(EXCLUDED.school, courses.school)
				WHERE courses.tenant_id = $2
				RETURNING (xmax = 0)`,
				cid, tenantID, name,
				cell(row, idx, "department"), cell(row, idx, "school")).Scan(&inserted)
			if err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d (%s): exists under another tenant or write failed", ln, cid))
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

func ExportCoursesXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT course_id, name,
			       COALESCE(department,''), COALESCE(school,'')
			FROM courses WHERE tenant_id = $1 ORDER BY name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		out := [][]string{{"course_id", "name", "department", "school"}}
		for rows.Next() {
			var cid, name, dept, school string
			rows.Scan(&cid, &name, &dept, &school) //nolint:errcheck
			out = append(out, []string{cid, name, dept, school})
		}
		writeXLSX(w, "courses.xlsx", out)
	}
}

// ─── Course units (the roadmap) ──────────────────────────────────────────────

func ImportCourseUnits(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		file, ok := uploadFile(w, r)
		if !ok {
			return
		}
		defer file.Close()
		rows, idx, err := readTabular(file)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", err.Error()))
			return
		}
		if _, ok := idx["unit_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", "missing required column: unit_id"))
			return
		}
		// Importing from a specific course's Units page sends target_course: every
		// unit then lands in THAT course, and a unit that currently belongs to a
		// DIFFERENT course is only moved with the admin's consent (confirm_transfers).
		// Tenant-wide curriculum import sends no target_course — it keeps the old
		// behaviour (course filed from the row's course_id / default).
		target := strings.TrimSpace(r.FormValue("target_course"))
		defaultCourse := strings.TrimSpace(r.FormValue("course_id"))
		confirm := r.FormValue("confirm_transfers") == "true"
		consentMode := target != ""

		type unitTransfer struct {
			UnitID     string `json:"unit_id"`
			Name       string `json:"name"`
			FromCourse string `json:"from_course"`
			ToCourse   string `json:"to_course"`
		}
		res := &importResult{Errors: []string{}}
		transfers := []unitTransfer{}

		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			uid := cell(row, idx, "unit_id")
			name := cell(row, idx, "name")
			// Effective course: the page's target in consent mode, else the row/default.
			cid := target
			if !consentMode {
				cid = cell(row, idx, "course_id")
				if cid == "" {
					cid = defaultCourse
				}
			}
			if uid == "" || cid == "" || name == "" {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: unit_id, course_id and name are required", ln))
				continue
			}
			// The destination course must exist for this tenant.
			var courseExists bool
			_ = adminPool.QueryRow(r.Context(),
				`SELECT true FROM courses WHERE course_id = $1 AND tenant_id = $2`, cid, tenantID).Scan(&courseExists)
			if !courseExists {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d (%s): course '%s' not found — import courses first", ln, uid, cid))
				continue
			}

			// Does this unit already live under a different course? In consent mode,
			// such a cross-course move waits for explicit approval.
			if consentMode {
				var cur string
				_ = adminPool.QueryRow(r.Context(),
					`SELECT course_id FROM course_units WHERE unit_id = $1 AND tenant_id = $2`, uid, tenantID).Scan(&cur)
				if cur != "" && cur != cid && !confirm {
					transfers = append(transfers, unitTransfer{UnitID: uid, Name: name, FromCourse: cur, ToCourse: cid})
					continue // held back until the admin approves the transfer
				}
			}

			yr, _ := strconv.Atoi(cell(row, idx, "year"))
			if yr <= 0 {
				yr = 1
			}
			sem, _ := strconv.Atoi(cell(row, idx, "semester"))
			if sem != 2 {
				sem = 1
			}
			var inserted bool
			err := adminPool.QueryRow(r.Context(), `
				INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester, level)
				VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,''))
				ON CONFLICT (unit_id) DO UPDATE SET
				    course_id = EXCLUDED.course_id,
				    name = EXCLUDED.name,
				    year = EXCLUDED.year,
				    semester = EXCLUDED.semester,
				    level = COALESCE(EXCLUDED.level, course_units.level)
				WHERE course_units.tenant_id = $2
				RETURNING (xmax = 0)`,
				uid, tenantID, cid, name, yr, sem, cell(row, idx, "level")).Scan(&inserted)
			if err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d (%s): exists under another tenant or write failed", ln, uid))
				continue
			}
			if inserted {
				res.Inserted++
			} else {
				res.Updated++
			}
		}
		writeJSON(w, http.StatusOK, struct {
			importResult
			Transfers []unitTransfer `json:"transfers"`
		}{*res, transfers})
	}
}

func ExportCourseUnitsXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT unit_id, course_id, name, COALESCE(year,1), COALESCE(semester,1), COALESCE(level,'')
			FROM course_units WHERE tenant_id = $1 ORDER BY course_id, year, semester, name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		out := [][]string{{"unit_id", "course_id", "name", "year", "semester", "level"}}
		for rows.Next() {
			var uid, cid, name, level string
			var yr, sem int
			rows.Scan(&uid, &cid, &name, &yr, &sem, &level) //nolint:errcheck
			out = append(out, []string{uid, cid, name, strconv.Itoa(yr), strconv.Itoa(sem), level})
		}
		writeXLSX(w, "course-units.xlsx", out)
	}
}

// ─── Lecturer assignments (auto-creates lecturers) ───────────────────────────

func ImportLecturerAssignmentsFile(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		file, ok := uploadFile(w, r)
		if !ok {
			return
		}
		defer file.Close()
		rows, idx, err := readTabular(file)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", err.Error()))
			return
		}
		if _, ok := idx["unit_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("CSV_PARSE_ERROR", "missing required column: unit_id"))
			return
		}
		res := &importResult{Errors: []string{}}
		for ln := 1; ln < len(rows); ln++ {
			row := rows[ln]
			uid := cell(row, idx, "unit_id")
			staffID := cell(row, idx, "lecturer_staff_id")
			lname := cell(row, idx, "lecturer_name")
			if uid == "" || (staffID == "" && lname == "") {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: unit_id and a lecturer (staff_id or name) are required", ln))
				continue
			}
			// Resolve the unit (must exist for this tenant) → its course_id.
			var courseID string
			if err := adminPool.QueryRow(r.Context(),
				`SELECT course_id FROM course_units WHERE unit_id = $1 AND tenant_id = $2`, uid, tenantID).Scan(&courseID); err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: unit '%s' not found — import units first", ln, uid))
				continue
			}
			// Resolve or AUTO-CREATE the lecturer (by staff_id, else by name).
			lecturerID, lerr := resolveOrCreateLecturer(r.Context(), adminPool, tenantID, staffID, lname)
			if lerr != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", ln, lerr.Error()))
				continue
			}
			ay := cell(row, idx, "academic_year")
			if ay == "" {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: academic_year is required", ln))
				continue
			}
			intake := cell(row, idx, "intake_session") // study session; '' is a stable key value
			yr, _ := strconv.Atoi(cell(row, idx, "year"))
			sem, _ := strconv.Atoi(cell(row, idx, "semester"))
			ct, err := adminPool.Exec(r.Context(), `
				INSERT INTO lecturer_assignments
				    (tenant_id, lecturer_id, unit_id, course_id, academic_year, intake_session, year, semester)
				VALUES ($1,$2,$3,$4,$5,$6,NULLIF($7,0),NULLIF($8,0))
				ON CONFLICT (lecturer_id, unit_id, academic_year, intake_session) DO NOTHING`,
				tenantID, lecturerID, uid, courseID, ay, intake, yr, sem)
			if err != nil {
				res.Skipped++
				res.Errors = append(res.Errors, fmt.Sprintf("line %d: %s", ln, err.Error()))
				continue
			}
			if ct.RowsAffected() > 0 {
				res.Inserted++
			} else {
				res.Updated++ // already mapped — idempotent
			}
		}
		writeJSON(w, http.StatusOK, res)
	}
}

// resolveOrCreateLecturer finds a lecturer by staff_id (preferred) or name, creating
// one if none exists — so imported units arrive with their dedicated lecturers.
func resolveOrCreateLecturer(ctx context.Context, pool *pgxpool.Pool, tenantID, staffID, name string) (string, error) {
	if staffID != "" {
		var id string
		err := pool.QueryRow(ctx, `
			INSERT INTO lecturers (tenant_id, full_name, staff_id)
			VALUES ($1, NULLIF($2,''), $3)
			ON CONFLICT (tenant_id, staff_id) WHERE staff_id IS NOT NULL AND staff_id <> ''
			DO UPDATE SET full_name = COALESCE(NULLIF(EXCLUDED.full_name,''), lecturers.full_name)
			RETURNING lecturer_id::text`,
			tenantID, name, staffID).Scan(&id)
		if err != nil {
			return "", fmt.Errorf("lecturer upsert by staff_id failed: %w", err)
		}
		return id, nil
	}
	// No staff_id → match by name, else insert.
	var id string
	err := pool.QueryRow(ctx,
		`SELECT lecturer_id::text FROM lecturers WHERE tenant_id = $1 AND lower(full_name) = lower($2) LIMIT 1`,
		tenantID, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO lecturers (tenant_id, full_name) VALUES ($1,$2) RETURNING lecturer_id::text`,
		tenantID, name).Scan(&id); err != nil {
		return "", fmt.Errorf("lecturer create failed: %w", err)
	}
	return id, nil
}

func ExportLecturerAssignmentsXLSX(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT la.unit_id, COALESCE(l.staff_id,''), COALESCE(l.full_name,''),
			       COALESCE(la.academic_year,''), COALESCE(la.intake_session,''),
			       COALESCE(la.year,0), COALESCE(la.semester,0)
			FROM lecturer_assignments la
			JOIN lecturers l ON l.lecturer_id = la.lecturer_id
			WHERE la.tenant_id = $1 ORDER BY la.unit_id`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		out := [][]string{{"unit_id", "lecturer_staff_id", "lecturer_name", "academic_year", "intake_session", "year", "semester"}}
		for rows.Next() {
			var uid, sid, name, ay, intake string
			var yr, sem int
			rows.Scan(&uid, &sid, &name, &ay, &intake, &yr, &sem) //nolint:errcheck
			yrs, sems := "", ""
			if yr > 0 {
				yrs = strconv.Itoa(yr)
			}
			if sem > 0 {
				sems = strconv.Itoa(sem)
			}
			out = append(out, []string{uid, sid, name, ay, intake, yrs, sems})
		}
		writeXLSX(w, "lecturer-assignments.xlsx", out)
	}
}
