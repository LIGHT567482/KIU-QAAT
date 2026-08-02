package handlers

// Course offerings = (program + study session), each owning a coordinator and a
// student body (next.txt "biggest thing"). One offering per (program, session)
// and one offering per coordinator per tenant. These run on the privileged
// adminPool (cross-tenant platform op); tenant is taken from the path + scoped by
// RequireOwnTenant.

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GET /api/v1/admin/tenants/{tenant_id}/offerings
func ListOfferings(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		rows, err := adminPool.Query(r.Context(), `
			SELECT o.offering_id::text, o.course_id, c.name, o.session_type,
			       o.study_year, o.semester, COALESCE(o.level,''), COALESCE(o.intake,''),
			       COALESCE(o.coordinator_id,''), COALESCE(u.full_name,''),
			       COALESCE(u.coordinator_code,''),
			       (SELECT COUNT(*) FROM students_extended se WHERE se.offering_id = o.offering_id)
			FROM course_offerings o
			JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
			LEFT JOIN users u ON u.user_id::text = o.coordinator_id AND u.tenant_id = o.tenant_id
			WHERE o.tenant_id = $1
			ORDER BY c.name, o.session_type, o.study_year, o.semester, o.level, o.intake`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type offering struct {
			OfferingID      string `json:"offering_id"`
			CourseID        string `json:"course_id"`
			CourseName      string `json:"course_name"`
			SessionType     string `json:"session_type"`
			StudyYear       int    `json:"study_year"`
			Semester        int    `json:"semester"`
			Level           string `json:"level"`
			Intake          string `json:"intake"`
			CoordinatorID   string `json:"coordinator_id"`
			CoordinatorName string `json:"coordinator_name"`
			CoordinatorCode string `json:"coordinator_code"`
			StudentCount    int    `json:"student_count"`
		}
		out := []offering{}
		for rows.Next() {
			var o offering
			rows.Scan(&o.OfferingID, &o.CourseID, &o.CourseName, &o.SessionType, //nolint:errcheck
				&o.StudyYear, &o.Semester, &o.Level, &o.Intake,
				&o.CoordinatorID, &o.CoordinatorName, &o.CoordinatorCode, &o.StudentCount)
			out = append(out, o)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/offerings
func CreateOffering(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		var req struct {
			CourseID      string `json:"course_id"`
			SessionType   string `json:"session_type"`
			StudyYear     int    `json:"study_year"`
			Semester      int    `json:"semester"`
			Level         string `json:"level"`
			Intake        string `json:"intake"`
			CoordinatorID string `json:"coordinator_id"`
		}
		if err := decodeJSON(r, &req); err != nil || req.CourseID == "" || strings.TrimSpace(req.SessionType) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "course_id and session_type are required"))
			return
		}
		if req.StudyYear < 1 || req.StudyYear > 8 {
			req.StudyYear = 1
		}
		if req.Semester != 1 && req.Semester != 2 {
			req.Semester = 1
		}
		var offeringID string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO course_offerings (tenant_id, course_id, session_type, study_year, semester, level, intake, coordinator_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,''))
			RETURNING offering_id::text`,
			tenantID, req.CourseID, strings.TrimSpace(req.SessionType),
			req.StudyYear, req.Semester, strings.TrimSpace(req.Level), strings.TrimSpace(req.Intake),
			req.CoordinatorID).Scan(&offeringID)
		if err != nil {
			msg := err.Error()
			switch {
			case strings.Contains(msg, "ux_offerings_tenant_coordinator"):
				writeJSON(w, http.StatusConflict, errBody("COORDINATOR_TAKEN",
					"that coordinator already coordinates another cohort — a coordinator may coordinate only one"))
			case strings.Contains(msg, "ux_offerings_cohort"):
				writeJSON(w, http.StatusConflict, errBody("OFFERING_EXISTS",
					"this exact cohort (course, session, year, semester, level, intake) already exists"))
			case strings.Contains(msg, "course_offerings_course_id_fkey"):
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_COURSE", "course not found"))
			default:
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", msg))
			}
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"offering_id": offeringID, "status": "CREATED"})
	}
}

// PATCH /api/v1/admin/offerings/{offering_id}
// Edit a cohort — assign/change its coordinator, or fix any of its dimensions.
func UpdateOffering(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		offeringID := chi.URLParam(r, "offering_id")
		var req struct {
			SessionType   *string `json:"session_type"`
			StudyYear     *int    `json:"study_year"`
			Semester      *int    `json:"semester"`
			Level         *string `json:"level"`
			Intake        *string `json:"intake"`
			CoordinatorID *string `json:"coordinator_id"` // "" → unassign
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed JSON"))
			return
		}

		set := []string{}
		args := []interface{}{}
		n := 1
		if req.SessionType != nil {
			set = append(set, fmt.Sprintf("session_type = $%d", n))
			args = append(args, strings.TrimSpace(*req.SessionType))
			n++
		}
		if req.StudyYear != nil {
			set = append(set, fmt.Sprintf("study_year = $%d", n))
			args = append(args, *req.StudyYear)
			n++
		}
		if req.Semester != nil {
			set = append(set, fmt.Sprintf("semester = $%d", n))
			args = append(args, *req.Semester)
			n++
		}
		if req.Level != nil {
			set = append(set, fmt.Sprintf("level = $%d", n))
			args = append(args, strings.TrimSpace(*req.Level))
			n++
		}
		if req.Intake != nil {
			set = append(set, fmt.Sprintf("intake = $%d", n))
			args = append(args, strings.TrimSpace(*req.Intake))
			n++
		}
		if req.CoordinatorID != nil {
			val := interface{}(nil)
			if strings.TrimSpace(*req.CoordinatorID) != "" {
				val = strings.TrimSpace(*req.CoordinatorID)
			}
			set = append(set, fmt.Sprintf("coordinator_id = $%d", n))
			args = append(args, val)
			n++
		}
		if len(set) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "no fields to update"))
			return
		}

		query := "UPDATE course_offerings SET " + joinComma(set) +
			fmt.Sprintf(" WHERE offering_id = $%d::uuid AND (%s)", n, tenantScopeClause(fmt.Sprintf("$%d", n+1)))
		args = append(args, offeringID, tenantScope(r))

		tag, err := adminPool.Exec(r.Context(), query, args...)
		if err != nil {
			msg := err.Error()
			switch {
			case strings.Contains(msg, "ux_offerings_tenant_coordinator"):
				writeJSON(w, http.StatusConflict, errBody("COORDINATOR_TAKEN",
					"that coordinator already coordinates another cohort — a coordinator may coordinate only one"))
			case strings.Contains(msg, "ux_offerings_cohort"):
				writeJSON(w, http.StatusConflict, errBody("OFFERING_EXISTS",
					"another cohort with that exact course, session, year, semester, level and intake already exists"))
			default:
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", msg))
			}
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "cohort not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"offering_id": offeringID, "status": "UPDATED"})
	}
}

// DELETE /api/v1/admin/offerings/{offering_id}
func DeleteOffering(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		offeringID := chi.URLParam(r, "offering_id")
		tag, err := adminPool.Exec(r.Context(),
			`DELETE FROM course_offerings WHERE offering_id = $1::uuid AND (`+tenantScopeClause("$2")+`)`,
			offeringID, tenantScope(r))
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("DELETE_FAILED", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "offering not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"offering_id": offeringID, "status": "DELETED"})
	}
}
