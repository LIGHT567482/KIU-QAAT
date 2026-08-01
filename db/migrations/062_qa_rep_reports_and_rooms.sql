-- 062: QA-rep report submissions + a structured room/room-code registry.
--
-- Two halves, both Phase-4 of the QA subsystem:
--
-- 1) QA REP SUBMISSIONS. A QA_DEPT_REP (one department) or QA_SCHOOL_HANDLER (one school) uploads
--    the monitoring workbook they already fill by hand. The recognised rows are parsed into
--    `lecturer_patrol_logs` (entry_method = 'QA_REP_UPLOAD') so they flow into every existing
--    teaching report next to the patroller's own observations, AND the original workbook is kept
--    verbatim as the evidence behind that submission (`qa_rep_submissions.file_bytes`).
--
-- 2) ROOMS. `timetable_slots.room` was free text, so "LR 101", "LR-101" and "lr101" were three
--    different rooms and nothing could be reported per room. `venues` is already the physical-room
--    table (venue_id IS the room code, e.g. LR-101), so this EXTENDS it rather than adding a second
--    registry: org linkage + a type + an active flag, plus a structured `timetable_slots.venue_id`
--    FK. The free-text `room` column is KEPT as the display value (same pattern as 059's
--    courses.school/department), so every current query keeps working.

-- ─── 1. QA rep submissions ───────────────────────────────────────────────────

CREATE TABLE IF NOT EXISTS qa_rep_submissions (
    submission_id  UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id      UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    submitted_by   UUID         NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    submitter_name VARCHAR(200),
    submitter_role VARCHAR(40)  NOT NULL,
    -- DEPARTMENT (a dept rep) or SCHOOL (a school handler) — which org unit the file covers.
    scope_kind     VARCHAR(20)  NOT NULL,
    department     VARCHAR(120),
    school         VARCHAR(120),
    period_label   VARCHAR(60),          -- free text as the rep names it, e.g. "July 2026", "Week 6"
    period_from    DATE,
    period_to      DATE,
    notes          TEXT,
    file_name      VARCHAR(255) NOT NULL,
    file_size      INTEGER      NOT NULL,
    file_bytes     BYTEA        NOT NULL, -- the original workbook, kept verbatim
    total_rows     INTEGER      NOT NULL DEFAULT 0,
    parsed_rows    INTEGER      NOT NULL DEFAULT 0,
    skipped_rows   INTEGER      NOT NULL DEFAULT 0,
    parse_errors   TEXT[]       NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_qa_rep_submissions_scope
    ON qa_rep_submissions (tenant_id, school, department, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_qa_rep_submissions_by
    ON qa_rep_submissions (tenant_id, submitted_by, created_at DESC);

-- ENABLE (not FORCE): the qaat_app data plane stays tenant-isolated by RLS, while the owner/
-- superuser admin connection can still read across tenants. 059+060 settled on this pairing —
-- FORCE also blocks the owner, which broke the admin dashboard's reads.
ALTER TABLE qa_rep_submissions ENABLE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS "tenant_isolation" ON qa_rep_submissions;
CREATE POLICY "tenant_isolation" ON qa_rep_submissions
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON qa_rep_submissions TO qaat_app;

-- Every observation parsed out of a workbook points back at the submission it came from, so a
-- re-upload can supersede its own rows and a deleted submission takes its derived rows with it.
ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS submission_id UUID REFERENCES qa_rep_submissions(submission_id) ON DELETE CASCADE;
-- The rep's free-text comment for the slot ("lecturer on study leave"), which the patroller app
-- has no field for but the paper workbook always does.
ALTER TABLE lecturer_patrol_logs ADD COLUMN IF NOT EXISTS remarks TEXT;

CREATE INDEX IF NOT EXISTS idx_patrol_logs_submission
    ON lecturer_patrol_logs (submission_id) WHERE submission_id IS NOT NULL;

-- ─── 2. Structured rooms / room codes ────────────────────────────────────────

-- A room belongs to a school (and optionally a department) so a dean/HOD/QA handler can be shown
-- "the rooms my unit teaches in". Both nullable: shared central halls belong to no one.
ALTER TABLE venues ADD COLUMN IF NOT EXISTS school_id     UUID REFERENCES schools(school_id)         ON DELETE SET NULL;
ALTER TABLE venues ADD COLUMN IF NOT EXISTS department_id UUID REFERENCES departments(department_id) ON DELETE SET NULL;
-- LECTURE_HALL | LAB | SEMINAR | OFFICE | OTHER — free-form enough to grow without a migration.
ALTER TABLE venues ADD COLUMN IF NOT EXISTS room_type     VARCHAR(30) NOT NULL DEFAULT 'LECTURE_HALL';
-- Decommissioned rooms are deactivated, never deleted: old sessions/slots still reference them.
ALTER TABLE venues ADD COLUMN IF NOT EXISTS is_active     BOOLEAN     NOT NULL DEFAULT true;

CREATE INDEX IF NOT EXISTS idx_venues_tenant ON venues (tenant_id, is_active);

-- Structured link from the weekly timetable to the room registry. `room` stays as the display text.
ALTER TABLE timetable_slots
    ADD COLUMN IF NOT EXISTS venue_id VARCHAR(50) REFERENCES venues(venue_id) ON DELETE SET NULL;
CREATE INDEX IF NOT EXISTS idx_timetable_slots_venue ON timetable_slots (tenant_id, venue_id);

-- Backfill A: link slots whose free-text room already names an existing venue (by code or by name),
-- ignoring case and surrounding space.
UPDATE timetable_slots ts
SET venue_id = v.venue_id
FROM venues v
WHERE ts.venue_id IS NULL
  AND v.tenant_id = ts.tenant_id
  AND COALESCE(btrim(ts.room), '') <> ''
  AND (btrim(lower(v.venue_id)) = btrim(lower(ts.room)) OR btrim(lower(v.name)) = btrim(lower(ts.room)));

-- Backfill B: promote every remaining free-text room into a real venue, so the admin's room list
-- starts out complete instead of empty. venue_id is a GLOBAL primary key (a pre-existing quirk of
-- 001), so a code already taken by another tenant is skipped — those slots simply stay unlinked
-- until an admin gives the room a distinct code.
INSERT INTO venues (venue_id, tenant_id, name)
SELECT btrim(ts.room), MIN(ts.tenant_id::text)::uuid, btrim(ts.room)
FROM timetable_slots ts
WHERE ts.venue_id IS NULL
  AND COALESCE(btrim(ts.room), '') <> ''
  AND length(btrim(ts.room)) <= 50
GROUP BY btrim(ts.room)
ON CONFLICT (venue_id) DO NOTHING;

-- Backfill C: re-run the link now that those venues exist (same-tenant match only).
UPDATE timetable_slots ts
SET venue_id = v.venue_id
FROM venues v
WHERE ts.venue_id IS NULL
  AND v.tenant_id = ts.tenant_id
  AND COALESCE(btrim(ts.room), '') <> ''
  AND btrim(lower(v.venue_id)) = btrim(lower(ts.room));

-- Backfill D: attach each room to the school of the courses most often timetabled in it, so the
-- freshly-promoted rooms are not orphaned. Only fills rooms that have no school yet.
UPDATE venues v
SET school_id = best.school_id
FROM (
    SELECT DISTINCT ON (ts.venue_id)
           ts.venue_id, c.school_id, COUNT(*) AS n
    FROM timetable_slots ts
    JOIN course_units cu ON cu.unit_id   = ts.unit_id  AND cu.tenant_id = ts.tenant_id
    JOIN courses      c  ON c.course_id  = cu.course_id AND c.tenant_id = cu.tenant_id
    WHERE ts.venue_id IS NOT NULL AND c.school_id IS NOT NULL
    GROUP BY ts.venue_id, c.school_id
    ORDER BY ts.venue_id, n DESC
) best
WHERE v.venue_id = best.venue_id AND v.school_id IS NULL;
