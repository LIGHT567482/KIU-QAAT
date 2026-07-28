package handlers

// Lecturer personal QR → passwordless dashboard login.
//
// Every lecturer has a stable QR (an HMAC-signed <lecturer_id|tenant_id> token).
// Scanning it opens the lecturer dashboard at /?lqr=<token>; the SPA posts the
// token to /api/v1/lecturer/qr-login, which verifies the HMAC, ensures the lecturer
// has a LECTURER login account (auto-creating + linking one if needed), and asks
// auth-service to mint a LECTURER token. The lecturer then sees every unit they're
// mapped to — each separated, each filterable (a unit can be shared across
// coordinators/courses). Admin issues/prints the QR from the Lecturers page.

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func lecturerQRSecret() []byte {
	s := os.Getenv("INTERNAL_SVC_KEY")
	if s == "" {
		s = "qaat-lecturer-qr-secret"
	}
	return []byte(s)
}

// makeLecturerQRToken builds a stable, verifiable token: base64url(payload).hmac.
func makeLecturerQRToken(lecturerID, tenantID string) string {
	payload := lecturerID + "|" + tenantID
	mac := hmac.New(sha256.New, lecturerQRSecret())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func verifyLecturerQRToken(token string) (lecturerID, tenantID string, ok bool) {
	// Accept either a bare token or a full "…/?lqr=<token>" URL.
	if i := strings.LastIndex(token, "lqr="); i >= 0 {
		token = token[i+len("lqr="):]
		if amp := strings.IndexAny(token, "&"); amp >= 0 {
			token = token[:amp]
		}
	}
	parts := strings.SplitN(strings.TrimSpace(token), ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", false
	}
	mac := hmac.New(sha256.New, lecturerQRSecret())
	mac.Write(raw)
	want, err := hex.DecodeString(parts[1])
	if err != nil || !hmac.Equal(want, mac.Sum(nil)) {
		return "", "", false
	}
	pp := strings.SplitN(string(raw), "|", 2)
	if len(pp) != 2 {
		return "", "", false
	}
	return pp[0], pp[1], true
}

// ensureLecturerLogin makes sure the lecturer has a linked LECTURER user account
// (so a JWT subject exists), auto-creating one with a synthesised institution email
// if necessary, and returns that user_id.
func ensureLecturerLogin(ctx context.Context, pool *pgxpool.Pool, tenantID, lecturerID string) (string, error) {
	var userID, email, name, domain string
	err := pool.QueryRow(ctx, `
		SELECT COALESCE(l.user_id::text,''), COALESCE(l.email,''), l.full_name, COALESCE(t.domain,'')
		FROM lecturers l JOIN tenants t ON t.tenant_id = l.tenant_id
		WHERE l.lecturer_id = $1::uuid AND l.tenant_id = $2`, lecturerID, tenantID).Scan(&userID, &email, &name, &domain)
	if err != nil {
		return "", fmt.Errorf("lecturer not found")
	}
	if userID != "" {
		return userID, nil
	}
	// Synthesise an email if the lecturer has none (the account is passwordless via QR).
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || !emailInDomain(email, strings.ToLower(domain)) {
		short := strings.ReplaceAll(lecturerID, "-", "")
		if len(short) > 10 {
			short = short[:10]
		}
		email = fmt.Sprintf("lecturer.%s@%s", short, domain)
	}
	// Default sign-in password for the unified app; the lecturer changes it later. (Was a random
	// unusable password when lecturers were QR-only — see migration 052 for existing rows.)
	hash, _ := bcrypt.GenerateFromPassword([]byte("Lecturer"), 12)

	if err := pool.QueryRow(ctx, `
		INSERT INTO users (tenant_id, email, password_hash, role, full_name, force_password_change)
		VALUES ($1, $2, $3, 'LECTURER', $4, true)
		ON CONFLICT (tenant_id, email) DO UPDATE SET role = users.role
		RETURNING user_id::text`,
		tenantID, email, string(hash), name).Scan(&userID); err != nil {
		return "", fmt.Errorf("could not create lecturer login: %w", err)
	}
	_, _ = pool.Exec(ctx, `UPDATE lecturers SET user_id = $1::uuid WHERE lecturer_id = $2::uuid AND tenant_id = $3`,
		userID, lecturerID, tenantID)
	return userID, nil
}

func mintLecturerToken(ctx context.Context, userID, tenantID string) ([]byte, int) {
	base := os.Getenv("AUTH_SERVICE_URL")
	if base == "" {
		base = "http://auth-service:8081"
	}
	body, _ := json.Marshal(map[string]string{"user_id": userID, "tenant_id": tenantID})
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(rctx, http.MethodPost, base+"/internal/lecturer-token", bytes.NewReader(body))
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

// lecturerScanURL builds the public scan URL for a lecturer's QR from the
// inbound request host (admin-console origin + lecturer portal port).
func lecturerScanURL(r *http.Request, token string) string {
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = strings.Split(fh, ",")[0]
	}
	hostname := strings.TrimSpace(strings.Split(host, ":")[0])
	port := os.Getenv("LECTURER_PORTAL_PORT")
	if port == "" {
		port = "3001"
	}
	return fmt.Sprintf("https://%s:%s/?lqr=%s", hostname, port, token)
}

// emailLecturerQR fires an async request to qr-generator to render the
// lecturer's permanent career QR and email it. No-op when email is blank.
// Best-effort: any failure is logged by qr-generator and never blocks the
// caller (create / bulk import). Tenant is derived by qr-generator from the
// forwarded Authorization JWT, so only the auth header is needed.
func emailLecturerQR(authHeader, qrURL, to, name, staffID string) {
	to = strings.TrimSpace(to)
	if authHeader == "" || to == "" || qrURL == "" {
		return
	}
	base := os.Getenv("QR_GENERATOR_URL")
	if base == "" {
		base = "http://qr-generator:3002"
	}
	body, _ := json.Marshal(map[string]string{
		"to":         to,
		"url":        qrURL,
		"name":       name,
		"subject_id": staffID,
		"heading":    "Your Lecturer QR",
		"intro":      "Scan this QR with your phone to open your attendance dashboard. It is permanent — keep it for your whole time at the institution.",
	})
	go func() {
		req, err := http.NewRequest(http.MethodPost, base+"/api/v1/qr/email-link", bytes.NewReader(body))
		if err != nil {
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", authHeader)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		_ = resp.Body.Close()
	}()
}

// POST /api/v1/lecturer/qr-login  (public) — body {"qr": "<token or URL>"}
func LecturerQRLogin(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			QR string `json:"qr"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.QR) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "qr token is required"))
			return
		}
		lecturerID, tenantID, ok := verifyLecturerQRToken(req.QR)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errBody("INVALID_QR", "this lecturer QR is invalid"))
			return
		}
		userID, err := ensureLecturerLogin(r.Context(), adminPool, tenantID, lecturerID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_LOGIN", err.Error()))
			return
		}
		tokenResp, code := mintLecturerToken(r.Context(), userID, tenantID)
		if code != http.StatusOK {
			writeJSON(w, http.StatusBadGateway, errBody("TOKEN_FAILED", "could not issue lecturer token"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenResp)
	}
}

// GET /api/v1/admin/tenants/{tenant_id}/lecturers/{lecturer_id}/qr  (ADMIN)
// Returns the lecturer's stable QR token + the scan URL (opens the lecturer
// dashboard on the admin-console origin). The frontend renders the QR image.
func AdminLecturerQR(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		lecturerID := chi.URLParam(r, "lecturer_id")
		var name, staffID string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT full_name, COALESCE(staff_id,'') FROM lecturers WHERE lecturer_id = $1::uuid AND tenant_id = $2`,
			lecturerID, tenantID).Scan(&name, &staffID); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "lecturer not found"))
			return
		}
		// Provision the login now so the first scan works instantly.
		_, _ = ensureLecturerLogin(r.Context(), adminPool, tenantID, lecturerID)

		token := makeLecturerQRToken(lecturerID, tenantID)
		// The lecturer dashboard lives on the admin-console origin (default :3001).
		url := lecturerScanURL(r, token)
		writeJSON(w, http.StatusOK, map[string]string{
			"lecturer_id": lecturerID, "full_name": name, "staff_id": staffID,
			"token": token, "url": url,
		})
	}
}
