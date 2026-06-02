package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditLog writes a row to admin_audit_log for every state-mutating request
// made by DQA_DIRECTOR, QA_OFFICER, VC, and ADMIN roles.
// Read-only (GET) requests are not logged.
func AuditLog(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only audit mutating methods from privileged roles.
			role := GetRole(r.Context())
			if r.Method == http.MethodGet || r.Method == http.MethodOptions ||
				(role == RoleCoordinator) {
				next.ServeHTTP(w, r)
				return
			}

			// Snapshot request body for audit payload (max 8 KiB).
			var bodySnap []byte
			if r.Body != nil {
				limited := io.LimitReader(r.Body, 8192)
				bodySnap, _ = io.ReadAll(limited)
				r.Body = io.NopCloser(bytes.NewReader(bodySnap))
			}

			// Capture response status.
			rec := &statusRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r)

			// Skip audit on 4xx client errors (bad input, not an action).
			if rec.status >= 400 && rec.status < 500 {
				return
			}

			tenantID := GetTenantID(r.Context())
			actorID  := GetUserID(r.Context())
			action   := r.Method + " " + r.URL.Path

			var payload interface{}
			if len(bodySnap) > 0 {
				json.Unmarshal(bodySnap, &payload) //nolint:errcheck
			}

			payloadJSON, _ := json.Marshal(payload)

			// Extract IP from RemoteAddr (strip port).
			ip := r.RemoteAddr
			if i := strings.LastIndex(ip, ":"); i != -1 {
				ip = ip[:i]
			}
			if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
				ip = strings.Split(forwarded, ",")[0]
			}

			go func() {
				pool.Exec(r.Context(), `
					INSERT INTO admin_audit_log
					  (tenant_id, actor_id, actor_role, action, payload, ip_address, occurred_at)
					VALUES ($1,$2,$3,$4,$5,$6,$7)`,
					tenantID, actorID, role, action,
					payloadJSON, ip, time.Now().UTC(),
				) //nolint:errcheck
			}()
		})
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
