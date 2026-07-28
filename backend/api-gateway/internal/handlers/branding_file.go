package handlers

import (
	_ "embed"
	"encoding/json"
	"sync"
)

// Single-institution branding: the whole system's identity comes from brand.json (repo root,
// synced here by scripts/sync-brand.sh). It's embedded at build time so the gateway serves it with
// no runtime file/DB dependency; the DB `tenants` branding columns are no longer the source.
//
//go:embed brand.json
var brandJSONBytes []byte

var (
	brandOnce sync.Once
	brandData *branding
)

// brandFile returns the embedded brand.json (parsed once), or nil if it failed to parse.
func brandFile() *branding {
	brandOnce.Do(func() {
		var b branding
		if json.Unmarshal(brandJSONBytes, &b) == nil {
			brandData = &b
		}
	})
	return brandData
}

// applyBrandFile overlays the file's VISUAL branding onto b (leaving operational fields like
// active_academic_year / active_semester, which still come from the tenant row).
func applyBrandFile(b *branding) {
	bf := brandFile()
	if bf == nil {
		return
	}
	b.Name = bf.Name
	b.Motto = bf.Motto
	b.Slogan = bf.Slogan
	b.LogoURL = bf.LogoURL
	b.BrandColor = bf.BrandColor
	b.SidebarColor = bf.SidebarColor
	b.BackgroundColor = bf.BackgroundColor
	b.FooterColor = bf.FooterColor
	b.TextColorLight = bf.TextColorLight
	b.TextColorDark = bf.TextColorDark
	b.BackgroundImage = bf.BackgroundImage
	b.BackgroundBlur = bf.BackgroundBlur
	b.BackgroundBright = bf.BackgroundBright
	b.BackgroundContrast = bf.BackgroundContrast
	b.BackgroundOverlay = bf.BackgroundOverlay
	b.BackgroundOverlayO = bf.BackgroundOverlayO
	b.Address = bf.Address
}
