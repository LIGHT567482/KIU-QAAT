-- QAAT — Course Roadmap, Nullable Coordinator, Active Semester
-- Migration: 012_course_roadmap_and_semester.sql
-- Run order: 12 of N
--
-- Changes:
--   courses.coordinator_id  — drop NOT NULL (courses can exist before a coordinator is assigned)
--   courses.total_years     — new: how many years the course runs (defines roadmap depth)
--   course_units.default_venue_id — new: optional default venue where the unit normally meets
--   tenants.active_academic_year  — new: which academic year is currently running
--   tenants.active_semester       — new: which semester (1 or 2) is currently active
--
-- The active_academic_year + active_semester on a tenant is the single switch the
-- admin flips at the start of each semester. The daily manifest uses these to filter
-- which course units the coordinator sees (only the current semester's units).

-- ─── courses ─────────────────────────────────────────────────────────────────

-- Allow a course to be created before a coordinator is assigned.
ALTER TABLE courses ALTER COLUMN coordinator_id DROP NOT NULL;

-- Roadmap depth: BSc = 3, BEd = 4, MBChB = 5, etc.
ALTER TABLE courses ADD COLUMN IF NOT EXISTS total_years SMALLINT NOT NULL DEFAULT 3 CHECK (total_years BETWEEN 1 AND 8);

-- ─── course_units ─────────────────────────────────────────────────────────────

-- Optional default venue for a unit (the room where it normally meets).
-- Used by the manifest so the coordinator doesn't have to pick a venue every day.
ALTER TABLE course_units
    ADD COLUMN IF NOT EXISTS default_venue_id VARCHAR(50) REFERENCES venues(venue_id) ON DELETE SET NULL;

-- ─── tenants ─────────────────────────────────────────────────────────────────

-- Active academic period — admin sets these once per semester.
-- Empty / zero means "show all" (useful before the first semester is configured).
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS active_academic_year TEXT NOT NULL DEFAULT '';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS active_semester      SMALLINT NOT NULL DEFAULT 0
    CHECK (active_semester IN (0, 1, 2));
-- 0 = not configured / show all; 1 = Semester 1; 2 = Semester 2.

-- ─── Index ────────────────────────────────────────────────────────────────────
-- Speeds up the manifest query: coordinator's units for the active semester.
CREATE INDEX IF NOT EXISTS idx_course_units_roadmap
    ON course_units(course_id, year, semester);
