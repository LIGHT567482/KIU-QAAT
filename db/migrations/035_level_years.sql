-- 035 — Years of study belong to the LEVEL, not the course.
--
-- A Degree in Computer Science may run 3 years while a Masters runs 2. We store a
-- per-level year count on the tenant (name → years) and stop using courses
-- duration. `tenants.levels` keeps the ordered names; `level_years` maps each to
-- its number of study years.

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS level_years JSONB NOT NULL DEFAULT '{}'::jsonb;

-- courses.total_years is no longer collected; keep the column (nullable) so old
-- rows/joins don't break, but the admin no longer sets it.
ALTER TABLE courses ALTER COLUMN total_years DROP NOT NULL;
