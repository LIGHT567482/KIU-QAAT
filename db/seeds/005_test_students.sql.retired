-- QAAT — Two student accounts for eligibility / IDOR testing
-- Passwords: alice.student@test.local / bob.student@test.local = Student123!
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/seeds/005_test_students.sql

-- ── Login users (role STUDENT — no MFA) ───────────────────────────────────────
INSERT INTO users (user_id, tenant_id, email, password_hash, role, full_name) VALUES
  ('e0000000-0000-0000-0000-0000000000aa',
   'a0000000-0000-0000-0000-000000000001',
   'alice.student@test.local',
   '$2b$12$FGIXMRY8XARTi8CWHgkCv.e/jLV/AOhzEaPH4kLL.3tOfR6.mZ/n2',
   'STUDENT', 'Alice Student'),
  ('e0000000-0000-0000-0000-0000000000bb',
   'a0000000-0000-0000-0000-000000000001',
   'bob.student@test.local',
   '$2b$12$FGIXMRY8XARTi8CWHgkCv.e/jLV/AOhzEaPH4kLL.3tOfR6.mZ/n2',
   'STUDENT', 'Bob Student')
ON CONFLICT (tenant_id, email) DO NOTHING;

-- ── Enrolment records (email links the login user → student record) ───────────
INSERT INTO students_extended (
    student_id, tenant_id, course_id, full_name, email,
    academic_year, enrollment_status, current_year, semester, intake_session
) VALUES
  ('STU-ALICE', 'a0000000-0000-0000-0000-000000000001', 'COURSE-E2E-001',
   'Alice Student', 'alice.student@test.local', '2024/2025', 'ACTIVE', 1, 1, 'Morning'),
  ('STU-BOB',   'a0000000-0000-0000-0000-000000000001', 'COURSE-E2E-001',
   'Bob Student',   'bob.student@test.local',   '2024/2025', 'ACTIVE', 1, 1, 'Morning')
ON CONFLICT (student_id) DO NOTHING;

-- ── Distinct attendance so each student's eligibility is visibly different ─────
INSERT INTO student_attendance_summary (
    student_id, unit_id, unit_name, course_id, tenant_id,
    sessions_held, sessions_attended, attendance_percentage
) VALUES
  ('STU-ALICE', 'UNIT-E2E-001', 'Introduction to Programming', 'COURSE-E2E-001',
   'a0000000-0000-0000-0000-000000000001', 10, 9, 90.00),
  ('STU-BOB',   'UNIT-E2E-001', 'Introduction to Programming', 'COURSE-E2E-001',
   'a0000000-0000-0000-0000-000000000001', 10, 4, 40.00)
ON CONFLICT (tenant_id, student_id, unit_id) DO NOTHING;
