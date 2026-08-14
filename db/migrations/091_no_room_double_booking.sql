-- 091: One room, one class, one time — enforced by the database.
--
-- THE PROBLEM. The timetable is not written by one person. Each department has its own TLC, and
-- each of them schedules their own units into the same finite pool of lecture rooms. Nothing stopped
-- two of them putting two different cohorts in the same room at the same hour, because the only
-- clash guard that existed asked a narrower question:
--
--     WHERE tenant_id = $1 AND offering_id = $2 AND day_of_week = $3 ...
--
-- That is "does this cohort already have a lecture then" — a check about the STUDENTS' diary. It is
-- a real check and it stays. But it is scoped to one offering, so a TLC in Computing and a TLC in
-- Business could each book Block C 101 for Tuesday at 14:00 and neither would be told. The first
-- anybody learns of it is two lecturers and eighty students arriving at the same door.
--
-- WHY A DATABASE CONSTRAINT AND NOT ONLY A HANDLER CHECK. The handler check (timetable_slots.go)
-- is what produces a readable message, and it runs first. But it is a read followed by a write, and
-- two TLCs pressing Save in the same second both read "free" before either writes — the same
-- read-modify-write race the attendance ledger has, with the same fix. Only the database can make
-- the overlap impossible rather than unlikely. An EXCLUSION constraint is exactly that: it is a
-- unique index that compares ranges with && (overlaps) instead of = (equals).
--
-- WHAT COUNTS AS "THE SAME ROOM". The room is free text a TLC types, so "Block C 101", "block c
-- 101" and " Block C 101 " are one room and must collide. The constraint therefore keys on
-- btrim(lower(room)). Slots with NO room named are exempt entirely — an unroomed slot books nothing
-- and must not collide with another unroomed slot, which is why the WHERE clause is not optional.
--
-- WHAT IT DELIBERATELY DOES NOT DO. It does not compare venue_id. A slot can carry a structured
-- venue_id resolved from the room registry, but only when the typed text happened to match a
-- managed room — so venue_id is NULL on plenty of real slots, and a constraint keyed on it would
-- silently stop protecting exactly the rooms nobody has registered yet. The text is what every slot
-- has, so the text is what is enforced.

CREATE EXTENSION IF NOT EXISTS btree_gist;

-- btree_gist is what lets the equality columns (tenant, room, weekday) sit in a GiST index
-- alongside the range column. Without it PostgreSQL cannot build this index at all.

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'timetable_slots_no_room_double_booking'
    ) THEN
        ALTER TABLE timetable_slots
            ADD CONSTRAINT timetable_slots_no_room_double_booking
            EXCLUDE USING gist (
                tenant_id WITH =,
                btrim(lower(room)) WITH =,
                day_of_week WITH =,
                -- The lecture as minutes-since-midnight, half-open so a class ending at 10:00 and
                -- one starting at 10:00 do NOT overlap. int4range is used rather than a range over
                -- `time` because PostgreSQL ships no range type for time-of-day, and defining one
                -- would put a custom type in the way of every future migration.
                int4range(
                    (date_part('hour', start_time) * 60 + date_part('minute', start_time))::int,
                    (date_part('hour', start_time) * 60 + date_part('minute', start_time))::int
                        + GREATEST(COALESCE(duration_minutes, 60), 1)
                ) WITH &&
            )
            WHERE (room IS NOT NULL AND btrim(room) <> '');
    END IF;
END $$;

COMMENT ON CONSTRAINT timetable_slots_no_room_double_booking ON timetable_slots IS
    'One room cannot hold two lectures at overlapping times on the same weekday, across every '
    'department and college in the institution. Rooms are matched case- and whitespace-insensitively '
    'on the typed text; slots with no room named are exempt. The readable error comes from the '
    'handler''s own check — this is the backstop that survives two TLCs saving at the same instant.';

-- NOTE ON EXISTING DATA. If an institution already holds overlapping bookings, ADD CONSTRAINT will
-- fail and this migration will stop — deliberately. There is no correct way for a migration to pick
-- which of two real bookings to discard, and silently dropping one would move a visible clash into
-- an invisible data loss. Find them with the query below, fix them on the timetable, then re-run:
--
--   SELECT a.slot_id, b.slot_id, a.room, a.day_of_week,
--          to_char(a.start_time,'HH24:MI'), to_char(b.start_time,'HH24:MI')
--   FROM timetable_slots a
--   JOIN timetable_slots b
--     ON b.tenant_id = a.tenant_id
--    AND b.slot_id <> a.slot_id
--    AND b.day_of_week = a.day_of_week
--    AND btrim(lower(b.room)) = btrim(lower(a.room))
--    AND a.start_time < (b.start_time + make_interval(mins => b.duration_minutes))
--    AND b.start_time < (a.start_time + make_interval(mins => a.duration_minutes))
--   WHERE COALESCE(btrim(a.room),'') <> '';
