-- 049: Per-theme text colours. Some tenants' brand/background colours leave the default
-- text unreadable in one theme, so the super-admin can now set an explicit text colour for
-- LIGHT and for DARK mode independently. Empty = use the theme default.
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS text_color_light VARCHAR(7),
    ADD COLUMN IF NOT EXISTS text_color_dark  VARCHAR(7);
