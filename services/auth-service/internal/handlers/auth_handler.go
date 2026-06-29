package handlers

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/qaat/auth-service/internal/crypto"
	"github.com/qaat/auth-service/internal/models"
	"github.com/qaat/auth-service/internal/store"
	"golang.org/x/crypto/bcrypt"
)

type AuthHandler struct {
	users       *store.UserStore
	tokens      *store.TokenStore
	jwt         *crypto.JWTService
	jwtTTL      time.Duration
	disableMFA  bool
	internalKey string // shared secret the API gateway presents to mint student tokens
}

func NewAuthHandler(users *store.UserStore, tokens *store.TokenStore, jwt *crypto.JWTService, jwtTTL time.Duration, disableMFA bool, internalKey string) *AuthHandler {
	return &AuthHandler{users: users, tokens: tokens, jwt: jwt, jwtTTL: jwtTTL, disableMFA: disableMFA, internalKey: internalKey}
}

// ─── POST /api/v1/auth/change-password ───────────────────────────────────────
//
// Any signed-in user (any role) changes their OWN password. Identity comes from
// the bearer token; the current password is re-verified before the change.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearer(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return
	}
	claims, err := h.jwt.Parse(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "TOKEN_INVALID", "token is invalid")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed JSON body")
		return
	}
	if len(req.NewPassword) < 8 {
		writeError(w, http.StatusBadRequest, "WEAK_PASSWORD", "new password must be at least 8 characters")
		return
	}
	user, err := h.users.GetByID(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "account not found")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "current password is incorrect")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), 12)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "password hashing failed")
		return
	}
	if err := h.users.UpdatePassword(r.Context(), user.UserID, string(hash)); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not update password")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "PASSWORD_CHANGED"})
}

// ─── POST /api/v1/auth/change-email ──────────────────────────────────────────
//
// Any signed-in user changes their OWN sign-in email after re-entering their
// password. Tenant users must keep the institution domain; the SUPER_ADMIN
// (platform owner) may use any address.
func (h *AuthHandler) ChangeEmail(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearer(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return
	}
	claims, err := h.jwt.Parse(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "TOKEN_INVALID", "token is invalid")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewEmail        string `json:"new_email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed JSON body")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.NewEmail))
	at := strings.LastIndex(email, "@")
	if at <= 0 || !strings.Contains(email[at:], ".") {
		writeError(w, http.StatusBadRequest, "INVALID_EMAIL", "a valid email address is required")
		return
	}
	user, err := h.users.GetByID(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "USER_NOT_FOUND", "account not found")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)) != nil {
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "password is incorrect")
		return
	}
	// Tenant users keep the institution domain; the platform owner is exempt.
	if user.Role != models.RoleSuperAdmin {
		domain, _ := h.users.TenantDomain(r.Context(), user.TenantID)
		domain = strings.ToLower(strings.TrimSpace(domain))
		d := email[at+1:]
		if domain == "" || (d != domain && !strings.HasSuffix(d, "."+domain)) {
			writeError(w, http.StatusBadRequest, "EMAIL_DOMAIN_MISMATCH", "email must use the institution domain @"+domain)
			return
		}
	}
	if err := h.users.UpdateEmail(r.Context(), user.UserID, email); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			writeError(w, http.StatusConflict, "EMAIL_TAKEN", "that email is already in use")
			return
		}
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not update email")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "EMAIL_CHANGED", "email": email})
}

// ─── POST /internal/lecturer-token ───────────────────────────────────────────
//
// Service-to-service only (shared secret): the gateway calls this AFTER verifying a
// lecturer's signed QR, to mint a LECTURER access token for their passwordless
// dashboard login. Mirrors IssueStudentToken but for the LECTURER role.
func (h *AuthHandler) IssueLecturerToken(w http.ResponseWriter, r *http.Request) {
	if h.internalKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Key")), []byte(h.internalKey)) != 1 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid internal key")
		return
	}
	var req struct {
		UserID   string `json:"user_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.TenantID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "user_id and tenant_id are required")
		return
	}
	user, err := h.users.GetByID(r.Context(), req.UserID)
	if err != nil || user.TenantID != req.TenantID || user.Role != models.RoleLecturer || !user.IsActive {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "not an active lecturer in this tenant")
		return
	}
	tokenStr, jti, expiresAt, err := h.jwt.Issue(user.UserID, user.TenantID, string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not issue token")
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: tokenStr,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		JTI:         jti,
		Role:        string(user.Role),
		UserID:      user.UserID,
		TenantID:    user.TenantID,
	})
}

// ─── POST /internal/coordinator-token ────────────────────────────────────────
//
// Service-to-service only (shared secret): the gateway calls this AFTER verifying a
// coordinator's standby delegation (emergency fallback), to mint a COORDINATOR token
// for the ABSENT coordinator's own identity so the standby student can run that
// session. ttl_seconds shortens the token (e.g. to end of day); never extends it.
func (h *AuthHandler) IssueCoordinatorToken(w http.ResponseWriter, r *http.Request) {
	if h.internalKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Key")), []byte(h.internalKey)) != 1 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid internal key")
		return
	}
	var req struct {
		UserID     string `json:"user_id"`
		TenantID   string `json:"tenant_id"`
		TTLSeconds int64  `json:"ttl_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.TenantID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "user_id and tenant_id are required")
		return
	}
	user, err := h.users.GetByID(r.Context(), req.UserID)
	if err != nil || user.TenantID != req.TenantID || user.Role != models.RoleCoordinator || !user.IsActive {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "not an active coordinator in this tenant")
		return
	}
	tokenStr, jti, expiresAt, err := h.jwt.IssueWithTTL(user.UserID, user.TenantID, string(user.Role), time.Duration(req.TTLSeconds)*time.Second)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not issue token")
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: tokenStr,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		JTI:         jti,
		Role:        string(user.Role),
		UserID:      user.UserID,
		TenantID:    user.TenantID,
	})
}

// ─── POST /internal/student-token ────────────────────────────────────────────
//
// Service-to-service only: the API gateway calls this AFTER it has cryptographically
// verified a student's signed QR, to obtain a STUDENT access token (auth-service is
// the sole holder of the JWT signing key). Gated by a shared secret so it cannot be
// abused even though auth-service is reachable on the internal network.
func (h *AuthHandler) IssueStudentToken(w http.ResponseWriter, r *http.Request) {
	if h.internalKey == "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Internal-Key")), []byte(h.internalKey)) != 1 {
		writeError(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid internal key")
		return
	}
	var req struct {
		UserID   string `json:"user_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" || req.TenantID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "user_id and tenant_id are required")
		return
	}
	user, err := h.users.GetByID(r.Context(), req.UserID)
	if err != nil || user.TenantID != req.TenantID || user.Role != models.RoleStudent || !user.IsActive {
		writeError(w, http.StatusForbidden, "FORBIDDEN", "not an active student in this tenant")
		return
	}
	tokenStr, jti, expiresAt, err := h.jwt.Issue(user.UserID, user.TenantID, string(user.Role))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not issue token")
		return
	}
	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: tokenStr,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		JTI:         jti,
		Role:        string(user.Role),
		UserID:      user.UserID,
		TenantID:    user.TenantID,
	})
}

// ─── Request / Response types ────────────────────────────────────────────────

type loginRequest struct {
	Email         string `json:"email"`
	Password      string `json:"password"`
	TOTPCode      string `json:"totp_code"`
	TenantID      string `json:"tenant_id"`
	InstitutionID string `json:"institution_id"`
}

type loginResponse struct {
	AccessToken      string `json:"access_token"`
	TokenType        string `json:"token_type"`
	ExpiresIn        int64  `json:"expires_in"`
	JTI              string `json:"jti"`
	Role             string `json:"role"`
	UserID           string `json:"user_id"`
	TenantID         string `json:"tenant_id"`
	DeviceBindingKey string `json:"device_binding_key,omitempty"`
}

type refreshRequest struct {
	AccessToken string `json:"access_token"`
}

type enrollTOTPResponse struct {
	OTPAuthURL    string `json:"otpauth_url"`
	BackupCodes   []string `json:"backup_codes"`
}

type verifyTOTPRequest struct {
	Code string `json:"code"`
}

// ─── POST /api/v1/auth/login ─────────────────────────────────────────────────

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "malformed JSON body")
		return
	}

	if req.Email == "" || req.Password == "" || req.TenantID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "email, password, and tenant_id are required")
		return
	}

	user, err := h.users.GetByEmailAndTenant(r.Context(), req.Email, req.TenantID)
	if errors.Is(err, store.ErrNotFound) {
		// Constant-time rejection — don't reveal whether the account exists.
		bcrypt.CompareHashAndPassword([]byte("$2a$12$dummy"), []byte(req.Password)) //nolint:errcheck
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "authentication failed")
		return
	}

	if !user.IsActive {
		writeError(w, http.StatusUnauthorized, "ACCOUNT_INACTIVE", "account is disabled")
		return
	}

	if user.IsLocked(time.Now()) {
		writeError(w, http.StatusTooManyRequests, "ACCOUNT_LOCKED", "account temporarily locked due to failed login attempts")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		h.users.RecordLoginFailure(r.Context(), user.UserID) //nolint:errcheck
		writeError(w, http.StatusUnauthorized, "INVALID_CREDENTIALS", "invalid email or password")
		return
	}

	// TOTP gate: required for VC and DQA_DIRECTOR (bypassed in dev via DISABLE_MFA=true).
	if user.Role.MFARequired() && !h.disableMFA {
		if !user.TOTPEnabled {
			writeError(w, http.StatusForbidden, "MFA_SETUP_REQUIRED", "TOTP MFA must be enrolled before login")
			return
		}
		if req.TOTPCode == "" {
			writeError(w, http.StatusForbidden, "MFA_REQUIRED", "TOTP code is required for this role")
			return
		}
		secret, err := crypto.DecryptSecret(*user.TOTPSecretEnc, user.PasswordHash)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "MFA verification failed")
			return
		}
		if !crypto.ValidateTOTPCode(secret, req.TOTPCode) {
			writeError(w, http.StatusUnauthorized, "INVALID_MFA_CODE", "TOTP code is invalid or expired")
			return
		}
	}

	// Institution-ID gate: a tenant ADMIN must supply the institution_id assigned
	// by the platform owner at tenant registration. Other roles are not asked.
	if user.Role == models.RoleAdmin {
		instID, err := h.users.TenantInstitutionID(r.Context(), user.TenantID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not verify institution")
			return
		}
		if instID == "" || strings.TrimSpace(req.InstitutionID) != instID {
			writeError(w, http.StatusUnauthorized, "INVALID_INSTITUTION_ID", "a valid Institution ID is required for administrators")
			return
		}
	}

	tokenStr, jti, expiresAt, err := h.jwt.Issue(
		user.UserID,
		user.TenantID,
		string(user.Role),
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not issue token")
		return
	}

	h.users.RecordLoginSuccess(r.Context(), user.UserID) //nolint:errcheck

	// Coordinators run the offline PWA, whose IndexedDB vault and session-package
	// encryption are derived (via HKDF) from a server-issued device binding key.
	// Issue one on first login and persist it (encrypted under the shared master
	// key so sync-receiver can later decrypt uploaded packages); return it so the
	// PWA can initialise its vault. Without this the PWA vault never initialises.
	var bindingKey string
	if user.Role == models.RoleCoordinator {
		bindingKey, err = h.resolveBindingKey(r.Context(), user)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not establish device binding")
			return
		}
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken:      tokenStr,
		TokenType:        "Bearer",
		ExpiresIn:        int64(time.Until(expiresAt).Seconds()),
		JTI:              jti,
		Role:             string(user.Role),
		UserID:           user.UserID,
		TenantID:         user.TenantID,
		DeviceBindingKey: bindingKey,
	})
}

// resolveBindingKey returns the coordinator's device binding key in plaintext,
// generating and persisting one (encrypted) on first login. The user_id is used
// as AAD so a stored key cannot be transplanted onto a different user.
func (h *AuthHandler) resolveBindingKey(ctx context.Context, user *models.User) (string, error) {
	if user.DeviceBindingKeyEnc != nil && *user.DeviceBindingKeyEnc != "" {
		return crypto.DecryptWithMasterKey(*user.DeviceBindingKeyEnc, user.UserID)
	}
	key, err := crypto.GenerateBindingKey()
	if err != nil {
		return "", err
	}
	enc, err := crypto.EncryptWithMasterKey(key, user.UserID)
	if err != nil {
		return "", err
	}
	if err := h.users.SetDeviceBindingKey(ctx, user.UserID, enc); err != nil {
		return "", err
	}
	return key, nil
}

// ─── POST /api/v1/auth/refresh ───────────────────────────────────────────────

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearer(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return
	}

	claims, err := h.jwt.Parse(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "TOKEN_INVALID", "token is invalid or expired")
		return
	}

	// Check blacklist.
	blacklisted, err := h.tokens.IsBlacklisted(r.Context(), claims.ID)
	if err != nil || blacklisted {
		writeError(w, http.StatusUnauthorized, "TOKEN_REVOKED", "token has been revoked")
		return
	}

	// Blacklist the old jti.
	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining > 0 {
		h.tokens.Blacklist(r.Context(), claims.ID, remaining) //nolint:errcheck
	}

	// Issue new token.
	newToken, newJTI, expiresAt, err := h.jwt.Issue(claims.Subject, claims.TenantID, claims.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not issue token")
		return
	}

	writeJSON(w, http.StatusOK, loginResponse{
		AccessToken: newToken,
		TokenType:   "Bearer",
		ExpiresIn:   int64(time.Until(expiresAt).Seconds()),
		JTI:         newJTI,
		Role:        claims.Role,
		UserID:      claims.Subject,
	})
}

// ─── POST /api/v1/auth/logout ────────────────────────────────────────────────

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearer(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return
	}

	claims, err := h.jwt.Parse(tokenStr)
	if err != nil {
		// Token is already invalid — treat as logged out.
		w.WriteHeader(http.StatusNoContent)
		return
	}

	remaining := time.Until(claims.ExpiresAt.Time)
	if remaining > 0 {
		h.tokens.Blacklist(r.Context(), claims.ID, remaining) //nolint:errcheck
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── POST /api/v1/auth/mfa/enroll ────────────────────────────────────────────

func (h *AuthHandler) EnrollMFA(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearer(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return
	}
	claims, err := h.jwt.Parse(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "TOKEN_INVALID", "token is invalid")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}

	if user.TOTPEnabled {
		writeError(w, http.StatusConflict, "MFA_ALREADY_ENROLLED", "TOTP is already active for this account")
		return
	}

	secret, otpauthURL, err := crypto.GenerateTOTPSecret(user.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not generate TOTP secret")
		return
	}

	encSecret, err := crypto.EncryptSecret(secret, user.PasswordHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not encrypt TOTP secret")
		return
	}

	if err := h.users.SetTOTPSecret(r.Context(), user.UserID, encSecret); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not store TOTP secret")
		return
	}

	backupCodes, err := crypto.GenerateBackupCodes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "could not generate backup codes")
		return
	}

	writeJSON(w, http.StatusOK, enrollTOTPResponse{
		OTPAuthURL:  otpauthURL,
		BackupCodes: backupCodes,
	})
}

// ─── POST /api/v1/auth/mfa/verify ────────────────────────────────────────────
// Called after scanning the QR code to activate TOTP.

func (h *AuthHandler) VerifyMFA(w http.ResponseWriter, r *http.Request) {
	tokenStr := extractBearer(r)
	if tokenStr == "" {
		writeError(w, http.StatusUnauthorized, "MISSING_TOKEN", "Authorization header required")
		return
	}
	claims, err := h.jwt.Parse(tokenStr)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "TOKEN_INVALID", "token is invalid")
		return
	}

	var req verifyTOTPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "code is required")
		return
	}

	user, err := h.users.GetByID(r.Context(), claims.Subject)
	if err != nil {
		writeError(w, http.StatusNotFound, "USER_NOT_FOUND", "user not found")
		return
	}

	if user.TOTPSecretEnc == nil {
		writeError(w, http.StatusBadRequest, "MFA_NOT_INITIATED", "enroll MFA first")
		return
	}

	secret, err := crypto.DecryptSecret(*user.TOTPSecretEnc, user.PasswordHash)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "MFA verification failed")
		return
	}

	if !crypto.ValidateTOTPCode(secret, req.Code) {
		writeError(w, http.StatusUnauthorized, "INVALID_MFA_CODE", "code is invalid or expired")
		return
	}

	backupCodes, _ := crypto.GenerateBackupCodes()
	encBackup, _ := crypto.EncryptSecret(strings.Join(backupCodes, ","), user.PasswordHash)
	h.users.EnableTOTP(r.Context(), user.UserID, encBackup) //nolint:errcheck

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "MFA_ENABLED",
		"backup_codes": backupCodes,
	})
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(h, "Bearer ")
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}

