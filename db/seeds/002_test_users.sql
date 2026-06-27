-- QAAT — Test Seed: Users (one per role, per tenant)
-- Passwords are all "Test1234!" — bcrypt hash generated at cost 12.
-- DO NOT use in production.

-- Alpha University users
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name)
VALUES
    -- COORDINATOR
    ('1',
     '1',
     'coordinator@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'COORDINATOR', 'Alice Coordinator'),

    -- QA_OFFICER
    ('2',
     '1',
     'qa.officer@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'QA_OFFICER', 'Bob QA Officer'),

    -- DQA_DIRECTOR
    ('3',
     '1',
     'dqa.director@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'DQA_DIRECTOR', 'Carol DQA Director'),

    -- VC
    ('4',
     '1',
     'vc@alpha.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'VC', 'Dr. David Vice Chancellor'),

    -- Beta University — COORDINATOR (for cross-tenant isolation tests)
    ('1',
     '2',
     'coordinator@beta.edu',
     '$2a$12$1xO1KXPJAnLHnTlUVG1qduJ4ZLsbN8F1DeHLR.dXOJhEwlmt33D52',
     'COORDINATOR', 'Eve Beta Coordinator')

ON CONFLICT DO NOTHING;
