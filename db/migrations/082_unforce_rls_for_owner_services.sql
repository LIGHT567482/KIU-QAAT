-- 082: Undo migration 079's FORCE. It locked the institution out of its own system.
--
-- WHAT HAPPENED. 079 set FORCE ROW LEVEL SECURITY on every table. FORCE makes a table's policies
-- apply to its OWNER as well as to everyone else. The auth-service, the sync-receiver and the
-- gateway's admin handlers all connect as the owner (qaat / ADMIN_DB_URL), and on managed Postgres
-- — Render, RDS, Cloud SQL — that role is NOT a superuser. So every one of their queries began
-- being filtered by `tenant_id = current_setting('app.current_tenant')`, a setting those paths do
-- not set and cannot set:
--
--     SELECT COUNT(*) FROM users;   ->   0
--
-- Nobody could sign in. Not an error, not a permission denied — an empty result, which reads all
-- the way up the stack as "no such account".
--
-- WHY IT PASSED EVERY TEST. Locally the owner role IS a superuser, and superusers ignore RLS
-- entirely, FORCE included. 079 was therefore a no-op on the machine it was written and verified
-- on, and a lockout on the machine it was for. The CI assertion added alongside it made this
-- worse rather than better: it demanded relforcerowsecurity on every table, so the mistake became
-- the thing the build enforced.
--
-- Migration 056 had already written the rule down — "RLS: ENABLE (not FORCE) so owner-based
-- services keep working on managed Postgres" — and 079 overrode it without noticing.
--
-- WHAT IS AND IS NOT PROTECTED AFTERWARDS. This is the honest accounting, because "we turned a
-- security control off" deserves one:
--
--   · qaat_app — the data-plane role the API gateway serves tenant traffic with — is NOT the
--     owner, so every policy still applies to it in full. It is NOSUPERUSER and NOBYPASSRLS, and
--     tests/security asserts cross-tenant isolation as that role on every CI run. Nothing about
--     the isolation that faces the internet changes here.
--
--   · qaat — the owner — bypasses RLS again, as it did before 079 and as it must, since the
--     work it does (authenticate before a tenant is known, ingest a sealed package, run the
--     admin console) has no tenant context to be scoped by.
--
-- So the guarantee is what it always was: isolation is enforced on the role that handles tenant
-- requests, and the owner is trusted because it is the one that has to be. FORCE only ever
-- promised more than that on a database where the owner is not a superuser — and the moment it
-- delivered, it took the system down.

DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT c.relname::text AS tbl
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relkind = 'r' AND n.nspname = 'public' AND c.relforcerowsecurity
    LOOP
        EXECUTE format('ALTER TABLE public.%I NO FORCE ROW LEVEL SECURITY', r.tbl);
    END LOOP;
END $$;

-- ENABLE stays exactly as it was: the policies are still there and still bind qaat_app.
-- Re-asserted here so a database that never ran 079 ends up in the same state as one that did.
DO $$
DECLARE r record;
BEGIN
    FOR r IN
        SELECT c.relname::text AS tbl
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relkind = 'r' AND n.nspname = 'public'
          AND c.relname NOT IN ('tenants', 'schema_migrations')
          AND EXISTS (SELECT 1 FROM pg_policy p WHERE p.polrelid = c.oid)
    LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', r.tbl);
    END LOOP;
END $$;
