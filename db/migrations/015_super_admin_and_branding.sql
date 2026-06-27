-- QAAT — Super-Admin role + Tenant identity/branding
-- Migration: 015_super_admin_and_branding.sql
--
-- Adds the platform-owner role (SUPER_ADMIN) and the tenant identity/branding
-- columns the super-admin dashboard configures (logo_url + brand_color already
-- exist from 001). The new enum value must be committed BEFORE it is used, so the
-- platform tenant + super-admin user are seeded in a separate file that runs after
-- this migration commits: db/seeds/004_super_admin.sql.
--
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/migrations/015_super_admin_and_branding.sql

-- ── Platform-owner role ───────────────────────────────────────────────────────
-- SUPER_ADMIN owns the SaaS platform: registers tenants and configures their
-- branding. It is distinct from the per-tenant ADMIN role.
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'SUPER_ADMIN';

-- ── Tenant identity / branding ────────────────────────────────────────────────
-- logo_url + brand_color already exist (migration 001). These extend the
-- institution identity surfaced on every dashboard header and captive portal.
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS emblem_url TEXT,
    ADD COLUMN IF NOT EXISTS badge_url  TEXT,
    ADD COLUMN IF NOT EXISTS motto      TEXT,
    ADD COLUMN IF NOT EXISTS slogan     TEXT,
    ADD COLUMN IF NOT EXISTS address    TEXT;
