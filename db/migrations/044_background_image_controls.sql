-- Migration 044: background image adjustment controls.
--
-- When a tenant uses a background IMAGE it can pop too much, so the super-admin can
-- now soften it: blur it, dim/brighten it, change its contrast, and lay a tint of
-- their chosen colour over it (not hard-coded black). All optional; defaults = no
-- change. Applied across the admin, coordinator and student/lecturer surfaces.

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS background_blur            SMALLINT NOT NULL DEFAULT 0;   -- px (0-20)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS background_brightness      SMALLINT NOT NULL DEFAULT 100; -- % (30-150)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS background_contrast        SMALLINT NOT NULL DEFAULT 100; -- % (30-150)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS background_overlay_color   VARCHAR(9);                    -- hex tint, NULL = none
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS background_overlay_opacity SMALLINT NOT NULL DEFAULT 0;   -- % (0-90)
