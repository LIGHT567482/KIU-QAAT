package handlers

// Lecturer gate — session-info + scan. The QR carries only the session_id (short,
// scannable, server-resolved like the student QR). Proof of presence is: (1) the
// assigned staff ID, (2) the live 10s digit code from the coordinator's screen,
// (3) the device fingerprint. The END scan only records contact time if a
// sufficient RATIO of enrolled students attended (so it works for units with one
// or two students). These are public endpoints → adminPool (no JWT); the session
// row carries the tenant.

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/middleware"
)

// GET /lecturer/session-info?s=<session_id> — display fields for the captive
// portal after the lecturer scans the short QR.
func LecturerSessionInfo(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID := r.URL.Query().Get("s")
		if !middleware.ValidTenantID(sessionID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_SESSION_ID"})
			return
		}
		var tenantID, unitName, coordName, lecturerName, venueName, plannedStart, status string
		var sessionDate time.Time
		var plannedMinutes int
		err := adminPool.QueryRow(r.Context(), `
			SELECT s.tenant_id::text, COALESCE(cu.name, s.unit_id), COALESCE(u.full_name,''),
			       COALESCE(l.full_name,''), COALESCE(v.name,''), s.session_date,
			       COALESCE(to_char(s.planned_start,'HH24:MI'),''), COALESCE(s.planned_duration_minutes,0),
			       s.session_status::text
			FROM sessions s
			LEFT JOIN course_units cu ON cu.unit_id = s.unit_id
			LEFT JOIN users u ON u.user_id::text = s.coordinator_id AND u.tenant_id = s.tenant_id
			LEFT JOIN lecturers l ON l.lecturer_id::text = s.lecturer_id AND l.tenant_id = s.tenant_id
			LEFT JOIN venues v ON v.venue_id = s.venue_id AND v.tenant_id = s.tenant_id
			WHERE s.session_id = $1`,
			sessionID).Scan(&tenantID, &unitName, &coordName, &lecturerName, &venueName,
			&sessionDate, &plannedStart, &plannedMinutes, &status)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "SESSION_NOT_FOUND"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id": sessionID, "tenant_id": tenantID, "unit_name": unitName,
			"coordinator_name": coordName, "lecturer_name": lecturerName, "venue_name": venueName,
			"session_date": sessionDate.Format("2006-01-02"), "planned_start": plannedStart,
			"planned_minutes": plannedMinutes, "session_status": status,
		})
	}
}

type lecturerScanRequest struct {
	SessionID   string `json:"session_id"`
	StaffID     string `json:"staff_id"`
	RoomCode    string `json:"room_code"` // the live 10s digit code on the coordinator's screen
	Fingerprint string `json:"fingerprint"`
}

// POST /api/v1/lecturer/gate-scan — records START then END for the lecturer.
func LecturerGateScan(adminPool *pgxpool.Pool, rdb *redis.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req lecturerScanRequest
		if err := decodeJSON(r, &req); err != nil || req.SessionID == "" || req.Fingerprint == "" || req.StaffID == "" || req.RoomCode == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_REQUEST", "message": "session_id, staff_id, room_code and fingerprint are required"})
			return
		}
		if !middleware.ValidTenantID(req.SessionID) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "INVALID_SESSION_ID"})
			return
		}

		var tenantID, lecturerID string
		var registeredStaffID *string
		var secret []byte
		var sessionStatus string
		var scannedAt, endedAt *time.Time
		var attended, enrolled, bioCredCount int
		var ratio float64
		var coordinatorIP *string
		err := adminPool.QueryRow(r.Context(), `
			SELECT s.tenant_id::text, s.lecturer_id::text, l.staff_id, s.checkin_secret, s.session_status::text,
			       lal.lecturer_scanned_at, lal.lecturer_ended_at,
			       (SELECT COUNT(*) FROM attendance_logs al WHERE al.session_id = s.session_id),
			       (SELECT COUNT(*) FROM students_extended se
			          WHERE se.tenant_id = s.tenant_id AND se.enrollment_status = 'ACTIVE'
			            AND ( (s.offering_id IS NOT NULL AND se.offering_id = s.offering_id)
			                  OR (s.offering_id IS NULL AND se.course_id = (SELECT course_id FROM course_units cu WHERE cu.unit_id = s.unit_id)) )),
			       COALESCE(t.lecturer_attendance_ratio, 0.5),
			       (SELECT COUNT(*) FROM lecturer_webauthn_credentials wc
			          WHERE wc.tenant_id = s.tenant_id AND wc.lecturer_id::text = s.lecturer_id),
			       s.coordinator_ip
			FROM sessions s
			JOIN lecturers l ON l.lecturer_id::text = s.lecturer_id
			JOIN tenants t ON t.tenant_id = s.tenant_id
			LEFT JOIN lecturer_attendance_logs lal ON lal.session_id = s.session_id AND lal.tenant_id = s.tenant_id
			WHERE s.session_id = $1`,
			req.SessionID).Scan(&tenantID, &lecturerID, &registeredStaffID, &secret, &sessionStatus,
			&scannedAt, &endedAt, &attended, &enrolled, &ratio, &bioCredCount, &coordinatorIP)
		if err != nil || registeredStaffID == nil || *registeredStaffID == "" {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "no_staff_id", "message": "no lecturer staff ID is registered for this session"})
			return
		}

		// (1) Staff-ID gate.
		if !strings.EqualFold(strings.TrimSpace(req.StaffID), strings.TrimSpace(*registeredStaffID)) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "staff_id_mismatch", "message": "staff ID does not match the assigned lecturer"})
			return
		}

		now := time.Now().UTC()

		// (2) Physical-presence gate: the digit code must be the one currently shown
		// on the coordinator's screen — only someone in the room can read it.
		if !checkin.Validate(secret, strings.TrimSpace(req.RoomCode), now) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "wrong_code", "message": "that digit code is not current — read the live code on the coordinator's screen"})
			return
		}

		// (2b) Network-proximity gate (MANDATORY): the scan MUST come from the
		// coordinator's LAN (their hotspot / same network). Being on the coordinator's
		// LAN is the proximity model, so this is required — no coordinator IP anchor
		// (set when they opened the session on the hotspot) means presence cannot be
		// proven → reject. Blocks a remote proxy who obtained the staff ID + a relayed
		// live code, and any scan attempted after the session's hotspot is gone.
		if coordinatorIP == nil || *coordinatorIP == "" || !onSameLAN(middleware.ClientIP(r), *coordinatorIP) {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
				"error":   "not_same_network",
				"message": "you must be connected to the coordinator's hotspot (same network) to confirm attendance",
			})
			return
		}

		// (3) Biometric identity gate: if this lecturer has enrolled a fingerprint
		// passkey, they must have just passed the on-device biometric check (the
		// captive portal sets wa:verified:<session> via WebAuthn before submitting).
		if bioCredCount > 0 {
			verified, _ := rdb.Get(r.Context(), "wa:verified:"+req.SessionID).Result()
			if verified != lecturerID {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "biometric_required", "message": "confirm your fingerprint on your phone before submitting"})
				return
			}
			rdb.Del(r.Context(), "wa:verified:"+req.SessionID) // single-use per scan
		}

		switch {
		case scannedAt == nil:
			// ── START of lecture ──
			if sessionStatus != "ACTIVE" {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "session_not_active", "message": "the session is not active"})
				return
			}
			ct, e := adminPool.Exec(r.Context(), `
				UPDATE lecturer_attendance_logs
				SET lecturer_scanned_at = $1, lecturer_fingerprint_hash = $2
				WHERE session_id = $3 AND tenant_id = $4 AND lecturer_scanned_at IS NULL`,
				now, req.Fingerprint, req.SessionID, tenantID)
			if e != nil || ct.RowsAffected() == 0 {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": "no_lecturer", "message": "no lecturer is assigned to this session"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "STARTED", "action": "START", "session_id": req.SessionID, "scanned_at": now.Format(time.RFC3339)})

		case endedAt == nil:
			// ── END of lecture ── only counts if a sufficient SHARE of enrolled
			// students attended. required = max(1, ceil(ratio * enrolled)).
			required := 1
			if q := int(math.Ceil(ratio * float64(enrolled))); q > required {
				required = q
			}
			if attended < required {
				writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
					"error":   "no_quorum",
					"message": fmt.Sprintf("not enough students attended (%d of %d present; %d needed) — the lecture cannot be closed", attended, enrolled, required),
				})
				return
			}
			_, e := adminPool.Exec(r.Context(), `
				UPDATE lecturer_attendance_logs
				SET lecturer_ended_at = $1, lecturer_end_fingerprint_hash = $2,
				    contact_hours = ROUND(EXTRACT(EPOCH FROM ($1 - lecturer_scanned_at)) / 3600.0, 2)
				WHERE session_id = $3 AND tenant_id = $4 AND lecturer_ended_at IS NULL`,
				now, req.Fingerprint, req.SessionID, tenantID)
			if e != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "INTERNAL_ERROR"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ENDED", "action": "END", "session_id": req.SessionID, "ended_at": now.Format(time.RFC3339)})

		default:
			writeJSON(w, http.StatusOK, map[string]string{"status": "ALREADY_COMPLETE", "action": "NONE", "session_id": req.SessionID})
		}
	}
}
