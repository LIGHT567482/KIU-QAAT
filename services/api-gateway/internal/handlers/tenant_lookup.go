package handlers

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TenantLookup resolves the tenant_id for any user email (any role).
// Public, no auth. Uses the privileged adminPool so it can query cross-tenant.
// Only returns tenant_id — never exposes other user data.
func TenantLookup(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		email := r.URL.Query().Get("email")
		if email == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email query param required"})
			return
		}

		var tenantID string
		err := adminPool.QueryRow(r.Context(),
			`SELECT tenant_id FROM users WHERE email = $1 LIMIT 1`,
			email).Scan(&tenantID)
		if err != nil || tenantID == "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "no account found for that email"})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"tenant_id": tenantID})
	}
}
