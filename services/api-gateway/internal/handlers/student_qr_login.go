package handlers

// StudentQRLogin — passwordless portal login by scanning the student's personal QR.
//
// A student owns no app and types no password: they scan their QR with the phone's
// native camera/browser, which opens the student portal at /?qr=<token>. The portal
// posts that token here. We cryptographically verify the QR (RSA signature against
// the tenant's public key — only qr-generator could have produced it), confirm the
// student is active and the serial is current (a reissued QR revokes the old one),
// then ask auth-service (the sole holder of the JWT signing key) to mint a short
// STUDENT token. The student lands in the portal able to attend + track attendance.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/checkin"
	"github.com/qaat/api-gateway/internal/middleware"
)

type qrLoginRequest struct {
	QR string `json:"qr"` // base64url(JSON signed QR payload) — the ?qr= value
}

func StudentQRLogin(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req qrLoginRequest
		if err := decodeJSON(r, &req); err != nil || req.QR == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "qr token is required"))
			return
		}

		// Accept either a bare base64url token or a full URL the camera opened.
		token := req.QR
		if i := indexOfQRParam(token); i >= 0 {
			token = token[i:]
		}

		qr, err := checkin.ParseQRToken(token)
		if err != nil {
			// Compact QR form: a bare serial number (qr_serial_number is globally
			// unique, so it identifies the student). This keeps the QR small enough
			// to scan reliably; the server resolves + verifies it against the DB.
			serial := strings.TrimSpace(token)
			if serial == "" || len(serial) > 80 {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_QR", "could not read the QR code"))
				return
			}
			var userID, tenantID string
			e := adminPool.QueryRow(r.Context(), `
				SELECT u.user_id::text, se.tenant_id::text
				FROM students_extended se
				JOIN users u ON u.email = se.email AND u.tenant_id = se.tenant_id
				WHERE se.qr_serial_number = $1 AND se.enrollment_status = 'ACTIVE'
				  AND u.role = 'STUDENT' AND u.is_active = true`, serial).Scan(&userID, &tenantID)
			if e != nil {
				writeJSON(w, http.StatusUnauthorized, errBody("QR_REVOKED", "this QR is no longer valid — ask for a reissue, or your enrolment is inactive"))
				return
			}
			tokenResp, code := mintStudentToken(r.Context(), userID, tenantID)
			if code != http.StatusOK {
				writeJSON(w, http.StatusBadGateway, errBody("LOGIN_FAILED", "could not establish a session"))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(tokenResp)
			return
		}
		if !middleware.ValidTenantID(qr.TenantID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_QR", "QR has no valid tenant"))
			return
		}

		// Verify the RSA signature against the tenant's public key (adminPool: the
		// key registry is platform-managed and tenant is taken from the signed body).
		var publicKeyPEM string
		err = adminPool.QueryRow(r.Context(), `
			SELECT COALESCE(rsa_public_key_pem, '')
			FROM tenant_rsa_keys
			WHERE tenant_id = $1 AND revoked_at IS NULL
			ORDER BY created_at DESC LIMIT 1`, qr.TenantID).Scan(&publicKeyPEM)
		if err != nil || publicKeyPEM == "" {
			writeJSON(w, http.StatusUnauthorized, errBody("INVALID_QR", "QR could not be verified"))
			return
		}
		if err := qr.VerifySignature(publicKeyPEM); err != nil {
			writeJSON(w, http.StatusUnauthorized, errBody("INVALID_QR", "QR signature is invalid"))
			return
		}

		// Expiry check (printed QRs are valid ~1 year; expired ones are rejected).
		if exp, perr := time.Parse("2006-01-02", qr.ExpiryDate); perr == nil &&
			exp.Before(time.Now().UTC().Truncate(24*time.Hour)) {
			writeJSON(w, http.StatusUnauthorized, errBody("QR_EXPIRED", "this QR code has expired — ask for a reissue"))
			return
		}

		// Resolve the student → login account, and confirm the serial is the current
		// one (a reissued QR bumps qr_serial_number, invalidating older prints).
		var userID string
		err = adminPool.QueryRow(r.Context(), `
			SELECT u.user_id::text
			FROM students_extended se
			JOIN users u ON u.email = se.email AND u.tenant_id = se.tenant_id
			WHERE se.student_id = $1 AND se.tenant_id = $2
			  AND se.enrollment_status = 'ACTIVE'
			  AND COALESCE(se.qr_serial_number,'') = $3
			  AND u.role = 'STUDENT' AND u.is_active = true`,
			qr.StudentID, qr.TenantID, qr.SerialNumber).Scan(&userID)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errBody("QR_REVOKED", "this QR is no longer valid — a newer one was issued, or enrolment is inactive"))
			return
		}

		// Mint the STUDENT token via auth-service (it alone holds the signing key).
		tokenResp, code := mintStudentToken(r.Context(), userID, qr.TenantID)
		if code != http.StatusOK {
			writeJSON(w, http.StatusBadGateway, errBody("LOGIN_FAILED", "could not establish a session"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenResp)
	}
}

// mintStudentToken calls auth-service's internal endpoint with the shared secret.
func mintStudentToken(ctx context.Context, userID, tenantID string) ([]byte, int) {
	base := os.Getenv("AUTH_SERVICE_URL")
	if base == "" {
		base = "http://auth-service:8081"
	}
	body, _ := json.Marshal(map[string]string{"user_id": userID, "tenant_id": tenantID})
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(rctx, http.MethodPost, base+"/internal/student-token", bytes.NewReader(body))
	if err != nil {
		return nil, http.StatusBadGateway
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Key", os.Getenv("INTERNAL_SVC_KEY"))
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, http.StatusBadGateway
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), resp.StatusCode
}

// indexOfQRParam returns the start index of the base64url token if the value is a
// full "…/?qr=<token>" URL, else -1 (the value is already a bare token).
func indexOfQRParam(s string) int {
	const marker = "qr="
	for i := 0; i+len(marker) <= len(s); i++ {
		if s[i:i+len(marker)] == marker {
			return i + len(marker)
		}
	}
	return -1
}
