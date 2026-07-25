package handlers

// Online session lifecycle for the cloud check-in flow.
//
//   POST /api/v1/sessions/open                 (COORDINATOR) — create an ACTIVE
//        session with a per-session check-in secret; returns the session id and
//        the current rotating room code.
//   GET  /api/v1/sessions/{id}/checkin-code    (COORDINATOR) — the current room
//        code + seconds remaining, polled by the coordinator's display. The
//        secret itself never leaves the server.

import (
	"context"
	"crypto/rand"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/middleware"
)

type openSessionRequest struct {
	UnitID      string `json:"unit_id"`
	VenueID     string `json:"venue_id"`
	LecturerID  string `json:"lecturer_id"`
	SessionDate string `json:"session_date"` // optional ISO date; defaults to today
}

type openSessionResponse struct {
	SessionID        string `json:"session_id"`
	TenantID         string `json:"tenant_id"`
	CheckinCode      string `json:"checkin_code"`  // rotating (lecturer)
	StudentCode      string `json:"student_code"`  // static (students)
	SecondsRemaining int    `json:"seconds_remaining"`
	CheckinWindowEnd string `json:"checkin_window_end"`
}

// OpenSession creates an ACTIVE session owned by the calling coordinator and
// seeds its rotating-room-code secret.
func OpenSession(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())

		var req openSessionRequest
		if err := decodeJSON(r, &req); err != nil || req.UnitID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_REQUEST", "message": "unit_id required"})
			return
		}

		// Daily session window: refuse to open outside the tenant's configured
		// hours/active days (defends against a stale client manifest).
		if open, msg := withinSessionWindow(r.Context(), pool, tenantID); !open {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "WINDOW_CLOSED", "message": msg})
			return
		}

		sessionDate := time.Now().UTC().Format("2006-01-02")
		if req.SessionDate != "" {
			sessionDate = req.SessionDate
		}

		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}

		// One open session per coordinator: if they already have an ACTIVE (or
		// awaiting-lecturer) session, make them end it before starting another.
		var openID string
		_ = conn.QueryRow(r.Context(),
			`SELECT session_id::text FROM sessions
			 WHERE coordinator_id = $1 AND tenant_id = $2
			   AND session_status IN ('ACTIVE','PENDING_LECTURER') LIMIT 1`,
			coordID, tenantID).Scan(&openID)
		if openID != "" {
			writeJSON(w, http.StatusConflict, map[string]string{
				"error":      "SESSION_ALREADY_OPEN",
				"message":    "You already have an open session — please end it before starting another.",
				"session_id": openID,
			})
			return
		}

		// Check-in window length comes from tenant policy.
		var windowMinutes int
		if err := conn.QueryRow(r.Context(),
			`SELECT checkin_window_minutes FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&windowMinutes); err != nil || windowMinutes <= 0 {
			windowMinutes = 30
		}

		now := time.Now().UTC()
		windowEnd := now.Add(time.Duration(windowMinutes) * time.Minute)

		// Resolve the coordinator's offering (session cohort) so the attendance
		// session + roster are scoped to it (one coordinator can't see another's).
		var offeringID string
		_ = conn.QueryRow(r.Context(),
			`SELECT offering_id::text FROM course_offerings WHERE coordinator_id = $1 AND tenant_id = $2`,
			coordID, tenantID).Scan(&offeringID)

		// Units are mapped to lecturers via lecturer_assignments — auto-pick the
		// unit's assigned lecturer so the coordinator doesn't choose one manually.
		if req.LecturerID == "" {
			_ = conn.QueryRow(r.Context(),
				`SELECT la.lecturer_id::text FROM lecturer_assignments la
				 WHERE la.unit_id = $1 AND la.tenant_id = $2
				 ORDER BY la.created_at LIMIT 1`,
				req.UnitID, tenantID).Scan(&req.LecturerID)
		}

		var sessionID string
		// Inherit the offering's locked schedule (planned start + length) for this
		// unit so check-in timestamps can be judged against the planned window (#2).
		// The coordinator's egress IP anchors the LAN-proximity anti-proxy gate:
		// the lecturer's gate scan must later originate from this same network.
		coordIP := middleware.ClientIP(r)
		err = conn.QueryRow(r.Context(), `
			INSERT INTO sessions
			    (tenant_id, coordinator_id, unit_id, venue_id, lecturer_id, session_date,
			     gate_open_time, checkin_window_start, checkin_window_end,
			     session_status, checkin_secret, offering_id, planned_start, planned_duration_minutes,
			     coordinator_ip)
			SELECT $1, $2, $3::text, NULLIF($4,''), NULLIF($5,''), $6, $7, $7, $8, 'ACTIVE', $9,
			       NULLIF($10,'')::uuid,
			       -- Prefer TODAY's imported timetable slot; fall back to the set-once schedule.
			       COALESCE((SELECT ts.start_time FROM timetable_slots ts
			                 WHERE ts.unit_id = cu.unit_id AND ts.offering_id = NULLIF($10,'')::uuid
			                   AND ts.day_of_week = EXTRACT(ISODOW FROM now())::int
			                 ORDER BY ts.start_time LIMIT 1), ous.session_start),
			       COALESCE((SELECT ts.duration_minutes FROM timetable_slots ts
			                 WHERE ts.unit_id = cu.unit_id AND ts.offering_id = NULLIF($10,'')::uuid
			                   AND ts.day_of_week = EXTRACT(ISODOW FROM now())::int
			                 ORDER BY ts.start_time LIMIT 1), ous.session_duration_minutes),
			       NULLIF($11,'')
			FROM course_units cu
			LEFT JOIN offering_unit_schedules ous
			       ON ous.unit_id = cu.unit_id AND ous.offering_id = NULLIF($10,'')::uuid
			WHERE cu.unit_id = $3::text AND cu.tenant_id = $1
			RETURNING session_id`,
			tenantID, coordID, req.UnitID, req.VenueID, req.LecturerID, sessionDate,
			now, windowEnd, secret, offeringID, coordIP,
		).Scan(&sessionID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "OPEN_FAILED", "message": "could not open session (check unit_id/venue_id exist for tenant)",
			})
			return
		}

		// Record lecturer attendance: write gate_open_time so we have a formal
		// attendance entry even before the session closes. gate_close_time and
		// contact_hours are filled by CloseSession.
		if req.LecturerID != "" {
			conn.Exec(r.Context(), `
				INSERT INTO lecturer_attendance_logs
				    (tenant_id, session_id, lecturer_id, gate_open_time, unit_id, venue_id, session_date)
				VALUES ($1, $2, $3, $4, $5, NULLIF($6,''), $7)
				ON CONFLICT DO NOTHING`,
				tenantID, sessionID, req.LecturerID, now, req.UnitID, req.VenueID, sessionDate) //nolint:errcheck
		}

		writeJSON(w, http.StatusCreated, openSessionResponse{
			SessionID:        sessionID,
			TenantID:         tenantID,
			CheckinCode:      checkin.Derive(secret, now),
			StudentCode:      checkin.StaticCode(secret),
			SecondsRemaining: checkin.SecondsRemaining(now),
			CheckinWindowEnd: windowEnd.Format(time.RFC3339),
		})
	}
}

type checkinCodeResponse struct {
	Code             string `json:"code"`           // rotating — the LECTURER's live digit code
	StudentCode      string `json:"student_code"`   // static — the STUDENT room code (does not rotate)
	SecondsRemaining int    `json:"seconds_remaining"`
	SessionStatus    string `json:"session_status"`
	CheckinCount     int    `json:"checkin_count"`
}

// CheckinCode returns the current rotating code for a session the coordinator owns.
func CheckinCode(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		sessionID := chi.URLParam(r, "session_id")
		if !middleware.ValidTenantID(sessionID) { // session_id is also a UUID
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_SESSION_ID"})
			return
		}

		secret, status, count, err := loadSessionSecret(r.Context(), pool, tenantID, sessionID, coordID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "SESSION_NOT_FOUND"})
			return
		}

		now := time.Now().UTC()
		writeJSON(w, http.StatusOK, checkinCodeResponse{
			Code:             checkin.Derive(secret, now),
			StudentCode:      checkin.StaticCode(secret),
			SecondsRemaining: checkin.SecondsRemaining(now),
			SessionStatus:    status,
			CheckinCount:     count,
		})
	}
}

// CloseSession allows the coordinator to explicitly close an active session.
// It transitions ACTIVE → CLOSED, records gate_close_time, and calculates
// contact_hours in lecturer_attendance_logs.
func CloseSession(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		sessionID := chi.URLParam(r, "session_id")

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
			return
		}

		now := time.Now().UTC()
		tag, err := conn.Exec(r.Context(), `
			UPDATE sessions SET
			    session_status      = 'CLOSED',
			    gate_close_time     = $1,
			    coordinator_end_time = $1
			WHERE session_id   = $2
			  AND coordinator_id = $3
			  AND session_status IN ('ACTIVE','PENDING_LECTURER')`,
			now, sessionID, coordID)
		if err != nil || tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"error": "NOT_FOUND", "message": "session not found or already closed",
			})
			return
		}

		// Update lecturer attendance log with departure time + contact hours.
		conn.Exec(r.Context(), `
			UPDATE lecturer_attendance_logs
			SET gate_close_time = $1,
			    contact_hours   = ROUND(
			        EXTRACT(EPOCH FROM ($1 - gate_open_time)) / 3600.0, 2
			    )
			WHERE session_id = $2 AND tenant_id = $3`,
			now, sessionID, tenantID) //nolint:errcheck

		// Get attendance count to return to coordinator.
		var count int
		conn.QueryRow(r.Context(), //nolint:errcheck
			`SELECT COUNT(*) FROM attendance_logs WHERE session_id = $1`, sessionID).Scan(&count)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id":    sessionID,
			"status":        "CLOSED",
			"closed_at":     now.Format(time.RFC3339),
			"checkin_count": count,
		})
	}
}

// loadSessionSecret fetches a session's check-in secret, status, and live
// attendance count, RLS-scoped to the tenant and restricted to the owning coordinator.
func loadSessionSecret(ctx context.Context, pool *pgxpool.Pool, tenantID, sessionID, coordID string) ([]byte, string, int, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, "", 0, err
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(ctx, conn, tenantID); err != nil {
		return nil, "", 0, err
	}
	var secret []byte
	var status string
	var count int
	err = conn.QueryRow(ctx, `
		SELECT s.checkin_secret, s.session_status::text,
		       (SELECT COUNT(*) FROM attendance_logs WHERE session_id = s.session_id)
		FROM sessions s
		WHERE s.session_id = $1 AND s.coordinator_id = $2 AND s.checkin_secret IS NOT NULL`,
		sessionID, coordID).Scan(&secret, &status, &count)
	if err != nil {
		return nil, "", 0, err
	}
	return secret, status, count, nil
}
