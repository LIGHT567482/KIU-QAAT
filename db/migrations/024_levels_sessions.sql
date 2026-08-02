-- 024 — Course levels & study sessions (next.txt "biggest thing")
--
-- An institution runs ONE course at several LEVELS (Certificate/Diploma/Degree/
-- Masters/PhD) and, within each level, several study SESSIONS (Morning/Day/
-- Evening/Distance/Weekend), each session having its OWN coordinator who must not
-- see another session's data. Curriculum (course_units) is shared per level.
--
-- Model: `courses` is reinterpreted as a PROGRAM (course + level). A new
-- `course_offerings` row = (program + session) and owns the coordinator + the
-- students. The per-session timetable (day + time) lives in
-- `offering_unit_schedules`. Tenant admins define the level/session label lists.

-- ── Tenant-configurable label lists (mirror tenants.intakes from migration 022) ──
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS levels TEXT[] NOT NULL
        DEFAULT ARRAY['Certificate', 'Diploma', 'Degree', 'Masters', 'PhD'],
    ADD COLUMN IF NOT EXISTS study_sessions TEXT[] NOT NULL
        DEFAULT ARRAY['Morning', 'Day', 'Evening', 'Distance', 'Weekend'];

-- ── courses becomes the PROGRAM (course + level) ─────────────────────────────
ALTER TABLE courses
    ADD COLUMN IF NOT EXISTS level        VARCHAR(40),
    ADD COLUMN IF NOT EXISTS course_group VARCHAR(160);

UPDATE courses SET level = COALESCE(NULLIF(level,''), 'Degree') WHERE level IS NULL OR level = '';
UPDATE courses SET course_group = name WHERE course_group IS NULL OR course_group = '';

-- The one-course-per-coordinator rule moves to offerings (below).
DROP INDEX IF EXISTS ux_courses_tenant_coordinator;

-- ── Offerings = (program + session), owning the coordinator + students ───────
CREATE TABLE IF NOT EXISTS course_offerings (
    offering_id    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    course_id      VARCHAR(50) NOT NULL REFERENCES courses(course_id) ON DELETE CASCADE,
    session_type   VARCHAR(40) NOT NULL,
    coordinator_id VARCHAR(50),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One offering per (program, session); one offering per coordinator (per tenant).
CREATE UNIQUE INDEX IF NOT EXISTS ux_offerings_course_session
    ON course_offerings (tenant_id, course_id, session_type);
CREATE UNIQUE INDEX IF NOT EXISTS ux_offerings_tenant_coordinator
    ON course_offerings (tenant_id, coordinator_id)
    WHERE coordinator_id IS NOT NULL;

ALTER TABLE course_offerings ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_offerings FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON course_offerings
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON course_offerings TO qaat_app;

-- ── Per-offering, per-unit timetable (day + time differ per session) ─────────
CREATE TABLE IF NOT EXISTS offering_unit_schedules (
    offering_id              UUID        NOT NULL REFERENCES course_offerings(offering_id) ON DELETE CASCADE,
    unit_id                  VARCHAR(50) NOT NULL REFERENCES course_units(unit_id) ON DELETE CASCADE,
    tenant_id                UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    day_of_week              SMALLINT    CHECK (day_of_week BETWEEN 1 AND 7),
    session_start            TIME,
    session_duration_minutes INTEGER,
    schedule_locked          BOOLEAN     NOT NULL DEFAULT false,
    PRIMARY KEY (offering_id, unit_id)
);

ALTER TABLE offering_unit_schedules ENABLE ROW LEVEL SECURITY;
ALTER TABLE offering_unit_schedules FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON offering_unit_schedules
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON offering_unit_schedules TO qaat_app;

-- ── Bind students + attendance sessions to an offering ───────────────────────
ALTER TABLE students_extended
    ADD COLUMN IF NOT EXISTS offering_id UUID REFERENCES course_offerings(offering_id) ON DELETE SET NULL;
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS offering_id UUID REFERENCES course_offerings(offering_id) ON DELETE SET NULL;

-- ── Backfill existing data: one 'Day' offering per existing course ───────────
-- Guarded with NOT EXISTS rather than ON CONFLICT. Two unique indexes govern this table —
-- ux_offerings_course_session (tenant, course, session_type) and ux_offerings_tenant_coordinator
-- (tenant, coordinator) — and the original clause named only the first, so a course whose
-- coordinator already ran a non-'Day' offering violated the second and aborted the migration on any
-- database that already held cohort data. Naming both is not possible either: migration 036 later
-- makes the cohort index DEFERRABLE, and a deferrable constraint cannot be an ON CONFLICT arbiter.
-- Explicit guards sidestep arbiters entirely. A course that cannot be given a 'Day' offering is
-- simply left as it is.
--
-- On a fresh database both tables are empty here, so this inserts exactly what it always did.
INSERT INTO course_offerings (tenant_id, course_id, session_type, coordinator_id)
SELECT DISTINCT ON (c.tenant_id, COALESCE(NULLIF(c.coordinator_id, ''), c.course_id))
       c.tenant_id, c.course_id, 'Day', NULLIF(c.coordinator_id, '')
FROM courses c
WHERE NOT EXISTS (   -- this course already has a 'Day' offering
        SELECT 1 FROM course_offerings o
        WHERE o.tenant_id = c.tenant_id AND o.course_id = c.course_id AND o.session_type = 'Day')
  AND NOT EXISTS (   -- this coordinator already runs some offering (one per coordinator per tenant)
        SELECT 1 FROM course_offerings o
        WHERE o.tenant_id = c.tenant_id AND o.coordinator_id = NULLIF(c.coordinator_id, ''))
-- …and, among the courses that survive, keep only one per coordinator so the batch cannot
-- collide with itself. Courses with no coordinator are keyed by course_id, so all of them pass.
ORDER BY c.tenant_id, COALESCE(NULLIF(c.coordinator_id, ''), c.course_id), c.course_id;

UPDATE students_extended se
SET offering_id = o.offering_id
FROM course_offerings o
WHERE o.course_id = se.course_id AND o.tenant_id = se.tenant_id
  AND se.offering_id IS NULL;

UPDATE sessions s
SET offering_id = o.offering_id
FROM course_units cu
JOIN course_offerings o ON o.course_id = cu.course_id AND o.tenant_id = cu.tenant_id
WHERE s.unit_id = cu.unit_id AND s.tenant_id = cu.tenant_id
  AND s.offering_id IS NULL;

INSERT INTO offering_unit_schedules (offering_id, unit_id, tenant_id, session_start, session_duration_minutes, schedule_locked)
SELECT o.offering_id, cu.unit_id, cu.tenant_id, cu.session_start, cu.session_duration_minutes, COALESCE(cu.schedule_locked, false)
FROM course_units cu
JOIN course_offerings o ON o.course_id = cu.course_id AND o.tenant_id = cu.tenant_id
WHERE cu.session_start IS NOT NULL OR cu.schedule_locked = true
ON CONFLICT (offering_id, unit_id) DO NOTHING;
