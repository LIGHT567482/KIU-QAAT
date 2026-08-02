-- QAAT — Enforce Row-Level Security at runtime
-- Migration: 009_force_rls_and_app_role.sql
-- Run order: 9 of N
--
-- Closes update.md C1 (the critical finding): the application used to connect as
-- the `qaat` SUPERUSER/owner, and PostgreSQL superusers + table owners BYPASS RLS
-- unless the table is set to FORCE ROW LEVEL SECURITY. As a result the
-- well-formed tenant_isolation policies were inert at runtime: a Tenant-A token
-- could read AND write Tenant-B rows, and the append-only/immutable guards were
-- void. This was proven empirically.
--
-- This migration:
--   1. Hardens the `qaat_app` role so it can never bypass RLS.
--   2. Forces RLS on every tenant-scoped table (defence-in-depth, so even an
--      accidental owner connection is constrained).
--   3. (Re)grants the privileges qaat_app needs on objects created after 003
--      (tenant_rsa_keys/005, student_attendance_summary/007, checkin_secret/008),
--      and re-revokes DELETE/UPDATE on the append-only tables.
--
-- The application services (api-gateway data plane, qr-generator, session-manager)
-- must connect as `qaat_app`. auth-service and sync-receiver intentionally remain
-- on a privileged connection: they are identity/sync authorities that derive and
-- scope the tenant themselves (auth by globally-unique user_id / explicit tenant
-- filter; sync from the verified JWT claim). See update.md §"fixes applied".

-- ─── 1. Make qaat_app incapable of bypassing RLS ──────────────────────────────
-- (CREATE ROLE in 003 already makes it a plain LOGIN role; be explicit and
--  idempotent here in case the role pre-exists with other attributes.)
DO $$
BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'qaat_app') THEN
        CREATE ROLE qaat_app LOGIN PASSWORD 'changeme_app';
    END IF;
END $$;

ALTER ROLE qaat_app NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION;

-- ─── 2. FORCE RLS on every tenant-scoped table ────────────────────────────────
DO $$
DECLARE
    t text;
    tables text[] := ARRAY[
        'users', 'venues', 'courses', 'course_units',
        'students_extended', 'sessions', 'attendance_logs',
        'lecturer_attendance_logs', 'hardware_vault', 'admin_audit_log',
        'sync_uploads', 'tenant_rsa_keys', 'student_attendance_summary'
    ];
BEGIN
    FOREACH t IN ARRAY tables LOOP
        IF to_regclass('public.' || t) IS NOT NULL THEN
            EXECUTE format('ALTER TABLE %I ENABLE ROW LEVEL SECURITY', t);
            EXECUTE format('ALTER TABLE %I FORCE ROW LEVEL SECURITY', t);
        END IF;
    END LOOP;
END $$;

-- ─── 3. Grants for qaat_app on later-migration objects ────────────────────────
-- 003 granted only tables that existed at the time; re-run blanket grants so
-- tenant_rsa_keys (005), student_attendance_summary (007) and any new sequences
-- are covered. RLS still confines every row to the active app.current_tenant.
GRANT SELECT, INSERT, UPDATE ON ALL TABLES    IN SCHEMA public TO qaat_app;
GRANT USAGE, SELECT            ON ALL SEQUENCES IN SCHEMA public TO qaat_app;
GRANT EXECUTE                  ON ALL FUNCTIONS IN SCHEMA public TO qaat_app;

-- Future tables created by the owner also grant to qaat_app automatically.
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT SELECT, INSERT, UPDATE ON TABLES TO qaat_app;
ALTER DEFAULT PRIVILEGES IN SCHEMA public
    GRANT USAGE, SELECT ON SEQUENCES TO qaat_app;

-- Re-assert append-only / immutable guards at the privilege level (RESTRICTIVE
-- policies already block these, but revoking is belt-and-braces and also covers
-- any future owner-FORCE edge cases).
REVOKE DELETE         ON attendance_logs  FROM qaat_app;
REVOKE UPDATE, DELETE ON admin_audit_log  FROM qaat_app;

-- tenant_rsa_keys is written by qr-generator (key issuance) and read on check-in;
-- both run scoped to app.current_tenant, so SELECT/INSERT/UPDATE is sufficient.
-- No DELETE is needed (annual rotation sets revoked_at).
REVOKE DELETE ON tenant_rsa_keys FROM qaat_app;
