package handlers

// Semester archives — the compress-and-store side of the end-of-semester clear.
//
// Before an intake's attendance data is deleted (see clear_semester.go), we snapshot
// everything being removed into a zip of CSVs and store it in `semester_archives`, so
// the institution keeps a downloadable record. The admin reaches these from the
// Reports feature:
//
//   GET  /api/v1/admin/tenants/{tenant_id}/semester-archives            (ADMIN) — list
//   GET  /api/v1/admin/tenants/{tenant_id}/semester-archives/{id}/download (ADMIN) — zip
//   DELETE /api/v1/admin/tenants/{tenant_id}/semester-archives/{id}     (ADMIN) — remove

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// rowQuerier is satisfied by both *pgxpool.Pool and pgx.Tx, so the archive can be
// built inside the clear transaction (before the rows are deleted) or standalone.
type rowQuerier interface {
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
}

// archiveCounts reports how many rows of each kind were captured.
type archiveCounts struct {
	Attendance int
	Sessions   int
	Lecturer   int
}

// buildSemesterArchive produces a zip (attendance.csv + sessions.csv +
// lecturer_attendance.csv) of the data in scope. When studentIDs is nil/empty the
// scope is the whole tenant; otherwise only those students' attendance (and the
// sessions/lecturer logs it touches). It reads only — deletion is the caller's job.
func buildSemesterArchive(ctx context.Context, q rowQuerier, tenantID string, studentIDs []string) ([]byte, archiveCounts, error) {
	var counts archiveCounts
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	// scope arg: a NULL text[] means "all students of the tenant".
	var scope interface{}
	if len(studentIDs) > 0 {
		scope = studentIDs
	}

	// ── attendance.csv (+ collect the touched session ids) ───────────────────
	touched := map[string]struct{}{}
	{
		w, err := zw.Create("attendance.csv")
		if err != nil {
			return nil, counts, err
		}
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"log_id", "session_id", "student_id", "full_name", "intake_session",
			"academic_year", "unit_id", "unit_name", "session_date", "checkin_timestamp", "entry_method"})
		rows, err := q.Query(ctx, `
			SELECT al.log_id::text, al.session_id::text, al.student_id,
			       COALESCE(se.full_name,''), COALESCE(se.intake_session::text,''), COALESCE(se.academic_year,''),
			       s.unit_id, COALESCE(cu.name,''), s.session_date::text,
			       al.checkin_timestamp::text, al.entry_method::text
			FROM attendance_logs al
			JOIN sessions s        ON s.session_id = al.session_id
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN students_extended se ON se.student_id = al.student_id AND se.tenant_id = al.tenant_id
			WHERE al.tenant_id = $1 AND ($2::text[] IS NULL OR al.student_id = ANY($2))
			ORDER BY s.session_date, al.session_id, al.student_id`, tenantID, scope)
		if err != nil {
			return nil, counts, err
		}
		for rows.Next() {
			var logID, sessID, studID, name, intake, ay, unitID, unitName, sdate, ts, method string
			if err := rows.Scan(&logID, &sessID, &studID, &name, &intake, &ay, &unitID, &unitName, &sdate, &ts, &method); err != nil {
				rows.Close()
				return nil, counts, err
			}
			touched[sessID] = struct{}{}
			_ = cw.Write([]string{logID, sessID, studID, name, intake, ay, unitID, unitName, sdate, ts, method})
			counts.Attendance++
		}
		rows.Close()
		cw.Flush()
	}

	sessIDs := make([]string, 0, len(touched))
	for id := range touched {
		sessIDs = append(sessIDs, id)
	}

	// ── sessions.csv ─────────────────────────────────────────────────────────
	{
		w, err := zw.Create("sessions.csv")
		if err != nil {
			return nil, counts, err
		}
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"session_id", "unit_id", "unit_name", "coordinator_id", "session_date", "session_status"})
		if len(sessIDs) > 0 {
			rows, err := q.Query(ctx, `
				SELECT s.session_id::text, s.unit_id, COALESCE(cu.name,''), s.coordinator_id,
				       s.session_date::text, s.session_status::text
				FROM sessions s
				LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
				WHERE s.tenant_id = $1 AND s.session_id = ANY($2)
				ORDER BY s.session_date`, tenantID, sessIDs)
			if err != nil {
				return nil, counts, err
			}
			for rows.Next() {
				var id, unitID, unitName, coord, sdate, status string
				if err := rows.Scan(&id, &unitID, &unitName, &coord, &sdate, &status); err != nil {
					rows.Close()
					return nil, counts, err
				}
				_ = cw.Write([]string{id, unitID, unitName, coord, sdate, status})
				counts.Sessions++
			}
			rows.Close()
		}
		cw.Flush()
	}

	// ── lecturer_attendance.csv ──────────────────────────────────────────────
	{
		w, err := zw.Create("lecturer_attendance.csv")
		if err != nil {
			return nil, counts, err
		}
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"session_id", "lecturer_id", "unit_id", "session_date", "gate_open_time", "gate_close_time", "contact_hours"})
		if len(sessIDs) > 0 {
			rows, err := q.Query(ctx, `
				SELECT session_id::text, lecturer_id, unit_id, session_date::text,
				       gate_open_time::text, COALESCE(gate_close_time::text,''), COALESCE(contact_hours::text,'')
				FROM lecturer_attendance_logs
				WHERE tenant_id = $1 AND session_id = ANY($2)
				ORDER BY session_date`, tenantID, sessIDs)
			if err != nil {
				return nil, counts, err
			}
			for rows.Next() {
				var id, lec, unit, sdate, open, close, hours string
				if err := rows.Scan(&id, &lec, &unit, &sdate, &open, &close, &hours); err != nil {
					rows.Close()
					return nil, counts, err
				}
				_ = cw.Write([]string{id, lec, unit, sdate, open, close, hours})
				counts.Lecturer++
			}
			rows.Close()
		}
		cw.Flush()
	}

	if err := zw.Close(); err != nil {
		return nil, counts, err
	}
	return buf.Bytes(), counts, nil
}

// ── GET /api/v1/admin/tenants/{tenant_id}/semester-archives ──────────────────
func ListSemesterArchives(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT archive_id::text, label, COALESCE(intakes,'{}'), COALESCE(academic_year,''),
			       COALESCE(semester,0), filename, size_bytes, attendance_rows, session_rows,
			       lecturer_rows, COALESCE(created_by,''), created_at::text
			FROM semester_archives
			WHERE tenant_id = $1
			ORDER BY created_at DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type archive struct {
			ArchiveID      string   `json:"archive_id"`
			Label          string   `json:"label"`
			Intakes        []string `json:"intakes"`
			AcademicYear   string   `json:"academic_year"`
			Semester       int      `json:"semester"`
			Filename       string   `json:"filename"`
			SizeBytes      int64    `json:"size_bytes"`
			AttendanceRows int      `json:"attendance_rows"`
			SessionRows    int      `json:"session_rows"`
			LecturerRows   int      `json:"lecturer_rows"`
			CreatedBy      string   `json:"created_by"`
			CreatedAt      string   `json:"created_at"`
		}
		var list []archive
		for rows.Next() {
			var a archive
			rows.Scan(&a.ArchiveID, &a.Label, &a.Intakes, &a.AcademicYear, &a.Semester, &a.Filename,
				&a.SizeBytes, &a.AttendanceRows, &a.SessionRows, &a.LecturerRows, &a.CreatedBy, &a.CreatedAt) //nolint:errcheck
			list = append(list, a)
		}
		if list == nil {
			list = []archive{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// ── GET /api/v1/admin/tenants/{tenant_id}/semester-archives/{archive_id}/download ──
func DownloadSemesterArchive(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		archiveID := chi.URLParam(r, "archive_id")

		var filename string
		var content []byte
		err := adminPool.QueryRow(r.Context(), `
			SELECT filename, content FROM semester_archives
			WHERE archive_id = $1::uuid AND tenant_id = $2`, archiveID, tenantID).Scan(&filename, &content)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "archive not found"))
			return
		}
		if filename == "" {
			filename = fmt.Sprintf("semester-archive-%s.zip", time.Now().Format("2006-01-02"))
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		_, _ = w.Write(content)
	}
}

// ── DELETE /api/v1/admin/tenants/{tenant_id}/semester-archives/{archive_id} ───
func DeleteSemesterArchive(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		archiveID := chi.URLParam(r, "archive_id")

		tag, err := adminPool.Exec(r.Context(),
			`DELETE FROM semester_archives WHERE archive_id = $1::uuid AND tenant_id = $2`, archiveID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "archive not found"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
