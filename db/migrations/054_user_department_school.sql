-- Oversight staff (esp. QA officers, who act as ambassadors under a school/department)
-- carry a department and an optional college/school, so the DQA can target reports and
-- notifications by department or by school/college. Free text (mirrors courses.department
-- / courses.school). NULL for roles where it doesn't apply.
ALTER TABLE users ADD COLUMN IF NOT EXISTS department VARCHAR(120);
ALTER TABLE users ADD COLUMN IF NOT EXISTS school     VARCHAR(120);

-- Helps the DQA→QA targeting queries (WHERE role='QA_OFFICER' AND department=…/school=…).
CREATE INDEX IF NOT EXISTS idx_users_dept_school ON users (tenant_id, role, department, school);
