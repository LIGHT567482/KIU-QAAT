-- Remove everything seed_load_cohort.sql created, and nothing else.
--
-- Keyed on the LOAD- prefix and the one load offering, in foreign-key order. Deliberately not
-- a CASCADE from the tenant: a load cohort seeded into a live institution has to leave without
-- taking anything real with it.
--
--   docker exec -i infra-postgres-1 psql -U qaat -d qaat -f - < teardown_load_cohort.sql

\set ON_ERROR_STOP on

DO $$
DECLARE
  v_offering uuid := 'dddddddd-0000-4000-8000-0000000d0ad0'::uuid;
  v_removed  int;
BEGIN
  DELETE FROM student_attendance_summary WHERE student_id LIKE 'LOAD-%';
  GET DIAGNOSTICS v_removed = ROW_COUNT;
  RAISE NOTICE 'removed % attendance summary rows', v_removed;

  DELETE FROM students_extended WHERE student_id LIKE 'LOAD-%';
  GET DIAGNOSTICS v_removed = ROW_COUNT;
  RAISE NOTICE 'removed % students', v_removed;

  DELETE FROM timetable_slots  WHERE offering_id = v_offering;
  DELETE FROM course_offerings WHERE offering_id = v_offering;
  DELETE FROM course_units     WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM courses          WHERE course_id = 'LOAD-COURSE';
  RAISE NOTICE 'load cohort removed';
END $$;
