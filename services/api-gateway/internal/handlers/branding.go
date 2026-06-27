package handlers

// Tenant identity / branding handlers.
//
//   - GetBranding         GET  /api/v1/branding              (any authenticated role; own tenant)
//   - GetPublicBranding   GET  /api/v1/branding/public?...   (public; captive portals)
//   - UpdateTenantBranding PATCH /api/v1/admin/tenants/{id}/branding (SUPER_ADMIN)
//
// The logo + motto feed the top-left header on every dashboard; the captive-portal
// pages use the public variant (no JWT) keyed by the tenant_id in the QR token.

import (
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/qaat/api-gateway/internal/middleware"
)

// validLogo accepts only an empty value, an https URL, or a base64 data URL of a
// RASTER image. It rejects SVG (which can carry scripts), data:text/html, and
// javascript: payloads — defence-in-depth against a stored-XSS via the branding
// logo, even though the UI renders it through <img src>. Bounds the size so a
// data URL cannot bloat the row / responses.
func validLogo(s string) bool {
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "https://") {
		return len(s) <= 2048 && !strings.ContainsAny(s, "\"'<>")
	}
	for _, p := range []string{
		"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/jpg;base64,",
		"data:image/webp;base64,", "data:image/gif;base64,",
	} {
		if strings.HasPrefix(s, p) {
			return len(s) <= 1_400_000
		}
	}
	return false
}

// validColor accepts only an empty value or a #rgb / #rrggbb hex string. Rejecting
// anything else stops CSS-injection / breakout through the brand palette (the
// values are written into CSS custom properties on the client).
func validColor(s string) bool {
	if s == "" {
		return true
	}
	if (len(s) != 4 && len(s) != 7) || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// branding is the display-safe identity of a tenant. No secrets — safe to expose
// to unauthenticated captive-portal pages.
type branding struct {
	TenantID           string `json:"tenant_id"`
	Name               string `json:"name"`
	Domain             string `json:"domain"`
	LogoURL            string `json:"logo_url"`
	Motto              string `json:"motto"`
	Slogan             string `json:"slogan"`
	BrandColor         string `json:"brand_color"`
	SidebarColor       string `json:"sidebar_color"`
	BackgroundColor    string `json:"background_color"`
	FooterColor        string `json:"footer_color"`
	Address            string `json:"address"`
	ActiveAcademicYear string `json:"active_academic_year"`
	ActiveSemester     int    `json:"active_semester"`
}

// brandingSelect returns display-safe identity fields ONLY — never institution_id
// (the tenant ADMIN's login secret).
const brandingSelect = `
	SELECT tenant_id::text, name, COALESCE(domain, ''),
	       COALESCE(logo_url, ''), COALESCE(motto, ''), COALESCE(slogan, ''),
	       COALESCE(brand_color, ''), COALESCE(sidebar_color, ''),
	       COALESCE(background_color, ''), COALESCE(footer_color, ''),
	       COALESCE(address, ''), COALESCE(active_academic_year, ''),
	       COALESCE(active_semester, 0)
	FROM tenants WHERE tenant_id = $1`

// GET /api/v1/branding — branding for the caller's own tenant (from JWT).
// Runs on the tenant-scoped pool, so RLS guarantees a tenant can only read itself.
func GetBranding(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := middleware.GetTenantID(r.Context())
		if !middleware.ValidTenantID(tenantID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_TENANT", "no tenant in context"))
			return
		}
		var b branding
		err := pool.QueryRow(r.Context(), brandingSelect, tenantID).Scan(
			&b.TenantID, &b.Name, &b.Domain, &b.LogoURL,
			&b.Motto, &b.Slogan, &b.BrandColor,
			&b.SidebarColor, &b.BackgroundColor, &b.FooterColor, &b.Address,
			&b.ActiveAcademicYear, &b.ActiveSemester)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, b)
	}
}

// GET /api/v1/branding/public?tenant_id=... — unauthenticated branding lookup for
// the student/lecturer captive portals. Uses the privileged pool (no RLS) because
// the caller has no JWT/tenant context; only display-safe fields are returned.
func GetPublicBranding(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant_id")
		// The student portal link carries the tenant by its domain (or institution
		// id) rather than the raw UUID — resolve it so the portal can show the logo
		// + name and scope itself to that institution.
		if org := strings.TrimSpace(r.URL.Query().Get("org")); org != "" && tenantID == "" {
			_ = adminPool.QueryRow(r.Context(),
				`SELECT tenant_id::text FROM tenants WHERE lower(domain) = lower($1) OR lower(institution_id) = lower($1) LIMIT 1`,
				org).Scan(&tenantID)
		}
		if !middleware.ValidTenantID(tenantID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_TENANT", "valid tenant_id or org query param required"))
			return
		}
		var b branding
		err := adminPool.QueryRow(r.Context(), brandingSelect, tenantID).Scan(
			&b.TenantID, &b.Name, &b.Domain, &b.LogoURL,
			&b.Motto, &b.Slogan, &b.BrandColor,
			&b.SidebarColor, &b.BackgroundColor, &b.FooterColor, &b.Address,
			&b.ActiveAcademicYear, &b.ActiveSemester)
		if err != nil {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, b)
	}
}

// PATCH /api/v1/admin/tenants/{tenant_id}/branding — SUPER_ADMIN updates a tenant's
// identity/branding. Runs on the privileged adminPool (cross-tenant platform op).
func UpdateTenantBranding(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := extractPathID(r.URL.Path, "/api/v1/admin/tenants/", "/branding")
		if !middleware.ValidTenantID(tenantID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_TENANT", "valid tenant_id required"))
			return
		}
		var req struct {
			InstitutionID   string `json:"institution_id"`
			LogoURL         string `json:"logo_url"`
			BrandColor      string `json:"brand_color"`
			SidebarColor    string `json:"sidebar_color"`
			BackgroundColor string `json:"background_color"`
			FooterColor     string `json:"footer_color"`
			Motto           string `json:"motto"`
			Slogan          string `json:"slogan"`
			Address         string `json:"address"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_REQUEST", "malformed body"))
			return
		}
		req.InstitutionID = strings.TrimSpace(req.InstitutionID)
		if req.InstitutionID != "" && !validInstitutionID(req.InstitutionID) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_INSTITUTION_ID", "institution_id must be 2-40 chars: letters, digits, '-' or '_'"))
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
		// institution_id: only overwrite when a non-empty value is supplied (COALESCE
		// keeps the existing one otherwise), so a branding-only edit won't clear it.
		ct, err := adminPool.Exec(r.Context(), `
			UPDATE tenants SET
			  institution_id   = COALESCE(NULLIF($2,''), institution_id),
			  logo_url         = NULLIF($3,''),
			  brand_color      = NULLIF($4,''),
			  sidebar_color    = NULLIF($5,''),
			  background_color = NULLIF($6,''),
			  footer_color     = NULLIF($7,''),
			  motto            = NULLIF($8,''),
			  slogan           = NULLIF($9,''),
			  address          = NULLIF($10,'')
			WHERE tenant_id = $1`,
			tenantID, req.InstitutionID, req.LogoURL, req.BrandColor, req.SidebarColor,
			req.BackgroundColor, req.FooterColor,
			req.Motto, req.Slogan, req.Address)
		if err != nil {
			msg := err.Error()
			if strings.Contains(msg, "institution_id") {
				writeJSON(w, http.StatusConflict, errBody("ALREADY_EXISTS", "institution ID already in use"))
				return
			}
			writeJSON(w, http.StatusInternalServerError, errBody("INTERNAL_ERROR", msg))
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"tenant_id": tenantID, "status": "UPDATED"})
	}
}
