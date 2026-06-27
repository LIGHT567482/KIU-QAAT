-- QAAT — Add STUDENT to the user role enum
-- Migration: 018_student_role.sql
--
-- The gateway defines RoleStudent = "STUDENT" and gates the student endpoints
-- (/api/v1/eligibility/{id}, /api/v1/student/checkin) for it, and the student
-- portal logs in via /api/v1/auth/login — but user_role_enum had no STUDENT
-- value, so no student account could exist and those paths were unreachable.
-- Adding the value makes the student login path representable. Must be committed
-- before it is used (Postgres rule), so student rows are seeded separately.
--
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/migrations/018_student_role.sql

ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'STUDENT';
