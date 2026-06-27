-- QAAT — migration 038: platform tenant + Super-Admin baseline seed.
-- This is NOT test data — it's the minimum a FRESH install needs to log in and
-- start (the platform owner then registers real tenants). Lives in migrations so
-- it auto-applies on any fresh laptop via /docker-entrypoint-initdb.d, after the
-- SUPER_ADMIN enum (migration 015). Idempotent (ON CONFLICT DO NOTHING).
--
-- Default login:  superadmin@qaat.platform  /  Super1234!  (change it after first login)

INSERT INTO tenants (
    tenant_id, name, domain, rsa_key_id, attendance_threshold, motto, slogan
)
VALUES (
    '00000000-0000-0000-0000-000000000000',
    'QAAT Platform', 'platform.local', 'platform-rsa-key-v1', 75,
    'Trust, Verified.', 'Attendance integrity for every campus'
)
ON CONFLICT (tenant_id) DO NOTHING;

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
