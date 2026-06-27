package middleware

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RequireOwnTenant guards the per-tenant admin sub-resource routes
// (/api/v1/admin/tenants/{tenant_id}/...). It rejects the request unless the
// {tenant_id} path param equals the caller's JWT tenant_id — UNLESS the caller is
// the platform owner (SUPER_ADMIN), who legitimately operates across tenants.
//
// Without this, a per-tenant ADMIN could manage another tenant's users/courses/etc.
// simply by changing the path, because those handlers run on the privileged
// adminPool (no RLS). JWTAuth must run first (it sets the role + tenant in context).
func RequireOwnTenant(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetRole(r.Context()) == RoleSuperAdmin {
			next.ServeHTTP(w, r)
			return
		}
		pathTenant := chi.URLParam(r, "tenant_id")
		jwtTenant := GetTenantID(r.Context())
		if pathTenant == "" || jwtTenant == "" || pathTenant != jwtTenant {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
				"error":   "CROSS_TENANT_FORBIDDEN",
				"message": "you may only manage your own tenant",
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
