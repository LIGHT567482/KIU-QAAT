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
func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

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
	BackgroundImage    string `json:"background_image"`
	BackgroundBlur     int    `json:"background_blur"`
	BackgroundBright   int    `json:"background_brightness"`
	BackgroundContrast int    `json:"background_contrast"`
	BackgroundOverlay  string `json:"background_overlay_color"`
	BackgroundOverlayO int    `json:"background_overlay_opacity"`
	FooterColor        string `json:"footer_color"`
	TextColorLight     string `json:"text_color_light"`
	TextColorDark      string `json:"text_color_dark"`
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
	       COALESCE(background_color, ''), COALESCE(background_image, ''),
	       COALESCE(background_blur, 0), COALESCE(background_brightness, 100),
	       COALESCE(background_contrast, 100), COALESCE(background_overlay_color, ''),
	       COALESCE(background_overlay_opacity, 0), COALESCE(footer_color, ''),
	       COALESCE(text_color_light, ''), COALESCE(text_color_dark, ''),
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
			&b.SidebarColor, &b.BackgroundColor, &b.BackgroundImage,
			&b.BackgroundBlur, &b.BackgroundBright, &b.BackgroundContrast,
			&b.BackgroundOverlay, &b.BackgroundOverlayO, &b.FooterColor, &b.TextColorLight, &b.TextColorDark, &b.Address,
			&b.ActiveAcademicYear, &b.ActiveSemester)
		if err != nil {
			// No tenant row (or DB hiccup): still serve the institution's brand.json identity.
			if bf := brandFile(); bf != nil {
				writeJSON(w, http.StatusOK, *bf)
				return
			}
			writeJSON(w, http.StatusNotFound, errBody("NOT_FOUND", "tenant not found"))
			return
		}
		applyBrandFile(&b) // brand.json is the source of visual identity
		writeJSON(w, http.StatusOK, b)
	}
}

// GET /api/v1/branding/public — unauthenticated branding for the login pages / captive portals.
// Single-institution: it just returns the embedded brand.json (no tenant_id / org needed).
func GetPublicBranding(adminPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bf := brandFile(); bf != nil {
			writeJSON(w, http.StatusOK, *bf)
			return
		}
		writeJSON(w, http.StatusInternalServerError, errBody("NO_BRANDING", "branding unavailable"))
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
			BackgroundImage string `json:"background_image"`
			BackgroundBlur     int    `json:"background_blur"`
			BackgroundBright   int    `json:"background_brightness"`
			BackgroundContrast int    `json:"background_contrast"`
			BackgroundOverlay  string `json:"background_overlay_color"`
			BackgroundOverlayO int    `json:"background_overlay_opacity"`
			FooterColor     string `json:"footer_color"`
			TextColorLight  string `json:"text_color_light"`
			TextColorDark   string `json:"text_color_dark"`
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
		// Background image is validated the same way as the logo (https URL or a
		// base64 image data URL) — also prevents CSS-injection when applied as url().
		if !validLogo(req.BackgroundImage) {
			writeJSON(w, http.StatusBadRequest, errBody("INVALID_BACKGROUND", "background image must be an https URL or a base64 PNG/JPEG/WebP/GIF data URL (no SVG)"))
			return
		}
		for _, c := range []string{req.BrandColor, req.SidebarColor, req.BackgroundColor, req.FooterColor, req.BackgroundOverlay, req.TextColorLight, req.TextColorDark} {
			if !validColor(c) {
				writeJSON(w, http.StatusBadRequest, errBody("INVALID_COLOR", "colours must be #rgb or #rrggbb hex"))
				return
			}
		}
		// Clamp the image adjustment controls to safe ranges (sliders).
		req.BackgroundBlur = clampInt(req.BackgroundBlur, 0, 20)
		req.BackgroundBright = clampInt(req.BackgroundBright, 30, 150)
		req.BackgroundContrast = clampInt(req.BackgroundContrast, 30, 150)
		req.BackgroundOverlayO = clampInt(req.BackgroundOverlayO, 0, 90)
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
			  address          = NULLIF($10,''),
			  background_image = NULLIF($11,''),
			  background_blur            = $12,
			  background_brightness      = $13,
			  background_contrast        = $14,
			  background_overlay_color   = NULLIF($15,''),
			  background_overlay_opacity = $16,
			  text_color_light           = NULLIF($17,''),
			  text_color_dark            = NULLIF($18,'')
			WHERE tenant_id = $1`,
			tenantID, req.InstitutionID, req.LogoURL, req.BrandColor, req.SidebarColor,
			req.BackgroundColor, req.FooterColor,
			req.Motto, req.Slogan, req.Address, req.BackgroundImage,
			req.BackgroundBlur, req.BackgroundBright, req.BackgroundContrast,
			req.BackgroundOverlay, req.BackgroundOverlayO, req.TextColorLight, req.TextColorDark)
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
