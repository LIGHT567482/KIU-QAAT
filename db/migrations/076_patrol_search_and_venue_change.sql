-- 076: What the patroller actually found, and a key that can hold two cohorts.
--
-- WHERE THEY FOUND IT. Lecturers move rooms. A lecture timetabled in A02 Old Building happens in
-- B04 because A02 is double-booked, or is pushed an hour, or runs on a different day — informally,
-- with nobody told. Today a patrol tick records only a boolean against the timetabled slot, so the
-- room change is either invisible (the patroller ticks "taught" and the discrepancy is lost) or it
-- becomes a false accusation (nothing found in A02 → "not taught"). Both outcomes are wrong about
-- the thing QA exists to observe.
--
-- So a tick can now carry where, when and on what date the lecture was ACTUALLY found. Left NULL
-- when it matched the timetable, which is the common case and keeps the row small. `venue_changed`
-- is stored rather than derived because the comparison it summarises is fuzzy — free-text rooms,
-- an abbreviation against a full name — and the patroller standing in the doorway is a better
-- judge of "this is a different room" than a string comparison run later.
--
-- THE UNIQUE KEY WAS TOO NARROW. It was (tenant_id, unit_id, session_date, scheduled_time). Two
-- cohorts of the same unit at the same clock time — a morning and an evening intake, or two groups
-- split across rooms — collide on that key, and the ON CONFLICT means the second patroller's tick
-- silently OVERWRITES the first. One of the two lecturers then has no record, and nobody is told.
-- Adding the offering makes the key match what a session actually is.

-- ─── Where the lecture was really found ───────────────────────────────────────
ALTER TABLE lecturer_patrol_logs ADD COLUMN IF NOT EXISTS found_venue      TEXT;
ALTER TABLE lecturer_patrol_logs ADD COLUMN IF NOT EXISTS found_start_time TEXT;   -- "HH:MM"
ALTER TABLE lecturer_patrol_logs ADD COLUMN IF NOT EXISTS found_date       DATE;
-- Set by the patroller, not inferred: they are in the room and can tell a moved lecture from a
-- differently-spelled room name.
ALTER TABLE lecturer_patrol_logs ADD COLUMN IF NOT EXISTS venue_changed    BOOLEAN NOT NULL DEFAULT false;
-- Which cohort's session this tick belongs to. Nullable, because every existing row predates it
-- and there is no way to attribute them after the fact.
ALTER TABLE lecturer_patrol_logs ADD COLUMN IF NOT EXISTS offering_id      UUID;

CREATE INDEX IF NOT EXISTS idx_patrol_logs_changed
    ON lecturer_patrol_logs (tenant_id, session_date) WHERE venue_changed;

-- ─── Widen the uniqueness key ─────────────────────────────────────────────────
-- The old constraint's name depends on how the table was created (058 used an inline UNIQUE, which
-- Postgres names lecturer_patrol_logs_tenant_id_unit_id_session_date_sched_key). Drop by lookup so
-- this works on a hand-built database too, which is what the migrate package exists for.
DO $$
DECLARE con_name text;
BEGIN
    SELECT conname INTO con_name
      FROM pg_constraint
     WHERE conrelid = 'lecturer_patrol_logs'::regclass
       AND contype = 'u'
       AND pg_get_constraintdef(oid) LIKE '%unit_id%session_date%scheduled_time%'
     LIMIT 1;
    IF con_name IS NOT NULL THEN
        EXECUTE format('ALTER TABLE lecturer_patrol_logs DROP CONSTRAINT %I', con_name);
    END IF;
END $$;

-- COALESCE the nullable offering into a fixed sentinel: a UNIQUE index treats NULLs as distinct,
-- which would let the same slot be ticked repeatedly for pre-migration rows and defeat the
-- ON CONFLICT the sync relies on.
CREATE UNIQUE INDEX IF NOT EXISTS ux_patrol_logs_slot
    ON lecturer_patrol_logs (tenant_id, unit_id, session_date, scheduled_time,
                             COALESCE(offering_id, '00000000-0000-0000-0000-000000000000'::uuid));

-- ─── Make the patroller's search fast ─────────────────────────────────────────
-- The two searches are "by lecturer staff id" and "by unit code", both restricted to today. The
-- staff id lives on `lecturers`, the unit code on `course_units`, and both are reached from
-- timetable_slots — which 074 already indexed by (tenant, day, lecturer) and (tenant, unit).
-- What is missing is a case-insensitive lookup of the staff id itself, since a patroller types
-- "kiu/044" and the record holds "KIU/044".
CREATE INDEX IF NOT EXISTS idx_lecturers_staffid_lower
    ON lecturers (tenant_id, btrim(lower(staff_id)));
CREATE INDEX IF NOT EXISTS idx_course_units_id_lower
    ON course_units (tenant_id, btrim(lower(unit_id)));
