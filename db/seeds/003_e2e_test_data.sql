-- QAAT — E2E Test Seed
-- Fixed UUIDs so the E2E script can reference them.
-- Passwords: admin@test.local=Admin1234!  coordinator@test.local=Coord1234!
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/seeds/003_e2e_test_data.sql

-- ── Tenant ────────────────────────────────────────────────────────────────────
INSERT INTO tenants (
    tenant_id, name, domain, rsa_key_id, attendance_threshold,
    active_academic_year, active_semester
)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'Test University', 'test.local', 'test-rsa-key-v1', 75,
    '2024/2025', 1
)
ON CONFLICT (tenant_id) DO NOTHING;

-- ── Users ─────────────────────────────────────────────────────────────────────
-- ADMIN user
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name)
VALUES (
    'b0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'admin@test.local',
    '$2b$12$94aOId3Qt3gAuUdh8gYV1u4Yac01ot9k0JzbWD6479wIGp7qoGpX.',
    'ADMIN',
    'Test Admin'
)
ON CONFLICT (tenant_id, email) DO NOTHING;

-- COORDINATOR user
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name)
VALUES (
    'b0000000-0000-0000-0000-000000000002',
    'a0000000-0000-0000-0000-000000000001',
    'coordinator@test.local',
    '$2b$12$1HumftofNSNfbJiLEp3.iebZr2Y1flDHluH8f/wDn1QmtFjCN1YTq',
    'COORDINATOR',
    'Test Coordinator'
)
ON CONFLICT (tenant_id, email) DO NOTHING;

-- ── Course ────────────────────────────────────────────────────────────────────
INSERT INTO courses (course_id, tenant_id, name, coordinator_id, department, total_years)
VALUES (
    'COURSE-E2E-001',
    'a0000000-0000-0000-0000-000000000001',
    'Bachelor of Science in Computer Science',
    'b0000000-0000-0000-0000-000000000002',
    'Computer Science',
    3
)
ON CONFLICT (course_id) DO NOTHING;

-- ── Course Unit ───────────────────────────────────────────────────────────────
INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester, academic_year)
VALUES (
    'UNIT-E2E-001',
    'a0000000-0000-0000-0000-000000000001',
    'COURSE-E2E-001',
    'Introduction to Programming',
    1,
    1,
    '2024/2025'
)
ON CONFLICT (unit_id) DO NOTHING;

-- ── Lecturer ──────────────────────────────────────────────────────────────────
INSERT INTO lecturers (lecturer_id, tenant_id, full_name, email, department)
VALUES (
    'c0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'Dr. Jane Smith',
    'jane.smith@test.local',
    'Computer Science'
)
ON CONFLICT (lecturer_id) DO NOTHING;

-- ── Lecturer Assignment ───────────────────────────────────────────────────────
INSERT INTO lecturer_assignments (
    tenant_id, lecturer_id, unit_id, course_id, academic_year, year, semester, intake_session
)
VALUES (
    'a0000000-0000-0000-0000-000000000001',
    'c0000000-0000-0000-0000-000000000001',
    'UNIT-E2E-001',
    'COURSE-E2E-001',
    '2024/2025',
    1,
    1,
    'Morning'
)
ON CONFLICT DO NOTHING;

-- ── Student ──────────────────────────────────────────────────────────────────
INSERT INTO students_extended (
    student_id, tenant_id, course_id, full_name, email,
    academic_year, enrollment_status, current_year, semester, intake_session
)
VALUES (
    's0000000-0000-0000-0000-000000000001',
    'a0000000-0000-0000-0000-000000000001',
    'COURSE-E2E-001',
    'Test Student',
    'jzany17@gmail.com',
    '2024/2025',
    'ACTIVE',
    1,
    1,
    'Morning'
)
ON CONFLICT (student_id) DO NOTHING;
