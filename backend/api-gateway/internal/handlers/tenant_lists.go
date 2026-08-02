package handlers

// Tenant-configurable label lists: study levels and study sessions (next.txt
// "biggest thing"). Same shape as the intakes list (handlers/intakes.go): the
// admin defines the labels and they drive program/offering creation.

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// getTenantList reads a TEXT[] column from the caller's own tenant. `column` and
// `jsonKey` are compile-time constants from the router (never user input).
func getTenantList(pool *pgxpool.Pool, column, jsonKey string) http.HandlerFunc {
	query := "SELECT COALESCE(" + column + ", '{}') FROM tenants WHERE tenant_id = $1"
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var list []string
		if err := pool.QueryRow(r.Context(), query, tenantID).Scan(&list); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		if list == nil {
			list = []string{}
		}
		writeJSON(w, http.StatusOK, map[string][]string{jsonKey: list})
	}
}

func putTenantList(pool *pgxpool.Pool, column, jsonKey string) http.HandlerFunc {
	update := "UPDATE tenants SET " + column + " = $2 WHERE tenant_id = $1"
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req map[string][]string
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		clean := normaliseLabels(req[jsonKey])
		if len(clean) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "at least one value is required"))
			return
		}
		ct, err := pool.Exec(r.Context(), update, tenantID, clean)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string][]string{jsonKey: clean})
	}
}

// normaliseLabels trims, drops blanks/over-long, de-duplicates case-insensitively,
// and caps the list — same rules as the intakes list.
func normaliseLabels(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, raw := range in {
		v := strings.TrimSpace(raw)
		if v == "" || len(v) > 64 {
			continue
		}
		k := strings.ToLower(v)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, v)
	}
	if len(out) > 24 {
		out = out[:24]
	}
	return out
}

// GetStaffIDPrefix / PutStaffIDPrefix manage the short institution code used to
// auto-generate lecturer staff IDs (<prefix>/STAFF/00001).
func GetStaffIDPrefix(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var v string
		if err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(staff_id_prefix,'') FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&v); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"staff_id_prefix": v})
	}
}

func PutStaffIDPrefix(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req struct {
			StaffIDPrefix string `json:"staff_id_prefix"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		// Keep it short + safe for an ID prefix (letters/digits/-).
		p := strings.ToUpper(strings.TrimSpace(req.StaffIDPrefix))
		if len(p) > 16 {
			p = p[:16]
		}
		if _, err := pool.Exec(r.Context(),
			`UPDATE tenants SET staff_id_prefix = NULLIF($2,'') WHERE tenant_id = $1`, tenantID, p); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"staff_id_prefix": p})
	}
}

// Router-facing wrappers.
func GetTitles(pool *pgxpool.Pool) http.HandlerFunc { return getTenantList(pool, "titles", "titles") }
func PutTitles(pool *pgxpool.Pool) http.HandlerFunc { return putTenantList(pool, "titles", "titles") }

// GetLevels returns the level names plus their per-level years of study.
func GetLevels(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var levels []string
		var years map[string]int
		if err := pool.QueryRow(r.Context(),
			`SELECT COALESCE(levels, '{}'), COALESCE(level_years, '{}') FROM tenants WHERE tenant_id = $1`,
			tenantID).Scan(&levels, &years); err != nil {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		if levels == nil {
			levels = []string{}
		}
		if years == nil {
			years = map[string]int{}
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"levels": levels, "level_years": years})
	}
}

// PutLevels stores the level names + their years of study (1–10).
func PutLevels(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req struct {
			Levels     []string       `json:"levels"`
			LevelYears map[string]int `json:"level_years"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		clean := normaliseLabels(req.Levels)
		if len(clean) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "at least one level is required"))
			return
		}
		// Keep only years for levels that exist; clamp 1–10; default 3.
		years := map[string]int{}
		for _, name := range clean {
			y := req.LevelYears[name]
			if y < 1 || y > 10 {
				y = 3
			}
			years[name] = y
		}
		if _, err := pool.Exec(r.Context(),
			`UPDATE tenants SET levels = $2, level_years = $3 WHERE tenant_id = $1`,
			tenantID, clean, years); err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"levels": clean, "level_years": years})
	}
}
func GetStudySessions(pool *pgxpool.Pool) http.HandlerFunc {
	return getTenantList(pool, "study_sessions", "study_sessions")
}
func PutStudySessions(pool *pgxpool.Pool) http.HandlerFunc {
	return putTenantList(pool, "study_sessions", "study_sessions")
}
