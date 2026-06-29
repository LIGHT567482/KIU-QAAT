package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

// ─── Courses ──────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/courses
func ListCourses(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT c.course_id, c.name, COALESCE(c.department,''), COALESCE(c.school,''),
			       COALESCE(c.level,''),
			       COUNT(cu.unit_id) AS unit_count,
			       COALESCE(c.total_years, 3) AS total_years,
			       COALESCE(c.level_years, '{}') AS level_years,
			       (SELECT COUNT(*) FROM course_offerings o WHERE o.course_id = c.course_id) AS offering_count
			FROM courses c
			LEFT JOIN course_units cu ON cu.course_id = c.course_id
			WHERE c.tenant_id = $1
			GROUP BY c.course_id, c.name, c.department, c.school, c.level, c.total_years, c.level_years
			ORDER BY c.name, c.level`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type course struct {
			CourseID      string         `json:"course_id"`
			Name          string         `json:"name"`
			Department    string         `json:"department"`
			School        string         `json:"school"`
			Level         string         `json:"level"`
			UnitCount     int            `json:"unit_count"`
			TotalYears    int            `json:"total_years"`
			LevelYears    map[string]int `json:"level_years"`
			OfferingCount int            `json:"offering_count"`
		}
		var list []course
		for rows.Next() {
			var c course
			rows.Scan(&c.CourseID, &c.Name, &c.Department, &c.School,
				&c.Level, &c.UnitCount, &c.TotalYears, &c.LevelYears, &c.OfferingCount) //nolint:errcheck
			if c.LevelYears == nil {
				c.LevelYears = map[string]int{}
			}
			list = append(list, c)
		}
		if list == nil {
			list = []course{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/courses
func CreateCourse(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		// A course is just a course (id, name, department, school). Years of study are
		// a property of the LEVEL (Degree 3y, Masters 2y…), set in the level list —
		// not the course. The coordinator belongs to a cohort (course + session + …).
		var req struct {
			CourseID   string `json:"course_id"`
			Name       string `json:"name"`
			Department string `json:"department"`
			School     string `json:"school"`
		}
		if err := decodeJSON(r, &req); err != nil || req.CourseID == "" || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "course_id and name are required"))
			return
		}

		_, err := adminPool.Exec(r.Context(), `
			INSERT INTO courses (course_id, tenant_id, name, department, school)
			VALUES ($1, $2, $3, $4, $5)`,
			req.CourseID, tenantID, req.Name, req.Department, req.School)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", "course_id already exists: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"course_id": req.CourseID, "status": "CREATED"})
	}
}

// ─── Course Units ─────────────────────────────────────────────────────────────

// GET /api/v1/admin/courses/{course_id}/units
func ListCourseUnits(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		courseID := chi.URLParam(r, "course_id")

		// IDOR guard: a non-SUPER_ADMIN may only list units of their own tenant's course.
		rows, err := adminPool.Query(r.Context(), `
			SELECT unit_id, name, COALESCE(year,0), COALESCE(semester,1), COALESCE(academic_year,'')
			FROM course_units
			WHERE course_id = $1 AND ($2 = '' OR tenant_id::text = $2)
			ORDER BY year, semester, name`, courseID, tenantScope(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type unit struct {
			UnitID       string `json:"unit_id"`
			Name         string `json:"name"`
			Year         int    `json:"year"`
			Semester     int    `json:"semester"`
			AcademicYear string `json:"academic_year"`
		}
		var list []unit
		for rows.Next() {
			var u unit
			rows.Scan(&u.UnitID, &u.Name, &u.Year, &u.Semester, &u.AcademicYear) //nolint:errcheck
			list = append(list, u)
		}
		if list == nil {
			list = []unit{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// POST /api/v1/admin/courses/{course_id}/units
func CreateCourseUnit(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		courseID := chi.URLParam(r, "course_id")

		// Look up the tenant_id from the course so we can set it on the unit.
		var tenantID string
		err := adminPool.QueryRow(r.Context(),
			`SELECT tenant_id FROM courses WHERE course_id = $1`, courseID).Scan(&tenantID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "course not found"))
			return
		}

		var req struct {
			UnitID         string `json:"unit_id"`
			Name           string `json:"name"`
			Year           int    `json:"year"`
			Semester       int    `json:"semester"`
			Level          string `json:"level"` // the level this unit belongs to (Degree, Masters, …)
			AcademicYear   string `json:"academic_year"`
			DefaultVenueID string `json:"default_venue_id"`
		}
		if err := decodeJSON(r, &req); err != nil || req.UnitID == "" || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "unit_id and name are required"))
			return
		}

		// A course unit is a template within a course's (level, year, semester) — it
		// is not tied to a calendar year. We never ask the admin for it: default it to
		// the tenant's active academic year so the daily manifest still resolves.
		if req.AcademicYear == "" {
			_ = adminPool.QueryRow(r.Context(),
				`SELECT COALESCE(active_academic_year, '') FROM tenants WHERE tenant_id = $1`,
				tenantID).Scan(&req.AcademicYear) //nolint:errcheck
		}

		_, err = adminPool.Exec(r.Context(), `
			INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester, level, academic_year, default_venue_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8,''), NULLIF($9,''))`,
			req.UnitID, tenantID, courseID, req.Name, req.Year, req.Semester, strings.TrimSpace(req.Level), req.AcademicYear, req.DefaultVenueID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", "unit_id already exists: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"unit_id": req.UnitID, "status": "CREATED"})
	}
}

// ─── Venues ───────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/venues
func ListVenues(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT venue_id, name, COALESCE(building,''), COALESCE(floor,0)::int, COALESCE(capacity,0)::int
			FROM venues WHERE tenant_id = $1 ORDER BY name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type venue struct {
			VenueID  string `json:"venue_id"`
			Name     string `json:"name"`
			Building string `json:"building"`
			Floor    int    `json:"floor"`
			Capacity int    `json:"capacity"`
		}
		var list []venue
		for rows.Next() {
			var v venue
			rows.Scan(&v.VenueID, &v.Name, &v.Building, &v.Floor, &v.Capacity) //nolint:errcheck
			list = append(list, v)
		}
		if list == nil {
			list = []venue{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/venues
func CreateVenue(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var req struct {
			VenueID  string `json:"venue_id"`
			Name     string `json:"name"`
			Building string `json:"building"`
			Floor    int    `json:"floor"`
			Capacity int    `json:"capacity"`
		}
		if err := decodeJSON(r, &req); err != nil || req.VenueID == "" || req.Name == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "venue_id and name are required"))
			return
		}

		_, err := adminPool.Exec(r.Context(), `
			INSERT INTO venues (venue_id, tenant_id, name, building, floor, capacity)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			req.VenueID, tenantID, req.Name, req.Building, req.Floor, req.Capacity)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", "venue_id already exists: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"venue_id": req.VenueID, "status": "CREATED"})
	}
}

// ─── Course Update ────────────────────────────────────────────────────────────

// PATCH /api/v1/admin/courses/{course_id}
// Updates mutable fields of a course — name, department, school, coordinator_id,
// total_years. Only fields present in the JSON body are updated (partial PATCH).
func UpdateCourse(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		courseID := chi.URLParam(r, "course_id")

		var req struct {
			Name        *string `json:"name"`
			Department  *string `json:"department"`
			School      *string `json:"school"`
			Level       *string         `json:"level"`
			TotalYears  *int            `json:"total_years"`
			LevelYears  *map[string]int `json:"level_years"` // per-level years for THIS course
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed JSON"))
			return
		}

		// Build a dynamic SET clause from whichever fields were provided. The
		// coordinator is no longer a course field — it lives on the offering.
		setClauses := []string{}
		args := []interface{}{}
		n := 1

		if req.Name != nil {
			setClauses = append(setClauses, fmt.Sprintf("name = $%d", n)); args = append(args, *req.Name); n++
		}
		if req.Department != nil {
			setClauses = append(setClauses, fmt.Sprintf("department = $%d", n)); args = append(args, *req.Department); n++
		}
		if req.School != nil {
			setClauses = append(setClauses, fmt.Sprintf("school = $%d", n)); args = append(args, *req.School); n++
		}
		if req.Level != nil {
			setClauses = append(setClauses, fmt.Sprintf("level = $%d", n)); args = append(args, *req.Level); n++
		}
		if req.LevelYears != nil {
			// Clamp each level's years to 1–10 before storing the JSONB map.
			clean := map[string]int{}
			for k, v := range *req.LevelYears {
				if v < 1 {
					v = 1
				} else if v > 10 {
					v = 10
				}
				clean[strings.TrimSpace(k)] = v
			}
			setClauses = append(setClauses, fmt.Sprintf("level_years = $%d", n)); args = append(args, clean); n++
		}
		if req.TotalYears != nil {
			setClauses = append(setClauses, fmt.Sprintf("total_years = $%d", n)); args = append(args, *req.TotalYears); n++
		}
		if len(setClauses) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "no fields to update"))
			return
		}

		// IDOR guard: confine non-SUPER_ADMIN callers to their own tenant's course.
		query := "UPDATE courses SET " + joinComma(setClauses) +
			fmt.Sprintf(" WHERE course_id = $%d AND ($%d = '' OR tenant_id::text = $%d)", n, n+1, n+1)
		args = append(args, courseID, tenantScope(r))

		tag, err := adminPool.Exec(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "course not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"course_id": courseID, "status": "UPDATED"})
	}
}

// ─── Course Unit Update ───────────────────────────────────────────────────────

// PATCH /api/v1/admin/courses/{course_id}/units/{unit_id}
func UpdateCourseUnit(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		unitID := chi.URLParam(r, "unit_id")

		var req struct {
			Name            *string `json:"name"`
			Year            *int    `json:"year"`
			Semester        *int    `json:"semester"`
			Level           *string `json:"level"`
			AcademicYear    *string `json:"academic_year"`
			DefaultVenueID  *string `json:"default_venue_id"`
			SessionStart    *string `json:"session_start"`             // "HH:MM"; admin override (#2)
			DurationMinutes *int    `json:"session_duration_minutes"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed JSON"))
			return
		}

		setClauses := []string{}
		args := []interface{}{}
		n := 1

		if req.Name != nil {
			setClauses = append(setClauses, fmt.Sprintf("name = $%d", n)); args = append(args, *req.Name); n++
		}
		if req.Year != nil {
			setClauses = append(setClauses, fmt.Sprintf("year = $%d", n)); args = append(args, *req.Year); n++
		}
		if req.Semester != nil {
			setClauses = append(setClauses, fmt.Sprintf("semester = $%d", n)); args = append(args, *req.Semester); n++
		}
		if req.Level != nil {
			setClauses = append(setClauses, fmt.Sprintf("level = $%d", n)); args = append(args, *req.Level); n++
		}
		if req.AcademicYear != nil {
			setClauses = append(setClauses, fmt.Sprintf("academic_year = $%d", n)); args = append(args, *req.AcademicYear); n++
		}
		if req.DefaultVenueID != nil {
			val := interface{}(nil)
			if *req.DefaultVenueID != "" {
				val = *req.DefaultVenueID
			}
			setClauses = append(setClauses, fmt.Sprintf("default_venue_id = $%d", n)); args = append(args, val); n++
		}
		// Admin can change the schedule even when locked (keeps schedule_locked=true).
		if req.SessionStart != nil {
			setClauses = append(setClauses, fmt.Sprintf("session_start = NULLIF($%d,'')::time", n)); args = append(args, *req.SessionStart); n++
		}
		if req.DurationMinutes != nil {
			setClauses = append(setClauses, fmt.Sprintf("session_duration_minutes = $%d", n)); args = append(args, *req.DurationMinutes); n++
		}
		if req.SessionStart != nil || req.DurationMinutes != nil {
			setClauses = append(setClauses, "schedule_locked = true")
		}
		if len(setClauses) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "no fields to update"))
			return
		}

		// IDOR guard: confine non-SUPER_ADMIN callers to their own tenant's unit.
		query := "UPDATE course_units SET " + joinComma(setClauses) +
			fmt.Sprintf(" WHERE unit_id = $%d AND ($%d = '' OR tenant_id::text = $%d)", n, n+1, n+1)
		args = append(args, unitID, tenantScope(r))

		tag, err := adminPool.Exec(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "unit not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"unit_id": unitID, "status": "UPDATED"})
	}
}

// ─── Course Roadmap ───────────────────────────────────────────────────────────

// GET /api/v1/admin/courses/{course_id}/roadmap
// Returns all units for a course grouped by year → semester, plus the course
// metadata (total_years). This drives the admin roadmap UI.
func GetCourseRoadmap(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		courseID := chi.URLParam(r, "course_id")

		var courseName, department, school, tenantID string
		var coordinatorID, coordinatorName string
		var totalYears int
		var levelYears map[string]int
		err := adminPool.QueryRow(r.Context(), `
			SELECT c.name, COALESCE(c.department,''), COALESCE(c.school,''),
			       COALESCE(c.coordinator_id,''), COALESCE(u.full_name, c.coordinator_id, ''),
			       COALESCE(c.total_years, 3), COALESCE(c.level_years, '{}'), c.tenant_id::text
			FROM courses c
			LEFT JOIN users u ON u.user_id::text = c.coordinator_id
			WHERE c.course_id = $1 AND ($2 = '' OR c.tenant_id::text = $2)`, courseID, tenantScope(r)).
			Scan(&courseName, &department, &school, &coordinatorID, &coordinatorName, &totalYears, &levelYears, &tenantID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "course not found in your tenant"))
			return
		}

		rows, err := adminPool.Query(r.Context(), `
			SELECT unit_id, name, COALESCE(year,1), COALESCE(semester,1),
			       COALESCE(level,''),
			       COALESCE(academic_year,''), COALESCE(default_venue_id,''),
			       COALESCE(to_char(session_start,'HH24:MI'),''),
			       COALESCE(session_duration_minutes,0), schedule_locked
			FROM course_units
			WHERE course_id = $1
			ORDER BY year, semester, name`, courseID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type unit struct {
			UnitID          string `json:"unit_id"`
			Name            string `json:"name"`
			Year            int    `json:"year"`
			Semester        int    `json:"semester"`
			Level           string `json:"level"`
			AcademicYear    string `json:"academic_year"`
			DefaultVenueID  string `json:"default_venue_id"`
			SessionStart    string `json:"session_start"`
			DurationMinutes int    `json:"session_duration_minutes"`
			ScheduleLocked  bool   `json:"schedule_locked"`
		}
		// roadmap[year][semester] = []unit
		roadmap := map[int]map[int][]unit{}
		for rows.Next() {
			var u unit
			rows.Scan(&u.UnitID, &u.Name, &u.Year, &u.Semester, &u.Level, &u.AcademicYear, &u.DefaultVenueID,
				&u.SessionStart, &u.DurationMinutes, &u.ScheduleLocked) //nolint:errcheck
			if roadmap[u.Year] == nil {
				roadmap[u.Year] = map[int][]unit{}
			}
			roadmap[u.Year][u.Semester] = append(roadmap[u.Year][u.Semester], u)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"course_id":        courseID,
			"tenant_id":        tenantID,
			"name":             courseName,
			"department":       department,
			"school":           school,
			"coordinator_id":   coordinatorID,
			"coordinator_name": coordinatorName,
			"total_years":      totalYears,
			"level_years":      levelYears,
			"roadmap":          roadmap,
		})
	}
}

// ─── Tenant Academic Period ───────────────────────────────────────────────────

// PATCH /api/v1/admin/tenants/{tenant_id}/academic-period
// Admin sets which academic year + semester is currently active for a tenant.
// The daily manifest uses this to filter course units to the active semester.
func UpdateTenantAcademicPeriod(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var req struct {
			AcademicYear string `json:"active_academic_year"`
			Semester     int    `json:"active_semester"`
		}
		if err := decodeJSON(r, &req); err != nil || req.AcademicYear == "" || (req.Semester != 1 && req.Semester != 2) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"active_academic_year (string) and active_semester (1 or 2) are required"))
			return
		}

		tag, err := adminPool.Exec(r.Context(), `
			UPDATE tenants SET active_academic_year = $1, active_semester = $2
			WHERE tenant_id = $3`, req.AcademicYear, req.Semester, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		// The academic year is institution-wide: roll EVERY active student onto it in
		// one move, so the admin never has to touch students one by one. (Their
		// study-year/semester — which differ across the cohort — are untouched.)
		ct, _ := adminPool.Exec(r.Context(), `
			UPDATE students_extended SET academic_year = $1, updated_at = now()
			WHERE tenant_id = $2 AND enrollment_status = 'ACTIVE'`, req.AcademicYear, tenantID) //nolint:errcheck

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tenant_id":            tenantID,
			"active_academic_year": req.AcademicYear,
			"active_semester":      req.Semester,
			"students_updated":     ct.RowsAffected(),
			"status":               "UPDATED",
		})
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/academic-period/advance
// End-of-semester rollover (next.txt #8): sets the new active period AND advances
// students. Year +1 only when moving into a NEW academic year's Semester 1;
// final-year students (current_year would exceed the program's total_years) become
// GRADUATED. Everyone's academic_year/semester is set to the new period. The admin
// confirms this in the UI before it runs (it mass-mutates student records).
// AdvanceAcademicPeriod is the single "advance to next semester" button. It takes
// NO inputs: it reads the tenant's current period and moves the WHOLE institution
// forward one step — Sem 1 → Sem 2 (same year); Sem 2 → Sem 1 of the next academic
// year. Every active student AND every cohort (offering) shifts the same way, and
// final-year/semester ones GRADUATE / complete (years come from the LEVEL). One
// click updates the system so the admin never re-creates cohorts each semester.
func AdvanceAcademicPeriod(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		// This is a heavy, irreversible institution-wide change → re-authenticate the
		// admin's own password before proceeding.
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

		// Read the current pointer to compute the next one.
		var curYear string
		var curSem int
		if err := adminPool.QueryRow(r.Context(),
			`SELECT COALESCE(active_academic_year,''), COALESCE(active_semester,1) FROM tenants WHERE tenant_id = $1`,
			tenantID).Scan(&curYear, &curSem); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		newSem := 2
		newYear := curYear
		if curSem == 2 {
			newSem = 1
			newYear = incAcademicYear(curYear) // S2 → S1 of next academic year
		}

		tx, err := adminPool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx failed"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		// Years of study come from the LEVEL (tenants.level_years: name → years); a
		// blank/unknown level has no cap (99). Count who graduates, then advance.
		var graduated, advanced int
		_ = tx.QueryRow(r.Context(), `
			SELECT
			  COUNT(*) FILTER (WHERE se.semester = 2 AND se.current_year + 1 > COALESCE((SELECT (c.level_years ->> NULLIF(se.level,''))::int FROM courses c WHERE c.course_id = se.course_id AND c.tenant_id = se.tenant_id), (t.level_years ->> NULLIF(se.level,''))::int, 99)),
			  COUNT(*)
			FROM students_extended se JOIN tenants t ON t.tenant_id = se.tenant_id
			WHERE se.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'`,
			tenantID).Scan(&graduated, &advanced) //nolint:errcheck

		// 1. Advance every active student along their own track.
		if _, err := tx.Exec(r.Context(), `
			UPDATE students_extended se SET
			  semester = CASE WHEN se.semester = 2 THEN 1 ELSE 2 END,
			  current_year = CASE WHEN se.semester = 2
			      THEN LEAST(se.current_year + 1, COALESCE((SELECT (c.level_years ->> NULLIF(se.level,''))::int FROM courses c WHERE c.course_id = se.course_id AND c.tenant_id = se.tenant_id), (t.level_years ->> NULLIF(se.level,''))::int, 99))
			      ELSE se.current_year END,
			  academic_year = CASE
			      WHEN se.semester = 2 AND se.academic_year ~ '^[0-9]{4}/[0-9]{4}$'
			      THEN ((left(se.academic_year,4))::int + 1)::text || '/' || ((right(se.academic_year,4))::int + 1)::text
			      ELSE se.academic_year END,
			  enrollment_status = CASE
			      WHEN se.semester = 2 AND se.current_year + 1 > COALESCE((SELECT (c.level_years ->> NULLIF(se.level,''))::int FROM courses c WHERE c.course_id = se.course_id AND c.tenant_id = se.tenant_id), (t.level_years ->> NULLIF(se.level,''))::int, 99)
			      THEN 'GRADUATED' ELSE se.enrollment_status END,
			  updated_at = now()
			FROM tenants t
			WHERE se.tenant_id = $1 AND t.tenant_id = $1 AND se.enrollment_status = 'ACTIVE'`,
			tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		// 2. Remove cohorts that have just completed (Sem 2, final year of their level).
		var cohortsCompleted int
		_ = tx.QueryRow(r.Context(), `
			WITH del AS (
			  DELETE FROM course_offerings o USING tenants t
			  WHERE o.tenant_id = $1 AND t.tenant_id = $1 AND o.semester = 2
			    AND o.study_year + 1 > COALESCE((SELECT (c.level_years ->> NULLIF(o.level,''))::int FROM courses c WHERE c.course_id = o.course_id AND c.tenant_id = o.tenant_id), (t.level_years ->> NULLIF(o.level,''))::int, 99)
			  RETURNING 1)
			SELECT COUNT(*) FROM del`, tenantID).Scan(&cohortsCompleted) //nolint:errcheck

		// 3. Advance the remaining cohorts (deferrable constraint absorbs the shift).
		var cohortsAdvanced int
		_ = tx.QueryRow(r.Context(), `
			WITH upd AS (
			  UPDATE course_offerings SET
			    semester   = CASE WHEN semester = 2 THEN 1 ELSE 2 END,
			    study_year = CASE WHEN semester = 2 THEN study_year + 1 ELSE study_year END
			  WHERE tenant_id = $1
			  RETURNING 1)
			SELECT COUNT(*) FROM upd`, tenantID).Scan(&cohortsAdvanced) //nolint:errcheck

		// 4. Move the tenant pointer to the new period.
		if _, err := tx.Exec(r.Context(),
			`UPDATE tenants SET active_academic_year = $1, active_semester = $2 WHERE tenant_id = $3`,
			newYear, newSem, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit failed"))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tenant_id": tenantID, "active_academic_year": newYear, "active_semester": newSem,
			"students_advanced": advanced, "students_graduated": graduated,
			"cohorts_advanced": cohortsAdvanced, "cohorts_completed": cohortsCompleted,
			"status": "ADVANCED",
		})
	}
}

// incAcademicYear bumps a "YYYY/YYYY" string by one year ("2025/2026" → "2026/2027");
// returns the input unchanged if it isn't in that form.
func incAcademicYear(s string) string {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 {
		return s
	}
	a := atoiSafe(strings.TrimSpace(parts[0]))
	b := atoiSafe(strings.TrimSpace(parts[1]))
	if a < 0 || b < 0 {
		return s
	}
	return fmt.Sprintf("%d/%d", a+1, b+1)
}

// derivePrefix builds a short institution prefix from its name when the admin
// hasn't set staff_id_prefix: initials of the words (e.g. "King Ceasor
// University" -> "KCU"), falling back to the first letters of the name.
func derivePrefix(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "ORG"
	}
	var initials []rune
	for _, word := range strings.Fields(name) {
		for _, ch := range word {
			if ch >= 'a' && ch <= 'z' {
				initials = append(initials, ch-32)
			} else if ch >= 'A' && ch <= 'Z' {
				initials = append(initials, ch)
			}
			break
		}
	}
	if len(initials) >= 2 {
		return string(initials)
	}
	// Single-word name: take up to 3 leading letters, upper-cased.
	up := strings.ToUpper(name)
	out := []rune{}
	for _, ch := range up {
		if ch >= 'A' && ch <= 'Z' {
			out = append(out, ch)
		}
		if len(out) == 3 {
			break
		}
	}
	if len(out) == 0 {
		return "ORG"
	}
	return string(out)
}

// joinComma joins strings with ", " for dynamic SQL SET clauses.
func joinComma(ss []string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += ", "
		}
		result += s
	}
	return result
}

// ─── Lecturers ────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/lecturers
func ListLecturers(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT lecturer_id::text, COALESCE(title,''), full_name, COALESCE(gender,''),
			       COALESCE(email,''), COALESCE(phone,''), COALESCE(department,''), COALESCE(staff_id,'')
			FROM lecturers WHERE tenant_id = $1 ORDER BY full_name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type lecturer struct {
			LecturerID string `json:"lecturer_id"`
			Title      string `json:"title"`
			FullName   string `json:"full_name"`
			Gender     string `json:"gender"`
			Email      string `json:"email"`
			Phone      string `json:"phone"`
			Department string `json:"department"`
			StaffID    string `json:"staff_id"`
		}
		var list []lecturer
		for rows.Next() {
			var l lecturer
			rows.Scan(&l.LecturerID, &l.Title, &l.FullName, &l.Gender, &l.Email, &l.Phone, &l.Department, &l.StaffID) //nolint:errcheck
			list = append(list, l)
		}
		if list == nil {
			list = []lecturer{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/lecturers
func CreateLecturer(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var req struct {
			FullName   string `json:"full_name"`
			Email      string `json:"email"`
			Phone      string `json:"phone"`
			Department string `json:"department"`
			StaffID    string `json:"staff_id"`
			Title      string `json:"title"`
			Gender     string `json:"gender"`
		}
		if err := decodeJSON(r, &req); err != nil || req.FullName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "full_name is required"))
			return
		}

		staffID := strings.TrimSpace(req.StaffID)
		insert := func(sid string) (string, error) {
			var id string
			e := adminPool.QueryRow(r.Context(), `
				INSERT INTO lecturers (tenant_id, full_name, email, phone, department, staff_id, title, gender)
				VALUES ($1, $2, NULLIF($3,''), NULLIF($4,''), NULLIF($5,''), NULLIF($6,''), NULLIF($7,''), NULLIF($8,''))
				RETURNING lecturer_id::text`,
				tenantID, req.FullName, req.Email, req.Phone, req.Department, sid, req.Title, req.Gender).Scan(&id)
			return id, e
		}

		var lecturerID string
		var err error
		if staffID != "" {
			// Manual staff ID — use as entered (the institution already has it).
			lecturerID, err = insert(staffID)
			if err != nil {
				if strings.Contains(err.Error(), "ux_lecturers_tenant_staffid") {
					writeJSON(w, http.StatusConflict, errBody("STAFF_ID_TAKEN", "that staff ID is already in use"))
					return
				}
				writeJSON(w, http.StatusConflict, errBody("CONFLICT", "could not create lecturer: "+err.Error()))
				return
			}
		} else {
			// Fallback: auto-generate <PREFIX>/STAFF/00001 using the tenant short name.
			var prefix, tname string
			_ = adminPool.QueryRow(r.Context(),
				`SELECT COALESCE(staff_id_prefix,''), COALESCE(name,'') FROM tenants WHERE tenant_id = $1`,
				tenantID).Scan(&prefix, &tname)
			prefix = strings.TrimSpace(prefix)
			if prefix == "" {
				prefix = derivePrefix(tname)
			}
			var cnt int
			_ = adminPool.QueryRow(r.Context(), `SELECT COUNT(*) FROM lecturers WHERE tenant_id = $1`, tenantID).Scan(&cnt)
			seq := cnt + 1
			for attempt := 0; attempt < 100; attempt++ {
				cand := fmt.Sprintf("%s/STAFF/%05d", prefix, seq)
				lecturerID, err = insert(cand)
				if err == nil {
					staffID = cand
					break
				}
				if strings.Contains(err.Error(), "ux_lecturers_tenant_staffid") {
					seq++ // collision — try the next number
					continue
				}
				writeJSON(w, http.StatusConflict, errBody("CONFLICT", "could not create lecturer: "+err.Error()))
				return
			}
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not allocate a staff ID"))
				return
			}
		}
		// Dispatch the lecturer's permanent career QR by email when one was given
		// (optional — email is only used for QR delivery, not for login).
		if strings.TrimSpace(req.Email) != "" {
			emailLecturerQR(r.Header.Get("Authorization"),
				lecturerScanURL(r, makeLecturerQRToken(lecturerID, tenantID)),
				req.Email, req.FullName, staffID)
		}
		writeJSON(w, http.StatusCreated, map[string]string{"lecturer_id": lecturerID, "staff_id": staffID, "status": "CREATED"})
	}
}

// PATCH /api/v1/admin/lecturers/{lecturer_id} — edit a lecturer (partial).
func UpdateLecturer(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lecturerID := chi.URLParam(r, "lecturer_id")
		var req struct {
			FullName   *string `json:"full_name"`
			Email      *string `json:"email"`
			Phone      *string `json:"phone"`
			Department *string `json:"department"`
			StaffID    *string `json:"staff_id"`
			Title      *string `json:"title"`
			Gender     *string `json:"gender"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed JSON"))
			return
		}
		setClauses := []string{}
		args := []interface{}{}
		n := 1
		add := func(col string, v *string) {
			if v == nil {
				return
			}
			setClauses = append(setClauses, fmt.Sprintf("%s = NULLIF($%d,'')", col, n))
			args = append(args, *v)
			n++
		}
		add("full_name", req.FullName)
		add("email", req.Email)
		add("phone", req.Phone)
		add("department", req.Department)
		add("staff_id", req.StaffID)
		add("title", req.Title)
		add("gender", req.Gender)
		if len(setClauses) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "no fields to update"))
			return
		}
		query := "UPDATE lecturers SET " + joinComma(setClauses) +
			fmt.Sprintf(" WHERE lecturer_id = $%d::uuid AND (%s)", n, tenantScopeClause(fmt.Sprintf("$%d", n+1)))
		args = append(args, lecturerID, tenantScope(r))
		tag, err := adminPool.Exec(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "lecturer not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"lecturer_id": lecturerID, "status": "UPDATED"})
	}
}

// GET /api/v1/admin/tenants/{tenant_id}/lecturer-assignments
func ListLecturerAssignments(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT la.assignment_id::text, la.lecturer_id::text, l.full_name,
			       la.unit_id, cu.name AS unit_name,
			       la.course_id, c.name AS course_name,
			       la.academic_year, la.year, la.semester, la.intake_session::text
			FROM lecturer_assignments la
			JOIN lecturers l    ON l.lecturer_id  = la.lecturer_id
			JOIN course_units cu ON cu.unit_id    = la.unit_id
			JOIN courses c       ON c.course_id   = la.course_id
			WHERE la.tenant_id = $1
			ORDER BY l.full_name, cu.name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type assignment struct {
			AssignmentID  string `json:"assignment_id"`
			LecturerID    string `json:"lecturer_id"`
			LecturerName  string `json:"lecturer_name"`
			UnitID        string `json:"unit_id"`
			UnitName      string `json:"unit_name"`
			CourseID      string `json:"course_id"`
			CourseName    string `json:"course_name"`
			AcademicYear  string `json:"academic_year"`
			Year          int    `json:"year"`
			Semester      int    `json:"semester"`
			IntakeSession string `json:"intake_session"`
		}
		var list []assignment
		for rows.Next() {
			var a assignment
			rows.Scan(&a.AssignmentID, &a.LecturerID, &a.LecturerName,
				&a.UnitID, &a.UnitName, &a.CourseID, &a.CourseName,
				&a.AcademicYear, &a.Year, &a.Semester, &a.IntakeSession) //nolint:errcheck
			list = append(list, a)
		}
		if list == nil {
			list = []assignment{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/lecturer-assignments
func CreateLecturerAssignment(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var req struct {
			LecturerID    string `json:"lecturer_id"`
			UnitID        string `json:"unit_id"`
			CourseID      string `json:"course_id"`
			AcademicYear  string `json:"academic_year"`
			Year          int    `json:"year"`
			Semester      int    `json:"semester"`
			IntakeSession string `json:"intake_session"`
		}
		if err := decodeJSON(r, &req); err != nil ||
			req.LecturerID == "" || req.UnitID == "" || req.CourseID == "" || req.AcademicYear == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"lecturer_id, unit_id, course_id and academic_year are required"))
			return
		}
		if req.Year == 0 {
			req.Year = 1
		}
		if req.Semester == 0 {
			req.Semester = 1
		}
		if req.IntakeSession == "" {
			req.IntakeSession = "Morning"
		}

		var assignmentID string
		err := adminPool.QueryRow(r.Context(), `
			INSERT INTO lecturer_assignments
			  (tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session)
			VALUES ($1, $2::uuid, $3, $4, $5, $6, $7, $8)
			RETURNING assignment_id::text`,
			tenantID, req.LecturerID, req.UnitID, req.CourseID,
			req.AcademicYear, req.Year, req.Semester, req.IntakeSession).Scan(&assignmentID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT",
				"assignment already exists or invalid reference: "+err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"assignment_id": assignmentID, "status": "CREATED"})
	}
}

// DELETE /api/v1/admin/lecturer-assignments/{assignment_id}
func DeleteLecturerAssignment(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assignmentID := chi.URLParam(r, "assignment_id")

		// IDOR guard: confine non-SUPER_ADMIN callers to their own tenant's assignment.
		tag, err := adminPool.Exec(r.Context(),
			`DELETE FROM lecturer_assignments WHERE assignment_id = $1::uuid AND ($2 = '' OR tenant_id::text = $2)`,
			assignmentID, tenantScope(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "assignment not found in your tenant"))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// ─── Students ─────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/students
func ListStudents(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT se.student_id, se.full_name, se.email,
			       se.course_id, c.name AS course_name,
			       se.current_year, se.semester, se.academic_year,
			       se.intake_session::text, se.enrollment_status::text,
			       COALESCE(se.level,'')
			FROM students_extended se
			JOIN courses c ON c.course_id = se.course_id
			WHERE se.tenant_id = $1
			ORDER BY se.full_name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type student struct {
			StudentID        string `json:"student_id"`
			FullName         string `json:"full_name"`
			Email            string `json:"email"`
			CourseID         string `json:"course_id"`
			CourseName       string `json:"course_name"`
			CurrentYear      int    `json:"current_year"`
			Semester         int    `json:"semester"`
			AcademicYear     string `json:"academic_year"`
			IntakeSession    string `json:"intake_session"`
			EnrollmentStatus string `json:"enrollment_status"`
			Level            string `json:"level"`
		}
		var list []student
		for rows.Next() {
			var s student
			rows.Scan(&s.StudentID, &s.FullName, &s.Email, &s.CourseID, &s.CourseName,
				&s.CurrentYear, &s.Semester, &s.AcademicYear, &s.IntakeSession, &s.EnrollmentStatus,
				&s.Level) //nolint:errcheck
			list = append(list, s)
		}
		if list == nil {
			list = []student{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/students
// Creates both a users record (for portal login) and a students_extended record (for attendance tracking).
func CreateStudent(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		var req struct {
			StudentID       string `json:"student_id"` // registration number e.g. "NUT/CS/2024/001"
			FullName        string `json:"full_name"`
			Email           string `json:"email"`
			Password        string `json:"password"`
			CourseID        string `json:"course_id"`
			OfferingID      string `json:"offering_id"` // session the student joins (course+session)
			Level           string `json:"level"`        // the student's level of study in this course
			CurrentYear     int    `json:"current_year"`
			Semester        int    `json:"semester"`
			AcademicYear    string `json:"academic_year"`
			IntakeSession   string `json:"intake_session"`   // Jan/May/Aug intake
			AdditionalEmail string `json:"additional_email"` // optional 2nd address for the QR
		}
		if err := decodeJSON(r, &req); err != nil ||
			req.StudentID == "" || req.FullName == "" || req.AcademicYear == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST",
				"student_id, full_name and academic_year are required"))
			return
		}
		// A student joins an OFFERING (session); its program supplies course_id (units).
		// offering_id is preferred; course_id alone is still accepted (e.g. CSV import).
		if req.OfferingID != "" {
			var progCourse string
			if err := adminPool.QueryRow(r.Context(),
				`SELECT course_id FROM course_offerings WHERE offering_id = $1::uuid AND tenant_id = $2`,
				req.OfferingID, tenantID).Scan(&progCourse); err != nil {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_OFFERING", "offering not found for this tenant"))
				return
			}
			req.CourseID = progCourse
		}
		if req.CourseID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "offering_id or course_id is required"))
			return
		}
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
		// Track whether a genuine address was supplied — the QR is emailed only then
		// (email is optional, used solely for QR dispatch; identity is the reg-no).
		realEmail := req.Email != ""

		var domain string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT domain FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&domain); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		domain = strings.ToLower(strings.TrimSpace(domain))

		// Students are identified by their reg-no; we no longer ask for email/password.
		// Synthesise an in-domain login from the reg-no so the QR + check-in path (which
		// resolve identity through the users table) keep working unchanged. An explicit
		// email is still accepted for back-compat and must match the domain.
		if req.Email == "" {
			req.Email = synthEmail(req.StudentID, domain)
		} else if !emailInDomain(req.Email, domain) {
			writeJSON(w, http.StatusBadRequest, errBody("EMAIL_DOMAIN_MISMATCH",
				"student email must end with the institution domain @"+domain+" (a sub-domain like @studmc."+domain+" is allowed)"))
			return
		}

		if req.Password == "" {
			req.Password = randPassword() // students never log in with a password; the QR is their key
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "password hash failed"))
			return
		}

		if req.IntakeSession == "" {
			req.IntakeSession = "Morning"
		}
		if req.CurrentYear == 0 {
			req.CurrentYear = 1
		}
		if req.Semester == 0 {
			req.Semester = 1
		}

		// Use a transaction so both inserts succeed or both roll back.
		tx, err := adminPool.Begin(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "tx failed"))
			return
		}
		defer tx.Rollback(r.Context()) //nolint:errcheck

		// 1. Create login record in users table.
		var userID string
		err = tx.QueryRow(r.Context(), `
			INSERT INTO users (tenant_id, email, password_hash, role, full_name)
			VALUES ($1, $2, $3, 'STUDENT', $4)
			RETURNING user_id::text`,
			tenantID, req.Email, string(hash), req.FullName).Scan(&userID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT",
				fmt.Sprintf("email already registered or tenant not found: %v", err)))
			return
		}

		// 2. Create extended profile record (bound to the offering when supplied).
		_, err = tx.Exec(r.Context(), `
			INSERT INTO students_extended
			  (student_id, tenant_id, full_name, email, course_id, offering_id,
			   intake_session, current_year, semester, academic_year, level, enrollment_status)
			VALUES ($1, $2, $3, $4, $5, NULLIF($6,'')::uuid, $7, $8, $9, $10, NULLIF($11,''), 'ACTIVE')`,
			req.StudentID, tenantID, req.FullName, req.Email, req.CourseID, req.OfferingID,
			req.IntakeSession, req.CurrentYear, req.Semester, req.AcademicYear, req.Level)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("CONFLICT",
				fmt.Sprintf("student_id already exists or course not found: %v", err)))
			return
		}

		if err := tx.Commit(r.Context()); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "commit failed"))
			return
		}

		// Generate + email the student's QR only when a real destination exists (the
		// registered email or an explicit additional address). Fire-and-forget so
		// registration is never blocked by mail latency; qr-generator verifies the
		// forwarded JWT and derives the tenant itself (#4c). Reg-no-only students
		// need no email — their identity (and portal) is the reg-no.
		if realEmail || strings.TrimSpace(req.AdditionalEmail) != "" {
			issueStudentQR(r.Header.Get("Authorization"), req.StudentID, req.AdditionalEmail)
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"student_id":  req.StudentID,
			"user_id":     userID,
			"status":      "CREATED",
			"qr_delivery": "QUEUED",
		})
	}
}

// synthEmail builds a stable in-domain login address from a student's reg-no, so
// students get a working (hidden) identity for the QR / check-in path without the
// admin ever entering an email. e.g. "NUT/CS/2024/001" → "nut.cs.2024.001@domain".
func synthEmail(studentID, domain string) string {
	var b strings.Builder
	prevDot := false
	for _, r := range strings.ToLower(studentID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDot = false
		} else if !prevDot {
			b.WriteByte('.')
			prevDot = true
		}
	}
	s := strings.Trim(b.String(), ".")
	if s == "" {
		s = "student"
	}
	return s + "@" + domain
}

// issueStudentQR asks qr-generator to mint + email the student's QR code right
// after registration. Best-effort and asynchronous: failures are logged, never
// surfaced to the registrar, so a mail hiccup can't fail a registration (#4c).
func issueStudentQR(authHeader, studentID, additionalEmail string) {
	if authHeader == "" || studentID == "" {
		return
	}
	base := os.Getenv("QR_GENERATOR_URL")
	if base == "" {
		base = "http://qr-generator:3002"
	}
	body, _ := json.Marshal(map[string]string{
		"student_id":       studentID,
		"additional_email": additionalEmail,
	})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
			strings.TrimRight(base, "/")+"/api/v1/qr/issue", bytes.NewReader(body))
		if err != nil {
			log.Printf("[qr-issue] build request failed for %s: %v", studentID, err)
			return
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", authHeader)
		resp, err := http.DefaultClient.Do(httpReq)
		if err != nil {
			log.Printf("[qr-issue] request failed for %s: %v", studentID, err)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode >= 300 {
			log.Printf("[qr-issue] qr-generator returned %d for %s", resp.StatusCode, studentID)
		}
	}()
}

// ─── Lecturer Attendance ──────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/lecturer-attendance
// Returns all lecturer_attendance_log rows for the tenant, joined to lecturer
// names, unit names, and session status. Ordered newest-first.
func GetLecturerAttendanceLogs(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT
			    lal.log_id::text,
			    lal.lecturer_id,
			    COALESCE(l.full_name, lal.lecturer_id)  AS lecturer_name,
			    COALESCE(l.department, '')               AS department,
			    lal.unit_id,
			    COALESCE(cu.name, lal.unit_id)           AS unit_name,
			    lal.session_date,
			    lal.gate_open_time,
			    lal.gate_close_time,
			    COALESCE(lal.contact_hours, 0),
			    COALESCE(s.session_status::text, 'UNKNOWN')
			FROM lecturer_attendance_logs lal
			LEFT JOIN lecturers  l  ON l.lecturer_id::text = lal.lecturer_id
			                       AND l.tenant_id = lal.tenant_id
			LEFT JOIN course_units cu ON cu.unit_id = lal.unit_id
			LEFT JOIN sessions     s  ON s.session_id = lal.session_id
			WHERE lal.tenant_id = $1
			ORDER BY lal.session_date DESC, lal.gate_open_time DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type logRow struct {
			LogID          string  `json:"log_id"`
			LecturerID     string  `json:"lecturer_id"`
			LecturerName   string  `json:"lecturer_name"`
			Department     string  `json:"department"`
			UnitID         string  `json:"unit_id"`
			UnitName       string  `json:"unit_name"`
			SessionDate    string  `json:"session_date"`
			GateOpenTime   string  `json:"gate_open_time"`
			GateCloseTime  string  `json:"gate_close_time"`
			ContactHours   float64 `json:"contact_hours"`
			SessionStatus  string  `json:"session_status"`
		}
		var list []logRow
		for rows.Next() {
			var lr logRow
			var sessionDate time.Time
			var gateOpen time.Time
			var gateClose *time.Time
			var contactHours float64
			rows.Scan(&lr.LogID, &lr.LecturerID, &lr.LecturerName, &lr.Department,
				&lr.UnitID, &lr.UnitName, &sessionDate,
				&gateOpen, &gateClose, &contactHours, &lr.SessionStatus) //nolint:errcheck
			lr.SessionDate   = sessionDate.Format("2006-01-02")
			lr.GateOpenTime  = gateOpen.Format(time.RFC3339)
			lr.ContactHours  = contactHours
			if gateClose != nil {
				lr.GateCloseTime = gateClose.Format(time.RFC3339)
			}
			list = append(list, lr)
		}
		if list == nil {
			list = []logRow{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}

// GET /api/v1/admin/tenants/{tenant_id}/lecturer-attendance/summary
// Returns per-lecturer aggregate: total sessions attended, total contact hours,
// average contact hours per session, and last session date.
func GetLecturerAttendanceSummary(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")

		rows, err := adminPool.Query(r.Context(), `
			SELECT
			    lal.lecturer_id,
			    COALESCE(l.full_name, lal.lecturer_id)   AS lecturer_name,
			    COALESCE(l.department, '')                AS department,
			    COALESCE(l.email, '')                     AS email,
			    COUNT(*)                                   AS total_sessions,
			    COALESCE(SUM(lal.contact_hours), 0)       AS total_contact_hours,
			    COALESCE(AVG(lal.contact_hours), 0)       AS avg_contact_hours,
			    MAX(lal.session_date)                      AS last_session_date
			FROM lecturer_attendance_logs lal
			LEFT JOIN lecturers l ON l.lecturer_id::text = lal.lecturer_id
			                     AND l.tenant_id = lal.tenant_id
			WHERE lal.tenant_id = $1
			GROUP BY lal.lecturer_id, l.full_name, l.department, l.email
			ORDER BY total_sessions DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type summaryRow struct {
			LecturerID        string  `json:"lecturer_id"`
			LecturerName      string  `json:"lecturer_name"`
			Department        string  `json:"department"`
			Email             string  `json:"email"`
			TotalSessions     int     `json:"total_sessions"`
			TotalContactHours float64 `json:"total_contact_hours"`
			AvgContactHours   float64 `json:"avg_contact_hours"`
			LastSessionDate   string  `json:"last_session_date"`
		}
		var list []summaryRow
		for rows.Next() {
			var sr summaryRow
			var lastDate *time.Time
			rows.Scan(&sr.LecturerID, &sr.LecturerName, &sr.Department, &sr.Email,
				&sr.TotalSessions, &sr.TotalContactHours, &sr.AvgContactHours, &lastDate) //nolint:errcheck
			if lastDate != nil {
				sr.LastSessionDate = lastDate.Format("2006-01-02")
			}
			list = append(list, sr)
		}
		if list == nil {
			list = []summaryRow{}
		}
		writeJSON(w, http.StatusOK, list)
	}
}
