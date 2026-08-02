-- QAAT — Row-Level Security Policies
-- Migration: 003_rls_policies.sql
-- Run order: 3 of 4
--
-- Every query must SET LOCAL app.current_tenant = '<uuid>' before touching data.
-- The API Gateway middleware does this automatically from the JWT tenant_id claim.

-- ─── Enable RLS on all tenant-scoped tables ───────────────────────────────────

ALTER TABLE users                    ENABLE ROW LEVEL SECURITY;
ALTER TABLE venues                   ENABLE ROW LEVEL SECURITY;
ALTER TABLE courses                  ENABLE ROW LEVEL SECURITY;
ALTER TABLE course_units             ENABLE ROW LEVEL SECURITY;
ALTER TABLE students_extended        ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions                 ENABLE ROW LEVEL SECURITY;
ALTER TABLE attendance_logs          ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_attendance_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE hardware_vault           ENABLE ROW LEVEL SECURITY;
ALTER TABLE admin_audit_log          ENABLE ROW LEVEL SECURITY;
ALTER TABLE sync_uploads             ENABLE ROW LEVEL SECURITY;

-- ─── Helper: current tenant from session variable ─────────────────────────────

-- Tenants table itself has no RLS (superuser-managed).
-- All other tables use this policy pattern:

CREATE POLICY tenant_isolation ON users
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON venues
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON courses
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON course_units
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON students_extended
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON sessions
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON attendance_logs
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON lecturer_attendance_logs
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON hardware_vault
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON admin_audit_log
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

CREATE POLICY tenant_isolation ON sync_uploads
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

-- ─── Append-only guard on attendance_logs ────────────────────────────────────
-- Blocks DELETE at the policy level. UPDATE is also blocked.
-- Corrections must use entry_method = MANUAL_OVERRIDE (new INSERT).

CREATE POLICY no_delete_attendance ON attendance_logs
    AS RESTRICTIVE
    FOR DELETE USING (false);

CREATE POLICY no_update_attendance ON attendance_logs
    AS RESTRICTIVE
    FOR UPDATE USING (false);

-- ─── Application role (used by all services) ─────────────────────────────────
-- Create a limited-privilege role; superuser is never used by the app.

DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'qaat_app') THEN
        CREATE ROLE qaat_app LOGIN PASSWORD 'changeme_app';
    END IF;
END $$;

GRANT CONNECT ON DATABASE qaat TO qaat_app;
GRANT USAGE ON SCHEMA public TO qaat_app;
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO qaat_app;
GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO qaat_app;

-- Explicitly deny DELETE on attendance_logs even at the role level
REVOKE DELETE ON attendance_logs FROM qaat_app;
