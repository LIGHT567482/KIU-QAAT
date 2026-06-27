-- 034 — Course units are per LEVEL: a Degree's units differ from a Masters'/PhD's.
--
-- Units already belong to a course + year + semester; now they also belong to a
-- LEVEL, so each level of a course has its own curriculum. A cohort (offering)
-- sees only the units of its own level (+ year + semester). Existing units keep
-- level '' which matches the existing level-'' cohorts, so nothing breaks.

ALTER TABLE course_units
    ADD COLUMN IF NOT EXISTS level VARCHAR(40) NOT NULL DEFAULT '';
