package handlers

// Student attendance for the QA Officer dashboard (also viewable by VC / DQA):
// a per-student SUMMARY of progress with filters by course, unit, session,
// semester and year, plus Excel export and import (manual attendance backfill).

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type studentAttendanceRow struct {
	StudentID  string  `json:"student_id"`
	FullName   string  `json:"full_name"`
	Course     string  `json:"course"`
	Level      string  `json:"level"`
	Session    string  `json:"session"`
	Year       int     `json:"year"`
	Semester   int     `json:"semester"`
	Held       int     `json:"sessions_held"`
	Attended   int     `json:"sessions_attended"`
	Percentage float64 `json:"attendance_percentage"`
}

// queryStudentAttendance aggregates held vs attended sessions per student, scoped
// to the caller's tenant + the optional filters.
func queryStudentAttendance(ctx context.Context, pool *pgxpool.Pool, tenantID string, q map[string]string) ([]studentAttendanceRow, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(ctx, conn, tenantID); err != nil {
		return nil, err
	}

	// Dynamic, parameterised filters.
	args := []interface{}{tenantID}
	conds := []string{}
	add := func(clause string, val string) {
		if val == "" {
			return
		}
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(clause, len(args)))
	}
	add("se.course_id = $%d", q["course_id"])
	add("o.session_type = $%d", q["session"])
	if v, err := strconv.Atoi(q["year"]); err == nil && v > 0 {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf("se.current_year = $%d", len(args)))
	}
	if v, err := strconv.Atoi(q["semester"]); err == nil && (v == 1 || v == 2) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf("se.semester = $%d", len(args)))
	}
	add("se.academic_year = $%d", q["academic_year"])
	where := ""
	if len(conds) > 0 {
		where = " AND " + strings.Join(conds, " AND ")
	}

	// unit filter applies to the session-join scope, not the student row.
	unitArg := ""
	if q["unit_id"] != "" {
		args = append(args, q["unit_id"])
		unitArg = fmt.Sprintf(" AND cu.unit_id = $%d", len(args))
	}

	sql := `
		SELECT se.student_id, se.full_name,
		       COALESCE(c.name, ''), COALESCE(c.level,''),
		       COALESCE(o.session_type,''), COALESCE(se.current_year,0), COALESCE(se.semester,0),
		       COUNT(DISTINCT s.session_id) AS held,
		       COUNT(DISTINCT al.session_id) AS attended
		FROM students_extended se
		JOIN courses c ON c.course_id = se.course_id AND c.tenant_id = se.tenant_id
		LEFT JOIN course_offerings o ON o.offering_id = se.offering_id
		LEFT JOIN course_units cu ON cu.course_id = se.course_id AND cu.tenant_id = se.tenant_id` + unitArg + `
		LEFT JOIN sessions s ON s.unit_id = cu.unit_id AND s.tenant_id = se.tenant_id
		     -- COHORT ISOLATION: only count the sessions of the student's OWN study
		     -- session (Day / Evening / Weekend…). Sharing a course is not enough — a
		     -- Weekend student must never be measured against Day sessions. Sessions
		     -- predating the cohort model (offering_id NULL) fall back to the student's
		     -- own offering being unset, so neither side guesses.
		     AND s.offering_id IS NOT DISTINCT FROM se.offering_id
		LEFT JOIN attendance_logs al ON al.session_id = s.session_id AND al.student_id = se.student_id
		WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'` + where + `
		GROUP BY se.student_id, se.full_name, c.name, c.level, o.session_type, se.current_year, se.semester
		ORDER BY se.full_name`

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []studentAttendanceRow{}
	for rows.Next() {
		var r studentAttendanceRow
		rows.Scan(&r.StudentID, &r.FullName, &r.Course, &r.Level, &r.Session, &r.Year, &r.Semester, &r.Held, &r.Attended) //nolint:errcheck
		if r.Held > 0 {
			r.Percentage = float64(int((float64(r.Attended)/float64(r.Held))*1000+0.5)) / 10
		}
		out = append(out, r)
	}
	return out, nil
}

func qaFilters(r *http.Request) map[string]string {
	q := r.URL.Query()
	return map[string]string{
		"course_id": q.Get("course_id"), "unit_id": q.Get("unit_id"), "session": q.Get("session"),
		"year": q.Get("year"), "semester": q.Get("semester"), "academic_year": q.Get("academic_year"),
	}
}

// GET /api/v1/dashboard/qa/student-attendance
func QAStudentAttendance(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := queryStudentAttendance(r.Context(), pool, middleware.GetTenantID(r.Context()), qaFilters(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// GET /api/v1/dashboard/qa/student-attendance/export.xlsx
func QAStudentAttendanceExport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := queryStudentAttendance(r.Context(), pool, middleware.GetTenantID(r.Context()), qaFilters(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		out := [][]string{{"reg_no", "full_name", "course", "level", "session", "year", "semester", "sessions_held", "sessions_attended", "attendance_percentage"}}
		for _, s := range list {
			out = append(out, []string{s.StudentID, s.FullName, s.Course, s.Level, s.Session,
				itoa(s.Year), itoa(s.Semester), fmt.Sprintf("%d", s.Held), fmt.Sprintf("%d", s.Attended), fmt.Sprintf("%.1f", s.Percentage)})
		}
		xlsx, err := buildXLSX(out)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="student-attendance.xlsx"`)
		_, _ = w.Write(xlsx)
	}
}

// POST /api/v1/dashboard/qa/student-attendance/import — multipart "roster" (CSV or
// XLSX) of attendance records: columns session_id, student_id. Inserts a manual
// attendance entry per row (idempotent; existing check-ins are left untouched).
func QAStudentAttendanceImport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		if err := r.ParseMultipartForm(16 << 20); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, _, err := r.FormFile("roster")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'roster' not found"))
			return
		}
		defer file.Close()
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(file)

		var rows [][]string
		if looksXLSX(buf.Bytes()) {
			rows, err = parseXLSX(buf.Bytes())
		} else {
			cr := csv.NewReader(bytes.NewReader(buf.Bytes()))
			cr.TrimLeadingSpace = true
			cr.FieldsPerRecord = -1
			rows, err = cr.ReadAll()
		}
		if err != nil || len(rows) == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", "could not read the file"))
			return
		}
		col := map[string]int{}
		for i, h := range rows[0] {
			col[strings.ToLower(strings.TrimSpace(h))] = i
		}
		if _, ok := col["session_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", "missing required column: session_id"))
			return
		}
		if _, ok := col["student_id"]; !ok {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", "missing required column: student_id"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		middleware.SetTenantConn(r.Context(), conn, tenantID) //nolint:errcheck

		get := func(row []string, name string) string {
			if i, ok := col[name]; ok && i < len(row) {
				return strings.TrimSpace(row[i])
			}
			return ""
		}
		res := struct {
			Inserted int      `json:"inserted"`
			Skipped  int      `json:"skipped"`
			Errors   []string `json:"errors"`
		}{Errors: []string{}}

		for ln := 1; ln < len(rows); ln++ {
			sid := get(rows[ln], "session_id")
			stu := get(rows[ln], "student_id")
			if sid == "" || stu == "" {
				res.Skipped++
				continue
			}
			ct, e := conn.Exec(r.Context(), `
				INSERT INTO attendance_logs (tenant_id, session_id, student_id, checkin_timestamp, entry_method, sequence_number)
				SELECT $1, $2::uuid, $3, now(), 'MANUAL_OVERRIDE',
				       COALESCE((SELECT MAX(sequence_number) FROM attendance_logs WHERE session_id = $2::uuid), 0) + 1
				WHERE NOT EXISTS (
				  SELECT 1 FROM attendance_logs WHERE session_id = $2::uuid AND student_id = $3)`,
				tenantID, sid, stu)
			if e != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("row %d: %s", ln+1, e.Error()))
				res.Skipped++
				continue
			}
			if ct.RowsAffected() > 0 {
				res.Inserted++
			} else {
				res.Skipped++
			}
		}
		writeJSON(w, http.StatusOK, res)
	}
}
