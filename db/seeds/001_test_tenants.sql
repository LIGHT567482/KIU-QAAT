-- QAAT — Test Seed: Two Tenants
-- Used for RLS isolation testing and local development.

INSERT INTO tenants (tenant_id, name, domain, rsa_key_id, attendance_threshold, brand_color)
VALUES
    ('a0000000-0000-0000-0000-000000000001', 'Alpha University',    'alpha.edu',    'alpha-rsa-key-v1',  75, '#1a73e8'),
    ('b0000000-0000-0000-0000-000000000002', 'Beta University',     'beta.edu',     'beta-rsa-key-v1',   80, '#0f9d58')
ON CONFLICT DO NOTHING;
