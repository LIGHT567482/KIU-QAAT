package handlers

// Emergency standby coordinator (migration 042). If a coordinator is absent, they
// — and only they — can pre-authorise a student of their OWN cohort to act as the
// coordinator for the rest of the day. The student exchanges a code + their reg-no
// for a COORDINATOR token minted for the absent coordinator's own identity, scoped
// to that one offering and capped to end of day, then runs the session normally.
//
//   POST   /api/v1/coordinator/standby            (COORDINATOR) — issue a standby
//   GET    /api/v1/coordinator/standby            (COORDINATOR) — list active
//   POST   /api/v1/coordinator/standby/{id}/revoke (COORDINATOR)
//   POST   /api/v1/auth/coordinator-standby-login  (public)     — code + reg → token

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// randStandbyCode returns an 8-char human-readable code (no ambiguous chars).
func randStandbyCode() string {
	const alpha = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	out := make([]byte, 8)
	for i := range b {
		out[i] = alpha[int(b[i])%len(alpha)]
	}
	return string(out)
}

// POST /api/v1/coordinator/standby  {deputy_reg}
func CreateCoordinatorStandby(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())

		var req struct {
			DeputyReg string `json:"deputy_reg"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.DeputyReg) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "deputy_reg (the standby student's registration number) is required"))
			return
		}
		deputyReg := strings.TrimSpace(req.DeputyReg)

		// The caller's own offering (one coordinator owns one offering).
		var offeringID string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT offering_id::text FROM course_offerings WHERE coordinator_id = $1 AND tenant_id = $2`,
			coordID, tenantID).Scan(&offeringID); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("NO_OFFERING", "you are not assigned to a cohort"))
			return
		}

		// The standby MUST be a student of this coordinator's own cohort.
		var deputyName string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT full_name FROM students_extended
			 WHERE student_id = $1 AND tenant_id = $2 AND offering_id = $3::uuid AND enrollment_status = 'ACTIVE'`,
			deputyReg, tenantID, offeringID).Scan(&deputyName); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NOT_IN_COHORT", "that registration number is not an active student in your cohort"))
			return
		}

		// Valid until end of today (server TZ).
		now := time.Now()
		endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

		var delegationID, code string
		var err error
		for attempt := 0; attempt < 5; attempt++ {
			code = randStandbyCode()
			err = adminPool.QueryRow(r.Context(), `
				INSERT INTO coordinator_delegations
				    (tenant_id, offering_id, coordinator_id, deputy_student_id, deputy_name, code, expires_at)
				VALUES ($1, $2::uuid, $3, $4, $5, $6, $7)
				RETURNING delegation_id::text`,
				tenantID, offeringID, coordID, deputyReg, deputyName, code, endOfDay).Scan(&delegationID)
			if err == nil || !strings.Contains(err.Error(), "coordinator_delegations") {
				break // success, or a non-unique-code error
			}
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"delegation_id": delegationID,
			"code":          code,
			"deputy_name":   deputyName,
			"deputy_reg":    deputyReg,
			"expires_at":    endOfDay.Format(time.RFC3339),
		})
	}
}

// GET /api/v1/coordinator/standby — this coordinator's active (non-revoked, unexpired) standbys.
func ListCoordinatorStandby(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		rows, err := adminPool.Query(r.Context(), `
			SELECT delegation_id::text, code, deputy_student_id, COALESCE(deputy_name,''),
			       expires_at::text, COALESCE(last_used_at::text,'')
			FROM coordinator_delegations
			WHERE tenant_id = $1 AND coordinator_id = $2 AND NOT revoked AND expires_at > now()
			ORDER BY created_at DESC`, tenantID, coordID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()
		type item struct {
			DelegationID string `json:"delegation_id"`
			Code         string `json:"code"`
			DeputyReg    string `json:"deputy_reg"`
			DeputyName   string `json:"deputy_name"`
			ExpiresAt    string `json:"expires_at"`
			LastUsedAt   string `json:"last_used_at,omitempty"`
		}
		out := []item{}
		for rows.Next() {
			var it item
			rows.Scan(&it.DelegationID, &it.Code, &it.DeputyReg, &it.DeputyName, &it.ExpiresAt, &it.LastUsedAt) //nolint:errcheck
			out = append(out, it)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// POST /api/v1/coordinator/standby/{id}/revoke
func RevokeCoordinatorStandby(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		coordID := middleware.GetUserID(r.Context())
		id := chi.URLParam(r, "id")
		tag, err := adminPool.Exec(r.Context(),
			`UPDATE coordinator_delegations SET revoked = true
			 WHERE delegation_id = $1::uuid AND tenant_id = $2 AND coordinator_id = $3`,
			id, tenantID, coordID)
		if err != nil || tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "standby not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "REVOKED"})
	}
}

// POST /api/v1/auth/coordinator-standby-login  {code, reg}  (public)
func CoordinatorStandbyLogin(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Code string `json:"code"`
			Reg  string `json:"reg"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.Code) == "" || strings.TrimSpace(req.Reg) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "code and reg are required"))
			return
		}
		code := strings.ToUpper(strings.TrimSpace(req.Code))
		reg := strings.TrimSpace(req.Reg)

		var delegationID, tenantID, coordID, deputyReg string
		var expiresAt time.Time
		var revoked bool
		err := adminPool.QueryRow(r.Context(), `
			SELECT delegation_id::text, tenant_id::text, coordinator_id, deputy_student_id, expires_at, revoked
			FROM coordinator_delegations WHERE code = $1`, code).Scan(
			&delegationID, &tenantID, &coordID, &deputyReg, &expiresAt, &revoked)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, errBody("INVALID_CODE", "that standby code is not valid"))
			return
		}
		if revoked || time.Now().After(expiresAt) {
			writeJSON(w, http.StatusUnauthorized, errBody("EXPIRED", "this standby is no longer valid"))
			return
		}
		if !strings.EqualFold(reg, deputyReg) {
			writeJSON(w, http.StatusUnauthorized, errBody("WRONG_REG", "this code was issued to a different student"))
			return
		}

		ttlSeconds := int64(time.Until(expiresAt).Seconds())
		tokenResp, ok := mintCoordinatorToken(r.Context(), coordID, tenantID, ttlSeconds)
		if !ok {
			writeJSON(w, http.StatusBadGateway, errBody("TOKEN_FAILED", "could not issue standby token"))
			return
		}
		_, _ = adminPool.Exec(r.Context(),
			`UPDATE coordinator_delegations SET last_used_at = now() WHERE delegation_id = $1::uuid`, delegationID)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenResp)
	}
}

// mintCoordinatorToken asks auth-service to mint a COORDINATOR token for the absent
// coordinator's user_id, capped to ttlSeconds. Mirrors mintLecturerToken.
func mintCoordinatorToken(ctx context.Context, userID, tenantID string, ttlSeconds int64) ([]byte, bool) {
	base := os.Getenv("AUTH_SERVICE_URL")
	if base == "" {
		base = "http://auth-service:8081"
	}
	body, _ := json.Marshal(map[string]interface{}{"user_id": userID, "tenant_id": tenantID, "ttl_seconds": ttlSeconds})
	rctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(rctx, http.MethodPost, base+"/internal/coordinator-token", bytes.NewReader(body))
	if err != nil {
		return nil, false
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Key", os.Getenv("INTERNAL_SVC_KEY"))
	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	return buf.Bytes(), resp.StatusCode == http.StatusOK
}
