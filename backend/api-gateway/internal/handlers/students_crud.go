package handlers

// Student edit + delete for the admin Students page.
//
//   PATCH  /api/v1/admin/tenants/{tenant_id}/students  (ADMIN) — body carries
//          student_id + the editable fields. Keyed by student_id in the body
//          (registration numbers may contain '/', so never in the path).
//   DELETE /api/v1/admin/tenants/{tenant_id}/students?student_id=... (ADMIN) —
//          removes the students_extended row AND the linked login user.

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type updateStudentRequest struct {
	StudentID        string `json:"student_id"`
	FullName         string `json:"full_name"`
	CurrentYear      int    `json:"current_year"`
	Semester         int    `json:"semester"`
	AcademicYear     string `json:"academic_year"`
	IntakeSession    string `json:"intake_session"`
	EnrollmentStatus string `json:"enrollment_status"`
	OfferingID       string `json:"offering_id"`
}

// PATCH /api/v1/admin/tenants/{tenant_id}/students
func UpdateStudent(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var req updateStudentRequest
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.StudentID) == "" || req.FullName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "student_id and full_name are required"))
			return
		}
		if req.EnrollmentStatus == "" {
			req.EnrollmentStatus = "ACTIVE"
		}
		if req.CurrentYear == 0 {
			req.CurrentYear = 1
		}
		if req.Semester == 0 {
			req.Semester = 1
		}

		// If an offering is supplied, re-derive the program course_id from it.
		var courseID string
		if req.OfferingID != "" {
			if err := adminPool.QueryRow(r.Context(),
				`SELECT course_id FROM course_offerings WHERE offering_id = $1::uuid AND tenant_id = $2`,
				req.OfferingID, tenantID).Scan(&courseID); err != nil {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_OFFERING", "offering not found for this tenant"))
				return
			}
		}

		tx, err := adminPool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx failed"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		// Look up the current email so we can keep the login record's name in sync.
		var email string
		if err := tx.QueryRow(r.Context(),
			`SELECT email FROM students_extended WHERE student_id = $1 AND tenant_id = $2`,
			req.StudentID, tenantID).Scan(&email); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "student not found"))
			return
		}

		ct, err := tx.Exec(r.Context(), `
			UPDATE students_extended SET
			    full_name         = $3,
			    current_year      = $4,
			    semester          = $5,
			    academic_year     = $6,
			    intake_session    = $7,
			    enrollment_status = $8::text::enrollment_status_enum,
			    offering_id       = COALESCE(NULLIF($9,'')::uuid, offering_id),
			    course_id         = COALESCE(NULLIF($10,''), course_id)
			WHERE student_id = $1 AND tenant_id = $2`,
			req.StudentID, tenantID, req.FullName, req.CurrentYear, req.Semester,
			req.AcademicYear, req.IntakeSession, req.EnrollmentStatus, req.OfferingID, courseID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("UPDATE_FAILED", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "student not found"))
			return
		}

		// Keep the login user's name aligned.
		_, _ = tx.Exec(r.Context(),
			`UPDATE users SET full_name = $3 WHERE email = $1 AND tenant_id = $2`,
			email, tenantID, req.FullName)

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"student_id": req.StudentID, "status": "UPDATED"})
	}
}

// DELETE /api/v1/admin/tenants/{tenant_id}/students?student_id=...
func DeleteStudent(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		studentID := strings.TrimSpace(r.URL.Query().Get("student_id"))
		if studentID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "student_id query parameter is required"))
			return
		}

		tx, err := adminPool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx failed"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		var email string
		if err := tx.QueryRow(r.Context(),
			`DELETE FROM students_extended WHERE student_id = $1 AND tenant_id = $2 RETURNING email`,
			studentID, tenantID).Scan(&email); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "student not found"))
			return
		}
		// Remove the matching login record (STUDENT role) for this tenant.
		_, _ = tx.Exec(r.Context(),
			`DELETE FROM users WHERE email = $1 AND tenant_id = $2 AND role = 'STUDENT'`,
			email, tenantID)

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit failed"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"student_id": studentID, "status": "DELETED"})
	}
}
