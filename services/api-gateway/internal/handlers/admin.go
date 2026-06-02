package handlers

// Admin handlers — tenant onboarding + user management.
// All routes require Role: ADMIN.
// plan.md Week 16: tenant onboarding for full campus rollout.

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// ─── Tenants ──────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants
func ListTenants(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(), `
			SELECT tenant_id, name, domain, attendance_threshold,
			       is_active, created_at
			FROM tenants ORDER BY created_at DESC`)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type tenant struct {
			TenantID            string `json:"tenant_id"`
			Name                string `json:"name"`
			Domain              string `json:"domain"`
			AttendanceThreshold int    `json:"attendance_threshold"`
			IsActive            bool   `json:"is_active"`
			CreatedAt           string `json:"created_at"`
		}
		var tenants []tenant
		for rows.Next() {
			var t tenant
			var createdAt time.Time
			rows.Scan(&t.TenantID, &t.Name, &t.Domain, &t.AttendanceThreshold,
				&t.IsActive, &createdAt) //nolint:errcheck
			t.CreatedAt = createdAt.Format(time.RFC3339)
			tenants = append(tenants, t)
		}
		if tenants == nil {
			tenants = []tenant{}
		}
		writeJSON(w, http.StatusOK, tenants)
	}
}

// POST /api/v1/admin/tenants
func CreateTenant(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name                  string `json:"name"`
			Domain                string `json:"domain"`
			AttendanceThreshold   int    `json:"attendance_threshold"`
			CheckinWindowMinutes  int    `json:"checkin_window_minutes"`
			AutoKillMinutes       int    `json:"auto_kill_minutes"`
			RSSIThresholdDBM      int    `json:"rssi_threshold_dbm"`
			LogoURL               string `json:"logo_url"`
			BrandColor            string `json:"brand_color"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		if req.Name == "" || req.Domain == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "name and domain are required"))
			return
		}
		if req.AttendanceThreshold == 0  { req.AttendanceThreshold = 75 }
		if req.CheckinWindowMinutes == 0 { req.CheckinWindowMinutes = 120 }
		if req.AutoKillMinutes == 0      { req.AutoKillMinutes = 180 }
		if req.RSSIThresholdDBM == 0     { req.RSSIThresholdDBM = -65 }

		var tenantID string
		err := pool.QueryRow(r.Context(), `
			INSERT INTO tenants
			  (name, domain, rsa_key_id, attendance_threshold,
			   checkin_window_minutes, auto_kill_minutes, rssi_threshold_dbm,
			   logo_url, brand_color)
			VALUES ($1,$2,$3,$4,$5,$6,$7,NULLIF($8,''),NULLIF($9,''))
			RETURNING tenant_id::text`,
			req.Name, req.Domain,
			fmt.Sprintf("%s-rsa-key-v1", req.Domain),
			req.AttendanceThreshold, req.CheckinWindowMinutes,
			req.AutoKillMinutes, req.RSSIThresholdDBM,
			req.LogoURL, req.BrandColor,
		).Scan(&tenantID)

		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("DOMAIN_TAKEN", "domain already registered"))
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"tenant_id":  tenantID,
			"name":       req.Name,
			"domain":     req.Domain,
			"status":     "CREATED",
		})
	}
}

// PATCH /api/v1/admin/tenants/{tenant_id}/status — activate or suspend a tenant
func SetTenantStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "/status")
		var req struct{ IsActive bool `json:"is_active"` }
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		pool.Exec(r.Context(), `UPDATE tenants SET is_active = $1 WHERE tenant_id = $2`, req.IsActive, tenantID) //nolint:errcheck
		writeJSON(w, http.StatusOK, map[string]interface{}{"tenant_id": tenantID, "is_active": req.IsActive})
	}
}

// ─── Users ────────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/users
func ListTenantUsers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "/users")

		rows, err := pool.Query(r.Context(), `
			SELECT user_id, email, role, full_name, is_active, last_login_at, created_at
			FROM users WHERE tenant_id = $1 ORDER BY role, email`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type user struct {
			UserID      string  `json:"user_id"`
			Email       string  `json:"email"`
			Role        string  `json:"role"`
			FullName    string  `json:"full_name"`
			IsActive    bool    `json:"is_active"`
			LastLoginAt *string `json:"last_login_at"`
			CreatedAt   string  `json:"created_at"`
		}
		var users []user
		for rows.Next() {
			var u user
			var lastLogin *time.Time
			var createdAt time.Time
			rows.Scan(&u.UserID, &u.Email, &u.Role, &u.FullName, &u.IsActive, &lastLogin, &createdAt) //nolint:errcheck
			u.CreatedAt = createdAt.Format(time.RFC3339)
			if lastLogin != nil {
				s := lastLogin.Format(time.RFC3339)
				u.LastLoginAt = &s
			}
			users = append(users, u)
		}
		if users == nil {
			users = []user{}
		}
		writeJSON(w, http.StatusOK, users)
	}
}

// POST /api/v1/admin/tenants/{tenant_id}/users
func CreateUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "/users")

		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Role     string `json:"role"`
			FullName string `json:"full_name"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		if req.Email == "" || req.Password == "" || req.Role == "" || req.FullName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "email, password, role, full_name required"))
			return
		}

		validRoles := map[string]bool{
			"COORDINATOR": true, "QA_OFFICER": true,
			"DQA_DIRECTOR": true, "VC": true, "ADMIN": true,
		}
		if !validRoles[req.Role] {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_ROLE", "invalid role"))
			return
		}
		if len(req.Password) < 8 {
			writeJSON(w, http.StatusBadRequest, errBody("WEAK_PASSWORD", "password must be ≥ 8 characters"))
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "password hashing failed"))
			return
		}

		var userID string
		err = pool.QueryRow(r.Context(), `
			INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active)
			VALUES ($1,$2,$3,$4,$5,true)
			RETURNING user_id::text`,
			tenantID, req.Email, string(hash), req.Role, req.FullName,
		).Scan(&userID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("EMAIL_TAKEN", "email already registered in this tenant"))
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"user_id":   userID,
			"email":     req.Email,
			"role":      req.Role,
			"tenant_id": tenantID,
			"status":    "CREATED",
		})
	}
}

// PATCH /api/v1/admin/users/{user_id}/status
func SetUserStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := extractPathID(r.URL.Path, "/api/v1/admin/users/", "/status")
		var req struct{ IsActive bool `json:"is_active"` }
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		pool.Exec(r.Context(), `UPDATE users SET is_active = $1, updated_at = now() WHERE user_id = $2`, req.IsActive, userID) //nolint:errcheck
		writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": userID, "is_active": req.IsActive})
	}
}

// ─── Beacon registration ──────────────────────────────────────────────────────

// POST /api/v1/admin/tenants/{tenant_id}/beacons
func RegisterBeacon(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "/beacons")

		var req struct {
			BeaconUUID string `json:"beacon_uuid"`
			VenueID    string `json:"venue_id"`
			Format     string `json:"format"`
			Major      int    `json:"major"`
			Minor      int    `json:"minor"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		if req.BeaconUUID == "" || req.VenueID == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "beacon_uuid and venue_id required"))
			return
		}
		if req.Format == "" { req.Format = "IBEACON" }

		_, err := pool.Exec(r.Context(), `
			INSERT INTO ble_beacons (beacon_uuid, tenant_id, venue_id, format, major, minor)
			VALUES ($1::uuid, $2, $3, $4, $5, $6)
			ON CONFLICT (beacon_uuid) DO UPDATE
			  SET venue_id = EXCLUDED.venue_id,
			      format   = EXCLUDED.format,
			      major    = EXCLUDED.major,
			      minor    = EXCLUDED.minor`,
			req.BeaconUUID, tenantID, req.VenueID, req.Format, req.Major, req.Minor)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{
			"beacon_uuid": req.BeaconUUID,
			"venue_id":    req.VenueID,
			"status":      "REGISTERED",
		})
	}
}

func extractPathID(path, prefix, suffix string) string {
	s := path
	if len(s) > len(prefix) {
		s = s[len(prefix):]
	}
	if i := len(s) - len(suffix); i > 0 && suffix != "" {
		if s[i:] == suffix {
			s = s[:i]
		}
	}
	return s
}
