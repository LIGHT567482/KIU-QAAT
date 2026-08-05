-- Load-test cohort — thousands of students, one command to create, one to remove.
--
-- The stress harness (tests/load/storm) needs a population the size of a real institution's
-- morning: thousands of students who all open the app in the same minute. The seeded database has
-- five, and an endpoint measured against five rows has not been measured.
--
-- EVERYTHING here is prefixed LOAD- and hangs off one offering, so teardown_load_cohort.sql
-- removes it exactly and leaves real records untouched. Run as the OWNER role (qaat), never
-- qaat_app — RLS would hide these rows from the seeder itself.
--
--   docker exec -i infra-postgres-1 psql -U qaat -d qaat -v n=5000 -f - < seed_load_cohort.sql
--
-- :n  how many students (default 5000). They land in the oldest non-sentinel tenant, which is
--     the one the single-institution student endpoints resolve to — seeding anywhere else would
--     produce a cohort the endpoint under test cannot see.

\set ON_ERROR_STOP on
\if :{?n}
\else
  \set n 5000
\endif

-- Handed to the block through a session GUC rather than substituted into it: psql does not
-- interpolate its variables inside a dollar-quoted body, so `:n` would arrive as literal text.
SET load.n = :n;

DO $$
DECLARE
  v_tenant   uuid;
  v_course   varchar(50) := 'LOAD-COURSE';
  v_offering uuid := 'dddddddd-0000-4000-8000-0000000d0ad0'::uuid;
  v_unit     varchar(50);
  v_n        int := current_setting('load.n')::int;
BEGIN
  SELECT tenant_id INTO v_tenant FROM tenants
   WHERE tenant_id <> '00000000-0000-0000-0000-000000000000'
   ORDER BY created_at LIMIT 1;
  IF v_tenant IS NULL THEN RAISE EXCEPTION 'no tenant to seed into'; END IF;

  RAISE NOTICE 'seeding % load students into tenant %', v_n, v_tenant;

  INSERT INTO courses (course_id, tenant_id, name, school, department)
  VALUES (v_course, v_tenant, 'LOAD TEST COURSE', 'LOAD TEST', 'LOAD TEST')
  ON CONFLICT (course_id) DO NOTHING;

  INSERT INTO course_offerings (offering_id, tenant_id, course_id, session_type,
                                study_year, semester, level, intake)
  VALUES (v_offering, v_tenant, v_course, 'DAY', 1, 1, '', 'LOAD TEST COHORT')
  ON CONFLICT (offering_id) DO NOTHING;

  -- Three units, so a student's progress page has more than one row to assemble, and three
  -- weekly slots so the coordinator manifest's grid has something to carry.
  FOR i IN 1..3 LOOP
    v_unit := 'LOAD-UNIT-' || i;
    INSERT INTO course_units (unit_id, tenant_id, course_id, name, year, semester, academic_year)
    VALUES (v_unit, v_tenant, v_course, 'Load Test Unit ' || i, 1, 1, '2025/2026')
    ON CONFLICT (unit_id) DO NOTHING;

    INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time,
                                 duration_minutes, room)
    VALUES (v_tenant, v_offering, v_unit, ((i - 1) % 5) + 1, make_time(7 + i, 0, 0), 120,
            'LOAD-ROOM-' || i)
    ON CONFLICT (offering_id, unit_id, day_of_week, start_time) DO NOTHING;
  END LOOP;

  INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id,
                                 intake_session, current_year, semester, academic_year,
                                 enrollment_status, offering_id)
  SELECT 'LOAD-' || lpad(g::text, 6, '0'),
         v_tenant,
         'Load Student ' || g,
         'load' || g || '@loadtest.invalid',
         v_course,
         'LOAD TEST COHORT', 1, 1, '2025/2026', 'ACTIVE', v_offering
  FROM generate_series(1, v_n) g
  ON CONFLICT (student_id) DO NOTHING;

  -- Attendance history, so the progress endpoint assembles a real answer instead of an empty
  -- list. A read path benchmarked against no rows measures the router, not the query.
  INSERT INTO student_attendance_summary (student_id, unit_id, unit_name, course_id, tenant_id,
                                          sessions_held, sessions_attended, attendance_percentage)
  SELECT 'LOAD-' || lpad(g::text, 6, '0'), 'LOAD-UNIT-' || u, 'Load Test Unit ' || u,
         v_course, v_tenant, 20, 12 + (g % 9), round(((12 + (g % 9)) * 100.0) / 20, 2)
  FROM generate_series(1, v_n) g, generate_series(1, 3) u
  ON CONFLICT (tenant_id, student_id, unit_id) DO NOTHING;

  RAISE NOTICE 'seeded % students, 3 units, 3 timetable slots', v_n;
END $$;
