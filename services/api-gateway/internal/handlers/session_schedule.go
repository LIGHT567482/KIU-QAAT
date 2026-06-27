package handlers

// Per-unit session schedule (#2).
//
// The COORDINATOR sets a unit's lecture start time + length ONCE; it then locks
// and is reused for every future session of that unit. Only the institution ADMIN
// can change a locked schedule (via UpdateCourseUnit). The session inherits the
// schedule (planned_start/planned_duration) so student & lecturer check-in
// timestamps can be judged against the planned window.

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/middleware"
)

type unitSchedule struct {
	UnitID          string `json:"unit_id"`
	DayOfWeek       int    `json:"day_of_week"`              // 1=Mon…7=Sun, 0 when unset
	SessionStart    string `json:"session_start"`            // "HH:MM" or ""
	DurationMinutes int    `json:"session_duration_minutes"` // 0 when unset
	Locked          bool   `json:"schedule_locked"`
}

// coordinatorOffering resolves the single offering the coordinator owns.
func coordinatorOffering(ctx context.Context, conn *pgxpool.Conn, coordID, tenantID string) (string, error) {
	var offeringID string
	err := conn.QueryRow(ctx,
		`SELECT offering_id::text FROM course_offerings WHERE coordinator_id = $1 AND tenant_id = $2`,
		coordID, tenantID).Scan(&offeringID)
	return offeringID, err
}

// GET /api/v1/coordinator/units/{unit_id}/schedule
// The schedule (day + time + length) is per-OFFERING: morning and evening sessions
// of the same program study the same unit at different times.
func GetUnitSchedule(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		unitID := chi.URLParam(r, "unit_id")

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		offeringID, err := coordinatorOffering(r.Context(), conn, coordID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NO_OFFERING", "you are not assigned to a session"))
			return
		}

		var s unitSchedule
		s.UnitID = unitID
		err = conn.QueryRow(r.Context(), `
			SELECT COALESCE(day_of_week, 0),
			       COALESCE(to_char(session_start, 'HH24:MI'), ''),
			       COALESCE(session_duration_minutes, 0),
			       schedule_locked
			FROM offering_unit_schedules
			WHERE offering_id = $1::uuid AND unit_id = $2 AND tenant_id = $3`,
			offeringID, unitID, tenantID).Scan(&s.DayOfWeek, &s.SessionStart, &s.DurationMinutes, &s.Locked)
		if err == pgx.ErrNoRows {
			// Not configured yet — return an unset, unlocked schedule.
			writeJSON(w, http.StatusOK, s)
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

// PUT /api/v1/coordinator/units/{unit_id}/schedule
// Sets the schedule for the coordinator's offering ONLY if not yet locked; locks it
// on success. A coordinator cannot change a locked schedule — reserved for the ADMIN.
func SetUnitSchedule(pool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		unitID := chi.URLParam(r, "unit_id")
		var req struct {
			DayOfWeek       int    `json:"day_of_week"` // 1=Mon…7=Sun (0 = unset)
			SessionStart    string `json:"session_start"`
			DurationMinutes int    `json:"session_duration_minutes"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		if !validHHMM(req.SessionStart) || req.DurationMinutes < 5 || req.DurationMinutes > 600 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SCHEDULE", "valid start time (HH:MM) and a duration of 5–600 minutes are required"))
			return
		}
		if req.DayOfWeek < 0 || req.DayOfWeek > 7 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_SCHEDULE", "day_of_week must be 1–7 (or 0 for unset)"))
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		offeringID, err := coordinatorOffering(r.Context(), conn, coordID, tenantID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NO_OFFERING", "you are not assigned to a session"))
			return
		}

		// Upsert + lock-on-set: a locked row is left untouched (0 rows → 409).
		ct, err := conn.Exec(r.Context(), `
			INSERT INTO offering_unit_schedules
			    (offering_id, unit_id, tenant_id, day_of_week, session_start, session_duration_minutes, schedule_locked)
			VALUES ($1::uuid, $2, $3, NULLIF($4,0), $5::time, $6, true)
			ON CONFLICT (offering_id, unit_id) DO UPDATE
			   SET day_of_week = EXCLUDED.day_of_week,
			       session_start = EXCLUDED.session_start,
			       session_duration_minutes = EXCLUDED.session_duration_minutes,
			       schedule_locked = true
			   WHERE offering_unit_schedules.schedule_locked = false`,
			offeringID, unitID, tenantID, req.DayOfWeek, req.SessionStart, req.DurationMinutes)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusConflict, errBody("SCHEDULE_LOCKED",
				"this unit's schedule is already set and locked — ask your institution admin to change it"))
			return
		}
		// Clear this coordinator's cached daily manifest so the new schedule shows
		// on their dashboard immediately rather than at the next midnight rollover.
		bustCoordinatorManifest(r.Context(), rdb, tenantID, coordID)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"unit_id": unitID, "day_of_week": req.DayOfWeek, "session_start": req.SessionStart,
			"session_duration_minutes": req.DurationMinutes, "schedule_locked": true,
		})
	}
}

// validHHMM accepts "H:MM"/"HH:MM" 24-hour time.
func validHHMM(s string) bool {
	if len(s) < 4 || len(s) > 5 {
		return false
	}
	c := -1
	for i, ch := range s {
		if ch == ':' {
			c = i
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
	}
	if c < 1 {
		return false
	}
	h := atoiSafe(s[:c])
	m := atoiSafe(s[c+1:])
	return h >= 0 && h <= 23 && m >= 0 && m <= 59
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}
