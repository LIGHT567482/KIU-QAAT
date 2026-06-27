-- 036 — Make the cohort uniqueness DEFERRABLE so the "advance semester" button can
-- shift every cohort by one step in a single transaction without tripping on the
-- transient overlaps (the check runs once at COMMIT, when the shift is complete).

DROP INDEX IF EXISTS ux_offerings_cohort;

ALTER TABLE course_offerings
    ADD CONSTRAINT ux_offerings_cohort
    UNIQUE (tenant_id, course_id, session_type, study_year, semester, level, intake)
    DEFERRABLE INITIALLY DEFERRED;
