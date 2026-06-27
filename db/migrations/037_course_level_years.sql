-- 037 — Years of study can differ per COURSE for the SAME level.
--
-- A Masters in Computer Science may run 3 years while a Masters in Software
-- Engineering runs 2. So years are keyed by (course, level): courses.level_years
-- maps level name → years for THAT course, overriding the tenant default
-- (tenants.level_years). Fallback order: course → tenant → 3.

ALTER TABLE courses
    ADD COLUMN IF NOT EXISTS level_years JSONB NOT NULL DEFAULT '{}'::jsonb;
