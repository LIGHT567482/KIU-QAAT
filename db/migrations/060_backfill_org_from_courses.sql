-- 060: Backfill the managed schools/departments from existing free-text course.school/department,
-- and link existing courses to them. One-time + idempotent (safe to re-run).
--
-- FIX: migration 059 set FORCE ROW LEVEL SECURITY on schools/departments, which blocks even the
-- table OWNER (the connection running these migrations) from inserting — and would also block the
-- admin dashboard's owner/superuser reads. courses/venues are ENABLE-only in practice, so match them:
-- drop FORCE here (ENABLE stays, so the non-owner qaat_app data plane is still tenant-isolated).
ALTER TABLE schools     NO FORCE ROW LEVEL SECURITY;
ALTER TABLE departments NO FORCE ROW LEVEL SECURITY;

-- 1) Seed schools from every distinct non-empty course.school.
INSERT INTO schools (tenant_id, name)
SELECT DISTINCT c.tenant_id, btrim(c.school)
FROM courses c
WHERE COALESCE(btrim(c.school), '') <> ''
ON CONFLICT (tenant_id, name) DO NOTHING;

-- 2) Seed departments from every distinct (school, department), attached to the matching school.
INSERT INTO departments (tenant_id, school_id, name)
SELECT DISTINCT c.tenant_id, s.school_id, btrim(c.department)
FROM courses c
JOIN schools s ON s.tenant_id = c.tenant_id AND s.name = btrim(c.school)
WHERE COALESCE(btrim(c.department), '') <> '' AND COALESCE(btrim(c.school), '') <> ''
ON CONFLICT (tenant_id, school_id, name) DO NOTHING;

-- 3) Link courses → school_id by name (only where not already linked).
UPDATE courses c
SET school_id = s.school_id
FROM schools s
WHERE c.school_id IS NULL
  AND s.tenant_id = c.tenant_id
  AND s.name = btrim(c.school)
  AND COALESCE(btrim(c.school), '') <> '';

-- 4) Link courses → department_id by (school_id, name).
UPDATE courses c
SET department_id = d.department_id
FROM departments d
WHERE c.department_id IS NULL
  AND d.tenant_id = c.tenant_id
  AND d.school_id = c.school_id
  AND d.name = btrim(c.department)
  AND COALESCE(btrim(c.department), '') <> '';
