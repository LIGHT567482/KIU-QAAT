package handlers

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

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func coordinatorQRSecret() []byte {
	s := os.Getenv("INTERNAL_SVC_KEY")
	if s == "" {
		s = "qaat-coordinator-qr-secret"
	}
	return []byte(s)
}

func makeCoordinatorQRToken(coordinatorID, offeringID, tenantID string) string {
	payload := coordinatorID + "|" + offeringID + "|" + tenantID
	mac := hmac.New(sha256.New, coordinatorQRSecret())
	mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + hex.EncodeToString(mac.Sum(nil))
}

func verifyCoordinatorQRToken(token string) (coordinatorID, offeringID, tenantID string, ok bool) {
	if i := strings.LastIndex(token, "cqr="); i >= 0 {
		token = token[i+len("cqr="):]
		if amp := strings.IndexAny(token, "&"); amp >= 0 {
			token = token[:amp]
		}
	}
	parts := strings.SplitN(strings.TrimSpace(token), ".", 2)
	if len(parts) != 2 {
		return "", "", "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", "", "", false
	}
	mac := hmac.New(sha256.New, coordinatorQRSecret())
	mac.Write(raw)
	want, err := hex.DecodeString(parts[1])
	if err != nil || !hmac.Equal(want, mac.Sum(nil)) {
		return "", "", "", false
	}
	pp := strings.SplitN(string(raw), "|", 3)
	if len(pp) != 3 {
		return "", "", "", false
	}
	return pp[0], pp[1], pp[2], true
}

func ensureCoordinatorLogin(ctx context.Context, pool *pgxpool.Pool, tenantID, coordinatorID string) (string, error) {
	var userID, email, name string
	err := pool.QueryRow(ctx, `
		SELECT u.user_id::text, u.email, u.full_name
		FROM users u
		WHERE u.user_id = $1::uuid AND u.tenant_id = $2 AND u.role = 'COORDINATOR' AND u.is_active = true`,
		coordinatorID, tenantID).Scan(&userID, &email, &name)
	if err != nil {
		return "", fmt.Errorf("active coordinator not found")
	}
	return userID, nil
}

func coordinatorScanURL(r *http.Request, token string) string {
	host := r.Host
	if fh := r.Header.Get("X-Forwarded-Host"); fh != "" {
		host = strings.Split(fh, ",")[0]
	}
	hostname := strings.TrimSpace(strings.Split(host, ":")[0])
	port := os.Getenv("COORDINATOR_PORTAL_PORT")
	if port == "" {
		port = "3001"
	}
	return fmt.Sprintf("https://%s:%s/?cqr=%s", hostname, port, token)
}

func emailCoordinatorQR(authHeader, qrURL, to, name, coordCode string) {
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
		"subject_id": coordCode,
		"heading":    "Your Coordinator QR",
		"intro":      "Scan this QR with your phone to open your cohort dashboard. It is permanent — keep it for your whole time coordinating this cohort.",
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

func CoordinatorQRLogin(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			QR string `json:"qr"`
		}
		if err := decodeJSON(r, &req); err != nil || strings.TrimSpace(req.QR) == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "qr token is required"))
			return
		}
		coordinatorID, offeringID, tenantID, ok := verifyCoordinatorQRToken(req.QR)
		if !ok {
			writeJSON(w, http.StatusUnauthorized, errBody("INVALID_QR", "this coordinator QR is invalid"))
			return
		}
		userID, err := ensureCoordinatorLogin(r.Context(), adminPool, tenantID, coordinatorID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_LOGIN", err.Error()))
			return
		}
		var dbOfferingID string
		if err := adminPool.QueryRow(r.Context(),
			`SELECT offering_id::text FROM course_offerings WHERE offering_id = $1::uuid AND coordinator_id = $2 AND tenant_id = $3`,
			offeringID, coordinatorID, tenantID).Scan(&dbOfferingID); err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NOT_ASSIGNED", "you are no longer assigned to this cohort"))
			return
		}
		tokenResp, ok := mintCoordinatorToken(r.Context(), userID, tenantID, 86400)
		if !ok {
			writeJSON(w, http.StatusBadGateway, errBody("TOKEN_FAILED", "could not issue coordinator token"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(tokenResp)
	}
}

func AdminCoordinatorQR(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := chi.URLParam(r, "tenant_id")
		coordinatorID := chi.URLParam(r, "user_id")
		var name, email, coordCode, offeringID string
		err := adminPool.QueryRow(r.Context(), `
			SELECT u.full_name, COALESCE(u.email,''), COALESCE(u.coordinator_code,''),
			       COALESCE(o.offering_id::text,'')
			FROM users u
			LEFT JOIN course_offerings o ON o.coordinator_id = u.user_id::text AND o.tenant_id = u.tenant_id
			WHERE u.user_id = $1::uuid AND u.tenant_id = $2 AND u.role = 'COORDINATOR'`,
			coordinatorID, tenantID).Scan(&name, &email, &coordCode, &offeringID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "coordinator not found"))
			return
		}
		if offeringID == "" {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_OFFERING", "coordinator has no cohort assignment"))
			return
		}
		userID, err := ensureCoordinatorLogin(r.Context(), adminPool, tenantID, coordinatorID)
		if err != nil {
			writeJSON(w, http.StatusUnprocessableEntity, errBody("NO_LOGIN", err.Error()))
			return
		}
		_ = userID
		token := makeCoordinatorQRToken(coordinatorID, offeringID, tenantID)
		url := coordinatorScanURL(r, token)
		writeJSON(w, http.StatusOK, map[string]string{
			"coordinator_id": coordinatorID,
			"full_name":      name,
			"coordinator_code": coordCode,
			"offering_id":    offeringID,
			"token":          token,
			"url":            url,
		})
		if email != "" {
			emailCoordinatorQR(r.Header.Get("Authorization"), url, email, name, coordCode)
		}
	}
}
