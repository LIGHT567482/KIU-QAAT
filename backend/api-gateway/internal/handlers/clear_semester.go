package handlers

// End-of-semester data clear — INTAKE-SCOPED and archive-first.
//
// A semester rarely ends for the whole institution at once: the August intake may
// close while the May intake is still studying. So the admin can NOT wipe everything
// in one move — they must pick which intake(s) (and, optionally, which academic year)
// to clear. Whatever is about to be deleted is first compressed into a zip and stored
// under the Reports feature (semester_archives), so nothing is ever lost.
//
// It removes that scope's attendance + the sessions/lecturer logs that become empty,
// while keeping students, lecturers, courses, cohorts and the timetable. Password-gated
// because it is destructive and irreversible.
//
//   POST /api/v1/admin/tenants/{tenant_id}/clear-semester-data   (ADMIN)
//   body: { password, intakes: ["August", ...], academic_year?: "2024/2025" }

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

func ClearSemesterData(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var body struct {
			Password     string   `json:"password"`
			Intakes      []string `json:"intakes"`
			AcademicYear string   `json:"academic_year"`
		}
		_ = decodeJSON(r, &body)

		// The admin must consciously choose which intake(s) to clear — no whole-tenant
		// wipe in a single click. (To clear everything they select every intake.)
		clean := make([]string, 0, len(body.Intakes))
		for _, s := range body.Intakes {
			if t := strings.TrimSpace(s); t != "" {
				clean = append(clean, t)
			}
		}
		if len(clean) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INTAKE_REQUIRED",
				"select at least one intake to clear — clearing the whole institution at once is not allowed"))
			return
		}

		// Re-authenticate the admin's own password before this destructive action.
		var hash, adminName string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT password_hash, COALESCE(full_name, email, '') FROM users WHERE user_id = $1`,
			middleware.GetUserID(r.Context())).Scan(&hash, &adminName); err != nil {
			writeJSON(w, http.StatusForbidden, errBody("AUTH_REQUIRED", "could not verify your account"))
			return
		}
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)) != nil {
			writeJSON(w, http.StatusForbidden, errBody("INVALID_PASSWORD", "your password is incorrect"))
			return
		}

		tx, err := adminPool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx failed"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		// 1. Resolve the students in scope (by intake, and optional academic year).
		ay := strings.TrimSpace(body.AcademicYear)
		studentIDs := []string{}
		srows, err := tx.Query(r.Context(), `
			SELECT student_id FROM students_extended
			WHERE tenant_id = $1 AND intake_session = ANY($2) AND ($3 = '' OR academic_year = $3)`,
			tenantID, clean, ay)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for srows.Next() {
			var id string
			_ = srows.Scan(&id)
			studentIDs = append(studentIDs, id)
		}
		srows.Close()

		if len(studentIDs) == 0 {
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"status": "NO_MATCHING_STUDENTS", "intakes": clean, "academic_year": ay,
				"attendance_logs_deleted": 0, "sessions_deleted": 0, "lecturer_logs_deleted": 0, "archive_id": nil,
			})
			return
		}

		// 2. Which sessions did these students attend? (needed to find the ones that
		//    become empty after we delete their logs — those get removed.)
		affected := []string{}
		arows, err := tx.Query(r.Context(),
			`SELECT DISTINCT session_id::text FROM attendance_logs WHERE tenant_id = $1 AND student_id = ANY($2)`,
			tenantID, studentIDs)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		for arows.Next() {
			var id string
			_ = arows.Scan(&id)
			affected = append(affected, id)
		}
		arows.Close()

		// 3. Archive EVERYTHING in scope before deleting a single row.
		zipBytes, counts, err := buildSemesterArchive(r.Context(), tx, tenantID, studentIDs)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("ARCHIVE_FAILED", "could not build the archive: "+err.Error()))
			return
		}
		label := strings.Join(clean, ", ")
		if ay != "" {
			label += " · " + ay
		}
		filename := fmt.Sprintf("semester-archive-%s-%s.zip",
			strings.ReplaceAll(strings.Join(clean, "_"), " ", ""), time.Now().Format("2006-01-02"))
		var archiveID string
		if err := tx.QueryRow(r.Context(), `
			INSERT INTO semester_archives
			  (tenant_id, label, intakes, academic_year, filename, content, size_bytes,
			   attendance_rows, session_rows, lecturer_rows, created_by)
			VALUES ($1,$2,$3,NULLIF($4,''),$5,$6,$7,$8,$9,$10,$11)
			RETURNING archive_id::text`,
			tenantID, label, clean, ay, filename, zipBytes, len(zipBytes),
			counts.Attendance, counts.Sessions, counts.Lecturer, adminName).Scan(&archiveID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("ARCHIVE_FAILED", "could not store the archive: "+err.Error()))
			return
		}

		// 4. Delete the scope's attendance logs.
		attTag, err := tx.Exec(r.Context(),
			`DELETE FROM attendance_logs WHERE tenant_id = $1 AND student_id = ANY($2)`, tenantID, studentIDs)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("CLEAR_FAILED", err.Error()))
			return
		}

		// 5. Of the affected sessions, find those now empty (no logs from ANY intake left)
		//    — a shared session attended by another, continuing intake is preserved.
		var emptied []string
		var sessTag, lecTag, upTag int64
		if len(affected) > 0 {
			erows, err := tx.Query(r.Context(), `
				SELECT s FROM unnest($2::uuid[]) AS s
				WHERE NOT EXISTS (SELECT 1 FROM attendance_logs al WHERE al.session_id = s AND al.tenant_id = $1)`,
				tenantID, affected)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("CLEAR_FAILED", err.Error()))
				return
			}
			for erows.Next() {
				var id string
				_ = erows.Scan(&id)
				emptied = append(emptied, id)
			}
			erows.Close()
		}
		if len(emptied) > 0 {
			// lecturer logs first (FK → sessions), then sync uploads fully contained in
			// the removed set, then the empty sessions themselves.
			if t, e := tx.Exec(r.Context(),
				`DELETE FROM lecturer_attendance_logs WHERE tenant_id = $1 AND session_id = ANY($2)`, tenantID, emptied); e == nil {
				lecTag = t.RowsAffected()
			}
			if t, e := tx.Exec(r.Context(),
				`DELETE FROM sync_uploads WHERE tenant_id = $1 AND session_ids <@ $2::uuid[]`, tenantID, emptied); e == nil {
				upTag = t.RowsAffected()
			}
			if t, e := tx.Exec(r.Context(),
				`DELETE FROM sessions WHERE tenant_id = $1 AND session_id = ANY($2)`, tenantID, emptied); e == nil {
				sessTag = t.RowsAffected()
			} else {
				writeJSON(w, http.StatusInternalServerError, errBody("CLEAR_FAILED", e.Error()))
				return
			}
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit failed"))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":                  "CLEARED",
			"intakes":                 clean,
			"academic_year":           ay,
			"students_in_scope":       len(studentIDs),
			"attendance_logs_deleted": attTag.RowsAffected(),
			"sessions_deleted":        sessTag,
			"lecturer_logs_deleted":   lecTag,
			"sync_uploads_deleted":    upTag,
			"archive_id":              archiveID,
			"archive_filename":        filename,
		})
	}
}
