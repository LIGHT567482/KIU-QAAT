-- QAAT — Test Seed: Users (one per role, per tenant)
-- Passwords are all "Test1234!" — bcrypt hash generated at cost 12.
-- DO NOT use in production.
--
-- Integer ids replaced with UUIDs: the columns are uuid, so every INSERT here used to
-- abort with `invalid input syntax for type uuid`. Seeds run without ON_ERROR_STOP, so
-- nothing reported it and the e2e login specs simply had no users to sign in as.

-- Alpha University users
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name)
VALUES
    -- COORDINATOR
    ('b1000000-0000-0000-0000-000000000001',
     'a1000000-0000-0000-0000-000000000001',
     'coordinator@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'COORDINATOR', 'Alice Coordinator'),

    -- QA_OFFICER
    ('b1000000-0000-0000-0000-000000000002',
     'a1000000-0000-0000-0000-000000000001',
     'qa.officer@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'QA_OFFICER', 'Bob QA Officer'),

    -- DQA_DIRECTOR
    ('b1000000-0000-0000-0000-000000000003',
     'a1000000-0000-0000-0000-000000000001',
     'dqa.director@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'DQA_DIRECTOR', 'Carol DQA Director'),

    -- VC
    ('b1000000-0000-0000-0000-000000000004',
     'a1000000-0000-0000-0000-000000000001',
     'vc@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'VC', 'Dr. David Vice Chancellor'),

    -- Beta University — COORDINATOR (for cross-tenant isolation tests)
    ('b1000000-0000-0000-0000-000000000001',
     'a2000000-0000-0000-0000-000000000002',
     'coordinator@beta.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'COORDINATOR', 'Eve Beta Coordinator')

ON CONFLICT DO NOTHING;
