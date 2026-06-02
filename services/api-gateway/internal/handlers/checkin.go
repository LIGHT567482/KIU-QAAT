package handlers

// Public student check-in endpoint: POST /api/v1/checkin
//
// This is the online replacement for the offline PWA-LAN scan path. It is PUBLIC
// (students have no JWT) and authenticates the request by two unforgeable facts:
//   1. a valid RSA signature on the student's personal QR (proves identity +
//      tenant — only qr-generator holds the tenant private key), and
//   2. the current rotating room code (proves physical presence in the room).
//
// It ports validator steps 1–4b and 6–8 from the PWA (apps/coordinator-pwa/src/
// qr/validator.ts); step 5 (BLE RSSI) is replaced by the room code.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/middleware"
)

type checkinRequest struct {
	SessionID   string `json:"session_id"`
	QR          string `json:"qr"`          // raw QR JSON string the student decoded
	RoomCode    string `json:"room_code"`   // the rotating code shown on the projector
	Fingerprint string `json:"fingerprint"` // device fingerprint hash computed in-browser
}

type checkinResponse struct {
	Status string `json:"status"`           // PRESENT | REJECTED
	Reason string `json:"reason,omitempty"` // rejection reason code
}

// Checkin handles a student submission.
func Checkin(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req checkinRequest
		if err := decodeJSON(r, &req); err != nil || req.QR == "" || req.SessionID == "" || req.Fingerprint == "" {
			writeJSON(w, http.StatusBadRequest, checkinResponse{Status: "REJECTED", Reason: "INVALID_REQUEST"})
			return
		}
		if !middleware.ValidTenantID(req.SessionID) { // session_id is a UUID
			writeJSON(w, http.StatusBadRequest, checkinResponse{Status: "REJECTED", Reason: "INVALID_REQUEST"})
			return
		}

		// ── Step 1: parse + locate tenant ──────────────────────────────────────
		qr, err := checkin.ParseQR(req.QR)
		if err != nil || !middleware.ValidTenantID(qr.TenantID) {
			writeJSON(w, http.StatusBadRequest, checkinResponse{Status: "REJECTED", Reason: "INVALID_SIGNATURE"})
			return
		}

		conn, err := pool.Acquire(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}
		defer conn.Release()
		// RLS scoped to the tenant CLAIMED by the QR. The claim is only trusted
		// after the signature verifies against that tenant's key (step 1b).
		if err := middleware.SetTenantConn(r.Context(), conn, qr.TenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}

		result := validateCheckin(r.Context(), conn, qr, &req)
		status := http.StatusOK
		if result.Status != "PRESENT" {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, result)
	}
}

func validateCheckin(ctx context.Context, conn *pgxpool.Conn, qr *checkin.SignedQR, req *checkinRequest) checkinResponse {
	reject := func(reason string) checkinResponse { return checkinResponse{Status: "REJECTED", Reason: reason} }

	// ── Step 1b: RSA signature (authenticates the tenant claim) ────────────────
	var publicKeyPEM string
	err := conn.QueryRow(ctx, `
		SELECT COALESCE(k.rsa_public_key_pem, '')
		FROM tenant_rsa_keys k
		WHERE k.tenant_id = $1 AND k.revoked_at IS NULL
		ORDER BY k.created_at DESC LIMIT 1`, qr.TenantID).Scan(&publicKeyPEM)
	if err != nil || publicKeyPEM == "" {
		return reject("INVALID_SIGNATURE")
	}
	if err := qr.VerifySignature(publicKeyPEM); err != nil {
		return reject("INVALID_SIGNATURE")
	}

	// ── Step 2: expiry ─────────────────────────────────────────────────────────
	expiry, err := time.Parse("2006-01-02", qr.ExpiryDate)
	if err != nil || expiry.Before(time.Now().UTC().Truncate(24*time.Hour)) {
		return reject("QR_EXPIRED")
	}

	// ── Step 4: session must exist, belong to this tenant, be ACTIVE & open ────
	// (Step 3 — tenant match — is implicit: RLS + signature both bind the tenant.)
	var (
		secret      []byte
		status      string
		coordID     string
		windowStart *time.Time
		windowEnd   *time.Time
	)
	err = conn.QueryRow(ctx, `
		SELECT checkin_secret, session_status::text, coordinator_id,
		       checkin_window_start, checkin_window_end
		FROM sessions WHERE session_id = $1`, req.SessionID).
		Scan(&secret, &status, &coordID, &windowStart, &windowEnd)
	if err != nil || len(secret) == 0 {
		return reject("SESSION_NOT_ACTIVE")
	}
	if status != "ACTIVE" {
		return reject("SESSION_NOT_ACTIVE")
	}
	now := time.Now().UTC()
	if (windowStart != nil && now.Before(*windowStart)) || (windowEnd != nil && now.After(*windowEnd)) {
		return reject("GATE_NOT_OPEN")
	}

	// ── Step 5 (replacement): rotating room code = proximity proof ─────────────
	if !checkin.Validate(secret, req.RoomCode, now) {
		return reject("PROXIMITY_FAILED")
	}

	// ── Step 4 (roster) + 4b (serial / revocation) ────────────────────────────
	var enrollment, currentSerial string
	err = conn.QueryRow(ctx, `
		SELECT enrollment_status::text, COALESCE(qr_serial_number,'')
		FROM students_extended WHERE student_id = $1`, qr.StudentID).
		Scan(&enrollment, &currentSerial)
	if errors.Is(err, pgx.ErrNoRows) || enrollment != "ACTIVE" {
		return reject("NOT_ON_ROSTER")
	}
	if err != nil {
		return reject("INTERNAL_ERROR")
	}
	if currentSerial == "" || currentSerial != qr.SerialNumber {
		return reject("SERIAL_REVOKED")
	}

	// ── Step 6: hardware fingerprint bind ──────────────────────────────────────
	var boundFingerprint string
	err = conn.QueryRow(ctx,
		`SELECT fingerprint_hash FROM hardware_vault WHERE student_id = $1`, qr.StudentID).
		Scan(&boundFingerprint)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		// First check-in for this student — bind this device for the year.
		if _, err := conn.Exec(ctx, `
			INSERT INTO hardware_vault (student_id, tenant_id, fingerprint_hash, first_bound_at, academic_year)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (student_id) DO NOTHING`,
			qr.StudentID, qr.TenantID, req.Fingerprint, now, qr.AcademicYear); err != nil {
			return reject("INTERNAL_ERROR")
		}
	case err != nil:
		return reject("INTERNAL_ERROR")
	case boundFingerprint != req.Fingerprint:
		return reject("DEVICE_MISMATCH")
	}

	// ── Step 7: duplicate scan (one row per session+student) ───────────────────
	var dup bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM attendance_logs
		    WHERE session_id = $1 AND student_id = $2 AND entry_method = 'QR_SCAN')`,
		req.SessionID, qr.StudentID).Scan(&dup); err != nil {
		return reject("INTERNAL_ERROR")
	}
	if dup {
		return reject("DUPLICATE_SCAN")
	}

	// ── Step 8: one device per session ─────────────────────────────────────────
	var deviceUsed bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM attendance_logs
		    WHERE session_id = $1 AND device_fingerprint_hash = $2
		      AND student_id <> $3 AND entry_method = 'QR_SCAN')`,
		req.SessionID, req.Fingerprint, qr.StudentID).Scan(&deviceUsed); err != nil {
		return reject("INTERNAL_ERROR")
	}
	if deviceUsed {
		return reject("DEVICE_ALREADY_USED")
	}

	// ── Record attendance (append-only) ────────────────────────────────────────
	var seq int
	if err := conn.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence_number),0)+1 FROM attendance_logs WHERE session_id = $1`,
		req.SessionID).Scan(&seq); err != nil {
		return reject("INTERNAL_ERROR")
	}
	_, err = conn.Exec(ctx, `
		INSERT INTO attendance_logs
		    (tenant_id, session_id, student_id, checkin_timestamp,
		     device_fingerprint_hash, sequence_number, entry_method, coordinator_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'QR_SCAN', $7)`,
		qr.TenantID, req.SessionID, qr.StudentID, now, req.Fingerprint, seq, coordID)
	if err != nil {
		// A concurrent submission for the same student trips the unique index
		// uq_attendance_session_student — treat as a duplicate, not an error.
		return reject("DUPLICATE_SCAN")
	}

	// NOTE: the eligibility summary (student_attendance_summary) is deliberately
	// NOT refreshed here. refresh_attendance_summary rebuilds the whole tenant's
	// summary, so calling it per check-in would be O(N^2) during a 300-student
	// rush. It is refreshed off the hot path (session close / scheduled job).
	return checkinResponse{Status: "PRESENT"}
}
