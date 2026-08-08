-- 083: Compensation lectures, and a TLC per department.
--
-- ── COMPENSATION ─────────────────────────────────────────────────────────────────────────────
--
-- A compensation lecture is one taught to make up a lecture that did not happen — after a public
-- holiday, an illness, a room clash. It is a normal, expected part of a semester, and until now
-- the system had no idea it existed.
--
-- That absence is not cosmetic. A compensation lecture is taught OFF the timetable, so every
-- record the institution keeps reads it as an anomaly:
--
--   · the QA monitor arrives at a room with a lecture in it that the timetable says is free, and
--     the only thing they can file is a tick against a slot that does not exist
--   · the lecture the compensation is FOR shows as not taught, forever, because the making-good
--     happened somewhere the record cannot see
--   · a lecturer who taught 15 hours in a week where 12 were timetabled looks like a data error
--
-- So the flag lives on the MONITOR's record, not the coordinator's. The monitor is the one who
-- walks into the room and sees a lecture happening; they are the only witness in a position to say
-- "this is a compensation" at the time, which is the only time anyone can say it reliably. Every
-- other view derives it from there rather than storing a second copy that can disagree.
--
-- compensation_for is the date of the lecture being made good, when the monitor knows it. It is
-- deliberately nullable: a monitor who is told "this is a compensation" but not which one is still
-- recording something true and useful, and refusing the record to get a tidier column would lose
-- the fact entirely.

ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS is_compensation  BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS compensation_for DATE;

COMMENT ON COLUMN lecturer_patrol_logs.is_compensation IS
    'The monitor found this lecture being taught to make good an earlier one. Set by the monitor '
    'in the round, because they are the only witness present when it can be established.';
COMMENT ON COLUMN lecturer_patrol_logs.compensation_for IS
    'The date of the lecture being made good, when the monitor was told which one. NULL means the '
    'compensation is recorded but unattributed — still true, and better than discarding it.';

-- Compensations are looked up per lecturer over a date range on every attendance view, and they
-- are a small minority of rows, so the index is partial.
CREATE INDEX IF NOT EXISTS ix_patrol_logs_compensation
    ON lecturer_patrol_logs (tenant_id, lecturer_id, session_date)
    WHERE is_compensation;

-- ── A TLC IN EVERY DEPARTMENT ────────────────────────────────────────────────────────────────
--
-- The Teaching & Learning Centre owns the timetable. Migration 075 introduced the role as a single
-- institution-wide account, and at one desk for a whole university that is a bottleneck standing
-- exactly where the term starts: one person building every department's schedule, with nobody able
-- to help and everybody blocked behind them.
--
-- A TLC now sits in a department, the way an HOD does, and designs the timetable for that
-- department. The scope comes from their own user row — never from the request — so a TLC cannot
-- reach another department's schedule by naming it, which is the same rule every other org-scoped
-- role in this system follows.
--
-- users.department and users.school already exist and already carry exactly this for HOD and Dean,
-- so there is no new column: what was missing was that nothing required them for a TLC and nothing
-- enforced them. The index makes "who is the TLC for this department" a lookup rather than a scan,
-- because the timetable pages ask it on every write.

CREATE INDEX IF NOT EXISTS ix_users_tlc_department
    ON users (tenant_id, department)
    WHERE role = 'TLC';

-- An institution-wide TLC (no department) stays legal and keeps working: it is what every existing
-- TLC account is, it is the right answer for a small institution, and taking it away would lock
-- the current post-holder out of the timetable the moment this migration ran. A departmental TLC
-- is confined to their department; a TLC without one still covers everything, as before.
COMMENT ON COLUMN users.department IS
    'The org unit this account is confined to. For HOD, and now TLC, it is the department they are '
    'responsible for; a TLC with no department is institution-wide, which is what TLC accounts '
    'created before migration 083 are.';
