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
	CheckinCode      string `json:"checkin_code"`
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

		// Check-in window length comes from tenant policy.
		var windowMinutes int
		if err := conn.QueryRow(r.Context(),
			`SELECT checkin_window_minutes FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&windowMinutes); err != nil || windowMinutes <= 0 {
			windowMinutes = 30
		}

		now := time.Now().UTC()
		windowEnd := now.Add(time.Duration(windowMinutes) * time.Minute)

		var sessionID string
		err = conn.QueryRow(r.Context(), `
			INSERT INTO sessions
			    (tenant_id, coordinator_id, unit_id, venue_id, lecturer_id, session_date,
			     gate_open_time, checkin_window_start, checkin_window_end,
			     session_status, checkin_secret)
			VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6, $7, $7, $8, 'ACTIVE', $9)
			RETURNING session_id`,
			tenantID, coordID, req.UnitID, req.VenueID, req.LecturerID, sessionDate,
			now, windowEnd, secret,
		).Scan(&sessionID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "OPEN_FAILED", "message": "could not open session (check unit_id/venue_id exist for tenant)",
			})
			return
		}

		writeJSON(w, http.StatusCreated, openSessionResponse{
			SessionID:        sessionID,
			CheckinCode:      checkin.Derive(secret, now),
			SecondsRemaining: checkin.SecondsRemaining(now),
			CheckinWindowEnd: windowEnd.Format(time.RFC3339),
		})
	}
}

type checkinCodeResponse struct {
	Code             string `json:"code"`
	SecondsRemaining int    `json:"seconds_remaining"`
	SessionStatus    string `json:"session_status"`
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

		secret, status, err := loadSessionSecret(r.Context(), pool, tenantID, sessionID, coordID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "SESSION_NOT_FOUND"})
			return
		}

		now := time.Now().UTC()
		writeJSON(w, http.StatusOK, checkinCodeResponse{
			Code:             checkin.Derive(secret, now),
			SecondsRemaining: checkin.SecondsRemaining(now),
			SessionStatus:    status,
		})
	}
}

// loadSessionSecret fetches a session's check-in secret + status, RLS-scoped to
// the tenant and restricted to the owning coordinator.
func loadSessionSecret(ctx context.Context, pool *pgxpool.Pool, tenantID, sessionID, coordID string) ([]byte, string, error) {
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, "", err
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(ctx, conn, tenantID); err != nil {
		return nil, "", err
	}
	var secret []byte
	var status string
	err = conn.QueryRow(ctx, `
		SELECT checkin_secret, session_status::text
		FROM sessions
		WHERE session_id = $1 AND coordinator_id = $2 AND checkin_secret IS NOT NULL`,
		sessionID, coordID).Scan(&secret, &status)
	if err != nil {
		return nil, "", err
	}
	return secret, status, nil
}
