-- ─── RLS ISOLATION FIXTURE ───────────────────────────────────────────────────
--
-- Two tenants with data in every tenant-scoped table the suite reads, so the isolation assertions
-- have something to actually isolate.
--
-- THIS LIVES HERE, NOT IN db/seeds. KIU runs as a single institution and the product must never
-- ship demo tenants — that is why db/seeds/*.sql were retired. But proving that row-level security
-- keeps tenants apart requires at least two of them, so the second tenant is a TEST FIXTURE and
-- belongs beside the test that needs it, where nobody can mistake it for seed data.
--
-- The IDs below are the ones rls_isolation_test.go declares as `tenantA` and `tenantB`. That is
-- the whole point of moving the file: CI used to load db/seeds/001_test_tenants.sql, which created
-- tenants a1000000-… and a2000000-…, while the suite queried a0000000-… and b0000000-…. The
-- constants never matched the data. Every assertion counted rows belonging to a tenant that did
-- not exist, found zero, and passed — a green gate over an untested claim. countCross() returning
-- 0 proves nothing when the tenant it is counting has no rows in the first place.
--
-- So each table below gets rows for BOTH tenants. If RLS were switched off tomorrow, the wildcard
-- test would see tenant B's rows in tenant A's session and fail — which is the only version of
-- this suite worth running.

INSERT INTO tenants (tenant_id, name, domain, rsa_key_id, attendance_threshold)
VALUES
  ('a0000000-0000-0000-0000-000000000001', 'Tenant A (test)', 'a.test', 'a-rsa-key-v1', 75),
  ('b0000000-0000-0000-0000-000000000002', 'Tenant B (test)', 'b.test', 'b-rsa-key-v1', 75)
ON CONFLICT (tenant_id) DO NOTHING;

INSERT INTO users (tenant_id, email, password_hash, role, full_name)
VALUES
  ('a0000000-0000-0000-0000-000000000001', 'a.admin@a.test', 'x', 'ADMIN',   'A Admin'),
  ('a0000000-0000-0000-0000-000000000001', 'a.qa@a.test',    'x', 'QA_OFFICER', 'A QA'),
  ('b0000000-0000-0000-0000-000000000002', 'b.admin@b.test', 'x', 'ADMIN',   'B Admin'),
  ('b0000000-0000-0000-0000-000000000002', 'b.qa@b.test',    'x', 'QA_OFFICER', 'B QA')
ON CONFLICT (tenant_id, email) DO NOTHING;

INSERT INTO courses (course_id, tenant_id, name)
VALUES
  ('A-CRS-1', 'a0000000-0000-0000-0000-000000000001', 'A Course'),
  ('B-CRS-1', 'b0000000-0000-0000-0000-000000000002', 'B Course')
ON CONFLICT DO NOTHING;

INSERT INTO course_units (unit_id, tenant_id, course_id, name)
VALUES
  ('A-UNIT-1', 'a0000000-0000-0000-0000-000000000001', 'A-CRS-1', 'A Unit'),
  ('B-UNIT-1', 'b0000000-0000-0000-0000-000000000002', 'B-CRS-1', 'B Unit')
ON CONFLICT DO NOTHING;

INSERT INTO venues (venue_id, tenant_id, name)
VALUES
  ('A-VEN-1', 'a0000000-0000-0000-0000-000000000001', 'A Hall'),
  ('B-VEN-1', 'b0000000-0000-0000-0000-000000000002', 'B Hall')
ON CONFLICT DO NOTHING;

INSERT INTO students_extended (student_id, tenant_id, full_name, email, course_id, academic_year)
VALUES
  ('A-STU-1', 'a0000000-0000-0000-0000-000000000001', 'A Student', 'a.stu@a.test', 'A-CRS-1', '2025/2026'),
  ('B-STU-1', 'b0000000-0000-0000-0000-000000000002', 'B Student', 'b.stu@b.test', 'B-CRS-1', '2025/2026')
ON CONFLICT DO NOTHING;

-- session_id is generated; captured so the attendance rows below can point at the right session
-- for the right tenant. A cross-tenant attendance row would be meaningless as a fixture.
WITH s AS (
  INSERT INTO sessions (tenant_id, coordinator_id, unit_id, session_date)
  VALUES
    ('a0000000-0000-0000-0000-000000000001', 'A-COORD', 'A-UNIT-1', CURRENT_DATE),
    ('b0000000-0000-0000-0000-000000000002', 'B-COORD', 'B-UNIT-1', CURRENT_DATE)
  RETURNING session_id, tenant_id
)
INSERT INTO attendance_logs (tenant_id, session_id, student_id, checkin_timestamp, sequence_number)
SELECT s.tenant_id, s.session_id,
       CASE WHEN s.tenant_id = 'a0000000-0000-0000-0000-000000000001' THEN 'A-STU-1' ELSE 'B-STU-1' END,
       now(), 1
FROM s;
