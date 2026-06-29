package handlers

// Public student check-in endpoint: POST /api/v1/checkin
//
// The student's identity is proven by their personal QR (RSA-signed, embedded in
// a captive-portal URL). Proximity is proven by the rotating room code displayed
// on the coordinator's screen. No JWT or login is required — the signed QR token
// IS the identity proof.
//
// Request body:
//
//	{
//	  "student_token": "<base64url-encoded signed QR JSON>",
//	  "room_code":     "123456",
//	  "fingerprint":   "<device-fingerprint hash>"
//	}
//
// Success response:
//
//	{
//	  "status":              "PRESENT",
//	  "unit_name":           "Introduction to Programming",
//	  "lecturer_name":       "Dr. Jane Smith",
//	  "session_date":        "2026-06-14",
//	  "attendance_percent":  72
//	}

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
	StudentToken string `json:"student_token"` // base64url-encoded signed QR JSON
	RoomCode     string `json:"room_code"`     // 6-digit rotating code from the projector
	Fingerprint  string `json:"fingerprint"`   // device fingerprint hash computed in-browser
}

type checkinResponse struct {
	Status            string `json:"status"`
	Reason            string `json:"reason,omitempty"`
	StudentName       string `json:"student_name,omitempty"`
	UnitName          string `json:"unit_name,omitempty"`
	LecturerName      string `json:"lecturer_name,omitempty"`
	SessionDate       string `json:"session_date,omitempty"`
	AttendancePercent int    `json:"attendance_percent,omitempty"`
}

// Checkin handles a student submission initiated by scanning their personal QR code.
func Checkin(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req checkinRequest
		if err := decodeJSON(r, &req); err != nil || req.StudentToken == "" || req.RoomCode == "" || req.Fingerprint == "" {
			writeJSON(w, http.StatusBadRequest, checkinResponse{Status: "REJECTED", Reason: "INVALID_REQUEST"})
			return
		}

		// Decode and parse the signed QR token embedded in the student's captive-portal URL.
		qr, err := checkin.ParseQRToken(req.StudentToken)
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

		// RLS scoped to the tenant claimed by the QR. The claim is trusted only after
		// the RSA signature verifies against that tenant's public key.
		if err := middleware.SetTenantConn(r.Context(), conn, qr.TenantID); err != nil {
			writeJSON(w, http.StatusInternalServerError, checkinResponse{Status: "REJECTED", Reason: "INTERNAL_ERROR"})
			return
		}

		result := validateCheckin(r.Context(), conn, qr, &req, middleware.ClientIP(r))
		status := http.StatusOK
		if result.Status != "PRESENT" {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, result)
	}
}

type activeSession struct {
	SessionID     string
	Secret        []byte
	CoordID       string
	UnitID        string
	WindowStart   *time.Time
	WindowEnd     *time.Time
	CoordinatorIP string
}

func validateCheckin(ctx context.Context, conn *pgxpool.Conn, qr *checkin.SignedQR, req *checkinRequest, clientIP string) checkinResponse {
	reject := func(reason string) checkinResponse { return checkinResponse{Status: "REJECTED", Reason: reason} }

	// ── Step 1b: RSA signature (authenticates the tenant claim) ───────────────
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

	// ── Step 2: expiry ────────────────────────────────────────────────────────
	expiry, err := time.Parse("2006-01-02", qr.ExpiryDate)
	if err != nil || expiry.Before(time.Now().UTC().Truncate(24*time.Hour)) {
		return reject("QR_EXPIRED")
	}

	// ── Step 4+5: find the active session whose room code matches ─────────────
	// The student does not provide session_id — they prove which room they are in
	// by knowing the current rotating code. We scan all ACTIVE sessions for this
	// tenant+course and pick the one whose code validates. The JOIN on course_units
	// constrains the search to sessions belonging to the student's course, so a
	// student in Course A cannot check in to Course B's session.
	//
	// Cohort gate: a session belongs to a specific offering (the coordinator's
	// cohort). We only consider sessions whose offering matches the student's own
	// offering — so only the coordinator's OWN students can attend. (Edge case: if
	// either side has no offering recorded, the course-level match above still holds.)
	rows, err := conn.Query(ctx, `
		SELECT s.session_id, s.checkin_secret, s.coordinator_id, s.unit_id,
		       s.checkin_window_start, s.checkin_window_end, COALESCE(s.coordinator_ip,'')
		FROM sessions s
		JOIN course_units cu ON cu.unit_id = s.unit_id AND cu.course_id = $2
		WHERE s.tenant_id = $1 AND s.session_status = 'ACTIVE'
		  AND (
		    s.offering_id IS NULL
		    OR (SELECT se.offering_id FROM students_extended se WHERE se.student_id = $3) IS NULL
		    OR s.offering_id = (SELECT se.offering_id FROM students_extended se WHERE se.student_id = $3)
		  )`,
		qr.TenantID, qr.CourseID, qr.StudentID)
	if err != nil {
		return reject("INTERNAL_ERROR")
	}

	now := time.Now().UTC()
	var candidateSessions []activeSession
	for rows.Next() {
		var s activeSession
		if err := rows.Scan(&s.SessionID, &s.Secret, &s.CoordID, &s.UnitID, &s.WindowStart, &s.WindowEnd, &s.CoordinatorIP); err != nil {
			continue
		}
		candidateSessions = append(candidateSessions, s)
	}
	rows.Close()

	// No ACTIVE sessions for this course → coordinator hasn't started yet.
	if len(candidateSessions) == 0 {
		return reject("SESSION_NOT_STARTED")
	}

	// Find the session whose room code matches the student's submission.
	var sess *activeSession
	for i := range candidateSessions {
		if checkin.Validate(candidateSessions[i].Secret, req.RoomCode, now) {
			sess = &candidateSessions[i]
			break
		}
	}

	if sess == nil {
		return reject("PROXIMITY_FAILED")
	}

	// Window check (optional per-session gate times).
	if (sess.WindowStart != nil && now.Before(*sess.WindowStart)) || (sess.WindowEnd != nil && now.After(*sess.WindowEnd)) {
		return reject("GATE_NOT_OPEN")
	}

	// ── Network-proximity gate (MANDATORY) ────────────────────────────────────
	// Attendance is recorded if and ONLY IF the scan comes from the coordinator's
	// LAN (their Wi-Fi hotspot / same network). The session's coordinator_ip is the
	// anchor, captured when the coordinator opened the session on that hotspot. No
	// anchor = no provable presence → reject. This defeats a remote student who was
	// merely relayed the live room code (scanning from home), since they are not on
	// the coordinator's network.
	if sess.CoordinatorIP == "" || !onSameLAN(clientIP, sess.CoordinatorIP) {
		return reject("NOT_SAME_NETWORK")
	}

	// ── Step 6a: cross-student device lock ────────────────────────────────────
	// Runs BEFORE the roster check so that a fraudster using a classmate's QR URL
	// on their own phone is always rejected with DEVICE_BELONGS_TO_ANOTHER_STUDENT
	// rather than passing all the way to attendance write.
	// If this device is already bound to a DIFFERENT student_id, reject immediately.
	var deviceOwner string
	crossErr := conn.QueryRow(ctx,
		`SELECT student_id::text FROM hardware_vault WHERE fingerprint_hash = $1`,
		req.Fingerprint).Scan(&deviceOwner)
	if crossErr == nil && deviceOwner != qr.StudentID {
		return reject("DEVICE_BELONGS_TO_ANOTHER_STUDENT")
	}
	if crossErr != nil && !errors.Is(crossErr, pgx.ErrNoRows) {
		return reject("INTERNAL_ERROR")
	}

	// ── Step 4 (roster) + 4b (serial / revocation) ───────────────────────────
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

	// ── Step 6b: per-student device bind / verify ──────────────────────────────
	var boundFingerprint string
	err = conn.QueryRow(ctx,
		`SELECT fingerprint_hash FROM hardware_vault WHERE student_id = $1`, qr.StudentID).
		Scan(&boundFingerprint)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
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

	// ── Step 7: duplicate scan ────────────────────────────────────────────────
	var dup bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM attendance_logs
		    WHERE session_id = $1 AND student_id = $2 AND entry_method = 'QR_SCAN')`,
		sess.SessionID, qr.StudentID).Scan(&dup); err != nil {
		return reject("INTERNAL_ERROR")
	}
	if dup {
		return reject("DUPLICATE_SCAN")
	}

	// ── Step 8: one device per session ────────────────────────────────────────
	var deviceUsed bool
	if err := conn.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM attendance_logs
		    WHERE session_id = $1 AND device_fingerprint_hash = $2
		      AND student_id <> $3 AND entry_method = 'QR_SCAN')`,
		sess.SessionID, req.Fingerprint, qr.StudentID).Scan(&deviceUsed); err != nil {
		return reject("INTERNAL_ERROR")
	}
	if deviceUsed {
		return reject("DEVICE_ALREADY_USED")
	}

	// ── Record attendance (append-only) ───────────────────────────────────────
	var seq int
	if err := conn.QueryRow(ctx,
		`SELECT COALESCE(MAX(sequence_number),0)+1 FROM attendance_logs WHERE session_id = $1`,
		sess.SessionID).Scan(&seq); err != nil {
		return reject("INTERNAL_ERROR")
	}
	_, err = conn.Exec(ctx, `
		INSERT INTO attendance_logs
		    (tenant_id, session_id, student_id, checkin_timestamp,
		     device_fingerprint_hash, sequence_number, entry_method, coordinator_id)
		VALUES ($1, $2, $3, $4, $5, $6, 'QR_SCAN', $7)`,
		qr.TenantID, sess.SessionID, qr.StudentID, now, req.Fingerprint, seq, sess.CoordID)
	if err != nil {
		return reject("DUPLICATE_SCAN")
	}

	// ── Build confirmation payload (FR-04.5) ──────────────────────────────────
	var unitName, lecturerName string
	// lal.lecturer_id is varchar(50); lecturers.lecturer_id is uuid — cast required.
	_ = conn.QueryRow(ctx, `
		SELECT cu.name, COALESCE(l.full_name, '')
		FROM course_units cu
		LEFT JOIN sessions s ON s.unit_id = cu.unit_id
		LEFT JOIN lecturer_attendance_logs lal ON lal.session_id = s.session_id
		LEFT JOIN lecturers l ON l.lecturer_id::text = lal.lecturer_id
		WHERE s.session_id = $1
		LIMIT 1`, sess.SessionID).Scan(&unitName, &lecturerName)

	var attendancePct int
	_ = conn.QueryRow(ctx, `
		SELECT COALESCE(ROUND(100.0 * attended / NULLIF(total, 0)), 0)
		FROM (
		  SELECT
		    COUNT(*) FILTER (WHERE al.student_id = $1) AS attended,
		    COUNT(*) AS total
		  FROM sessions s
		  LEFT JOIN attendance_logs al
		       ON al.session_id = s.session_id AND al.student_id = $1 AND al.entry_method = 'QR_SCAN'
		  WHERE s.unit_id = $2 AND s.session_status = 'CLOSED'
		) t`, qr.StudentID, sess.UnitID).Scan(&attendancePct)

	return checkinResponse{
		Status:            "PRESENT",
		StudentName:       qr.FullName,
		UnitName:          unitName,
		LecturerName:      lecturerName,
		SessionDate:       now.Format("2006-01-02"),
		AttendancePercent: attendancePct,
	}
}
