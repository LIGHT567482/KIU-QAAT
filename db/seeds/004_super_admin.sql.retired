-- QAAT — Platform tenant + Super-Admin seed
-- Runs AFTER db/migrations/015 commits (the 'SUPER_ADMIN' enum value must already
-- be committed before this file uses it).
--
-- Password: superadmin@qaat.platform = Super1234!
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/seeds/004_super_admin.sql

-- ── Sentinel platform tenant ──────────────────────────────────────────────────
-- The platform owner is modelled as a SUPER_ADMIN user belonging to this fixed
-- "platform" tenant, so the existing tenant_id-required login flow works unchanged.
-- The all-zeros UUID is a well-known constant the super-admin app bakes in.
INSERT INTO tenants (
    tenant_id, name, domain, rsa_key_id, attendance_threshold,
    motto, slogan
)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'QAAT Platform', 'platform.local', 'platform-rsa-key-v1', 75,
    'Trust, Verified.', 'Attendance integrity for every campus'
)
ON CONFLICT (tenant_id) DO NOTHING;

-- ── Super-admin user ──────────────────────────────────────────────────────────
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name)
VALUES (
    'd0000000-0000-0000-0000-000000000001',
    '00000000-0000-0000-0000-000000000000',
    'superadmin@qaat.platform',
    '$2b$12$Vy1oLnMsJKmZXrKYdFld8O3JIsfByRYa4Hh20p8iUIoI7.unf5Lie',
    'SUPER_ADMIN',
    'Platform Owner'
)
ON CONFLICT (tenant_id, email) DO NOTHING;
