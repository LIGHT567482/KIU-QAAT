-- QAAT — Collapse logo/emblem/badge into a single logo
-- Migration: 016_single_logo.sql
--
-- The super-admin branding UI now uses ONE image field ("Logo") that accepts a
-- file upload stored as a data URL in tenants.logo_url. The emblem_url/badge_url
-- columns added in migration 015 are unused and removed here.
--
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/migrations/016_single_logo.sql

ALTER TABLE tenants
    DROP COLUMN IF EXISTS emblem_url,
    DROP COLUMN IF EXISTS badge_url;
