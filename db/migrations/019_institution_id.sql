-- QAAT — Per-institution access ID
-- Migration: 019_institution_id.sql
--
-- Every tenant (university) gets a unique Institution ID, assigned by the
-- super-admin when the tenant is registered. The tenant ADMIN must supply this
-- ID (in addition to email + password) to sign in; other roles do not.
-- Nullable so pre-existing tenants keep working until the super-admin assigns one.
--
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/migrations/019_institution_id.sql

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS institution_id VARCHAR(40);

-- Unique when present (partial unique index ignores NULLs).
CREATE UNIQUE INDEX IF NOT EXISTS uq_tenants_institution_id
    ON tenants (institution_id) WHERE institution_id IS NOT NULL;
