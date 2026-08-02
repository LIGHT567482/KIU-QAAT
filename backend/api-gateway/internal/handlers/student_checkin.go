package handlers

// StudentCheckin handles POST /api/v1/student/checkin (STUDENT role, JWT-authenticated).
//
// Identity is proven by the student's JWT (email → student_id lookup).
// Proximity is proven by the rotating room code shown on the coordinator's screen.
// No QR image is required — the authenticated account IS the identity proof.

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/middleware"
)

type studentCheckinRequest struct {
	SessionID         string `json:"session_id"`
	RoomCode          string `json:"room_code"`
	DeviceFingerprint string `json:"device_fingerprint"` // hash of the student's device
}

// checkinResponse is the wire shape every check-in path answers with — shared with
// the in-room hub's sync payloads so a phone and the cloud speak the same language.
type checkinResponse struct {
	Status            string `json:"status"`
	Reason            string `json:"reason,omitempty"`
	StudentName       string `json:"student_name,omitempty"`
	UnitName          string `json:"unit_name,omitempty"`
	LecturerName      string `json:"lecturer_name,omitempty"`
	SessionDate       string `json:"session_date,omitempty"`
	AttendancePercent int    `json:"attendance_percent,omitempty"`
}

func StudentCheckin(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		userID := middleware.GetUserID(r.Context()) // user_id from users table

		var req studentCheckinRequest
		if err := decodeJSON(r, &req); err != nil || req.SessionID == "" || req.RoomCode == "" {
			writeJSON(w, http.StatusBadRequest, checkinResponse{Status: "REJECTED", Reason: "INVALID_REQUEST"})
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}
		defer conn.Release()
		if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}

		// Resolve student_id from the authenticated user's email within this tenant.
		var studentID, enrollment string
		err = conn.QueryRow(r.Context(), `
			SELECT se.student_id, se.enrollment_status::text
			FROM users u
			JOIN students_extended se ON se.email = u.email AND se.tenant_id = u.tenant_id
			WHERE u.user_id = $1 AND u.tenant_id = $2`,
			userID, tenantID).Scan(&studentID, &enrollment)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			writeJSON(w, http.StatusForbidden, checkinResponse{Status: "REJECTED", Reason: "NOT_ON_ROSTER"})
			return
		case err != nil:
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}
		if enrollment != "ACTIVE" {
			writeJSON(w, http.StatusForbidden, checkinResponse{Status: "REJECTED", Reason: "NOT_ON_ROSTER"})
			return
		}

		// Fetch active session and validate it belongs to this tenant.
		var (
			secret        []byte
			status        string
			coordID       string
			windowStart   *time.Time
			windowEnd     *time.Time
			coordinatorIP string
		)
		err = conn.QueryRow(r.Context(), `
			SELECT s.checkin_secret, s.session_status::text, s.coordinator_id,
			       s.checkin_window_start, s.checkin_window_end,
			       COALESCE(s.coordinator_ip,'')
			FROM sessions s
			WHERE s.session_id = $1`,
			req.SessionID).Scan(&secret, &status, &coordID, &windowStart, &windowEnd,
			&coordinatorIP)
		if err != nil || len(secret) == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "SESSION_NOT_ACTIVE"})
			return
		}
		if status != "ACTIVE" {
			writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "SESSION_NOT_ACTIVE"})
			return
		}
		now := time.Now().UTC()
		if (windowStart != nil && now.Before(*windowStart)) || (windowEnd != nil && now.After(*windowEnd)) {
			writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "GATE_NOT_OPEN"})
			return
		}

		// Proximity proof #1: the session's STATIC student room code (does not rotate).
		// The hard proximity proof is the mandatory LAN check below; this just selects
		// the room. (The lecturer gate keeps the rotating code.)
		if !checkin.ValidateStatic(secret, req.RoomCode) {
			writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "PROXIMITY_FAILED"})
			return
		}

		// Proximity proof #2: network proximity (MANDATORY) — the student MUST be on
		// the room's Wi-Fi (the coordinator's hotspot / same LAN). Connection to the
		// coordinator's LAN is the proximity model, so this is required: no coordinator
		// IP anchor (captured when they opened the session on the hotspot) means
		// presence cannot be proven → reject. Blocks a remote proxy texted the live code.
		if coordinatorIP == "" || !onSameLAN(middleware.ClientIP(r), coordinatorIP) {
			writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "NOT_SAME_NETWORK"})
			return
		}

		// Lecturer-started gate: attendance is valid only once the lecturer has scanned
		// the coordinator's QR to START the lecture. Reject until then.
		var lecturerStarted bool
		_ = conn.QueryRow(r.Context(),
			`SELECT EXISTS(SELECT 1 FROM lecturer_attendance_logs WHERE session_id = $1 AND lecturer_scanned_at IS NOT NULL)`,
			req.SessionID).Scan(&lecturerStarted)
		if !lecturerStarted {
			writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "LECTURER_NOT_STARTED"})
			return
		}

		// Device binding: one device can mark ONE person present per session. If this
		// device already checked in a DIFFERENT student this session, reject it — this
		// stops a proxy logging into several students' accounts on one phone.
		fp := strings.TrimSpace(req.DeviceFingerprint)
		if fp != "" {
			var deviceUsedBy string
			_ = conn.QueryRow(r.Context(), `
				SELECT student_id FROM attendance_logs
				WHERE session_id = $1 AND device_fingerprint_hash = $2 LIMIT 1`,
				req.SessionID, fp).Scan(&deviceUsedBy)
			if deviceUsedBy != "" && deviceUsedBy != studentID {
				writeJSON(w, http.StatusUnprocessableEntity, checkinResponse{Status: "REJECTED", Reason: "DEVICE_ALREADY_USED"})
				return
			}
		}

		// Duplicate check: one attendance record per student per session.
		var dup bool
		if err := conn.QueryRow(r.Context(), `
			SELECT EXISTS(SELECT 1 FROM attendance_logs
			    WHERE session_id = $1 AND student_id = $2)`,
			req.SessionID, studentID).Scan(&dup); err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}
		if dup {
			writeJSON(w, http.StatusOK, checkinResponse{Status: "PRESENT"}) // idempotent
			return
		}

		// Record attendance.
		var seq int
		if err := conn.QueryRow(r.Context(),
			`SELECT COALESCE(MAX(sequence_number),0)+1 FROM attendance_logs WHERE session_id = $1`,
			req.SessionID).Scan(&seq); err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}
		_, err = conn.Exec(r.Context(), `
			INSERT INTO attendance_logs
			    (tenant_id, session_id, student_id, checkin_timestamp,
			     device_fingerprint_hash, sequence_number, entry_method, coordinator_id)
			VALUES ($1, $2, $3, $4, $5, $6, 'AUTHENTICATED', $7)`,
			tenantID, req.SessionID, studentID, now, fp, seq, coordID)
		if err != nil {
			// Concurrent duplicate trips the unique index — treat as success.
			writeJSON(w, http.StatusOK, checkinResponse{Status: "PRESENT"})
			return
		}

		writeJSON(w, http.StatusOK, checkinResponse{Status: "PRESENT"})
	}
}
