-- 066: Support departments that belong to NO school.
--
-- Finance, Admissions, Bursary, Library, ICT, Estates, Security… are institution-wide units. They
-- report to administration, not to a faculty. Migration 059 required every department to sit under
-- a school and suggested parking these under a synthetic "Support Services" school — a fiction that
-- shows up in every school list and in the dean/school dashboards as a school nobody runs.
--
-- `school_id` becomes nullable instead: a SUPPORT department with no school is now a first-class
-- record. ACADEMIC departments are unaffected and still require their school.

ALTER TABLE departments ALTER COLUMN school_id DROP NOT NULL;

-- The original UNIQUE (tenant_id, school_id, name) does not constrain standalone departments,
-- because in SQL every NULL is distinct — "Finance" could be inserted a hundred times. Two partial
-- indexes cover both shapes: one per (school, name) as before, one per name when there is no school.
CREATE UNIQUE INDEX IF NOT EXISTS ux_departments_standalone_name
    ON departments (tenant_id, name)
    WHERE school_id IS NULL;

-- Any support department already parked under a "Support Services"-style placeholder school is
-- detached, and the placeholder school is dropped once nothing else references it. Deliberately
-- narrow: only SUPPORT rows, and only schools whose name marks them as the synthetic bucket.
UPDATE departments d
SET school_id = NULL
FROM schools s
WHERE d.school_id = s.school_id
  AND d.kind = 'SUPPORT'
  AND btrim(lower(s.name)) IN ('support services', 'support', 'administration', 'support departments');

DELETE FROM schools s
WHERE btrim(lower(s.name)) IN ('support services', 'support', 'administration', 'support departments')
  AND NOT EXISTS (SELECT 1 FROM departments d WHERE d.school_id = s.school_id)
  AND NOT EXISTS (SELECT 1 FROM courses     c WHERE c.school_id = s.school_id);
