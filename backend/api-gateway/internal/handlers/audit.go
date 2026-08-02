package handlers

// The administrative audit trail: who did what to whom, and when.
//
//	GET /api/v1/admin/audit?action=&actor=&from=&to=&limit=   (ADMIN)
//
// `admin_audit_log` has existed since migration 001 and, until now, NOTHING WROTE TO IT. The table
// was there, the RLS policy was there, and every sensitive admin action — releasing a patroller's
// device binding, resetting a student's device, correcting attendance by hand, deleting an account
// — happened silently. An audit trail nobody writes to is worse than none, because it looks like
// there is one.
//
// [auditAdmin] is the single write path, deliberately best-effort: an audit failure must never
// abort the action it is recording (a half-done device release with a clean log is worse than a
// done one with a missing line), but it is logged loudly enough to notice.

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// auditAdmin records one administrative action. `payloadJSON` is stored as JSONB and should be a
// small object explaining WHAT changed — never credentials, never a PIN, never a token.
//
// Best-effort by design: the caller has already performed the action.
func auditAdmin(r *http.Request, pool *pgxpool.Pool, tenantID, actorID, action, targetType, targetID, payloadJSON string) {
	role := middleware.GetRole(r.Context())
	if role == "" {
		role = "UNKNOWN"
	}
	if strings.TrimSpace(payloadJSON) == "" {
		payloadJSON = "{}"
	}
	// The client IP, as the proxy reported it. Stored as INET, so a malformed value must become
	// NULL rather than fail the insert.
	ip := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if ip == "" {
		ip, _, _ = strings.Cut(r.RemoteAddr, ":")
	}
	// The tenant GUC has to be set on THIS connection. admin_audit_log carries the same RLS policy
	// as every other tenant table, and on a connection without the setting the policy evaluates
	// `''::uuid` and the INSERT fails outright. Because this helper is best-effort, that failure
	// would be swallowed and the audit trail would stay permanently empty — the exact absence the
	// audit page exists to end. Harmless on adminPool, whose role bypasses RLS anyway.
	conn, err := pool.Acquire(r.Context())
	if err != nil {
		log.Printf("audit: no connection to record action=%s: %v", action, err)
		return
	}
	defer conn.Release()
	if err := middleware.SetTenantConn(r.Context(), conn, tenantID); err != nil {
		log.Printf("audit: could not scope connection for action=%s: %v", action, err)
		return
	}
	if _, err := conn.Exec(r.Context(), `
		INSERT INTO admin_audit_log
		  (tenant_id, actor_id, actor_role, action, target_type, target_id, payload, ip_address)
		VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''), $7::jsonb,
		        CASE WHEN $8 = '' THEN NULL ELSE $8::inet END)`,
		tenantID, actorID, role, action, targetType, targetID, payloadJSON, ip,
	); err != nil {
		// Never fatal — but never silent either.
		log.Printf("audit: failed to record action=%s target=%s/%s: %v", action, targetType, targetID, err)
	}
}

// jsonObject builds the small JSONB payload an audit entry carries, with blank values dropped so
// the stored object holds only what was actually supplied. Marshalling (rather than hand-built
// string concatenation) is what keeps a reason typed with a quote in it from producing invalid
// JSON and losing the whole audit line.
func jsonObject(fields map[string]string) string {
	clean := map[string]string{}
	for k, v := range fields {
		if v = strings.TrimSpace(v); v != "" {
			clean[k] = v
		}
	}
	b, err := json.Marshal(clean)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// ListAdminAudit — GET /api/v1/admin/audit
func ListAdminAudit(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		q := `SELECT a.audit_id::text, a.actor_id, COALESCE(u.full_name,''), a.actor_role, a.action,
		             COALESCE(a.target_type,''), COALESCE(a.target_id,''),
		             COALESCE(a.payload::text,'{}'), COALESCE(host(a.ip_address),''), a.occurred_at
		      FROM admin_audit_log a
		      LEFT JOIN users u ON u.user_id::text = a.actor_id AND u.tenant_id = a.tenant_id
		      WHERE a.tenant_id = $1`
		args := []interface{}{tenantID}

		if v := strings.TrimSpace(r.URL.Query().Get("action")); v != "" {
			args = append(args, v)
			q += " AND a.action = $" + strconv.Itoa(len(args))
		}
		// Match the actor by id OR by name, so the filter works from what is on screen.
		if v := strings.TrimSpace(r.URL.Query().Get("actor")); v != "" {
			args = append(args, "%"+strings.ToLower(v)+"%")
			n := strconv.Itoa(len(args))
			q += " AND (lower(a.actor_id) LIKE $" + n + " OR lower(COALESCE(u.full_name,'')) LIKE $" + n + ")"
		}
		if v := strings.TrimSpace(r.URL.Query().Get("from")); v != "" {
			args = append(args, v)
			q += " AND a.occurred_at >= $" + strconv.Itoa(len(args)) + "::date"
		}
		if v := strings.TrimSpace(r.URL.Query().Get("to")); v != "" {
			args = append(args, v)
			// Inclusive of the whole end day, which is what a person means by "to the 5th".
			q += " AND a.occurred_at < ($" + strconv.Itoa(len(args)) + "::date + 1)"
		}

		limit := 300
		if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 && n <= 2000 {
			limit = n
		}
		args = append(args, limit)
		q += " ORDER BY a.occurred_at DESC LIMIT $" + strconv.Itoa(len(args))

		rows, err := pool.Query(r.Context(), q, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type entry struct {
			AuditID    string `json:"audit_id"`
			ActorID    string `json:"actor_id"`
			ActorName  string `json:"actor_name"`
			ActorRole  string `json:"actor_role"`
			Action     string `json:"action"`
			TargetType string `json:"target_type"`
			TargetID   string `json:"target_id"`
			Payload    string `json:"payload"`
			IP         string `json:"ip_address"`
			OccurredAt string `json:"occurred_at"`
		}
		out := []entry{}
		for rows.Next() {
			var e entry
			var at time.Time
			if rows.Scan(&e.AuditID, &e.ActorID, &e.ActorName, &e.ActorRole, &e.Action,
				&e.TargetType, &e.TargetID, &e.Payload, &e.IP, &at) != nil {
				continue
			}
			e.OccurredAt = at.Format(time.RFC3339)
			out = append(out, e)
		}

		// The distinct actions present, so the filter offers what actually exists rather than a
		// hardcoded list that drifts as new audited actions are added.
		actions := []string{}
		aRows, _ := pool.Query(r.Context(),
			`SELECT DISTINCT action FROM admin_audit_log WHERE tenant_id = $1 ORDER BY 1`, tenantID)
		if aRows != nil {
			for aRows.Next() {
				var a string
				if aRows.Scan(&a) == nil {
					actions = append(actions, a)
				}
			}
			aRows.Close()
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{"entries": out, "actions": actions})
	}
}
