package handlers

// QA representative subsystem (Phase 4). Two org-scoped QA roles sit below the DQA:
//
//   QA_DEPT_REP        → one department (users.department)
//   QA_SCHOOL_HANDLER  → one school/college (users.school)
//
// Both walk their unit, fill the monitoring workbook they already use on paper, and upload it.
// The recognised rows are parsed into `lecturer_patrol_logs` with entry_method='QA_REP_UPLOAD' so
// they land in the same reports as the patroller app's observations, and the original workbook is
// kept verbatim as the evidence behind the submission.
//
//   GET    /api/v1/qa-rep/scope
//   GET    /api/v1/qa-rep/lecturers                  (see hod_dean.go — same org-scoped query)
//   GET    /api/v1/qa-rep/departments                (per-department roll-up within the scope)
//   GET    /api/v1/qa-rep/submissions
//   POST   /api/v1/qa-rep/submissions                (multipart: report=<file>, + period/notes)
//   GET    /api/v1/qa-rep/submissions/{id}/file
//   DELETE /api/v1/qa-rep/submissions/{id}
//   GET    /api/v1/qa-rep/template.xlsx

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// qaScope is the org unit a QA rep may see and write observations for.
type qaScope struct {
	Role       string `json:"role"`
	ScopeKind  string `json:"scope_kind"` // DEPARTMENT | SCHOOL | ALL
	Department string `json:"department"`
	School     string `json:"school"`
	Name       string `json:"full_name"`
	StaffID    string `json:"staff_id"`
	// Unscoped is true for the oversight roles (DQA/QA officer/VC/admin) that read every
	// submission rather than one department's.
	Unscoped bool `json:"unscoped"`
}

// Label is what the dashboard shows as "you are looking at …".
func (s qaScope) Label() string {
	switch {
	case s.Unscoped:
		return "the whole institution"
	case s.ScopeKind == "SCHOOL" && s.School != "":
		return s.School
	case s.Department != "":
		return s.Department
	}
	return ""
}

// resolveQAScope reads the caller's org unit off their user row. The role decides which of the two
// columns scopes them: a school handler is bounded by users.school, a dept rep by users.department.
func resolveQAScope(ctx context.Context, conn qaRowQuerier, userID, role string) (qaScope, error) {
	s := qaScope{Role: role}
	err := conn.QueryRow(ctx,
		`SELECT COALESCE(department,''), COALESCE(school,''), COALESCE(full_name,''), COALESCE(staff_id,'')
		 FROM users WHERE user_id = $1::uuid`, userID).
		Scan(&s.Department, &s.School, &s.Name, &s.StaffID)
	if err != nil {
		return s, err
	}
	switch role {
	case middleware.RoleQASchool, middleware.RoleDean:
		s.ScopeKind = "SCHOOL"
	case middleware.RoleQADeptRep, middleware.RoleHOD:
		s.ScopeKind = "DEPARTMENT"
	default:
		s.ScopeKind = "ALL"
		s.Unscoped = true
	}
	return s, nil
}

// qaRowQuerier is the sliver of pgx that both a pool and a pooled connection satisfy.
type qaRowQuerier interface {
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// withQAConn acquires a tenant-scoped connection and hands it, plus the caller's resolved scope,
// to fn. Every QA-rep handler starts this way, so the scope can never be taken from the request.
func withQAConn(pool *pgxpool.Pool, w http.ResponseWriter, r *http.Request,
	fn func(conn *pgxpool.Conn, tenantID string, scope qaScope)) {
	tenantID := middleware.GetTenantID(r.Context())
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
	scope, err := resolveQAScope(r.Context(), conn, middleware.GetUserID(r.Context()), middleware.GetRole(r.Context()))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not resolve your org unit"))
		return
	}
	fn(conn, tenantID, scope)
}

// scopeSQL returns a WHERE fragment + arg confining a query to the caller's org unit. `col` names
// the department column and `schoolCol` the school column of the table being filtered.
func (s qaScope) scopeSQL(deptCol, schoolCol string, argN int) (string, string, bool) {
	switch {
	case s.Unscoped:
		return "", "", false
	case s.ScopeKind == "SCHOOL" && s.School != "":
		return fmt.Sprintf(" AND btrim(lower(%s)) = btrim(lower($%d))", schoolCol, argN), s.School, true
	case s.ScopeKind == "DEPARTMENT" && s.Department != "":
		return fmt.Sprintf(" AND btrim(lower(%s)) = btrim(lower($%d))", deptCol, argN), s.Department, true
	}
	// Scoped role with no org unit set → match nothing rather than everything.
	return " AND false", "", false
}

// noScopeMessage explains an empty dashboard caused by an unset department/school.
func (s qaScope) noScopeMessage() string {
	if s.Unscoped {
		return ""
	}
	if s.ScopeKind == "SCHOOL" && s.School == "" {
		return "No school/college is set on your account — ask an admin to set it before you can file reports."
	}
	if s.ScopeKind == "DEPARTMENT" && s.Department == "" {
		return "No department is set on your account — ask an admin to set it before you can file reports."
	}
	return ""
}

// ─── Scope ───────────────────────────────────────────────────────────────────

// QARepScope — who am I and what do I cover? Drives the dashboard header.
func QARepScope(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			var submissions int
			_ = conn.QueryRow(r.Context(),
				`SELECT COUNT(*) FROM qa_rep_submissions WHERE tenant_id = $1 AND submitted_by = $2::uuid`,
				tenantID, middleware.GetUserID(r.Context())).Scan(&submissions)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"scope":            scope,
				"label":            scope.Label(),
				"my_submissions":   submissions,
				"message":          scope.noScopeMessage(),
				"can_submit":       !scope.Unscoped && scope.noScopeMessage() == "",
				"observation_kind": "QA_REP_UPLOAD",
			})
		})
	}
}

// ─── Per-department roll-up ──────────────────────────────────────────────────

// QARepDepartments — one row per department inside the caller's scope: how many lecturers, how many
// were observed teaching, and when that department last filed a report. A school handler sees every
// department in their school; a dept rep sees only their own.
func QARepDepartments(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			clause, val, hasVal := scope.scopeSQL("c.department", "c.school", 2)
			args := []interface{}{tenantID}
			if hasVal {
				args = append(args, val)
			}
			rows, err := conn.Query(r.Context(), `
				SELECT COALESCE(NULLIF(btrim(c.department),''),'(unassigned)') AS dept,
				       COALESCE(MAX(c.school),''),
				       COUNT(DISTINCT la.lecturer_id)                       AS lecturers,
				       COUNT(p.patrol_id)                                   AS patrolled,
				       COUNT(p.patrol_id) FILTER (WHERE p.taught)           AS taught
				FROM courses c
				JOIN course_units cu ON cu.course_id = c.course_id AND cu.tenant_id = c.tenant_id
				LEFT JOIN lecturer_assignments la ON la.unit_id = cu.unit_id AND la.tenant_id = cu.tenant_id
				LEFT JOIN lecturer_patrol_logs p  ON p.unit_id  = cu.unit_id AND p.tenant_id  = cu.tenant_id
				WHERE c.tenant_id = $1`+clause+`
				GROUP BY dept
				ORDER BY dept`, args...)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
			defer rows.Close()

			type deptRow struct {
				Department string `json:"department"`
				School     string `json:"school"`
				Lecturers  int    `json:"lecturers"`
				Patrolled  int    `json:"patrolled"`
				Taught     int    `json:"taught"`
				LastReport string `json:"last_report,omitempty"`
			}
			out := []deptRow{}
			for rows.Next() {
				var d deptRow
				if rows.Scan(&d.Department, &d.School, &d.Lecturers, &d.Patrolled, &d.Taught) == nil {
					out = append(out, d)
				}
			}
			rows.Close()

			// Last submission per department, so the handler can see who has not filed yet.
			last := map[string]string{}
			lr, err := conn.Query(r.Context(), `
				SELECT COALESCE(btrim(department),''), to_char(MAX(created_at),'YYYY-MM-DD')
				FROM qa_rep_submissions WHERE tenant_id = $1 GROUP BY 1`, tenantID)
			if err == nil {
				for lr.Next() {
					var d, t string
					if lr.Scan(&d, &t) == nil {
						last[strings.ToLower(d)] = t
					}
				}
				lr.Close()
			}
			for i := range out {
				out[i].LastReport = last[strings.ToLower(out[i].Department)]
			}

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"scope":       scope,
				"label":       scope.Label(),
				"message":     scope.noScopeMessage(),
				"departments": out,
			})
		})
	}
}

// ─── Submissions ─────────────────────────────────────────────────────────────

type qaSubmission struct {
	SubmissionID  string   `json:"submission_id"`
	SubmitterName string   `json:"submitter_name"`
	SubmitterRole string   `json:"submitter_role"`
	ScopeKind     string   `json:"scope_kind"`
	Department    string   `json:"department"`
	School        string   `json:"school"`
	PeriodLabel   string   `json:"period_label"`
	PeriodFrom    string   `json:"period_from,omitempty"`
	PeriodTo      string   `json:"period_to,omitempty"`
	Notes         string   `json:"notes"`
	FileName      string   `json:"file_name"`
	FileSize      int      `json:"file_size"`
	TotalRows     int      `json:"total_rows"`
	ParsedRows    int      `json:"parsed_rows"`
	SkippedRows   int      `json:"skipped_rows"`
	ParseErrors   []string `json:"parse_errors"`
	CreatedAt     string   `json:"created_at"`
	Mine          bool     `json:"mine"`
}

// QAListSubmissions — the submissions visible to the caller. A dept rep sees their department's, a
// school handler their whole school's, and the oversight roles (DQA/QA officer/VC/admin) see all.
func QAListSubmissions(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			clause, val, hasVal := scope.scopeSQL("s.department", "s.school", 2)
			args := []interface{}{tenantID}
			if hasVal {
				args = append(args, val)
			}
			rows, err := conn.Query(r.Context(), `
				SELECT s.submission_id::text, COALESCE(s.submitter_name,''), s.submitter_role,
				       s.scope_kind, COALESCE(s.department,''), COALESCE(s.school,''),
				       COALESCE(s.period_label,''),
				       COALESCE(to_char(s.period_from,'YYYY-MM-DD'),''),
				       COALESCE(to_char(s.period_to,'YYYY-MM-DD'),''),
				       COALESCE(s.notes,''), s.file_name, s.file_size,
				       s.total_rows, s.parsed_rows, s.skipped_rows, s.parse_errors,
				       to_char(s.created_at,'YYYY-MM-DD HH24:MI'),
				       (s.submitted_by = $`+strconv.Itoa(len(args)+1)+`::uuid)
				FROM qa_rep_submissions s
				WHERE s.tenant_id = $1`+clause+`
				ORDER BY s.created_at DESC
				LIMIT 200`, append(args, middleware.GetUserID(r.Context()))...)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
			defer rows.Close()
			out := []qaSubmission{}
			for rows.Next() {
				var s qaSubmission
				if rows.Scan(&s.SubmissionID, &s.SubmitterName, &s.SubmitterRole, &s.ScopeKind,
					&s.Department, &s.School, &s.PeriodLabel, &s.PeriodFrom, &s.PeriodTo, &s.Notes,
					&s.FileName, &s.FileSize, &s.TotalRows, &s.ParsedRows, &s.SkippedRows,
					&s.ParseErrors, &s.CreatedAt, &s.Mine) == nil {
					out = append(out, s)
				}
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"scope":       scope,
				"label":       scope.Label(),
				"message":     scope.noScopeMessage(),
				"submissions": out,
			})
		})
	}
}

// QASubmissionFile streams the original workbook back, unchanged.
func QASubmissionFile(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			clause, val, hasVal := scope.scopeSQL("s.department", "s.school", 3)
			args := []interface{}{id, tenantID}
			if hasVal {
				args = append(args, val)
			}
			var name string
			var data []byte
			err := conn.QueryRow(r.Context(), `
				SELECT s.file_name, s.file_bytes FROM qa_rep_submissions s
				WHERE s.submission_id = $1::uuid AND s.tenant_id = $2`+clause, args...).Scan(&name, &data)
			if err != nil {
				writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "submission not found"))
				return
			}
			ct := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
			if !looksXLSX(data) {
				ct = "text/csv"
			}
			w.Header().Set("Content-Type", ct)
			w.Header().Set("Content-Disposition", `attachment; filename="`+sanitizeFilename(name)+`"`)
			_, _ = w.Write(data)
		})
	}
}

// QADeleteSubmission removes a submission the caller filed (an oversight role may remove any).
// The observations parsed out of it go with it — the FK cascades.
func QADeleteSubmission(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			q := `DELETE FROM qa_rep_submissions WHERE submission_id = $1::uuid AND tenant_id = $2`
			args := []interface{}{id, tenantID}
			if !scope.Unscoped { // a rep may only withdraw their own file
				q += ` AND submitted_by = $3::uuid`
				args = append(args, middleware.GetUserID(r.Context()))
			}
			tag, err := conn.Exec(r.Context(), q, args...)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
			if tag.RowsAffected() == 0 {
				writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "submission not found, or not yours to withdraw"))
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "DELETED"})
		})
	}
}

// ─── Template ────────────────────────────────────────────────────────────────

// qaTemplateHeader is the column contract for the upload. Everything after `taught` is optional;
// blanks are filled in from the timetable where the system already knows the answer.
var qaTemplateHeader = []string{
	"unit_id", "unit_name", "course_code", "lecturer_staff_id", "lecturer_name",
	"room", "date", "time", "taught", "remarks",
}

// QARepTemplate hands back a pre-filled workbook: the header row, a worked example, and — when the
// rep has a scope — one row per timetabled session in their unit for the coming week, so filling it
// in is a matter of typing YES/NO down the `taught` column.
func QARepTemplate(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			out := [][]string{qaTemplateHeader, {
				"CS201", "Data Communication", "BSC-CS", "KIU/044", "Dr. A. Mwangi",
				"LR-101", time.Now().Format("2006-01-02"), "08:00", "YES", "example row — delete me",
			}}

			clause, val, hasVal := scope.scopeSQL("c.department", "c.school", 2)
			args := []interface{}{tenantID}
			if hasVal {
				args = append(args, val)
			}
			rows, err := conn.Query(r.Context(), `
				SELECT ts.unit_id, COALESCE(cu.name,''), COALESCE(cu.course_id,''),
				       COALESCE(lec.staff_id,''), COALESCE(lec.full_name,''),
				       COALESCE(NULLIF(ts.room,''), COALESCE(v.venue_id,'')),
				       ts.day_of_week, to_char(ts.start_time,'HH24:MI')
				FROM timetable_slots ts
				JOIN course_units cu ON cu.unit_id = ts.unit_id AND cu.tenant_id = ts.tenant_id
				JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
				LEFT JOIN venues v ON v.venue_id = ts.venue_id
				LEFT JOIN LATERAL (
				    SELECT l.staff_id, l.full_name FROM lecturers l
				    WHERE l.tenant_id = ts.tenant_id
				      AND ( l.lecturer_id = ts.lecturer_id
				         OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
				               SELECT la.lecturer_id FROM lecturer_assignments la
				               WHERE la.unit_id = ts.unit_id AND la.tenant_id = ts.tenant_id
				               ORDER BY la.academic_year DESC LIMIT 1) ) )
				    LIMIT 1
				) lec ON true
				WHERE ts.tenant_id = $1`+clause+`
				ORDER BY ts.day_of_week, ts.start_time`, args...)
			if err == nil {
				defer rows.Close()
				monday := startOfWeek(time.Now())
				for rows.Next() {
					var unitID, unitName, courseCode, staffID, lecName, room, start string
					var dow int
					if rows.Scan(&unitID, &unitName, &courseCode, &staffID, &lecName, &room, &dow, &start) != nil {
						continue
					}
					date := monday.AddDate(0, 0, dow-1).Format("2006-01-02")
					out = append(out, []string{unitID, unitName, courseCode, staffID, lecName, room, date, start, "", ""})
				}
			}

			xl, err := buildXLSX(out)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Header().Set("Content-Disposition", `attachment; filename="qa-monitoring-template.xlsx"`)
			_, _ = w.Write(xl)
		})
	}
}

// startOfWeek returns the Monday of t's week.
func startOfWeek(t time.Time) time.Time {
	off := (int(t.Weekday()) + 6) % 7 // Mon=0 … Sun=6
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, -off)
}

// ─── Upload ──────────────────────────────────────────────────────────────────

// qaMaxUpload caps a stored workbook. Real monitoring sheets are a few hundred KB; the ceiling is
// what keeps a stray 40 MB file out of the row.
const qaMaxUpload = 8 << 20 // 8 MiB

// QASubmitReport ingests one monitoring workbook: parse the recognised rows into observations, then
// store the file itself against the submission.
//
// POST /api/v1/qa-rep/submissions  (multipart: report=<file>, period_label, period_from, period_to, notes)
func QASubmitReport(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(qaMaxUpload + (1 << 20)); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "expected multipart/form-data"))
			return
		}
		file, hdr, err := r.FormFile("report")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "field 'report' (the workbook) not found"))
			return
		}
		defer file.Close()
		data, err := io.ReadAll(io.LimitReader(file, qaMaxUpload+1))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "could not read the uploaded file"))
			return
		}
		if len(data) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "the uploaded file is empty"))
			return
		}
		if len(data) > qaMaxUpload {
			writeJSON(w, http.StatusRequestEntityTooLarge, errBody("FILE_TOO_LARGE", "the workbook must be under 8 MB"))
			return
		}

		withQAConn(pool, w, r, func(conn *pgxpool.Conn, tenantID string, scope qaScope) {
			if scope.Unscoped {
				writeJSON(w, http.StatusForbidden, errBody("FORBIDDEN", "only a QA department rep or school handler files reports"))
				return
			}
			if msg := scope.noScopeMessage(); msg != "" {
				writeJSON(w, http.StatusPreconditionFailed, errBody("NO_SCOPE", msg))
				return
			}

			rows, perr := readQAWorkbook(data)
			if perr != nil {
				writeJSON(w, http.StatusUnprocessableEntity, errBody("PARSE_ERROR", perr.Error()))
				return
			}

			userID := middleware.GetUserID(r.Context())
			fileName := sanitizeFilename(hdr.Filename)
			if fileName == "" {
				fileName = "qa-report.xlsx"
			}

			// The submission row is written first so the parsed observations can point at it.
			var subID string
			err := conn.QueryRow(r.Context(), `
				INSERT INTO qa_rep_submissions
				    (tenant_id, submitted_by, submitter_name, submitter_role, scope_kind,
				     department, school, period_label, period_from, period_to, notes,
				     file_name, file_size, file_bytes)
				VALUES ($1, $2::uuid, $3, $4, $5, NULLIF($6,''), NULLIF($7,''), NULLIF($8,''),
				        NULLIF($9,'')::date, NULLIF($10,'')::date, NULLIF($11,''), $12, $13, $14)
				RETURNING submission_id::text`,
				tenantID, userID, scope.Name, scope.Role, scope.ScopeKind,
				scope.Department, scope.School,
				strings.TrimSpace(r.FormValue("period_label")),
				strings.TrimSpace(r.FormValue("period_from")),
				strings.TrimSpace(r.FormValue("period_to")),
				strings.TrimSpace(r.FormValue("notes")),
				fileName, len(data), data).Scan(&subID)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not save the submission: "+err.Error()))
				return
			}

			res := ingestQAObservations(r.Context(), conn, tenantID, subID, userID, scope, rows)

			_, _ = conn.Exec(r.Context(), `
				UPDATE qa_rep_submissions
				   SET total_rows = $2, parsed_rows = $3, skipped_rows = $4, parse_errors = $5
				 WHERE submission_id = $1::uuid`,
				subID, res.Total, res.Recorded+res.Updated, res.Skipped, res.Errors)

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"submission_id": subID,
				"file_name":     fileName,
				"total_rows":    res.Total,
				"recorded":      res.Recorded,
				"updated":       res.Updated,
				"skipped":       res.Skipped,
				"errors":        res.Errors,
				"message":       res.summary(),
			})
		})
	}
}

// qaMaxReportedErrors caps the per-row complaints kept from one upload. A workbook with the wrong
// columns produces one error per row, and neither the stored array nor the response should grow
// with the file — the first couple of hundred already tell the rep what is wrong.
const qaMaxReportedErrors = 200

// qaIngestResult counts what became of each data row in the workbook.
type qaIngestResult struct {
	Total    int
	Recorded int // new observations
	Updated  int // corrections to this rep's earlier upload
	Skipped  int
	Errors   []string
	// suppressed counts the complaints dropped once the cap was reached, so the rep is told the
	// list is partial rather than being left to assume the rest of the file was fine.
	suppressed int
}

// note records one per-row complaint, up to the cap.
func (r *qaIngestResult) note(format string, args ...interface{}) {
	if len(r.Errors) >= qaMaxReportedErrors {
		r.suppressed++
		return
	}
	r.Errors = append(r.Errors, fmt.Sprintf(format, args...))
}

// sealErrors appends the "and N more" line once every row has been seen.
func (r *qaIngestResult) sealErrors() {
	if r.suppressed > 0 {
		r.Errors = append(r.Errors, fmt.Sprintf("…and %d more row(s) with problems, not listed.", r.suppressed))
	}
}

func (r qaIngestResult) summary() string {
	switch {
	case r.Total == 0:
		return "The workbook had no data rows below the header."
	case r.Recorded+r.Updated == 0:
		return fmt.Sprintf("None of the %d row(s) could be recorded — see the details below. The file itself was kept.", r.Total)
	case r.Skipped == 0:
		return fmt.Sprintf("Recorded %d observation(s) from %d row(s).", r.Recorded+r.Updated, r.Total)
	}
	return fmt.Sprintf("Recorded %d of %d row(s); %d skipped.", r.Recorded+r.Updated, r.Total, r.Skipped)
}

// qaUnit is what the system already knows about a unit — used to fill blanks in the workbook and,
// more importantly, to prove the row belongs to the rep's own department/school.
type qaUnit struct {
	Name, CourseCode, Department, School string
	StaffID, LecturerName                string
}

// ingestQAObservations turns parsed spreadsheet rows into `lecturer_patrol_logs` entries.
//
// Two rules protect the ledger: a row for a unit outside the rep's org unit is refused, and an
// existing observation is only overwritten when it too came from a workbook — a patroller's live
// field observation always wins over a spreadsheet filled in afterwards.
func ingestQAObservations(ctx context.Context, conn *pgxpool.Conn, tenantID, submissionID, userID string,
	scope qaScope, rows [][]string) qaIngestResult {

	res := qaIngestResult{Errors: []string{}}
	if len(rows) == 0 {
		return res
	}

	col := map[string]int{}
	for i, h := range rows[0] {
		col[normalizeHeader(h)] = i
	}
	if _, ok := col["unit_id"]; !ok {
		res.note("missing required column 'unit_id' in the header row — download the template and use its columns")
		return res
	}

	units := loadQAUnits(ctx, conn, tenantID)

	for ln := 1; ln < len(rows); ln++ {
		row := rows[ln]
		get := func(c string) string {
			i, ok := col[c]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		unitID := get("unit_id")
		taughtRaw := get("taught")
		// A wholly blank line (Excel loves trailing ones) is not an error, just not a row.
		if unitID == "" && taughtRaw == "" && get("date") == "" {
			continue
		}
		res.Total++

		if unitID == "" {
			res.Skipped++
			res.note("row %d: unit_id is blank", ln+1)
			continue
		}
		u, known := units[strings.ToLower(unitID)]
		if !known {
			res.Skipped++
			res.note("row %d: unit %q is not a unit in this institution", ln+1, unitID)
			continue
		}
		// Scope guard — a dept rep files for their department only, a handler for their school.
		if scope.ScopeKind == "DEPARTMENT" && !strings.EqualFold(strings.TrimSpace(u.Department), strings.TrimSpace(scope.Department)) {
			res.Skipped++
			res.note("row %d: unit %s belongs to %s, not your department", ln+1, unitID, orDash(u.Department))
			continue
		}
		if scope.ScopeKind == "SCHOOL" && !strings.EqualFold(strings.TrimSpace(u.School), strings.TrimSpace(scope.School)) {
			res.Skipped++
			res.note("row %d: unit %s belongs to %s, not your school", ln+1, unitID, orDash(u.School))
			continue
		}

		taught, ok := parseTaught(taughtRaw)
		if !ok {
			res.Skipped++
			res.note("row %d: 'taught' must say YES or NO (got %q)", ln+1, taughtRaw)
			continue
		}
		date, ok := parseSheetDate(get("date"))
		if !ok {
			res.Skipped++
			res.note("row %d: 'date' must be a date like %s (got %q)", ln+1, time.Now().Format("2006-01-02"), get("date"))
			continue
		}
		// The scheduled time completes the observation's identity (unit + date + time), so two
		// sittings of the same unit on one day stay distinct. Blank is allowed and means "the
		// only session that day".
		schedTime := parseSheetTime(get("time"))

		// Blanks fall back to what the timetable already knows.
		staffID := firstNonEmpty(get("lecturer_staff_id"), u.StaffID)
		lecName := firstNonEmpty(get("lecturer_name"), u.LecturerName)
		unitName := firstNonEmpty(get("unit_name"), u.Name)
		courseCode := firstNonEmpty(get("course_code"), u.CourseCode)

		var inserted bool
		err := conn.QueryRow(ctx, `
			INSERT INTO lecturer_patrol_logs
			    (tenant_id, unit_id, unit_name, course_code, lecturer_id, lecturer_name, room,
			     session_date, scheduled_time, taught, patroller_id, patroller_name,
			     patroller_staff_id, entry_method, remarks, submission_id)
			VALUES ($1,$2,NULLIF($3,''),NULLIF($4,''),NULLIF($5,''),NULLIF($6,''),NULLIF($7,''),
			        $8::date,$9,$10,$11::uuid,$12,NULLIF($13,''),'QA_REP_UPLOAD',NULLIF($14,''),$15::uuid)
			ON CONFLICT (tenant_id, unit_id, session_date, scheduled_time) DO UPDATE
			   SET taught        = EXCLUDED.taught,
			       lecturer_id   = COALESCE(EXCLUDED.lecturer_id, lecturer_patrol_logs.lecturer_id),
			       lecturer_name = COALESCE(EXCLUDED.lecturer_name, lecturer_patrol_logs.lecturer_name),
			       room          = COALESCE(EXCLUDED.room, lecturer_patrol_logs.room),
			       remarks       = EXCLUDED.remarks,
			       taken_at      = now(),
			       submission_id = EXCLUDED.submission_id
			 WHERE lecturer_patrol_logs.entry_method = 'QA_REP_UPLOAD'
			RETURNING (xmax = 0)`,
			tenantID, unitID, unitName, courseCode, staffID, lecName, get("room"),
			date.Format("2006-01-02"), schedTime, taught, userID, scope.Name, scope.StaffID,
			get("remarks"), submissionID).Scan(&inserted)

		switch {
		case err == pgx.ErrNoRows:
			// The DO UPDATE guard rejected it: a patroller already logged this slot in the field.
			res.Skipped++
			res.note(
				"row %d: %s on %s is already recorded by a QA patroller in the field — the field record stands",
				ln+1, unitID, date.Format("2006-01-02"))
		case err != nil:
			res.Skipped++
			res.note("row %d: %s", ln+1, err.Error())
		case inserted:
			res.Recorded++
		default:
			res.Updated++
		}
	}
	res.sealErrors()
	return res
}

// loadQAUnits indexes every unit in the tenant by lower-cased unit_id, with its owning
// department/school and its currently assigned lecturer.
func loadQAUnits(ctx context.Context, conn *pgxpool.Conn, tenantID string) map[string]qaUnit {
	out := map[string]qaUnit{}
	rows, err := conn.Query(ctx, `
		SELECT cu.unit_id, COALESCE(cu.name,''), COALESCE(cu.course_id,''),
		       COALESCE(c.department,''), COALESCE(c.school,''),
		       COALESCE(lec.staff_id,''), COALESCE(lec.full_name,'')
		FROM course_units cu
		JOIN courses c ON c.course_id = cu.course_id AND c.tenant_id = cu.tenant_id
		LEFT JOIN LATERAL (
		    SELECT l.staff_id, l.full_name
		    FROM lecturer_assignments la
		    JOIN lecturers l ON l.lecturer_id = la.lecturer_id AND l.tenant_id = la.tenant_id
		    WHERE la.unit_id = cu.unit_id AND la.tenant_id = cu.tenant_id
		    ORDER BY la.academic_year DESC LIMIT 1
		) lec ON true
		WHERE cu.tenant_id = $1`, tenantID)
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var u qaUnit
		if rows.Scan(&id, &u.Name, &u.CourseCode, &u.Department, &u.School, &u.StaffID, &u.LecturerName) == nil {
			out[strings.ToLower(id)] = u
		}
	}
	return out
}

// ─── Spreadsheet value parsing ───────────────────────────────────────────────

// readQAWorkbook accepts either an .xlsx workbook or a CSV, so a rep who exports to CSV is not
// stuck. Unlike the shared readTabular it works on bytes already in hand (they are also being
// stored verbatim) and leaves header indexing to normalizeHeader.
func readQAWorkbook(data []byte) ([][]string, error) {
	if looksXLSX(data) {
		rows, err := parseXLSX(data)
		if err != nil {
			return nil, fmt.Errorf("that file is not a readable Excel workbook: %w", err)
		}
		return rows, nil
	}
	cr := csv.NewReader(bytes.NewReader(data))
	cr.TrimLeadingSpace = true
	cr.FieldsPerRecord = -1
	rows, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("could not read that file as Excel or CSV: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("that file has no rows at all — it needs at least the header row from the template")
	}
	return rows, nil
}

// normalizeHeader makes "Lecturer Staff ID", "lecturer-staff-id" and "lecturer_staff_id" the same
// column, and maps the handful of names people actually type onto the canonical ones.
func normalizeHeader(h string) string {
	s := strings.ToLower(strings.TrimSpace(h))
	s = strings.NewReplacer(" ", "_", "-", "_", ".", "", "/", "_").Replace(s)
	for strings.Contains(s, "__") {
		s = strings.ReplaceAll(s, "__", "_")
	}
	switch s {
	case "unit", "unit_code", "code":
		return "unit_id"
	case "unit_title", "subject":
		return "unit_name"
	case "course", "course_id":
		return "course_code"
	case "staff_id", "lecturer_id", "lecturer_staff_no", "staff_no":
		return "lecturer_staff_id"
	case "lecturer", "lecturer_names":
		return "lecturer_name"
	case "venue", "room_code", "room_no":
		return "room"
	case "session_date", "day_date":
		return "date"
	case "start_time", "scheduled_time", "session_time":
		return "time"
	case "taught?", "was_taught", "teaching", "status", "attended":
		return "taught"
	case "remark", "comment", "comments", "notes":
		return "remarks"
	}
	return s
}

// parseTaught reads the many ways a person writes yes and no.
func parseTaught(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "yes", "y", "true", "t", "1", "taught", "present", "attended", "✓", "x":
		return true, true
	case "no", "n", "false", "f", "0", "not taught", "not_taught", "absent", "missed", "no show", "no_show":
		return false, true
	}
	return false, false
}

// excelEpoch is 1899-12-30: Excel's day 1 is 1900-01-01 and it wrongly believes 1900 was a leap
// year, so counting from the 30th makes every modern serial land on the right day.
var excelEpoch = time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)

// parseSheetDate reads a date cell. Excel hands dates over as a serial number when the cell is
// formatted rather than typed as text, so both forms are accepted, along with the day-first and
// month-first written orders.
func parseSheetDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil && f > 20000 && f < 90000 {
		return excelEpoch.AddDate(0, 0, int(math.Floor(f))), true
	}
	for _, layout := range []string{
		"2006-01-02", "2006/01/02", "02/01/2006", "02-01-2006", "2/1/2006",
		"01/02/2006", "02 Jan 2006", "2 January 2006", "Jan 2, 2006",
		"2006-01-02T15:04:05Z07:00", "2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// parseSheetTime normalises a time cell to "HH:MM" (blank stays blank — it is part of an
// observation's identity, so an empty string must round-trip unchanged).
func parseSheetTime(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Excel stores a time-of-day as the fraction of a day; a datetime as serial + fraction.
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		frac := f - math.Floor(f)
		mins := int(math.Round(frac * 24 * 60))
		if mins >= 24*60 {
			mins = 0
		}
		return fmt.Sprintf("%02d:%02d", mins/60, mins%60)
	}
	for _, layout := range []string{"15:04", "15:04:05", "3:04PM", "3:04 PM", "3:04pm", "3:04 pm"} {
		if t, err := time.Parse(layout, strings.ToUpper(s)); err == nil {
			return t.Format("15:04")
		}
	}
	// "8:00" and other single-digit hours.
	if parts := strings.Split(s, ":"); len(parts) >= 2 {
		if h, err1 := strconv.Atoi(strings.TrimSpace(parts[0])); err1 == nil {
			if m, err2 := strconv.Atoi(strings.TrimSpace(parts[1])); err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
				return fmt.Sprintf("%02d:%02d", h, m)
			}
		}
	}
	return s // keep whatever they wrote rather than losing it
}

// ─── Small helpers ───────────────────────────────────────────────────────────

// humanRole renders a role enum the way a person would say it, for messages an admin reads.
func humanRole(role string) string {
	switch role {
	case middleware.RoleQADeptRep:
		return "a QA department rep"
	case middleware.RoleQASchool:
		return "a QA school handler"
	case middleware.RoleHOD:
		return "a head of department"
	case middleware.RoleDean:
		return "a dean"
	case middleware.RoleQAOfficer:
		return "a QA officer"
	}
	return strings.ToLower(strings.ReplaceAll(role, "_", " "))
}

func orDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "no department"
	}
	return s
}

// sanitizeFilename strips path separators and quotes so a filename can be echoed into a
// Content-Disposition header without letting the uploader shape it.
func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == '"' || r == ';' || r == 0x7f {
			return -1
		}
		return r
	}, name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}
