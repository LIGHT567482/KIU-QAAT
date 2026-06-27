-- 022 — Tenant-configurable intakes (#1)
--
-- Intakes (e.g. "January Intake", "May Intake", "August Intake") are defined by
-- each tenant admin rather than fixed in an enum. We relax the intake_session
-- columns to free text and store the per-tenant list of allowed intakes on the
-- tenant row. Existing values (Morning/Evening/…) survive the conversion.

ALTER TABLE students_extended
    ALTER COLUMN intake_session TYPE VARCHAR(64) USING intake_session::text;

ALTER TABLE lecturer_assignments
    ALTER COLUMN intake_session TYPE VARCHAR(64) USING intake_session::text;

-- Per-tenant configurable intake labels. Seeded with the common monthly intakes;
-- each tenant admin can edit this list.
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS intakes TEXT[] NOT NULL
        DEFAULT ARRAY['January Intake', 'May Intake', 'August Intake'];

-- The intake_session_enum type is intentionally left in place (now unused by the
-- relaxed columns) to avoid breaking any historical references; it can be dropped
-- in a later cleanup once confirmed unused.
