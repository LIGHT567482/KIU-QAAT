package handlers

// Shared cohort — define a cohort once (session + year + semester + level + intake)
// and create its offering for EVERY course in the tenant in one shot, instead of
// adding the cohort to each course by hand. Coordinators are attached afterwards,
// per offering, via the existing offerings panel.
//
//   POST /api/v1/admin/tenants/{tenant_id}/cohorts/apply-all  (ADMIN)

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func ApplyCohortAllCourses(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var req struct {
			SessionType string `json:"session_type"`
			StudyYear   int    `json:"study_year"`
			Semester    int    `json:"semester"`
			Level       string `json:"level"`
			Intake      string `json:"intake"`
		}
		if err := decodeJSON(r, &req); err != nil ||
			req.SessionType == "" || req.StudyYear == 0 || req.Semester == 0 || req.Level == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"session_type, study_year, semester and level are required"))
			return
		}

		// One offering per course (no coordinator); skip courses that already have
		// this exact cohort. A NOT EXISTS guard (rather than ON CONFLICT) because the
		// ux_offerings_cohort unique index is DEFERRABLE and can't be a conflict arbiter.
		tag, err := adminPool.Exec(r.Context(), `
			INSERT INTO course_offerings
			    (tenant_id, course_id, session_type, study_year, semester, level, intake)
			SELECT $1::uuid, c.course_id, $2::varchar, $3::smallint, $4::smallint, $5::varchar, $6::varchar
			FROM courses c
			WHERE c.tenant_id = $1::uuid
			  AND NOT EXISTS (
			      SELECT 1 FROM course_offerings o
			      WHERE o.tenant_id = $1::uuid AND o.course_id = c.course_id
			        AND o.session_type = $2::varchar AND o.study_year = $3::smallint AND o.semester = $4::smallint
			        AND o.level = $5::varchar AND o.intake = $6::varchar)`,
			tenantID, req.SessionType, req.StudyYear, req.Semester, req.Level, req.Intake)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		created := tag.RowsAffected()

		var total int64
		_ = adminPool.QueryRow(r.Context(), `SELECT COUNT(*) FROM courses WHERE tenant_id = $1`, tenantID).Scan(&total)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"created":       created,
			"skipped":       total - created,
			"total_courses": total,
		})
	}
}
