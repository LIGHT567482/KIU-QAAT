-- 085: The room that was not on the timetable.
--
-- THE SITUATION. A lecture is timetabled into a room, and on the day that room is unusable — it is
-- double-booked, the projector is dead, the block is being repainted, an exam has taken it. The
-- lecture still has to happen, so the coordinator finds an empty room and runs it there.
--
-- Until now the system had no idea. sessions.venue_id simply held whatever room was named, with
-- nothing to say whether that was the planned one or a substitution made in the corridor five
-- minutes earlier. Three things follow from that silence, and all three are already happening:
--
--   · the QA monitor walks to the TIMETABLED room, finds it empty, and files "not taught" against a
--     lecturer who was teaching thirty metres away. The monitor is not wrong — they recorded what
--     they saw — and the lecturer has no way to answer it
--   · nobody can tell how often rooms are being substituted, so a room that is unusable every week
--     looks like a series of unrelated incidents rather than one repair nobody has done
--   · the estate has no record of what was actually used, only of what was planned
--
-- WHAT THIS ADDS. Two columns that turn a silent substitution into a stated one:
--
--   room_is_provision  This session ran in a room other than its timetabled one, chosen by the
--                      coordinator at the time. Stored rather than derived: a timetable edited
--                      later would otherwise rewrite history, and "was this a provision?" is a fact
--                      about the day, not about the current schedule.
--
--   provision_note     Why, in the coordinator's words. Optional, because a coordinator standing in
--                      a corridor with a class waiting should not be blocked from recording the
--                      room by a mandatory text box — but when it is filled in it is the only
--                      account anyone will ever have of the reason.
--
-- The monitor is notified the moment it is set, so the round is redirected before the visit rather
-- than corrected after it. That is the whole point: the record exists to stop a false "not taught",
-- and a notification that arrives after the monitor has already been is worth very little.

ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS room_is_provision BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS provision_note    TEXT;

COMMENT ON COLUMN sessions.room_is_provision IS
    'The session ran in a room other than its timetabled one. Set by the coordinator when they '
    'pick a free room; notifies the QA monitors so the round is redirected before the visit.';
COMMENT ON COLUMN sessions.provision_note IS
    'The coordinator''s reason for the substitution, in their own words. Optional by design.';

-- The estate view ("which rooms are being substituted, and how often") scans by date across the
-- tenant, and provisions are a small minority of sessions.
CREATE INDEX IF NOT EXISTS ix_sessions_provision
    ON sessions (tenant_id, session_date)
    WHERE room_is_provision;

-- Finding a FREE room asks the opposite question of every other query against this table: not
-- "when is this unit taught" but "what is in this room at this hour". Without a room-leading index
-- that is a scan of the whole institution's week, and the coordinator is asking it with a class
-- waiting outside.
CREATE INDEX IF NOT EXISTS ix_timetable_slots_room_day
    ON timetable_slots (tenant_id, venue_id, day_of_week, start_time)
    WHERE venue_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS ix_sessions_venue_date
    ON sessions (tenant_id, venue_id, session_date)
    WHERE venue_id IS NOT NULL;
