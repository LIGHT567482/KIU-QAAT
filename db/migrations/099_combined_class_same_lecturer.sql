-- 099: One room, one time, ONE LECTURER — the combined class, allowed at last.
--
-- Migration 091 made it impossible for two units to occupy a room at the same hour. That was aimed
-- at a real fault: departmental TLCs schedule into a shared pool of rooms, and two of them could
-- each book Block C 101 for Tuesday 14:00 with nobody told until two lecturers and eighty students
-- arrived at the same door.
--
-- But it answered the question too broadly. KIU routinely teaches ONE lecture to several cohorts at
-- once — the same hour, the same room, the same lecturer — and those cohorts often carry the unit
-- under different codes and even different names ("Structured Programming" for one intake,
-- "Fundamentals of Programming" for another). That is one lecture, and 091 refused to let anybody
-- timetable it. The guard was blocking the institution's normal practice, not a mistake.
--
-- THE DISTINCTION THAT MATTERS IS THE LECTURER, NOT THE UNIT. Two units in one room at one hour is
-- a clash when two different people are meant to be teaching them, because only one of them can be.
-- It is not a clash when it is the same person: one lecturer, one room, one lecture, several
-- registers. So the constraint now conflicts only when the LECTURERS DIFFER.
--
-- `WITH <>` is what expresses that. An exclusion constraint fires when every listed operator
-- returns true for a pair of rows, so adding "and the lecturers are different" means two slots
-- sharing a room, day and time are rejected ONLY if they name different lecturers. btree_gist
-- supports <> for exactly this purpose.
--
-- WHY COALESCE AND NOT THE BARE COLUMN. A NULL never compares true to anything, so a NULL
-- lecturer_id would make the whole predicate NULL and the pair would silently never conflict —
-- turning "nobody assigned yet" into a hole that admits any double-booking at all. Unassigned
-- slots are folded onto one sentinel so they still collide with each other and with real
-- lecturers, which is the safe reading: a room booked twice with nobody named is exactly the
-- case 091 was written for.

CREATE EXTENSION IF NOT EXISTS btree_gist;

ALTER TABLE timetable_slots
    DROP CONSTRAINT IF EXISTS timetable_slots_no_room_double_booking;

ALTER TABLE timetable_slots
    ADD CONSTRAINT timetable_slots_no_room_double_booking
    EXCLUDE USING gist (
        tenant_id WITH =,
        btrim(lower(room)) WITH =,
        day_of_week WITH =,
        -- Minutes since midnight, half-open, so a class ending at 10:00 and one starting at 10:00
        -- do not overlap. GREATEST(...,1) keeps a zero/negative duration from making an empty
        -- range that could never conflict with anything.
        int4range(
            (EXTRACT(HOUR FROM start_time) * 60 + EXTRACT(MINUTE FROM start_time))::int,
            (EXTRACT(HOUR FROM start_time) * 60 + EXTRACT(MINUTE FROM start_time))::int
                + GREATEST(COALESCE(duration_minutes, 60), 1)
        ) WITH &&,
        -- The whole point of this migration: same lecturer, no clash.
        COALESCE(lecturer_id, '00000000-0000-0000-0000-000000000000'::uuid) WITH <>
    )
    WHERE (COALESCE(btrim(room), '') <> '');

COMMENT ON CONSTRAINT timetable_slots_no_room_double_booking ON timetable_slots IS
    'A room may hold two units at the same hour only when the SAME lecturer teaches both '
    '(the combined class). Different lecturers in one room at one time remain impossible.';
