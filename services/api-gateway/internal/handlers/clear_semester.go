package handlers

// End-of-semester data clear. Wipes the PREVIOUS semester's attendance + session
// records (and their sync uploads) so a new semester starts clean, while keeping
// students, lecturers, courses, cohorts and the timetable. Password-gated because
// it is destructive and irreversible.
//
//   POST /api/v1/admin/tenants/{tenant_id}/clear-semester-data   (ADMIN)

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

func ClearSemesterData(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		// Re-authenticate the admin's own password before this destructive action.
		var body struct {
			Password string `json:"password"`
		}
		_ = decodeJSON(r, &body)
		var hash string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT password_hash FROM users WHERE user_id = $1`, middleware.GetUserID(r.Context())).Scan(&hash); err != nil {
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

		// FK-safe order: children of sessions first, then sessions, then uploads.
		del := func(sql string) int64 {
			tag, e := tx.Exec(r.Context(), sql, tenantID)
			if e != nil {
				return -1
			}
			return tag.RowsAffected()
		}
		attendance := del(`DELETE FROM attendance_logs WHERE tenant_id = $1`)
		lecturerLogs := del(`DELETE FROM lecturer_attendance_logs WHERE tenant_id = $1`)
		sessions := del(`DELETE FROM sessions WHERE tenant_id = $1`)
		uploads := del(`DELETE FROM sync_uploads WHERE tenant_id = $1`)
		if attendance < 0 || lecturerLogs < 0 || sessions < 0 || uploads < 0 {
			writeJSON(w, http.StatusInternalServerError, errBody("CLEAR_FAILED", "could not clear semester data"))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit failed"))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":                  "CLEARED",
			"attendance_logs_deleted": attendance,
			"sessions_deleted":        sessions,
			"lecturer_logs_deleted":   lecturerLogs,
			"sync_uploads_deleted":    uploads,
		})
	}
}
