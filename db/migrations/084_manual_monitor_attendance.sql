-- 084: The lecture that was taught but never timetabled.
--
-- THE GAP. A QA monitor's round is generated from the timetable: they search a unit or a lecturer,
-- get that lecture's slot, and tick it. Every observation therefore has to be ABOUT a slot — and a
-- great many real lectures are not on the timetable at all. A unit added after the schedule was
-- locked, a make-up hour agreed in the corridor, a class moved into a free room, a visiting
-- lecturer covering a week: all of these are taught, attended, and completely invisible to quality
-- assurance, because the one form the monitor has to fill in starts by asking which timetabled slot
-- this is.
--
-- What the monitor did instead was the only thing available: nothing, or a tick against whatever
-- slot looked closest — which files a real observation under the wrong lecture, and that is worse
-- than the silence. So this adds the columns a monitor needs to describe a lecture from scratch,
-- standing in the room, with no timetable to lean on.
--
-- WHY THESE THREE. Everything else the monitor records (unit, lecturer, room, date, taught) already
-- has a column, because a timetabled observation carries it too. These three are the ones that only
-- a slot could previously supply:
--
--   students_counted  A HEADCOUNT, taken by eye. For a timetabled lecture the student total comes
--                     from the coordinator's check-in ledger — but an untimetabled lecture has no
--                     session, so there is no ledger, and "how many were in the room" would
--                     otherwise be unanswerable for exactly the lectures nobody else recorded.
--                     Deliberately separate from the derived count rather than merged with it: one
--                     is a register and the other is a person's estimate, and a report that cannot
--                     tell them apart invites the reader to trust the wrong one.
--
--   class_group       Which cohort was in the room, as free text ("2:1", "Year 2 Sem 1 Weekend").
--                     A timetabled lecture gets this from its offering; a typed one has no
--                     offering to get it from, and without it the record cannot say WHO was taught.
--
--   school            The college the lecture belongs to. Inherited from the course unit when the
--                     monitor picks a known one, typed when they do not — because a unit that is
--                     not in the curriculum yet is precisely the case this feature exists for.
--
-- entry_method already exists and already distinguishes sources ('PATROL', 'QA_REP_UPLOAD'). Manual
-- entries carry 'MANUAL' so every report can say where an observation came from — a monitor's
-- typed record and a scanned round are both evidence, but they are not the same kind of evidence,
-- and flattening them would be the same mistake as merging the ledger with the headcount.

ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS students_counted INTEGER,
    ADD COLUMN IF NOT EXISTS class_group      TEXT,
    ADD COLUMN IF NOT EXISTS school           TEXT;

COMMENT ON COLUMN lecturer_patrol_logs.students_counted IS
    'Heads counted in the room by the monitor. NULL for a timetabled observation, where the '
    'coordinator''s check-in ledger is the authority. Never merged with that ledger: an estimate '
    'and a register are different claims.';
COMMENT ON COLUMN lecturer_patrol_logs.class_group IS
    'The cohort as the monitor wrote it, for a lecture with no offering to read it from.';
COMMENT ON COLUMN lecturer_patrol_logs.school IS
    'The college, inherited from the course unit when it is a known one and typed when it is not.';

-- A monitor filing a manual entry has no scheduled_time to key on, so ux_patrol_logs_slot
-- (tenant, unit, date, scheduled_time, offering) would collapse every manual observation of the
-- same unit on the same day into one row — including two genuinely different lectures, an hour
-- apart, in different rooms. The client sends the OBSERVED time as scheduled_time for exactly this
-- reason; this index makes the intent explicit and keeps the lookup fast for the reports that ask
-- "what was recorded by hand".
CREATE INDEX IF NOT EXISTS ix_patrol_logs_manual
    ON lecturer_patrol_logs (tenant_id, session_date)
    WHERE entry_method = 'MANUAL';

-- ── The TLC staffs their own department's lectures ───────────────────────────────────────────
--
-- Migration 083 put a TLC in each department and gave them that department's timetable. Building a
-- timetable is not only placing an hour in a room: it is saying WHO teaches it, and until now
-- assigning a lecturer to a unit belonged to the head of department and the administrator alone.
-- That splits one job across two desks — the TLC lays out the week and then has to ask someone
-- else to name the lecturers in it — which is how a timetable ends up published with blank slots.
--
-- So the TLC assigns lecturers by default, inside their department, and the administrator keeps the
-- same power institution-wide (a department with no TLC in post must not become unstaffable). The
-- rule lives in resolveOrgScope, next to the identical rule for a head of department; nothing here
-- needs a new column. The index is the one that lookup wants.
CREATE INDEX IF NOT EXISTS ix_lecturer_assignments_unit
    ON lecturer_assignments (tenant_id, unit_id);
