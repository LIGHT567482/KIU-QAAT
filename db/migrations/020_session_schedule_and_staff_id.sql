-- QAAT — Per-unit session schedule + lecturer staff ID
-- Migration: 020_session_schedule_and_staff_id.sql
--
-- #2 Session time/length: each course unit gets a session start time + duration.
--    The coordinator sets it once; schedule_locked then prevents changes (only the
--    institution ADMIN can edit a locked schedule).
-- #3 Lecturer QR: lecturers get a staff_id, compared at the live lecturer gate.
--
-- Apply: psql -h 127.0.0.1 -p 5434 -U qaat -d qaat -f db/migrations/020_session_schedule_and_staff_id.sql

ALTER TABLE course_units
    ADD COLUMN IF NOT EXISTS session_start            TIME,
    ADD COLUMN IF NOT EXISTS session_duration_minutes INTEGER,
    ADD COLUMN IF NOT EXISTS schedule_locked          BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE lecturers
    ADD COLUMN IF NOT EXISTS staff_id VARCHAR(64);

-- Persist the planned window on the session so timestamps can be judged against it.
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS planned_start            TIME,
    ADD COLUMN IF NOT EXISTS planned_duration_minutes INTEGER;
