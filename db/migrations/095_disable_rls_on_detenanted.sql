-- 095: Turn RLS OFF on every table that no longer has a tenant_id.
--
-- A BUG INTRODUCED BY 092–094, AND A SILENT ONE. Dropping tenant_id CASCADE takes the
-- tenant_isolation POLICY with it — but it leaves `ALTER TABLE ... ENABLE ROW LEVEL SECURITY`
-- switched on. A table with RLS enabled and NO policy does not fall back to permitting everything.
-- It denies EVERYTHING to any role that is not the owner or a superuser, which is exactly what
-- qaat_app is.
--
-- Measured, not assumed: after 093, `lecturer_daily_codes` held one row as the owner and ZERO as
-- qaat_app; notification_log held three and returned zero. Nothing errored. The reads simply came
-- back empty, and the writes would have been refused just as quietly.
--
-- What that costs in this system:
--   lecturer_daily_codes  — the shared-lecture code is looked up, not found, regenerated, inserted
--                           (refused), looked up again, not found... the coordinator in the second
--                           room can never be given a code that works.
--   notification_log      — its ONLY job is idempotency. Deny the read and every scheduled
--                           reminder looks unsent, so it sends again on every sweep.
--
-- The endpoints smoke-tested after 093 all passed because they run on adminPool, which connects as
-- the owner and is not subject to RLS at all. The RLS pool is where this bites, and nothing in the
-- test set touched these tables through it.
--
-- THE FIX IS TO DISABLE, NOT TO ADD A POLICY BACK. With no tenant_id there is nothing to isolate:
-- one institution, one set of rows, and the column the policy compared against no longer exists.
--
-- FOR EVERY LATER STAGE: dropping tenant_id from a table must be followed by disabling RLS on it in
-- the same migration. 096 onwards do this inline; this one catches up the eleven already dropped.

ALTER TABLE employee_attendance_logs      DISABLE ROW LEVEL SECURITY;
ALTER TABLE hardware_vault                DISABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_biometric_templates  DISABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_daily_codes          DISABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_webauthn_credentials DISABLE ROW LEVEL SECURITY;
ALTER TABLE monitor_log_units             DISABLE ROW LEVEL SECURITY;
ALTER TABLE notification_log              DISABLE ROW LEVEL SECURITY;
ALTER TABLE patroller_pins                DISABLE ROW LEVEL SECURITY;
ALTER TABLE semester_archives             DISABLE ROW LEVEL SECURITY;
ALTER TABLE student_device_bindings       DISABLE ROW LEVEL SECURITY;
ALTER TABLE sync_uploads                  DISABLE ROW LEVEL SECURITY;

-- scheduled_job_runs was already in this state BEFORE any of this work — RLS enabled, no policy,
-- so the scheduler's own bookkeeping table has been invisible to qaat_app for as long as it has
-- existed. It is the same defect and it is fixed here rather than left because it predates us.
ALTER TABLE scheduled_job_runs            DISABLE ROW LEVEL SECURITY;
