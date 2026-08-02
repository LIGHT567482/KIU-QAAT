-- 064: Remove the SUPER_ADMIN platform-owner role entirely.
--
-- QAAT now runs as a single-institution deployment: the tenant already exists, and every
-- administrative action is performed by that institution's own ADMIN, confined to its own tenant.
-- The cross-tenant platform-owner role has no remaining purpose, and a dormant role that bypasses
-- `RequireOwnTenant` is a standing risk rather than a feature — so it is removed from the code and
-- from the database, not merely left unused.
--
-- Postgres cannot DROP a value from an enum, so the type is rebuilt. That is safe here because
-- `users.role` is the only column of this type (the two indexes on it are rebuilt automatically by
-- the column rewrite), and no function, view or default references it.

-- 1) Remove the accounts first — the type rewrite in step 3 fails while any row still holds the
--    value. The platform seed in migration 038 created exactly one; a deployment that never ran
--    that seed simply deletes nothing.
--
--    Compared as TEXT, not as the enum: once step 3 has run, the literal 'SUPER_ADMIN' is no
--    longer a valid value of the type, so `role = 'SUPER_ADMIN'` would raise
--    "invalid input value for enum" and make this migration fail on a second run. Casting the
--    column keeps it a plain string comparison and the whole file safely re-runnable.
DELETE FROM users WHERE role::text = 'SUPER_ADMIN';

-- 2) The sentinel platform tenant existed only to own that account. Migration 038 seeds it with a
--    fixed all-zero tenant_id and the domain 'platform.local'; it is matched on BOTH so a
--    deployment that was cleaned up by hand (scripts/prod-kiu-cleanup.sql) is still handled.
--    Guarded by "no users left" so a real institution can never be caught by this.
DELETE FROM tenants t
 WHERE (t.tenant_id = '00000000-0000-0000-0000-000000000000' OR t.domain = 'platform.local')
   AND NOT EXISTS (SELECT 1 FROM users u WHERE u.tenant_id = t.tenant_id);

-- 3) Rebuild user_role_enum without SUPER_ADMIN.
--    Guarded so the migration is safe to re-run: if the label is already gone, do nothing.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_enum e
        JOIN pg_type t ON t.oid = e.enumtypid
        WHERE t.typname = 'user_role_enum' AND e.enumlabel = 'SUPER_ADMIN'
    ) THEN
        ALTER TYPE user_role_enum RENAME TO user_role_enum_old;

        CREATE TYPE user_role_enum AS ENUM (
            'COORDINATOR', 'QA_OFFICER', 'DQA_DIRECTOR', 'VC', 'DVC', 'ADMIN',
            'STUDENT', 'LECTURER', 'QA_PATROLLER',
            'HOD', 'DEAN', 'QA_SCHOOL_HANDLER', 'QA_DEPT_REP'
        );

        -- Rewrites the column and every index over it. No default exists on users.role, so there
        -- is none to drop and restore.
        ALTER TABLE users
            ALTER COLUMN role TYPE user_role_enum USING role::text::user_role_enum;

        DROP TYPE user_role_enum_old;
    END IF;
END $$;
