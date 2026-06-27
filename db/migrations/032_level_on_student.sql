-- 032 — A course is just a course; level (and course group) belong to the STUDENT.
--
-- Previously a `courses` row was a course-at-a-level (a "program") and carried
-- level + course_group. We now treat a course as a plain course and record the
-- student's level of study (and the course group, which depends on level) on the
-- student. The courses.level / courses.course_group columns are left in place
-- (nullable, no longer collected) so existing display joins keep working; new
-- courses simply leave them NULL.

ALTER TABLE students_extended
    ADD COLUMN IF NOT EXISTS level        VARCHAR(40),
    ADD COLUMN IF NOT EXISTS course_group VARCHAR(160);

-- Backfill each existing student's level/course_group from the course they are on
-- (via their offering's program first, else their direct course_id) so history is
-- preserved.
UPDATE students_extended se
SET level        = COALESCE(se.level, c.level),
    course_group = COALESCE(se.course_group, c.course_group)
FROM course_offerings o
JOIN courses c ON c.course_id = o.course_id AND c.tenant_id = o.tenant_id
WHERE se.offering_id = o.offering_id
  AND se.tenant_id   = o.tenant_id;

UPDATE students_extended se
SET level        = COALESCE(se.level, c.level),
    course_group = COALESCE(se.course_group, c.course_group)
FROM courses c
WHERE se.course_id = c.course_id
  AND se.tenant_id = c.tenant_id
  AND se.level IS NULL;
