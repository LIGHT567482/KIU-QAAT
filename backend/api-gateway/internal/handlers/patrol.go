package handlers

// QA patroller endpoints (Phase 3).
//
//   GET  /api/v1/patrol/manifest  — today's timetable (unit↔lecturer↔room↔time) so the offline
//                                    patroller app can infer the rest from a chosen unit/lecturer/room.
//   POST /api/v1/patrol/sync      — ingest a batch of patrol logs (whether the lecturer was teaching).
//
// Role: QA_PATROLLER. Everything is tenant-scoped via RLS.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type patrolSlot struct {
	UnitID          string `json:"unit_id"`
	UnitName        string `json:"unit_name"`
	CourseCode      string `json:"course_code"`
	LecturerStaffID string `json:"lecturer_staff_id"`
	LecturerName    string `json:"lecturer_name"`
	Room            string `json:"room"`
	RoomCode        string `json:"room_code"` // managed room this slot resolved to, "" if free text only
	DayOfWeek       int    `json:"day_of_week"`
	StartTime       string `json:"start_time"` // "HH:MM"
	DurationMinutes int    `json:"duration_minutes"`
}

// PatrolManifest returns today's timetabled sessions for the patroller's tenant.
func PatrolManifest(pool *pgxpool.Pool) http.HandlerFunc {
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

		iso := int(time.Now().Weekday())
		if iso == 0 {
			iso = 7 // Sunday → 7
		}
		rows, err := conn.Query(r.Context(), `
			SELECT ts.unit_id, COALESCE(cu.name, ts.unit_id), COALESCE(cu.course_id, ''),
			       COALESCE(lec.staff_id, ''), COALESCE(lec.full_name, ''),
			       COALESCE(NULLIF(ts.room,''), ts.venue_id, ''), COALESCE(ts.venue_id,''), ts.day_of_week,
			       to_char(ts.start_time, 'HH24:MI'), COALESCE(ts.duration_minutes, 60)
			FROM timetable_slots ts
			JOIN course_units cu ON cu.unit_id = ts.unit_id AND cu.tenant_id = ts.tenant_id
			LEFT JOIN LATERAL (
			    SELECT l.staff_id, l.full_name
			    FROM lecturers l
			    WHERE l.tenant_id = ts.tenant_id
			      AND ( l.lecturer_id = ts.lecturer_id
			         OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
			               SELECT la.lecturer_id FROM lecturer_assignments la
			               WHERE la.unit_id = ts.unit_id AND la.tenant_id = ts.tenant_id
			               ORDER BY la.academic_year DESC LIMIT 1) ) )
			    LIMIT 1
			) lec ON true
			WHERE ts.tenant_id = $1 AND ts.day_of_week = $2
			ORDER BY ts.start_time`, tenantID, iso)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		slots := make([]patrolSlot, 0)
		for rows.Next() {
			var s patrolSlot
			if err := rows.Scan(&s.UnitID, &s.UnitName, &s.CourseCode, &s.LecturerStaffID,
				&s.LecturerName, &s.Room, &s.RoomCode, &s.DayOfWeek, &s.StartTime, &s.DurationMinutes); err != nil {
				continue
			}
			slots = append(slots, s)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"date":  time.Now().UTC().Format("2006-01-02"),
			"slots": slots,
		})
	}
}

type patrolLogIn struct {
	UnitID        string `json:"unit_id"`
	UnitName      string `json:"unit_name"`
	CourseCode    string `json:"course_code"`
	LecturerID    string `json:"lecturer_id"` // lecturer staff id
	LecturerName  string `json:"lecturer_name"`
	Room          string `json:"room"`
	SessionDate   string `json:"session_date"`   // YYYY-MM-DD
	ScheduledTime string `json:"scheduled_time"` // HH:MM
	Taught        bool   `json:"taught"`
	TakenAt       string `json:"taken_at"` // RFC3339
}

// PatrolSync ingests a batch of patrol logs, stamping the patroller's identity from the token.
// Idempotent per (tenant, unit, date, scheduled time): a re-tick UPDATEs the row.
func PatrolSync(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())

		var body struct {
			Logs []patrolLogIn `json:"logs"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
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

		// Resolve the patroller's display identity once.
		var patrollerName, patrollerStaffID string
		_ = conn.QueryRow(r.Context(),
			`SELECT COALESCE(full_name,''), COALESCE(staff_id,'') FROM users WHERE user_id = $1::uuid`,
			userID).Scan(&patrollerName, &patrollerStaffID)

		written := 0
		for _, l := range body.Logs {
			if l.UnitID == "" || l.SessionDate == "" {
				continue
			}
			_, execErr := conn.Exec(r.Context(), `
				INSERT INTO lecturer_patrol_logs
				  (tenant_id, unit_id, unit_name, course_code, lecturer_id, lecturer_name, room,
				   session_date, scheduled_time, taught, patroller_id, patroller_name, patroller_staff_id, taken_at)
				VALUES ($1,$2,$3,$4,$5,$6,$7,
				        $8::date, $9, $10, $11::uuid, $12, $13,
				        COALESCE(NULLIF($14,'')::timestamptz, now()))
				ON CONFLICT (tenant_id, unit_id, session_date, scheduled_time)
				DO UPDATE SET taught = EXCLUDED.taught,
				              patroller_id = EXCLUDED.patroller_id,
				              patroller_name = EXCLUDED.patroller_name,
				              patroller_staff_id = EXCLUDED.patroller_staff_id,
				              taken_at = EXCLUDED.taken_at`,
				tenantID, l.UnitID, l.UnitName, l.CourseCode, l.LecturerID, l.LecturerName, l.Room,
				l.SessionDate, l.ScheduledTime, l.Taught, userID, patrollerName, patrollerStaffID, l.TakenAt)
			if execErr == nil {
				written++
				// Persistent lecturer alert (stays in their inbox until they delete it): the patroller
				// recorded whether they were teaching. Best-effort — a failure never fails the sync.
				var lecUser string
				_ = conn.QueryRow(r.Context(),
					`SELECT user_id::text FROM lecturers
					 WHERE tenant_id = $1 AND btrim(lower(staff_id)) = btrim(lower($2)) AND user_id IS NOT NULL`,
					tenantID, l.LecturerID).Scan(&lecUser)
				if lecUser != "" {
					verdict := "NOT TAUGHT"
					if l.Taught {
						verdict = "TAUGHT"
					}
					subject := fmt.Sprintf("Patrol: %s", l.UnitName)
					bodyTxt := fmt.Sprintf("You were recorded as %s for %s%s%s by patroller %s.",
						verdict, l.UnitName,
						map[bool]string{true: " (" + l.CourseCode + ")", false: ""}[l.CourseCode != ""],
						map[bool]string{true: " in " + l.Room, false: ""}[l.Room != ""],
						patrollerName)
					var nid string
					if conn.QueryRow(r.Context(), `
						INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, subject, body)
						VALUES ($1, $2, $3, 'QA_PATROLLER', 'DIRECT', $4, $5) RETURNING notification_id::text`,
						tenantID, userID, patrollerName, subject, bodyTxt).Scan(&nid) == nil {
						_, _ = conn.Exec(r.Context(),
							`INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
							 VALUES ($1, $2, $3::uuid) ON CONFLICT DO NOTHING`, nid, tenantID, lecUser)
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "SYNCED", "records_written": written})
	}
}
