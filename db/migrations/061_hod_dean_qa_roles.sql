-- 061: Phase 2 org roles — HOD, Dean, QA school handler, QA department rep.
--
-- HOD oversees ONE department, Dean oversees ONE school/college, a QA department rep sits in a
-- department (academic OR support), a QA school handler oversees a school. Their scope is the
-- department/school already carried on their user account (users.department / users.school, added
-- in 054). A lecturer's department/school is derived from the courses whose units they teach
-- (lecturer_assignments → course_units → courses.department/school), so no lecturer column is needed.

ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'HOD';
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'DEAN';
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'QA_SCHOOL_HANDLER';
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'QA_DEPT_REP';
