-- QAAT — Test Seed: Users (one per role, per tenant)
-- Passwords are all "Test1234!" — bcrypt hash generated at cost 12.
-- DO NOT use in production.

-- Alpha University users
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name)
VALUES
    -- COORDINATOR
    ('a1000001-0000-0000-0000-000000000001',
     'a0000000-0000-0000-0000-000000000001',
     'coordinator@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'COORDINATOR', 'Alice Coordinator'),

    -- QA_OFFICER
    ('a1000002-0000-0000-0000-000000000001',
     'a0000000-0000-0000-0000-000000000001',
     'qa.officer@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'QA_OFFICER', 'Bob QA Officer'),

    -- DQA_DIRECTOR
    ('a1000003-0000-0000-0000-000000000001',
     'a0000000-0000-0000-0000-000000000001',
     'dqa.director@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'DQA_DIRECTOR', 'Carol DQA Director'),

    -- VC
    ('a1000004-0000-0000-0000-000000000001',
     'a0000000-0000-0000-0000-000000000001',
     'vc@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'VC', 'Dr. David Vice Chancellor'),

    -- Beta University — COORDINATOR (for cross-tenant isolation tests)
    ('b1000001-0000-0000-0000-000000000002',
     'b0000000-0000-0000-0000-000000000002',
     'coordinator@beta.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'COORDINATOR', 'Eve Beta Coordinator')

ON CONFLICT DO NOTHING;
