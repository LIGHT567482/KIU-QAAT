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

  -- Sign-ins, if the run needed real accounts to storm an authenticated route.
  DELETE FROM users WHERE registration_number LIKE 'LOAD-%' OR email LIKE '%@loadtest.invalid';
  GET DIAGNOSTICS v_removed = ROW_COUNT;
  RAISE NOTICE 'removed % load sign-ins', v_removed;

  -- Anything a storm RAN against a load unit. Without these the teardown stopped on
  -- sessions_unit_id_fkey the first time someone stormed the check-in path rather than a
  -- read-only one, and left the whole cohort in the database.
  DELETE FROM attendance_logs          WHERE session_id IN (SELECT session_id FROM sessions WHERE unit_id LIKE 'LOAD-UNIT-%');
  DELETE FROM lecturer_attendance_logs WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM lecturer_patrol_logs     WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM lecturer_presence_claims WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM lecturer_assignments     WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM offering_unit_schedules  WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM sessions                 WHERE unit_id LIKE 'LOAD-UNIT-%';
  GET DIAGNOSTICS v_removed = ROW_COUNT;
  RAISE NOTICE 'removed % sessions run against load units', v_removed;

  DELETE FROM timetable_slots  WHERE offering_id = v_offering;
  DELETE FROM course_offerings WHERE offering_id = v_offering;
  DELETE FROM course_units     WHERE unit_id LIKE 'LOAD-UNIT-%';
  DELETE FROM courses          WHERE course_id = 'LOAD-COURSE';
  RAISE NOTICE 'load cohort removed';
END $$;
