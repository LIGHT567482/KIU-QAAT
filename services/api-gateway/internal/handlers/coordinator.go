package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// GetUnitLecturers returns all lecturers assigned to a course unit, for the
// calling coordinator's tenant. Used by the coordinator PWA to populate the
// lecturer dropdown before opening a session.
//
// GET /api/v1/coordinator/units/{unit_id}/lecturers
func GetUnitLecturers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		unitID := chi.URLParam(r, "unit_id")

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "db unavailable"))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "rls setup failed"))
			return
		}

		rows, err := conn.Query(r.Context(), `
			SELECT l.lecturer_id::text, l.full_name,
			       COALESCE(l.email,''), COALESCE(l.department,''),
			       la.academic_year, la.year, la.semester, la.intake_session::text
			FROM lecturer_assignments la
			JOIN lecturers l ON l.lecturer_id = la.lecturer_id
			WHERE la.unit_id = $1 AND la.tenant_id = $2
			ORDER BY l.full_name`, unitID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type lecturerRow struct {
			LecturerID    string `json:"lecturer_id"`
			FullName      string `json:"full_name"`
			Email         string `json:"email"`
			Department    string `json:"department"`
			AcademicYear  string `json:"academic_year"`
			Year          int    `json:"year"`
			Semester      int    `json:"semester"`
			IntakeSession string `json:"intake_session"`
		}
		var list []lecturerRow
		for rows.Next() {
			var l lecturerRow
			rows.Scan(&l.LecturerID, &l.FullName, &l.Email, &l.Department,
				&l.AcademicYear, &l.Year, &l.Semester, &l.IntakeSession) //nolint:errcheck
			list = append(list, l)
		}
		if list == nil {
			list = []lecturerRow{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}
