-- QAAT — Multi-colour brand palette
-- Migration: 017_brand_palette.sql
--
-- The super-admin can colour different regions of the system independently
-- (e.g. green sidebar, white background, black footer) rather than a single
-- brand colour applied everywhere. brand_color (from 001) remains the accent
-- used for buttons/links/highlights.
--
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/migrations/017_brand_palette.sql

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS sidebar_color    VARCHAR(7),
    ADD COLUMN IF NOT EXISTS background_color VARCHAR(7),
    ADD COLUMN IF NOT EXISTS footer_color     VARCHAR(7);
