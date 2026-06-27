package handlers

// Tenant-configurable daily session window + the Users-page passcode.
//
//   GET/PUT /api/v1/admin/settings/session-window  (ADMIN) — the daily window
//        (start/end time + active weekdays) during which coordinators may begin
//        a session. Outside it, the coordinator sees nothing to start.
//   GET     /api/v1/admin/settings/users-passcode   (ADMIN) — whether a passcode
//        is set (never returns the hash).
//   PUT     /api/v1/admin/settings/users-passcode   (ADMIN) — set/replace it.
//   POST    /api/v1/admin/settings/users-passcode/verify (ADMIN) — check a passcode.
//
// The window is evaluated in the server's local timezone (the gateway container
// is pinned to Africa/Kampala via TZ), so "08:00–17:00" means local clock time.

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

type sessionWindowPayload struct {
	Start      string `json:"start"`       // "HH:MM"
	End        string `json:"end"`         // "HH:MM"
	ActiveDays []int  `json:"active_days"` // ISO 1=Mon..7=Sun
}

// loadSessionWindow reads a tenant's window. start/end are "HH:MM".
func loadSessionWindow(ctx context.Context, pool *pgxpool.Pool, tenantID string) (sessionWindowPayload, error) {
	var startT, endT time.Time
	var days []int16
	err := pool.QueryRow(ctx,
		`SELECT session_window_start, session_window_end, session_active_days
		   FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&startT, &endT, &days)
	out := sessionWindowPayload{Start: "08:00", End: "17:00", ActiveDays: []int{1, 2, 3, 4, 5, 6}}
	if err != nil {
		return out, err
	}
	out.Start = startT.Format("15:04")
	out.End = endT.Format("15:04")
	out.ActiveDays = nil
	for _, d := range days {
		out.ActiveDays = append(out.ActiveDays, int(d))
	}
	return out, nil
}

// withinSessionWindow reports whether 'now' (local) falls inside the tenant's
// daily window and on an active weekday. The string reason is set when closed.
func withinSessionWindow(ctx context.Context, pool *pgxpool.Pool, tenantID string) (bool, string) {
	w, err := loadSessionWindow(ctx, pool, tenantID)
	if err != nil {
		return true, "" // fail open — never block on a read error
	}
	now := time.Now()
	iso := int(now.Weekday())
	if iso == 0 {
		iso = 7 // Sunday → 7
	}
	dayOK := false
	for _, d := range w.ActiveDays {
		if d == iso {
			dayOK = true
			break
		}
	}
	if !dayOK {
		return false, "Sessions can only be started on the institution's active lecture days."
	}
	cur := now.Format("15:04")
	if cur < w.Start || cur >= w.End {
		return false, "Sessions can only be started between " + w.Start + " and " + w.End + "."
	}
	return true, ""
}

// GET /api/v1/admin/settings/session-window
func GetSessionWindow(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		out, err := loadSessionWindow(r.Context(), pool, tenantID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// PUT /api/v1/admin/settings/session-window
func PutSessionWindow(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req sessionWindowPayload
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		// Validate HH:MM.
		ts, errS := time.Parse("15:04", req.Start)
		te, errE := time.Parse("15:04", req.End)
		if errS != nil || errE != nil || !ts.Before(te) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "start/end must be HH:MM with start before end"))
			return
		}
		// Normalise days to 1..7, unique, sorted-ish.
		seen := map[int]bool{}
		var days []int16
		for _, d := range req.ActiveDays {
			if d >= 1 && d <= 7 && !seen[d] {
				seen[d] = true
				days = append(days, int16(d))
			}
		}
		if len(days) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "at least one active day is required"))
			return
		}
		ct, err := pool.Exec(r.Context(),
			`UPDATE tenants SET session_window_start = $2, session_window_end = $3, session_active_days = $4
			   WHERE tenant_id = $1`, tenantID, req.Start, req.End, days)
		if err != nil || ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not save window"))
			return
		}
		out, _ := loadSessionWindow(r.Context(), pool, tenantID)
		writeJSON(w, http.StatusOK, out)
	}
}

// GET /api/v1/admin/settings/users-passcode → {"is_set": bool}
func GetUsersPasscodeStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var hash *string
		_ = pool.QueryRow(r.Context(),
			`SELECT users_passcode_hash FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&hash)
		writeJSON(w, http.StatusOK, map[string]bool{"is_set": hash != nil && *hash != ""})
	}
}

type passcodePayload struct {
	Passcode string `json:"passcode"`
}

// PUT /api/v1/admin/settings/users-passcode — set/replace the Users-page passcode.
func PutUsersPasscode(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req passcodePayload
		if err := decodeJSON(r, &req); err != nil || len(req.Passcode) < 4 || len(req.Passcode) > 128 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "passcode must be 4–128 characters"))
			return
		}
		hash, herr := bcrypt.GenerateFromPassword([]byte(req.Passcode), 12)
		if herr != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "hash failed"))
			return
		}
		ct, err := pool.Exec(r.Context(),
			`UPDATE tenants SET users_passcode_hash = $2 WHERE tenant_id = $1`, tenantID, string(hash))
		if err != nil || ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "could not save passcode"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"is_set": true})
	}
}

// POST /api/v1/admin/settings/users-passcode/verify → {"ok": bool}
func VerifyUsersPasscode(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		var req passcodePayload
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		var hash *string
		_ = pool.QueryRow(r.Context(),
			`SELECT users_passcode_hash FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&hash)
		// If no passcode set, allow through (nothing to gate against yet).
		if hash == nil || *hash == "" {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true, "is_set": false})
			return
		}
		ok := bcrypt.CompareHashAndPassword([]byte(*hash), []byte(req.Passcode)) == nil
		writeJSON(w, http.StatusOK, map[string]bool{"ok": ok, "is_set": true})
	}
}
