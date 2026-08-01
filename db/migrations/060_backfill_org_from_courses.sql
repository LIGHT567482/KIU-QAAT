-- 060: Backfill the managed schools/departments from existing free-text course.school/department,
-- and link existing courses to them. One-time + idempotent (safe to re-run). After this, existing
-- courses "inherit" a structured school/department instead of only free text.

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
