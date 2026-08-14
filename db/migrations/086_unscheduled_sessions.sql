-- 086: Students must still be able to check in to a lecture nobody timetabled.
--
-- THE HOLE THIS CLOSES. Migration 084 gave the QA monitor a way to record a lecture that is being
-- taught but is not on the timetable. That fixed the OBSERVATION and left the ATTENDANCE behind:
-- the coordinator's app would only start a session for a unit timetabled for today, so for exactly
-- that lecture there was no session — and with no session there is no room code, no check-in, and
-- no register. The students who sat through it have no attendance for it.
--
-- The consequence is worse than a missing row. Attendance drives exam eligibility, so a student who
-- attended a make-up lecture is marked absent from it, and the number that decides whether they sit
-- the exam is wrong in the direction that costs them. Meanwhile the monitor's headcount — an
-- estimate, taken by eye from a doorway — was the only record that the lecture had anyone in it.
--
-- So the session may now be opened for a unit that is not on today's timetable, and the students
-- check in through EXACTLY the same flow they always do: the room code on the coordinator's screen,
-- the LAN proximity gate, the lecturer's gate scan, one device per student, no duplicates. Nothing
-- about the student's side changes, because nothing about it should: the reason a check-in means
-- something is those gates, and a lecture being off-timetable is no reason to relax any of them.
--
-- WHY A COLUMN RATHER THAN A DERIVATION. "Was this timetabled?" could be computed by looking for a
-- slot — but the timetable is edited, and a slot added next week would silently rewrite what
-- happened today. It is a fact about the day the lecture ran, exactly like room_is_provision in 085,
-- so it is recorded on the day.
--
-- The two flags are independent and both can be true: a lecture can be off-timetable AND in a
-- substituted room, which is the common shape of a make-up class squeezed into a free hall.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS unscheduled BOOLEAN NOT NULL DEFAULT false;

COMMENT ON COLUMN sessions.unscheduled IS
    'The unit was not on the timetable for this weekday when the session was opened. Students '
    'check in through the identical flow; the flag exists so the attendance record says the '
    'lecture was off-timetable rather than leaving a reader to infer it from a missing slot.';

-- "Which lectures ran off-timetable this term" is a QA question asked over a date range, and they
-- are a minority of sessions.
CREATE INDEX IF NOT EXISTS ix_sessions_unscheduled
    ON sessions (tenant_id, session_date)
    WHERE unscheduled;
