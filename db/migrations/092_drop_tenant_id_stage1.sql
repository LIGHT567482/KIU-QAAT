-- 092: Remove tenant_id — stage 1.
--
-- QAAT serves ONE institution. The tenant_id column on 41 tables, and the row-level security built
-- on it, exist to keep institutions apart in a database that only ever holds one. Everything a user
-- could see of that is already gone (no tenant in any URL, one row in `tenants`, no picker); this is
-- the removal of the machinery underneath.
--
-- WHY THIS IS STAGED, one table per step rather than one sweeping migration. Go code filters on
-- tenant_id in 405 predicates and 312 join conditions, and 302 of those bind the tenant as $1 — so
-- removing it renumbers every other placeholder in the query. Dropping a column out from under code
-- that still names it turns every affected endpoint into a 500, and doing all 41 at once means the
-- system is unrunnable until the last one lands. Each stage here drops only tables whose Go queries
-- have already been rewritten, so the system works after every step.
--
-- THE PART THAT IS EASY TO GET WRONG: unique indexes.
--
-- Twenty-three unique indexes across fifteen tables include tenant_id. `DROP COLUMN ... CASCADE`
-- removes them silently along with the column, and the result is not a smaller index — it is NO
-- index, and duplicate emails, duplicate staff ids and duplicate attendance rows all become legal.
-- So every one is RECREATED here on its remaining columns. With one institution that preserves the
-- exact semantics it had: UNIQUE(tenant_id, email) constrained one address per institution, and
-- UNIQUE(email) constrains one address, full stop.
--
-- `tenants.tenant_id` itself STAYS. It is that table's own primary key, and the row carries the
-- institution's settings — thresholds, academic period, branding, and student_hash_key, which is the
-- HMAC key the offline coordinator app derives student hashes from. Losing it would lock every
-- already-issued student QR out of the system.


-- ── employee_attendance_logs ────────────────────────────────────────────────
-- Go rewritten in: employee_attendance.go (insert + range read), noshow.go (the
-- NOT EXISTS correlation). Its ON CONFLICT target changes with the index below.
ALTER TABLE employee_attendance_logs DROP COLUMN IF EXISTS tenant_id CASCADE;

-- Recreated without tenant_id. The old name is gone with the column, so this takes a name that
-- says what it now guarantees: one punch per member of staff per timestamp per direction.
CREATE UNIQUE INDEX IF NOT EXISTS uq_employee_punch
    ON employee_attendance_logs (staff_id, event_time, event_type);

