-- 074: Make timetable_slots the timetable, and stop the surfaces disagreeing about it.
--
-- THE COMPLAINT. "The coordinator dashboard says there are no lecturers for that day." It is
-- true, and it is not one bug.
--
-- THREE SCHEDULE STORES. A unit's day and time can live in `course_units.session_start`
-- (migration 020), in `offering_unit_schedules` (024), or in `timetable_slots` (041). Nothing
-- keeps them in step. The admin web grid writes ONLY timetable_slots. The coordinator's phone
-- reads ONLY offering_unit_schedules, through manifest.go. So an institution that built its
-- timetable in the grid — which is the one the UI offers — leaves offering_unit_schedules empty,
-- the manifest returns day_of_week = 0 for every unit, and the app correctly concludes that
-- nothing is timetabled today. Meanwhile the patroller and the student, who read timetable_slots
-- directly, can see the very lectures the coordinator is being told do not exist.
--
-- A LECTURER THAT ERASES ITSELF. timetable_slots.lecturer_id is nullable, and the grid's slot
-- editor never sent one. The upsert did `lecturer_id = EXCLUDED.lecturer_id`, so every re-save of
-- a slot wrote NULL over whatever the CSV import had put there. Every lecturer name on every
-- surface then fell through to a lecturer_assignments lookup, and any unit without an assignment
-- row rendered blank. (The handler side of this is fixed in timetable_slots.go; this migration
-- repairs the rows that have already been blanked.)
--
-- WHAT THIS DOES. Backfills in BOTH directions so no surface loses data while the code is being
-- repointed, then leaves timetable_slots as the source of truth. Deliberately additive: nothing
-- is dropped, because a tenant mid-deploy must degrade to today's behaviour rather than to an
-- empty week.

-- ─── 1. timetable_slots ← offering_unit_schedules ─────────────────────────────
-- An institution that used the older set-once schedule has rows here and nothing in the grid.
-- Give them a real slot so the grid, the student portal and the patrol manifest can see it.
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes)
SELECT ous.tenant_id, ous.offering_id, ous.unit_id, ous.day_of_week, ous.session_start,
       COALESCE(NULLIF(ous.session_duration_minutes, 0), 60)
  FROM offering_unit_schedules ous
 WHERE ous.day_of_week BETWEEN 1 AND 7
   AND ous.session_start IS NOT NULL
ON CONFLICT (offering_id, unit_id, day_of_week, start_time) DO NOTHING;

-- ─── 2. offering_unit_schedules ← timetable_slots ─────────────────────────────
-- The other direction, and the one that actually fixes the reported bug for existing data:
-- manifest.go still consults this table as a fallback, so filling it means a coordinator whose
-- app has not been updated yet stops seeing an empty day.
--
-- A unit can legitimately have several slots a week; the set-once table holds one. Take the
-- earliest in the week, which is the best single answer that table can represent.
INSERT INTO offering_unit_schedules (offering_id, unit_id, tenant_id, day_of_week, session_start, session_duration_minutes)
SELECT DISTINCT ON (ts.offering_id, ts.unit_id)
       ts.offering_id, ts.unit_id, ts.tenant_id, ts.day_of_week, ts.start_time, ts.duration_minutes
  FROM timetable_slots ts
 ORDER BY ts.offering_id, ts.unit_id, ts.day_of_week, ts.start_time
ON CONFLICT (offering_id, unit_id) DO UPDATE
   SET day_of_week             = COALESCE(offering_unit_schedules.day_of_week, EXCLUDED.day_of_week),
       session_start           = COALESCE(offering_unit_schedules.session_start, EXCLUDED.session_start),
       session_duration_minutes = COALESCE(NULLIF(offering_unit_schedules.session_duration_minutes, 0),
                                           EXCLUDED.session_duration_minutes);

-- ─── 3. Repair the erased lecturers ───────────────────────────────────────────
-- Every slot whose lecturer was NULLed by a re-save gets it back from the assignment that the
-- fallback lookup was already using. Only fills blanks; an explicitly-set slot lecturer wins.
UPDATE timetable_slots ts
   SET lecturer_id = la.lecturer_id
  FROM lecturer_assignments la
 WHERE ts.lecturer_id IS NULL
   AND la.unit_id   = ts.unit_id
   AND la.tenant_id = ts.tenant_id
   AND la.lecturer_id = (
        SELECT l2.lecturer_id FROM lecturer_assignments l2
         WHERE l2.unit_id = ts.unit_id AND l2.tenant_id = ts.tenant_id
         ORDER BY l2.academic_year DESC, l2.created_at
         LIMIT 1);

-- ─── 4. Fill the unit-level default venue from the timetable ──────────────────
-- The manifest reads course_units.default_venue_id for the room while the grid writes
-- timetable_slots.venue_id — two different rooms for the same lecture. Seed the blank one.
UPDATE course_units cu
   SET default_venue_id = ts.venue_id
  FROM timetable_slots ts
 WHERE cu.default_venue_id IS NULL
   AND ts.venue_id IS NOT NULL
   AND ts.unit_id   = cu.unit_id
   AND ts.tenant_id = cu.tenant_id;

-- ─── 5. Index for the "today, for this lecturer" lookups ──────────────────────
-- The patroller's search and the lecture-reminder job both ask "which slots run on day N for
-- lecturer X", which today is a sequential scan of the tenant's whole week.
CREATE INDEX IF NOT EXISTS idx_timetable_slots_day_lecturer
    ON timetable_slots (tenant_id, day_of_week, lecturer_id);
CREATE INDEX IF NOT EXISTS idx_timetable_slots_unit
    ON timetable_slots (tenant_id, unit_id);
