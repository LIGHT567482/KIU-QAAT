-- 021 — Coordinator identity & one-course rule (#4b)
--
-- Every coordinator is given a unique, auto-generated coordinator code, and a
-- coordinator may coordinate at most ONE course. The code distinguishes
-- coordinators; the partial unique index enforces the one-course rule at both
-- the create-course and assign-coordinator paths.

ALTER TABLE users ADD COLUMN IF NOT EXISTS coordinator_code VARCHAR(32);

-- A coordinator code is unique within a tenant.
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_tenant_coordinator_code
    ON users (tenant_id, coordinator_code)
    WHERE coordinator_code IS NOT NULL;

-- A coordinator (user) may be assigned to at most one course per tenant.
CREATE UNIQUE INDEX IF NOT EXISTS ux_courses_tenant_coordinator
    ON courses (tenant_id, coordinator_id)
    WHERE coordinator_id IS NOT NULL;

-- Backfill: give every existing COORDINATOR a stable code derived from its id.
UPDATE users
SET coordinator_code = 'CO-' || upper(substr(md5(user_id::text), 1, 6))
WHERE role = 'COORDINATOR' AND coordinator_code IS NULL;
