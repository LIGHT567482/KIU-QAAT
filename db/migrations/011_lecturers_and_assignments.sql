-- QAAT — Lecturers and Lecturer Assignments
-- Migration: 011_lecturers_and_assignments.sql
-- Run order: 11 of N
--
-- Adds two tables:
--   lecturers             — tenant-scoped roster of teaching staff
--   lecturer_assignments  — maps a lecturer to a specific course unit for a given
--                           academic year, year-of-study, semester, and intake session.
--
-- The coordinator uses lecturer_assignments to populate a dropdown when opening
-- a session, so the session record always carries who taught that class.

-- ─── Lecturers ───────────────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS lecturers (
    lecturer_id   UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id     UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    full_name     TEXT NOT NULL,
    email         TEXT,
    phone         TEXT,
    department    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE lecturers ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturers FORCE ROW LEVEL SECURITY;

CREATE POLICY "tenant_isolation" ON lecturers
    USING (tenant_id = (current_setting('app.current_tenant', true))::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON lecturers TO qaat_app;

-- ─── Lecturer Assignments ─────────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS lecturer_assignments (
    assignment_id  UUID DEFAULT gen_random_uuid() PRIMARY KEY,
    tenant_id      UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    lecturer_id    UUID NOT NULL REFERENCES lecturers(lecturer_id) ON DELETE CASCADE,
    unit_id        VARCHAR(50) NOT NULL REFERENCES course_units(unit_id),
    course_id      VARCHAR(50) NOT NULL REFERENCES courses(course_id),
    academic_year  TEXT NOT NULL,
    year           SMALLINT NOT NULL DEFAULT 1,
    semester       SMALLINT NOT NULL DEFAULT 1,
    intake_session intake_session_enum NOT NULL DEFAULT 'Morning',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(lecturer_id, unit_id, academic_year, intake_session)
);

ALTER TABLE lecturer_assignments ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_assignments FORCE ROW LEVEL SECURITY;

CREATE POLICY "tenant_isolation" ON lecturer_assignments
    USING (tenant_id = (current_setting('app.current_tenant', true))::uuid);

GRANT SELECT, INSERT, UPDATE, DELETE ON lecturer_assignments TO qaat_app;

-- ─── Index ────────────────────────────────────────────────────────────────────
-- Fast lookup of all lecturers for a given course unit (used by the coordinator
-- dropdown endpoint at /api/v1/coordinator/units/{unit_id}/lecturers).
CREATE INDEX IF NOT EXISTS idx_lecturer_assignments_unit
    ON lecturer_assignments(unit_id, tenant_id);
