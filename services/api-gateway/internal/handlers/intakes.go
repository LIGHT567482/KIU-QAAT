package handlers

// Tenant-configurable intakes (#1).
//
//   GET /api/v1/admin/settings/intakes  (ADMIN) — the tenant's intake labels.
//   PUT /api/v1/admin/settings/intakes  (ADMIN) — replace the intake labels.
//
// Intakes (e.g. "January Intake", "May Intake", "August Intake") are defined by
// the institution admin and offered when registering students. Mirrors the
// threshold-settings handlers: tenant resolved from the JWT, scoped to the
// caller's own tenant row.

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

type intakesPayload struct {
	Intakes []string `json:"intakes"`
}

// GET /api/v1/admin/settings/intakes
func GetIntakes(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var out intakesPayload
		err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(intakes, '{}') FROM tenants WHERE tenant_id = $1`, tenantID).
			Scan(&out.Intakes)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		if out.Intakes == nil {
			out.Intakes = []string{}
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// PUT /api/v1/admin/settings/intakes
func PutIntakes(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())

		var req intakesPayload
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}

		// Normalise: trim, drop blanks, de-duplicate (case-insensitive), cap length.
		seen := map[string]bool{}
		clean := []string{}
		for _, raw := range req.Intakes {
			v := strings.TrimSpace(raw)
			if v == "" || len(v) > 64 {
				continue
			}
			key := strings.ToLower(v)
			if seen[key] {
				continue
			}
			seen[key] = true
			clean = append(clean, v)
		}
		if len(clean) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "at least one intake is required"))
			return
		}
		if len(clean) > 24 {
			clean = clean[:24]
		}

		ct, err := pool.Exec(r.Context(),
			`UPDATE tenants SET intakes = $2 WHERE tenant_id = $1`, tenantID, clean)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, intakesPayload{Intakes: clean})
	}
}
