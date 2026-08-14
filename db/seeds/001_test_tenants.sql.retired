-- QAAT — Test Seed: Two Tenants
-- Used for RLS isolation testing and local development.
--
-- The ids here were integers ('1', '2') from before tenant_id became a UUID, so every
-- one of these INSERTs has been failing with `invalid input syntax for type uuid` — and
-- since the seeds are run with ON_ERROR_STOP off, they failed silently. The e2e suite
-- signs in as these users, so its login tests were failing on an empty database rather
-- than on anything about the application.

INSERT INTO tenants (tenant_id, name, domain, rsa_key_id, attendance_threshold, brand_color)
VALUES
    ('a1000000-0000-0000-0000-000000000001', 'Alpha University',    'alpha.edu',    'alpha-rsa-key-v1',  75, '#1a73e8'),
    ('a2000000-0000-0000-0000-000000000002', 'Beta University',     'beta.edu',     'beta-rsa-key-v1',   80, '#0f9d58')
ON CONFLICT DO NOTHING;
