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
// THE ONE-HANDSET LOCK IS OFF. Every route here used to require that the call come from the phone
// the patroller had claimed (migration 069). That is commented out in checkPatrolDevice and
// BindPatrolDevice, and migration 078 drops the matching unique index — a patroller can now work
// from any phone, and a phone can be used by more than one patroller.
//
// The handset is still RECORDED on every call, so which phone filed a tick is answerable and the
// admin binding screens keep working. But it is no longer a factor: a patrol record accuses a
// named lecturer of not teaching, and the patrol PIN (migration 071) is now the only thing
// standing behind that accusation.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	// pgconn: only used by the one-device lock's DEVICE_IN_USE branch, which is
	// commented out in BindPatrolDevice. Re-add this import when restoring it.
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/clock"
	"github.com/qaat/api-gateway/internal/middleware"
)

// deviceFingerprint reads the handset id the app stamps on every patrol call.
func deviceFingerprint(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-Device-Fingerprint"))
}

var errDeviceMismatch = errors.New("device mismatch")

// checkPatrolDevice records which handset a patrol call came from.
//
// THE ONE-DEVICE LOCK IS COMMENTED OUT. It used to refuse any request whose
// X-Device-Fingerprint did not match the handset bound to this patroller, and refuse
// a patroller with no binding at all. That is off: a patroller who changes phone,
// borrows one, reinstalls the app, or whose fingerprint simply changes is otherwise
// dead in the water mid-round, and only an administrator can free them.
//
// What survives is the RECORD. The binding row is still written and its last_seen_at
// still updated, so "which phone filed this tick" remains answerable and the admin
// screens (ListPatrolBindings / ReleasePatrolBinding) keep working. Only the refusal
// is gone.
//
// WHAT THIS COSTS, stated plainly: the handset was one of the two factors behind a
// patrol tick, and a tick is an accusation that a named lecturer was or was not
// teaching. Without it, a lifted token can be replayed from any phone. The patrol PIN
// (migration 071) is now the only thing standing between a shared password and a
// false accusation — it is verified server-side, so it still holds, but it is holding
// alone.
//
// To restore: un-comment the block below and drop the `return nil`.
func checkPatrolDevice(r *http.Request, conn *pgxpool.Conn, tenantID, userID string) error {
	fp := deviceFingerprint(r)

	/*
		// ── one patroller, one handset (disabled) ─────────────────────────────────
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
	*/

	// Keep the trail: upsert this handset against the patroller so the record of which
	// phone is in use stays current even though nothing is refused. Best-effort — a
	// failure here must not block a round.
	if fp != "" {
		_, _ = conn.Exec(r.Context(), `
			INSERT INTO patroller_device_bindings (user_id, tenant_id, device_fingerprint_hash, last_seen_at)
			VALUES ($2::uuid, $1, $3, now())
			ON CONFLICT (user_id) DO UPDATE
			   SET device_fingerprint_hash = EXCLUDED.device_fingerprint_hash,
			       last_seen_at = now()`, tenantID, userID, fp)
	}
	return nil
}

// writeDeviceRefusal answers a handset the patroller is not registered on. The message is written
// for the person holding the phone, since it is the only thing they will see.
func writeDeviceRefusal(w http.ResponseWriter) {
	writeJSON(w, http.StatusForbidden, errBody("DEVICE_NOT_BOUND",
		"This phone is not the one registered to your monitor account. Ask an administrator to release your device binding if you have changed phones."))
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

		/*
			// ── the one-device lock (disabled — see checkPatrolDevice) ────────────────
			//
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
			// Only a unique-constraint violation means "taken" — anything else is our problem, not
			// the patroller's, and must not be reported to them as a device conflict they cannot
			// resolve.
			if _, err := conn.Exec(r.Context(), `
				INSERT INTO patroller_device_bindings (user_id, tenant_id, device_fingerprint_hash)
				VALUES ($2::uuid, $1, $3)`, tenantID, userID, fp); err != nil {
				var pgErr *pgconn.PgError
				if errors.As(err, &pgErr) && pgErr.Code == "23505" {
					writeJSON(w, http.StatusForbidden, errBody("DEVICE_IN_USE",
						"This phone is already registered to another monitor account. Each monitor needs their own handset."))
					return
				}
				writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
				return
			}
		*/

		// Binding is now a RECORD, not a claim: whichever handset the patroller is on
		// becomes the one on file. Nothing is refused — not a second phone, not a phone
		// another patroller has used. The app still calls this on entry, so the endpoint
		// keeps its contract and older builds continue to work unchanged.
		if _, err := conn.Exec(r.Context(), `
			INSERT INTO patroller_device_bindings (user_id, tenant_id, device_fingerprint_hash, last_seen_at)
			VALUES ($2::uuid, $1, $3, now())
			ON CONFLICT (user_id) DO UPDATE
			   SET device_fingerprint_hash = EXCLUDED.device_fingerprint_hash,
			       last_seen_at = now()`, tenantID, userID, fp); err != nil {
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
				`{"note":"handset binding cleared; the QA monitor may claim a new phone on next sign-in"}`)
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
	// Which cohort's session this is. Two intakes can run the same unit at the same
	// hour in different rooms; without this they are indistinguishable to the
	// patroller and collide on the log's uniqueness key.
	OfferingID string `json:"offering_id"`
	Cohort     string `json:"cohort"`

	// EVERY OTHER UNIT BEING TAUGHT IN THIS SAME HOUR, in this same room, by this same lecturer.
	//
	// One hour of teaching frequently satisfies several unit codes: the same content is required by
	// several programmes and each codes it differently. The search returned one of them — whichever
	// matched what the monitor typed — so the monitor ticked one unit and the students on the other
	// codes had a lecture with no QA record, while the lecturer got credit for one unit instead of
	// the several they actually delivered.
	//
	// Returned WITH the row rather than as separate results, because they are not separate lectures
	// to choose between: the monitor is standing in front of all of them at once.
	AlsoHere []patrolSlotUnit `json:"also_here"`
}

// patrolSlotUnit is one of the other unit codes running in the same room, hour and lecturer.
type patrolSlotUnit struct {
	UnitID     string `json:"unit_id"`
	UnitName   string `json:"unit_name"`
	CourseCode string `json:"course_code"`
	OfferingID string `json:"offering_id"`
	Cohort     string `json:"cohort"`
	Room       string `json:"room"`
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

		// The WHOLE WEEK, not just today.
		//
		// This used to be `WHERE ts.day_of_week = <today>`, and the phone cached whatever came
		// back. A patroller whose last signal was Monday then walked Tuesday's rounds searching
		// MONDAY's timetable — offline, with nothing on screen to say so, and every answer
		// confidently wrong. A week of slots is a few hundred rows; the phone filters to the
		// current weekday itself, so it is right on any day it is opened without signal.
		iso := clock.ISOWeekday()
		rows, err := conn.Query(r.Context(), `
			SELECT ts.unit_id, COALESCE(cu.name, ts.unit_id), COALESCE(cu.course_id, ''),
			       COALESCE(lec.staff_id, ''), COALESCE(lec.full_name, ''),
			       COALESCE(NULLIF(ts.room,''), ts.venue_id, ''), COALESCE(ts.venue_id,''), ts.day_of_week,
			       to_char(ts.start_time, 'HH24:MI'), COALESCE(ts.duration_minutes, 60),
			       -- WHICH COHORT. PatrolSearch has always returned these; this query never did,
			       -- and this is the one that fills the OFFLINE cache. So two intakes running the
			       -- same unit at the same hour in different rooms arrived indistinguishable: the
			       -- phone keyed them to the same cached slot and one silently replaced the other,
			       -- and the tick uploaded with a blank offering, which collides with the other
			       -- cohort's on the server's own ux_patrol_logs_slot. One lecture was unfindable
			       -- and one carried a verdict from a room nobody had visited.
			       COALESCE(ts.offering_id::text, ''),
			       COALESCE(NULLIF(CONCAT_WS(' · ', c.name, o.session_type,
			                                 'Yr' || o.study_year, 'Sem' || o.semester,
			                                 NULLIF(o.intake, '')), ''), '')
			FROM timetable_slots ts
			JOIN course_units cu ON cu.unit_id = ts.unit_id
			LEFT JOIN course_offerings o ON o.offering_id = ts.offering_id
			LEFT JOIN courses c ON c.course_id = o.course_id
			LEFT JOIN LATERAL (
			    SELECT l.staff_id, l.full_name
			    FROM lecturers l
			    WHERE ( l.lecturer_id = ts.lecturer_id
			         OR ( ts.lecturer_id IS NULL AND l.lecturer_id = (
			               SELECT la.lecturer_id FROM lecturer_assignments la
			               WHERE la.unit_id = ts.unit_id
			               ORDER BY la.academic_year DESC LIMIT 1) ) )
			    LIMIT 1
			) lec ON true
			WHERE ts.tenant_id = $1
			  -- A distance / e-learning cohort has no room to walk to (migration 087). Leaving
			  -- these on the round would send a monitor to an empty hall and produce a "not
			  -- taught" tick against a lecturer who was teaching online at the time — the exact
			  -- unfairness the provision-room announcement exists to prevent, arriving by another
			  -- door. Their lecturer's attendance is recorded by the online start/end instead.
			  AND COALESCE(o.delivery_mode, 'IN_PERSON') <> 'ONLINE'
			ORDER BY ts.day_of_week, ts.start_time`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		slots := make([]patrolSlot, 0)
		for rows.Next() {
			var s patrolSlot
			if err := rows.Scan(&s.UnitID, &s.UnitName, &s.CourseCode, &s.LecturerStaffID,
				&s.LecturerName, &s.Room, &s.RoomCode, &s.DayOfWeek, &s.StartTime, &s.DurationMinutes,
				&s.OfferingID, &s.Cohort); err != nil {
				continue
			}
			slots = append(slots, s)
		}
		rows.Close()

		// The companions travel with the OFFLINE round too. A monitor with no signal is exactly the
		// one who cannot look up what else is in the room, and a round that names one unit when
		// three are being delivered produces a record that understates the lecture.
		attachConcurrentUnits(r.Context(), conn, tenantID, slots)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"date": time.Now().UTC().Format("2006-01-02"),
			// Which weekday the server considers today (1=Mon…7=Sun). The phone filters the
			// week's slots by its own calendar, but a handset with a wrong clock — common on
			// the SIM-less phones this app runs on — would otherwise patrol the wrong day
			// with no way to notice.
			"day_of_week": iso,
			"slots":       slots,
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
	OfferingID    string `json:"offering_id"`

	// What the patroller ACTUALLY found, when it differed from the timetable.
	// Lecturers move rooms, and until now a tick could only say taught/not-taught
	// against the timetabled slot — so a lecture found in another room was either
	// recorded as if nothing had changed, or recorded as NOT TAUGHT, which is a
	// false accusation about the one thing QA exists to observe.
	FoundVenue     string `json:"found_venue"`
	FoundStartTime string `json:"found_start_time"` // HH:MM
	FoundDate      string `json:"found_date"`       // YYYY-MM-DD
	VenueChanged   bool   `json:"venue_changed"`
	Remarks        string `json:"remarks"`

	// A COMPENSATION lecture — one being taught to make good an earlier one that did not happen.
	// The monitor is the only witness who can establish this: they are standing in a room with a
	// lecture in it that the timetable says is empty, and the lecturer in front of them can say
	// what it is making up. Recorded here rather than inferred later, because after the round
	// nobody can tell a compensation from a lecture taught at the wrong time. CompensationFor is
	// the date being made good, and is optional — see migration 083.
	IsCompensation  bool   `json:"is_compensation"`
	CompensationFor string `json:"compensation_for"` // YYYY-MM-DD, "" if not known
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
				   taken_at, patroller_device_hash,
				   offering_id, found_venue, found_start_time, found_date, venue_changed, remarks,
				   is_compensation, compensation_for)
				VALUES ($1,$2,$3,$4,$5,$6,$7,
				        LEAST(COALESCE(NULLIF($8,'')::date, CURRENT_DATE), CURRENT_DATE),
				        $9, $10, $11::uuid, $12, $13,
				        -- The patroller works offline, so the phone's clock is the real
				        -- record of when a tick happened and a PAST timestamp is kept as
				        -- given. A FUTURE one is a wrong device clock, not evidence, so it
				        -- is clamped to now rather than filed under a day that hasn't
				        -- happened — where no report window would ever surface it.
				        LEAST(COALESCE(NULLIF($14,'')::timestamptz, now()), now()), $15,
				        NULLIF($16,'')::uuid, NULLIF($17,''), NULLIF($18,''),
				        NULLIF($19,'')::date, $20, NULLIF($21,''),
				        $22, NULLIF($23,'')::date)
				ON CONFLICT (tenant_id, unit_id, session_date, scheduled_time,
				             COALESCE(offering_id, '00000000-0000-0000-0000-000000000000'::uuid))
				DO UPDATE SET taught = EXCLUDED.taught,
				              patroller_id = EXCLUDED.patroller_id,
				              patroller_name = EXCLUDED.patroller_name,
				              patroller_staff_id = EXCLUDED.patroller_staff_id,
				              taken_at = EXCLUDED.taken_at,
				              patroller_device_hash = EXCLUDED.patroller_device_hash,
				              found_venue = EXCLUDED.found_venue,
				              found_start_time = EXCLUDED.found_start_time,
				              found_date = EXCLUDED.found_date,
				              venue_changed = EXCLUDED.venue_changed,
				              remarks = EXCLUDED.remarks,
				              -- A re-tick of the same slot may CORRECT a compensation in either
				              -- direction (the monitor learns what the lecture was, or that it was
				              -- not a compensation after all), so this takes the new value rather
				              -- than OR-ing it in and making the flag impossible to withdraw.
				              is_compensation = EXCLUDED.is_compensation,
				              compensation_for = EXCLUDED.compensation_for`,
				tenantID, l.UnitID, l.UnitName, l.CourseCode, l.LecturerID, l.LecturerName, l.Room,
				l.SessionDate, l.ScheduledTime, l.Taught, userID, patrollerName, patrollerStaffID,
				l.TakenAt, deviceHash,
				l.OfferingID, l.FoundVenue, l.FoundStartTime, l.FoundDate, l.VenueChanged, l.Remarks,
				l.IsCompensation, l.CompensationFor)
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
					// WHICH lecture, said in full. A lecturer teaching the same unit to three
					// cohorts used to get "recorded as NOT TAUGHT for Data Structures" and no way
					// to know which sitting was meant — and since ticks sync from a phone that may
					// have been offline for a day, the reader cannot assume it means today.
					when := lectureWhen(l.SessionDate, l.ScheduledTime)
					subject := fmt.Sprintf("Monitor: %s", l.UnitName)
					if when != "" {
						subject += " — " + when
					}
					// No patroller named — see patrolSenderName. The sentence is about the
					// lecture, and the identity of whoever walked the corridor is on the log row.
					bodyTxt := fmt.Sprintf("You were recorded as %s for %s%s%s%s.",
						verdict, l.UnitName,
						map[bool]string{true: " (" + l.CourseCode + ")", false: ""}[l.CourseCode != ""],
						map[bool]string{true: ", " + when, false: ""}[when != ""],
						map[bool]string{true: ", in " + l.Room, false: ""}[l.Room != ""])
					// A NOT TAUGHT message is an accusation, and until now it was a dead end: it
					// stated the finding and offered nothing to do about it, while the lecturer's
					// own account lived on a screen they had to know to go and find. It now carries
					// the reply with it, pointed at this exact lecture (migration 090), so the
					// answer is one tap from the thing being answered.
					action, actionRef := "", ""
					if !l.Taught {
						action = "APPEAL_NOT_TAUGHT"
						actionRef = l.UnitID + "|" + l.SessionDate + "|" + l.ScheduledTime
						bodyTxt += " If you did teach it, say so here — your account is filed " +
							"beside the monitor's and read with it. It does not overturn the tick " +
							"on its own; it makes sure there are two accounts and not one."
					}
					var nid string
					// sender_id NULL, and the impersonal name. NULL is not tidiness: the inbox
					// query LEFT JOINs users on sender_id to prefix the sender's title, so leaving
					// the patroller's id here would render "Mr. QA Monitor" — their honorific
					// pinned to the very name that replaced them.
					if conn.QueryRow(r.Context(), `
						INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, subject, body, action, action_ref)
						VALUES ($1, NULL, $2, 'QA_PATROLLER', 'DIRECT', $3, $4, NULLIF($5,''), NULLIF($6,'')) RETURNING notification_id::text`,
						tenantID, patrolSenderName, subject, bodyTxt, action, actionRef).Scan(&nid) == nil {
						_, _ = conn.Exec(r.Context(),
							`INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
							 VALUES ($1, $2, $3::uuid) ON CONFLICT DO NOTHING`, nid, tenantID, lecUser)
					}
				}

				// A lecture found away from its published slot is news to more people than
				// the lecturer: nobody updated the timetable, so students are still being
				// sent to the old room and the next patroller will repeat this search.
				// Best-effort, and never allowed to fail the sync — the tick is the record
				// that matters, the alert is a courtesy on top of it.
				if l.VenueChanged {
					if recips := venueChangeRecipients(r.Context(), conn, tenantID, l.UnitID, l.LecturerID); len(recips) > 0 {
						subj, bodyTxt := venueChangeMessage(l)
						var vid string
						// One row, several recipients — the lecturer AND their HOD, dean, QA
						// handler and the DQA — so it cannot name the patroller to the oversight
						// chain while hiding them from the lecturer. It names them to nobody; the
						// chain reads patroller_name off lecturer_patrol_logs in the QA reports,
						// where an audit trail belongs.
						if conn.QueryRow(r.Context(), `
							INSERT INTO app_notifications (tenant_id, sender_id, sender_name, sender_role, audience, unit_id, subject, body)
							VALUES ($1, NULL, $2, 'QA_PATROLLER', 'DIRECT', $3, $4, $5) RETURNING notification_id::text`,
							tenantID, patrolSenderName, l.UnitID, subj, bodyTxt).Scan(&vid) == nil {
							for _, uid := range recips {
								_, _ = conn.Exec(r.Context(),
									`INSERT INTO notification_recipients (notification_id, tenant_id, recipient_user_id)
									 VALUES ($1, $2, $3::uuid) ON CONFLICT DO NOTHING`, vid, tenantID, uid)
							}
						}
					}
				}
			}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"status": "SYNCED", "records_written": written})
	}
}

// ListPatrollers — GET /api/v1/dashboard/qa/patrollers
//
// Who QA can address. Distinct from ListPatrolBindings, which answers "whose handset is claimed":
// that list only ever contains patrollers who have signed in on a phone, so composing a message
// from it would silently omit the new patroller who has not started yet — precisely the person a
// round briefing is for.
//
// Inactive accounts are excluded. A message that reports "sent to 9" when three of them cannot
// sign in is worse than no number at all.
func ListPatrollers(pool *pgxpool.Pool) http.HandlerFunc {
	type row struct {
		UserID   string `json:"user_id"`
		StaffID  string `json:"staff_id"`
		FullName string `json:"full_name"`
		Email    string `json:"email"`
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
		rows, err := conn.Query(r.Context(), `
			SELECT user_id::text, COALESCE(staff_id,''), COALESCE(full_name,''), COALESCE(email,'')
			FROM users
			WHERE tenant_id = $1 AND role = 'QA_PATROLLER' AND COALESCE(is_active, true)
			ORDER BY full_name`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		out := []row{}
		for rows.Next() {
			var x row
			if rows.Scan(&x.UserID, &x.StaffID, &x.FullName, &x.Email) == nil {
				out = append(out, x)
			}
		}
		writeJSON(w, http.StatusOK, out)
	}
}
