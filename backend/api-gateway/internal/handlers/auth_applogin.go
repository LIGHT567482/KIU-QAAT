package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AppLogin is the SINGLE sign-in for the unified "KIU QAAT" app. Everyone (coordinator, student,
// lecturer, admin) signs in with an identifier + password + institution; we resolve the identifier
// to the account's email, reuse the normal auth-service /auth/login (bcrypt, lockout, TOTP, token),
// then AUGMENT the response with student_id / staff_id so the client can route by role and — for a
// student — check in offline (the JWT itself carries only user_id/role/tenant_id).
//
// Endpoint: POST /api/v1/auth/app-login  (public, rate-limited)
// Body:     {identifier, password, org, totp_code?}
//
//	identifier = email (staff/coordinator) OR registration number (student) OR staff ID (lecturer).
func AppLogin(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
			Org        string `json:"org"`
			TOTPCode   string `json:"totp_code"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "identifier, password and institution are required"))
			return
		}
		id := strings.TrimSpace(req.Identifier)
		if id == "" || req.Password == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "identifier and password are required"))
			return
		}
		ctx := r.Context()

		// Single institution — the one tenant, no org needed.
		tenantID := singleTenantID(ctx, adminPool)
		if tenantID == "" {
			writeJSON(w, http.StatusInternalServerError, errBody("NO_TENANT", "institution not configured"))
			return
		}

		// Resolve the identifier to the account email (+ student_id / staff_id when applicable).
		email, studentID, staffID := resolveIdentifier(ctx, adminPool, tenantID, id)
		if email == "" {
			writeJSON(w, http.StatusUnauthorized, errBody("INVALID_CREDENTIALS", "no account found for that ID at this institution"))
			return
		}

		// Reuse the real auth-service login (bcrypt, lockout, TOTP, token issuance).
		body, _ := json.Marshal(map[string]string{
			"email": email, "password": req.Password, "tenant_id": tenantID, "totp_code": req.TOTPCode,
		})
		base := os.Getenv("AUTH_SERVICE_URL")
		if base == "" {
			base = "http://auth-service:8081"
		}
		rctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		hreq, _ := http.NewRequestWithContext(rctx, http.MethodPost, base+"/api/v1/auth/login", bytes.NewReader(body))
		hreq.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(hreq)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, errBody("LOGIN_UNAVAILABLE", "sign-in service unreachable — try again"))
			return
		}
		defer resp.Body.Close()
		raw := new(bytes.Buffer)
		_, _ = raw.ReadFrom(resp.Body)

		// Pass through non-200s verbatim (invalid password, MFA required, locked, etc.).
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(resp.StatusCode)
			_, _ = w.Write(raw.Bytes())
			return
		}

		// Augment the success body with the role-specific identity the app needs.
		var obj map[string]interface{}
		if json.Unmarshal(raw.Bytes(), &obj) != nil {
			obj = map[string]interface{}{}
		}
		role, _ := obj["role"].(string)
		userID, _ := obj["user_id"].(string)
		switch role {
		case "STUDENT":
			if studentID == "" {
				adminPool.QueryRow(ctx, //nolint:errcheck
					`SELECT student_id FROM students_extended WHERE tenant_id = $1 AND lower(email) = lower($2) LIMIT 1`,
					tenantID, email).Scan(&studentID)
			}
			obj["student_id"] = studentID
		case "LECTURER":
			if staffID == "" && userID != "" {
				adminPool.QueryRow(ctx, //nolint:errcheck
					`SELECT staff_id FROM lecturers WHERE tenant_id = $1 AND user_id = $2 LIMIT 1`,
					tenantID, userID).Scan(&staffID)
			}
			obj["staff_id"] = staffID
		}
		writeJSON(w, http.StatusOK, obj)
	}
}

// resolveIdentifier maps an email / registration number / staff ID (within a tenant) to the account
// email, plus the student_id or staff_id when the identifier itself was one. Uses adminPool.
func resolveIdentifier(ctx context.Context, adminPool *pgxpool.Pool, tenantID, id string) (email, studentID, staffID string) {
	// 1) Email (staff/coordinator/admin — or a student/lecturer who typed their email).
	if strings.Contains(id, "@") {
		var e string
		if adminPool.QueryRow(ctx,
			`SELECT email FROM users WHERE tenant_id = $1 AND lower(email) = lower($2) LIMIT 1`,
			tenantID, id).Scan(&e) == nil && e != "" {
			return e, "", ""
		}
	}
	// 2) Student registration number → its account email. Ensure a login exists: some students were
	// imported with NO users row (the tenant had no domain set at import time, or they came in via a
	// path that didn't create the hidden login), so a valid reg-number resolved but then failed
	// auth-service with "no account". Provision lazily — default password "Student", force change on
	// first sign-in — so any ACTIVE student whose reg-number exists can always sign in.
	var sName string
	if adminPool.QueryRow(ctx,
		`SELECT email, student_id, COALESCE(full_name,'') FROM students_extended
		  WHERE tenant_id = $1 AND student_id = $2 AND enrollment_status = 'ACTIVE' LIMIT 1`,
		tenantID, id).Scan(&email, &studentID, &sName) == nil && email != "" {
		ensureStudentLogin(ctx, adminPool, tenantID, email, sName)
		return email, studentID, ""
	}
	// 3) Lecturer staff ID → ensure a login account exists, then return its email. Lecturers imported
	// by the coordinator live in `lecturers` with NO linked users row (user_id NULL) and thus no
	// password, so a plain JOIN found nothing and login wrongly reported "no account found" even
	// though the staff ID is valid. Provision the login lazily here — default password "Lecturer",
	// force change on first sign-in — exactly like the passwordless portal (ensureLecturerLogin),
	// so any lecturer whose staff ID exists can sign in with the default and be sent to change it.
	var lecturerID string
	if adminPool.QueryRow(ctx,
		`SELECT lecturer_id::text, staff_id FROM lecturers
		  WHERE tenant_id = $1 AND staff_id = $2 LIMIT 1`,
		tenantID, id).Scan(&lecturerID, &staffID) == nil && lecturerID != "" {
		if userID, err := ensureLecturerLogin(ctx, adminPool, tenantID, lecturerID); err == nil && userID != "" {
			var e string
			if adminPool.QueryRow(ctx,
				`SELECT email FROM users WHERE user_id = $1::uuid`, userID).Scan(&e) == nil && e != "" {
				return e, "", staffID
			}
		}
	}
	return "", "", ""
}
