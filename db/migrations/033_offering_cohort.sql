-- 033 — Offerings become full COHORTS: course + session + year + semester + level + intake.
--
-- Each cohort has its own coordinator and its own timetable, so a coordinator for
-- "Computer Science · Day · Year 1 · Sem 1 · Degree · January" sees only that
-- cohort's units/schedule — distinct from any other year/semester/level/intake.
-- year+semester actually select the units (units carry year+semester); level and
-- intake distinguish cohorts that share the same units but want their own
-- coordinator + timetable.

ALTER TABLE course_offerings
    ADD COLUMN IF NOT EXISTS study_year SMALLINT     NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS semester   SMALLINT     NOT NULL DEFAULT 1,
    ADD COLUMN IF NOT EXISTS level      VARCHAR(40)  NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS intake     VARCHAR(64)  NOT NULL DEFAULT '';

-- A cohort is unique per (tenant, course, session, year, semester, level, intake).
DROP INDEX IF EXISTS ux_offerings_course_session;
CREATE UNIQUE INDEX IF NOT EXISTS ux_offerings_cohort
    ON course_offerings (tenant_id, course_id, session_type, study_year, semester, level, intake);

-- ux_offerings_tenant_coordinator (one offering per coordinator) is unchanged.
