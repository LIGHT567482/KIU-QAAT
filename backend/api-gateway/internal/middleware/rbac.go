package middleware

import (
	"encoding/json"
	"net/http"
)

// RequireRole returns a middleware that allows only the listed roles.
// JWTAuth must run first (it sets CtxRole in the context).
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role := GetRole(r.Context())
			if _, ok := allowed[role]; !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
					"error":   "INSUFFICIENT_ROLE",
					"message": "your role does not have access to this resource",
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Role constants — single source of truth used by route definitions.
const (
	RoleCoordinator = "COORDINATOR"
	RoleQAOfficer   = "QA_OFFICER"
	RoleDQADirector = "DQA_DIRECTOR"
	RoleVC          = "VC"
	RoleDVC         = "DVC" // Deputy Vice-Chancellor (oversight tier, mirrors VC)
	RoleAdmin       = "ADMIN"
	RoleSuperAdmin  = "SUPER_ADMIN" // platform owner (registers tenants, configures branding)
	RoleStudent     = "STUDENT"
	RoleLecturer    = "LECTURER" // logs into the lecturer dashboard (read-only attendance)
)
