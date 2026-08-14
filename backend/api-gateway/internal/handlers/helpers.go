package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/qaat/api-gateway/internal/middleware"
)

// tenantOf is THE way a handler learns which institution it is serving.
//
// QAAT runs one institution. The admin routes nonetheless carried the institution in the URL —
// /api/v1/admin/tenants/{tenant_id}/lecturers — and every handler read it back out of the path,
// which meant a caller could name an institution and the server had to check they were allowed to.
// RequireOwnTenant existed for exactly that: to compare the id in the URL against the id in the
// token and refuse when they differed.
//
// Reading it from the TOKEN removes the question rather than answering it. There is no longer an
// institution id in the request for anyone to change, so there is nothing to validate and nothing
// to get wrong — the class of bug where one route forgets its own-tenant guard cannot occur.
//
// The tenant_id COLUMNS and the row-level security policies stay exactly as they are. They are
// cheap, they are the reason a query that forgets its WHERE clause returns nothing rather than
// everything, and none of that is visible to a user.
func tenantOf(r *http.Request) string { return middleware.GetTenantID(r.Context()) }

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(body) //nolint:errcheck
}

func decodeJSON(r *http.Request, dst interface{}) error {
	return json.NewDecoder(r.Body).Decode(dst)
}
