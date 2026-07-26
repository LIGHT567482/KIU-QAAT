package handlers

// Tenant timetable — the per-(offering, unit) weekly schedule (day + start + length)
// that COORDINATORS fill (set-once/lock via the coordinator schedule endpoints),
// surfaced to the ADMIN and QA OFFICER dashboards with OVERRIDE power.
//
//   GET /api/v1/dashboard/timetable   (ADMIN, QA_OFFICER) — every offering's units
//        with their scheduled day/time (or unscheduled).
//   PUT /api/v1/dashboard/timetable   (ADMIN, QA_OFFICER) — set/override one unit's
//        schedule for an offering, ignoring the coordinator's lock.
//
// This is the single source of truth the coordinator's own timetable + daily
// manifest read from, so a unit scheduled here shows up on the coordinator's day.

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

type timetableRow struct {
	OfferingID      string `json:"offering_id"`
	CourseID        string `json:"course_id"`
	CourseName      string `json:"course_name"`
	Department      string `json:"department"`
	School          string `json:"school"`
	SessionType     string `json:"session_type"`
	StudyYear       int    `json:"study_year"` // the cohort's year of study
	CohortSemester  int    `json:"cohort_semester"`
	Level           string `json:"level"`
	Intake          string `json:"intake"`
	CoordinatorName string `json:"coordinator_name"`
	UnitID          string `json:"unit_id"`
	UnitName        string `json:"unit_name"`
	Year            int    `json:"year"`
	Semester        int    `json:"semester"`
	DayOfWeek       int    `json:"day_of_week"`              // 1=Mon…7=Sun, 0 = unscheduled
	SessionStart    string `json:"session_start"`            // "HH:MM" or ""
	DurationMinutes int    `json:"session_duration_minutes"` // 0 when unset
	Locked          bool   `json:"schedule_locked"`
}

// GET /api/v1/dashboard/timetable
func TimetableOverview(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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

		rows, err := conn.Query(r.Context(), `
			SELECT o.offering_id::text, o.course_id, c.name AS course_name,
			       COALESCE(c.department,''), COALESCE(c.school,''), o.session_type,
			       o.study_year, o.semester, COALESCE(o.level,''), COALESCE(o.intake,''),
			       COALESCE(u.full_name, '') AS coordinator_name,
			       COALESCE(cu.unit_id, ''), COALESCE(cu.name, ''),
			       COALESCE(cu.year, o.study_year), COALESCE(cu.semester, o.semester),
			       COALESCE(ous.day_of_week, 0),
			       COALESCE(to_char(ous.session_start, 'HH24:MI'), ''),
			       COALESCE(ous.session_duration_minutes, 0),
			       COALESCE(ous.schedule_locked, false)
			FROM course_offerings o
			JOIN courses c       ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
			-- LEFT JOIN so every cohort shows even when its curriculum has no units yet,
			-- and a tolerant level match so empty/legacy level values don't drop units.
			LEFT JOIN course_units cu ON cu.course_id = o.course_id AND cu.tenant_id = o.tenant_id
			       AND cu.year = o.study_year AND cu.semester = o.semester
			       AND (cu.level = o.level
			            OR COALESCE(NULLIF(cu.level,''),'') = ''
			            OR COALESCE(NULLIF(o.level,''),'')  = '')
			LEFT JOIN users u ON u.user_id::text = o.coordinator_id AND u.tenant_id = o.tenant_id
			LEFT JOIN offering_unit_schedules ous
			       ON ous.offering_id = o.offering_id AND ous.unit_id = cu.unit_id AND ous.tenant_id = o.tenant_id
			WHERE o.tenant_id = $1
			ORDER BY c.name, o.session_type, o.study_year, o.semester, o.level, o.intake, cu.name NULLS FIRST`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		out := []timetableRow{}
		for rows.Next() {
			var t timetableRow
			rows.Scan(&t.OfferingID, &t.CourseID, &t.CourseName, &t.Department, &t.School, &t.SessionType,
				&t.StudyYear, &t.CohortSemester, &t.Level, &t.Intake, &t.CoordinatorName,
				&t.UnitID, &t.UnitName, &t.Year, &t.Semester,
				&t.DayOfWeek, &t.SessionStart, &t.DurationMinutes, &t.Locked) //nolint:errcheck
			out = append(out, t)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// PUT /api/v1/dashboard/timetable
// Admin / QA override: upserts a unit's schedule for an offering, ignoring the
// coordinator's lock. day_of_week 0 clears the day.
func SetTimetableSchedule(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var req struct {
			OfferingID      string `json:"offering_id"`
			UnitID          string `json:"unit_id"`
			DayOfWeek       int    `json:"day_of_week"` // 1=Mon…7=Sun (0 = unset)
			SessionStart    string `json:"session_start"`
			DurationMinutes int    `json:"session_duration_minutes"`
		}
		if err := decodeJSON(r, &req); err != nil || req.OfferingID == "" || req.UnitID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "offering_id and unit_id are required"))
			return
		}
		if req.DayOfWeek < 0 || req.DayOfWeek > 7 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SCHEDULE", "day_of_week must be 1–7 (or 0 for unset)"))
			return
		}
		if req.SessionStart != "" && !validHHMM(req.SessionStart) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SCHEDULE", "session_start must be HH:MM"))
			return
		}
		if req.DurationMinutes < 0 || req.DurationMinutes > 600 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SCHEDULE", "duration must be 0–600 minutes"))
			return
		}

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

		// Confirm the offering belongs to this tenant (RLS already scopes, but give a
		// clean 404 rather than a silent no-op) and grab its coordinator so we can
		// bust their cached manifest after the change.
		var coordinatorID string
		if err := conn.QueryRow(r.Context(),
			`SELECT COALESCE(coordinator_id,'') FROM course_offerings WHERE offering_id = $1::uuid AND tenant_id = $2`,
			req.OfferingID, tenantID).Scan(&coordinatorID); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "offering not found"))
			return
		}

		// Override upsert — no lock check: admin/QA can always change the schedule.
		_, err = conn.Exec(r.Context(), `
			INSERT INTO offering_unit_schedules
			    (offering_id, unit_id, tenant_id, day_of_week, session_start, session_duration_minutes, schedule_locked)
			VALUES ($1::uuid, $2, $3, $4, NULLIF($5,'')::time, $6, true)
			ON CONFLICT (offering_id, unit_id) DO UPDATE
			   SET day_of_week              = EXCLUDED.day_of_week,
			       session_start            = EXCLUDED.session_start,
			       session_duration_minutes = EXCLUDED.session_duration_minutes,
			       schedule_locked          = true`,
			req.OfferingID, req.UnitID, tenantID, req.DayOfWeek, req.SessionStart, req.DurationMinutes)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		// The coordinator's daily manifest is cached until midnight — clear it so
		// the new schedule is reflected on their dashboard right away.
		bustCoordinatorManifest(r.Context(), rdb, tenantID, coordinatorID)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"offering_id": req.OfferingID, "unit_id": req.UnitID,
			"day_of_week": req.DayOfWeek, "session_start": req.SessionStart,
			"session_duration_minutes": req.DurationMinutes, "schedule_locked": true,
		})
	}
}
