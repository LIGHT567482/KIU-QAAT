package handlers

// Admin handlers — tenant onboarding + user management.
// All routes require Role: ADMIN.
// plan.md Week 16: tenant onboarding for full campus rollout.

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/qaat/api-gateway/internal/middleware"
)

// genCoordinatorCode returns a short, human-friendly unique coordinator code
// such as "CO-9F3A2B". Each coordinator is given one automatically (#4b).
func genCoordinatorCode() string {
	b := make([]byte, 3)
	_, _ = rand.Read(b)
	return "CO-" + strings.ToUpper(hex.EncodeToString(b))
}

// emailInDomain reports whether an email belongs to the institution: its domain
// must equal the tenant's main domain OR be a sub-domain of it (e.g. main
// "kiu.ac.ug" admits "studmc.kiu.ac.ug"). This lets the admin give students
// per-faculty sub-domains while the main domain always sits at the end.
func emailInDomain(email, mainDomain string) bool {
	at := strings.LastIndex(email, "@")
	if at <= 0 || mainDomain == "" {
		return false
	}
	d := strings.ToLower(email[at+1:])
	return d == mainDomain || strings.HasSuffix(d, "."+mainDomain)
}

// validInstitutionID accepts a short, unique-per-tenant access code (2-40 chars,
// letters/digits/'-'/'_'). The tenant ADMIN must supply it to sign in.
func validInstitutionID(s string) bool {
	if len(s) < 2 || len(s) > 40 {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// ─── Tenants ──────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants
func ListTenants(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := pool.Query(r.Context(), `
			SELECT tenant_id, name, domain, COALESCE(institution_id, ''), attendance_threshold,
			       is_active, created_at,
			       COALESCE(active_academic_year, ''),
			       COALESCE(active_semester, 0),
			       COALESCE(logo_url, ''), COALESCE(brand_color, ''),
			       COALESCE(sidebar_color, ''), COALESCE(background_color, ''),
			       COALESCE(background_image, ''),
			       COALESCE(background_blur, 0), COALESCE(background_brightness, 100),
			       COALESCE(background_contrast, 100), COALESCE(background_overlay_color, ''),
			       COALESCE(background_overlay_opacity, 0),
			       COALESCE(footer_color, ''),
			       COALESCE(motto, ''), COALESCE(slogan, ''), COALESCE(address, '')
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
			InstitutionID       string `json:"institution_id"`
			AttendanceThreshold int    `json:"attendance_threshold"`
			IsActive            bool   `json:"is_active"`
			CreatedAt           string `json:"created_at"`
			ActiveAcademicYear  string `json:"active_academic_year"`
			ActiveSemester      int    `json:"active_semester"`
			LogoURL             string `json:"logo_url"`
			BrandColor          string `json:"brand_color"`
			SidebarColor        string `json:"sidebar_color"`
			BackgroundColor     string `json:"background_color"`
			BackgroundImage     string `json:"background_image"`
			BackgroundBlur      int    `json:"background_blur"`
			BackgroundBright    int    `json:"background_brightness"`
			BackgroundContrast  int    `json:"background_contrast"`
			BackgroundOverlay   string `json:"background_overlay_color"`
			BackgroundOverlayO  int    `json:"background_overlay_opacity"`
			FooterColor         string `json:"footer_color"`
			Motto               string `json:"motto"`
			Slogan              string `json:"slogan"`
			Address             string `json:"address"`
		}
		var tenants []tenant
		for rows.Next() {
			var t tenant
			var createdAt time.Time
			rows.Scan(&t.TenantID, &t.Name, &t.Domain, &t.InstitutionID, &t.AttendanceThreshold,
				&t.IsActive, &createdAt, &t.ActiveAcademicYear, &t.ActiveSemester,
				&t.LogoURL, &t.BrandColor, &t.SidebarColor, &t.BackgroundColor,
				&t.BackgroundImage, &t.BackgroundBlur, &t.BackgroundBright, &t.BackgroundContrast,
				&t.BackgroundOverlay, &t.BackgroundOverlayO, &t.FooterColor,
				&t.Motto, &t.Slogan, &t.Address) //nolint:errcheck
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
			InstitutionID         string `json:"institution_id"`
			AttendanceThreshold   int    `json:"attendance_threshold"`
			CheckinWindowMinutes  int    `json:"checkin_window_minutes"`
			AutoKillMinutes       int    `json:"auto_kill_minutes"`
			LogoURL               string `json:"logo_url"`
			BrandColor            string `json:"brand_color"`
			SidebarColor          string `json:"sidebar_color"`
			BackgroundColor       string `json:"background_color"`
			FooterColor           string `json:"footer_color"`
			Motto                 string `json:"motto"`
			Slogan                string `json:"slogan"`
			Address               string `json:"address"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Domain = strings.ToLower(strings.TrimSpace(req.Domain))
		req.InstitutionID = strings.TrimSpace(req.InstitutionID)
		if req.Name == "" || req.Domain == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "name and domain are required"))
			return
		}
		if !validInstitutionID(req.InstitutionID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_INSTITUTION_ID", "institution_id is required (2-40 chars: letters, digits, '-' or '_')"))
			return
		}
		if !validLogo(req.LogoURL) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_LOGO", "logo must be an https URL or a base64 PNG/JPEG/WebP/GIF data URL (no SVG)"))
			return
		}
		for _, c := range []string{req.BrandColor, req.SidebarColor, req.BackgroundColor, req.FooterColor} {
			if !validColor(c) {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_COLOR", "colours must be #rgb or #rrggbb hex"))
				return
			}
		}
		if req.AttendanceThreshold < 75 { req.AttendanceThreshold = 75 } // 75% floor — never lower
		if req.AttendanceThreshold > 100 { req.AttendanceThreshold = 100 }
		if req.CheckinWindowMinutes == 0 { req.CheckinWindowMinutes = 120 }
		if req.AutoKillMinutes == 0      { req.AutoKillMinutes = 180 }

		var tenantID string
		err := pool.QueryRow(r.Context(), `
			INSERT INTO tenants
			  (name, domain, institution_id, rsa_key_id, attendance_threshold,
			   checkin_window_minutes, auto_kill_minutes,
			   logo_url, brand_color, sidebar_color, background_color, footer_color,
			   motto, slogan, address)
			VALUES ($1,$2,$3,$4,$5,$6,$7,
			        NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),NULLIF($12,''),
			        NULLIF($13,''),NULLIF($14,''),NULLIF($15,''))
			RETURNING tenant_id::text`,
			req.Name, req.Domain, req.InstitutionID,
			fmt.Sprintf("%s-rsa-key-v1", req.Domain),
			req.AttendanceThreshold, req.CheckinWindowMinutes,
			req.AutoKillMinutes,
			req.LogoURL, req.BrandColor, req.SidebarColor, req.BackgroundColor, req.FooterColor,
			req.Motto, req.Slogan, req.Address,
		).Scan(&tenantID)

		if err != nil {
			msg := "domain already registered"
			if strings.Contains(err.Error(), "institution_id") {
				msg = "institution ID already in use"
			}
			writeJSON(w, http.StatusConflict, errBody("ALREADY_EXISTS", msg))
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

// DELETE /api/v1/admin/tenants/{tenant_id} — SUPER_ADMIN permanently removes a
// tenant. All tenant-scoped rows cascade via the tenant_id FKs (ON DELETE CASCADE).
func DeleteTenant(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "")
		if !middleware.ValidTenantID(tenantID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_TENANT", "valid tenant_id required"))
			return
		}
		// Never delete the sentinel platform tenant — it owns the SUPER_ADMIN account.
		if tenantID == "00000000-0000-0000-0000-000000000000" {
			writeJSON(w, http.StatusForbidden, errBody("PROTECTED", "the platform tenant cannot be deleted"))
			return
		}
		ct, err := adminPool.Exec(r.Context(), `DELETE FROM tenants WHERE tenant_id = $1`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("DELETE_FAILED", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"tenant_id": tenantID, "status": "DELETED"})
	}
}

// ─── Users ────────────────────────────────────────────────────────────────────

// GET /api/v1/admin/tenants/{tenant_id}/users
func ListTenantUsers(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "/users")

		rows, err := pool.Query(r.Context(), `
			SELECT user_id, email, role, full_name, is_active, last_login_at, created_at,
			       COALESCE(coordinator_code, ''), COALESCE(department, ''), COALESCE(school, '')
			FROM users WHERE tenant_id = $1 ORDER BY role, email`, tenantID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		defer rows.Close()

		type user struct {
			UserID          string  `json:"user_id"`
			Email           string  `json:"email"`
			Role            string  `json:"role"`
			FullName        string  `json:"full_name"`
			IsActive        bool    `json:"is_active"`
			LastLoginAt     *string `json:"last_login_at"`
			CreatedAt       string  `json:"created_at"`
			CoordinatorCode string  `json:"coordinator_code"`
			Department      string  `json:"department"`
			School          string  `json:"school"`
		}
		var users []user
		for rows.Next() {
			var u user
			var lastLogin *time.Time
			var createdAt time.Time
			rows.Scan(&u.UserID, &u.Email, &u.Role, &u.FullName, &u.IsActive, &lastLogin, &createdAt, &u.CoordinatorCode, &u.Department, &u.School) //nolint:errcheck
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
			Email              string `json:"email"`
			Password           string `json:"password"`
			Role               string `json:"role"`
			FullName           string `json:"full_name"`
			Phone              string `json:"phone"`
			Whatsapp           string `json:"whatsapp"`
			RegistrationNumber string `json:"registration_number"`
			Title              string `json:"title"`
			Gender             string `json:"gender"`
			Department         string `json:"department"`
			School             string `json:"school"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
		req.Department = strings.TrimSpace(req.Department)
		req.School = strings.TrimSpace(req.School)
		if req.Email == "" || req.Password == "" || req.Role == "" || req.FullName == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "email, password, role, full_name required"))
			return
		}
		// QA officers act as ambassadors under a department, so it is mandatory for them
		// (the school/college stays optional). Other roles may leave both blank.
		if req.Role == "QA_OFFICER" && req.Department == "" {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "department is required for a QA officer"))
			return
		}

		validRoles := map[string]bool{
			"COORDINATOR": true, "QA_OFFICER": true,
			"DQA_DIRECTOR": true, "VC": true, "DVC": true, "ADMIN": true,
			"LECTURER": true,
		}
		if !validRoles[req.Role] {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_ROLE", "invalid role"))
			return
		}
		if len(req.Password) < 8 {
			writeJSON(w, http.StatusBadRequest, errBody("WEAK_PASSWORD", "password must be ≥ 8 characters"))
			return
		}

		// Every user's email MUST belong to the institution's domain. Look up the
		// tenant domain and require the address to end with "@<domain>".
		var domain string
		if err := pool.QueryRow(r.Context(),
			`SELECT domain FROM tenants WHERE tenant_id = $1`, tenantID).Scan(&domain); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("TENANT_NOT_FOUND", "tenant not found"))
			return
		}
		domain = strings.ToLower(strings.TrimSpace(domain))
		if !emailInDomain(req.Email, domain) {
			writeJSON(w, http.StatusBadRequest, errBody("EMAIL_DOMAIN_MISMATCH",
				"email must use the institution domain @"+domain+" (a sub-domain like @studmc."+domain+" is allowed)"))
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", "password hashing failed"))
			return
		}

		// Coordinators are issued a unique, auto-generated code that distinguishes
		// them (#4b). Retry a few times in the unlikely event the random code clashes.
		var coordCode *string
		if req.Role == "COORDINATOR" {
			c := genCoordinatorCode()
			coordCode = &c
		}

		var userID string
		var err2 error
		for attempt := 0; attempt < 6; attempt++ {
			err2 = pool.QueryRow(r.Context(), `
				INSERT INTO users (tenant_id, email, password_hash, role, full_name, is_active, coordinator_code,
				                   phone, whatsapp, registration_number, title, gender, department, school)
				VALUES ($1,$2,$3,$4,$5,true,$6, NULLIF($7,''), NULLIF($8,''), NULLIF($9,''), NULLIF($10,''), NULLIF($11,''), NULLIF($12,''), NULLIF($13,''))
				RETURNING user_id::text`,
				tenantID, req.Email, string(hash), req.Role, req.FullName, coordCode,
				req.Phone, req.Whatsapp, req.RegistrationNumber, req.Title, req.Gender, req.Department, req.School,
			).Scan(&userID)
			if err2 != nil && coordCode != nil && strings.Contains(err2.Error(), "coordinator_code") {
				c := genCoordinatorCode() // code clash — regenerate and retry
				coordCode = &c
				continue
			}
			break
		}
		if err2 != nil {
			writeJSON(w, http.StatusConflict, errBody("EMAIL_TAKEN", "email already registered in this tenant"))
			return
		}

		resp := map[string]string{
			"user_id":   userID,
			"email":     req.Email,
			"role":      req.Role,
			"tenant_id": tenantID,
			"status":    "CREATED",
		}
		if coordCode != nil {
			resp["coordinator_code"] = *coordCode
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

// PATCH /api/v1/admin/users/{user_id}/status
// PATCH /api/v1/admin/users/{user_id} — edit a staff user's profile fields
// (used by the coordinators directory). Email/role/password are not changed here.
func UpdateUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := extractPathID(r.URL.Path, "/api/v1/admin/users/", "")
		if !middleware.ValidTenantID(userID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_USER", "valid user_id required"))
			return
		}
		var req struct {
			FullName           *string `json:"full_name"`
			Phone              *string `json:"phone"`
			Whatsapp           *string `json:"whatsapp"`
			RegistrationNumber *string `json:"registration_number"`
			Title              *string `json:"title"`
			Gender             *string `json:"gender"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		sets := []string{}
		args := []interface{}{}
		n := 1
		add := func(col string, v *string, nullable bool) {
			if v == nil {
				return
			}
			if nullable {
				sets = append(sets, fmt.Sprintf("%s = NULLIF($%d,'')", col, n))
			} else {
				sets = append(sets, fmt.Sprintf("%s = $%d", col, n))
			}
			args = append(args, *v)
			n++
		}
		add("full_name", req.FullName, false)
		add("phone", req.Phone, true)
		add("whatsapp", req.Whatsapp, true)
		add("registration_number", req.RegistrationNumber, true)
		add("title", req.Title, true)
		add("gender", req.Gender, true)
		if len(sets) == 0 {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "no fields to update"))
			return
		}
		query := "UPDATE users SET " + joinComma(sets) + ", updated_at = now()" +
			fmt.Sprintf(" WHERE user_id = $%d AND (%s)", n, tenantScopeClause(fmt.Sprintf("$%d", n+1)))
		args = append(args, userID, tenantScope(r))
		ct, err := pool.Exec(r.Context(), query, args...)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "user not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"user_id": userID, "status": "UPDATED"})
	}
}

func SetUserStatus(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := extractPathID(r.URL.Path, "/api/v1/admin/users/", "/status")
		var req struct{ IsActive bool `json:"is_active"` }
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		// IDOR guard: a tenant ADMIN may only flip users in their own tenant.
		// These by-id routes run on the no-RLS adminPool, so we constrain explicitly.
		ct, err := pool.Exec(r.Context(),
			`UPDATE users SET is_active = $1, updated_at = now()
			 WHERE user_id = $2 AND (`+tenantScopeClause("$3")+`)`,
			req.IsActive, userID, tenantScope(r))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", err.Error()))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "user not found in your tenant"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"user_id": userID, "is_active": req.IsActive})
	}
}

// DELETE /api/v1/admin/users/{user_id} — permanently delete a user (own tenant for
// ADMIN; any tenant for SUPER_ADMIN). For STUDENT users the linked enrolment row
// (students_extended, joined by email) is removed too. If the user is still
// referenced (e.g. a coordinator with sessions), the FK blocks it and we tell the
// caller to deactivate instead — so the DB stays consistent.
func DeleteUser(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := extractPathID(r.URL.Path, "/api/v1/admin/users/", "")
		scope := tenantScope(r)

		var email, role string
		_ = pool.QueryRow(r.Context(),
			`SELECT email, role FROM users WHERE user_id = $1 AND (`+tenantScopeClause("$2")+`)`,
			userID, scope).Scan(&email, &role)

		ct, err := pool.Exec(r.Context(),
			`DELETE FROM users WHERE user_id = $1 AND (`+tenantScopeClause("$2")+`)`, userID, scope)
		if err != nil {
			writeJSON(w, http.StatusConflict, errBody("DELETE_BLOCKED",
				"this user has linked records (e.g. sessions or attendance); deactivate them instead"))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "user not found in your tenant"))
			return
		}
		// Remove the matching student enrolment profile, if any (linked by email).
		if role == "STUDENT" && email != "" {
			pool.Exec(r.Context(), //nolint:errcheck
				`DELETE FROM students_extended WHERE email = $1 AND (`+tenantScopeClause("$2")+`)`, email, scope)
		}
		writeJSON(w, http.StatusOK, map[string]string{"user_id": userID, "status": "DELETED"})
	}
}

// tenantScope returns the tenant a non-SUPER_ADMIN caller is confined to, or ""
// for SUPER_ADMIN (cross-tenant). Pair it with tenantScopeClause in a WHERE.
func tenantScope(r *http.Request) string {
	if middleware.GetRole(r.Context()) == middleware.RoleSuperAdmin {
		return ""
	}
	return middleware.GetTenantID(r.Context())
}

// tenantScopeClause builds a predicate that is a no-op when the bound value is ''
// (SUPER_ADMIN) and otherwise restricts to tenant_id. Prevents cross-tenant IDOR
// on by-id admin routes that run on the no-RLS adminPool.
func tenantScopeClause(param string) string {
	return param + " = '' OR tenant_id::text = " + param
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
