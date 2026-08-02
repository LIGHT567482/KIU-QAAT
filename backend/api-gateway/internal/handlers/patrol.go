package handlers

// QA patroller endpoints (Phase 3).
//
//   POST /api/v1/patrol/bind-device — claim this handset for the signed-in patroller.
//   GET  /api/v1/patrol/manifest    — today's timetable (unit↔lecturer↔room↔time) so the offline
//                                     patrol screen can infer the rest from a chosen unit/room.
//   POST /api/v1/patrol/sync        — ingest a batch of patrol logs (was the lecturer teaching).
//
// Role: QA_PATROLLER. Everything is tenant-scoped via RLS.
//
// On top of the role check, every route here is bound to ONE handset. A patrol record states that
// a named lecturer was or was not teaching and feeds the QA reports as the independent second
// record, so possession of a valid token is deliberately not enough: the call must also come from
// the phone the patroller claimed. See migration 069 for why the binding is shaped as it is.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// deviceFingerprint reads the handset id the app stamps on every patrol call.
func deviceFingerprint(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Device-Fingerprint"))
}

var errDeviceMismatch = errors.New("device mismatch")

// checkPatrolDevice verifies that this request came from the handset bound to this patroller.
//
// It never binds — binding is an explicit act, done once by BindPatrolDevice. A patroller with no
// binding at all is refused rather than silently trusted, because the sync endpoint is exactly
// where a lifted token would be replayed, and "no binding yet" is indistinguishable from "binding
// was released because the phone was stolen".
func checkPatrolDevice(r *http.Request, conn *pgxpool.Conn, tenantID, userID string) error {
	fp := deviceFingerprint(r)
	if fp == "" {
		return errDeviceMismatch
	}
	var bound string
	err := conn.QueryRow(r.Context(),
		`SELECT device_fingerprint_hash FROM patroller_device_bindings
		  WHERE tenant_id = $1 AND user_id = $2::uuid`, tenantID, userID).Scan(&bound)
	if err != nil || bound == "" || bound != fp {
		return errDeviceMismatch
	}
	_, _ = conn.Exec(r.Context(),
		`UPDATE patroller_device_bindings SET last_seen_at = now()
		  WHERE tenant_id = $1 AND user_id = $2::uuid`, tenantID, userID)
	return nil
}

// writeDeviceRefusal answers a handset the patroller is not registered on. The message is written
// for the person holding the phone, since it is the only thing they will see.
func writeDeviceRefusal(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, errBody("DEVICE_NOT_BOUND",
		"This phone is not the one registered to your patrol account. Ask an administrator to release your device binding if you have changed phones."))
}

// BindPatrolDevice claims this handset for the signed-in patroller (trust on first use), and is a
// no-op confirmation when called again from the same phone. A call from a DIFFERENT phone, or
// from a phone already claimed by another patroller, is refused — releasing a binding is an admin
// action, so a patroller cannot walk their own account onto a new device unsupervised.
func BindPatrolDevice(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context())
		fp := deviceFingerprint(r)
		if fp == "" {
			var body struct {
				DeviceFingerprint string `json:"device_fingerprint"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			fp = strings.TrimSpace(body.DeviceFingerprint)
		}
		if fp == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "device fingerprint is required"))
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

		// Already bound? Only the same handset may continue.
		var bound string
		switch err := conn.QueryRow(r.Context(),
			`SELECT device_fingerprint_hash FROM patroller_device_bindings
			  WHERE tenant_id = $1 AND user_id = $2::uuid`, tenantID, userID).Scan(&bound); {
		case err == nil && bound == fp:
			_, _ = conn.Exec(r.Context(),
				`UPDATE patroller_device_bindings SET last_seen_at = now()
				  WHERE tenant_id = $1 AND user_id = $2::uuid`, tenantID, userID)
			writeJSON(w, http.StatusOK, map[string]interface{}{"status": "BOUND", "changed": false})
			return
		case err == nil:
			writeDeviceRefusal(w)
			return
		}

		// Unbound account: claim the handset, unless another patroller already holds it.
		//
		// Only a unique-constraint violation means "taken" — anything else is our problem, not the
		// patroller's, and must not be reported to them as a device conflict they cannot resolve.
		if _, err := conn.Exec(r.Context(), `
			INSERT INTO patroller_device_bindings (user_id, tenant_id, device_fingerprint_hash)
			VALUES ($2::uuid, $1, $3)`, tenantID, userID, fp); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				writeJSON(w, http.StatusForbidden, errBody("DEVICE_IN_USE",
					"This phone is already registered to another patrol account. Each patroller needs their own handset."))
				return
			}
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "BOUND", "changed": true})
	}
}

// ListPatrolBindings shows the administrator which patroller is on which handset, so a lost or
// replaced phone can be found and released. ADMIN only.
func ListPatrolBindings(pool *pgxpool.Pool) http.HandlerFunc {
	type row struct {
		UserID     string `json:"user_id"`
		FullName   string `json:"full_name"`
		Email      string `json:"email"`
		StaffID    string `json:"staff_id"`
		BoundAt    string `json:"bound_at"`
		LastSeenAt string `json:"last_seen_at"`
	}
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
		// The fingerprint itself is deliberately NOT returned — an administrator needs to know
		// that a binding exists and when it was last used, never the value that would let them
		// impersonate the handset.
		rows, err := conn.Query(r.Context(), `
			SELECT b.user_id::text, COALESCE(u.full_name,''), COALESCE(u.email,''),
			       COALESCE(u.staff_id,''), b.bound_at::text, b.last_seen_at::text
			FROM patroller_device_bindings b
			JOIN users u ON u.user_id = b.user_id
			WHERE b.tenant_id = $1
			ORDER BY b.last_seen_at DESC`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		out := make([]row, 0)
		for rows.Next() {
			var b row
			if rows.Scan(&b.UserID, &b.FullName, &b.Email, &b.StaffID, &b.BoundAt, &b.LastSeenAt) == nil {
				out = append(out, b)
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"bindings": out})
	}
}

// ReleasePatrolBinding frees a patroller to claim a new handset — the lost-phone path. ADMIN only,
// and audited by the router's AuditLog middleware, which is the point: a rebind is precisely the
// event you want a trail for when a patrol record is later disputed.
func ReleasePatrolBinding(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := strings.TrimSpace(chi.URLParam(r, "user_id"))
		if userID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "user_id is required"))
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
		tag, err := conn.Exec(r.Context(),
			`DELETE FROM patroller_device_bindings WHERE tenant_id = $1 AND user_id = $2::uuid`,
			tenantID, userID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		// A rebind is exactly the moment worth being able to look back at — see migration 069.
		if tag.RowsAffected() > 0 {
			auditAdmin(r, pool, tenantID, middleware.GetUserID(r.Context()),
				"PATROL_DEVICE_RELEASED", "user", userID,
				`{"note":"handset binding cleared; the patroller may claim a new phone on next sign-in"}`)
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status": "RELEASED", "released": tag.RowsAffected(),
		})
	}
}

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

		if err := checkPatrolDevice(r, conn, tenantID, middleware.GetUserID(r.Context())); err != nil {
			writeDeviceRefusal(w)
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

		if err := checkPatrolDevice(r, conn, tenantID, userID); err != nil {
			writeDeviceRefusal(w)
			return
		}
		deviceHash := deviceFingerprint(r)

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
				   session_date, scheduled_time, taught, patroller_id, patroller_name, patroller_staff_id,
				   taken_at, patroller_device_hash)
				VALUES ($1,$2,$3,$4,$5,$6,$7,
				        $8::date, $9, $10, $11::uuid, $12, $13,
				        COALESCE(NULLIF($14,'')::timestamptz, now()), $15)
				ON CONFLICT (tenant_id, unit_id, session_date, scheduled_time)
				DO UPDATE SET taught = EXCLUDED.taught,
				              patroller_id = EXCLUDED.patroller_id,
				              patroller_name = EXCLUDED.patroller_name,
				              patroller_staff_id = EXCLUDED.patroller_staff_id,
				              taken_at = EXCLUDED.taken_at,
				              patroller_device_hash = EXCLUDED.patroller_device_hash`,
				tenantID, l.UnitID, l.UnitName, l.CourseCode, l.LecturerID, l.LecturerName, l.Room,
				l.SessionDate, l.ScheduledTime, l.Taught, userID, patrollerName, patrollerStaffID,
				l.TakenAt, deviceHash)
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
