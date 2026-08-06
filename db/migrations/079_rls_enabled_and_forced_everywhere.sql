-- Row-level security ENABLED and FORCED on every tenant table, and a policy on every table that
-- gets forced.
--
-- Tables added by later migrations drifted: some got RLS enabled but never FORCED (so the table
-- owner — which the data plane connects as in some deployments — silently bypassed the very
-- isolation the policy describes), and two got neither, along with no policy at all.
--
-- THE ORDER BELOW IS THE WHOLE POINT. FORCE ROW LEVEL SECURITY on a table with no policy denies
-- every row to every non-superuser, permanently and silently: not an error, just nothing. Forcing
-- first and adding policies later would have taken `lecturer_daily_codes` away from qaat_app,
-- which is the data-plane role that reads it on the manifest path — the shared-lecturer code
-- would have stopped being issued, with no error anywhere to say why. So each table gets its
-- policy BEFORE it is forced, in the same transaction.
--
-- The loop is deliberate rather than a list. A hand-written list is how the drift happened in the
-- first place: every table added since is one somebody had to remember. Any future table carrying
-- a tenant_id now gets the standard isolation policy and both flags automatically.
--
-- NO EXPLICIT BEGIN/COMMIT. cmd/migrate already runs each migration inside its own transaction
-- and brackets it with a savepoint; a COMMIT in the file ends that transaction underneath the
-- tool, which then fails on `RELEASE SAVEPOINT can only be used in transaction blocks`. Worse
-- than the error: the COMMIT means the work IS applied while the tool reports "nothing from the
-- failing migration was applied" and leaves it out of the ledger. No other migration in this
-- directory opens its own transaction.

DO $$
DECLARE
    r            record;
    has_tenant   boolean;
    policy_count int;
BEGIN
    FOR r IN
        SELECT c.oid, c.relname::text AS tbl
        FROM pg_class c
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE c.relkind = 'r'
          AND n.nspname = 'public'
          -- tenants is the tenant REGISTRY, not tenant data: isolating it by
          -- current_setting('app.current_tenant') would hide the very row a connection needs to
          -- resolve before it can set that setting. It is excluded from the CI assertion too.
          AND c.relname <> 'tenants'
          -- The migration ledger. Applied-migration bookkeeping belongs to the operator running
          -- migrations as the owner, not to any tenant, and locking it would strand the migrator.
          AND c.relname <> 'schema_migrations'
    LOOP
        SELECT EXISTS (
            SELECT 1 FROM pg_attribute a
            WHERE a.attrelid = r.oid AND a.attname = 'tenant_id' AND a.attnum > 0 AND NOT a.attisdropped
        ) INTO has_tenant;

        SELECT count(*) INTO policy_count FROM pg_policy p WHERE p.polrelid = r.oid;

        -- A tenant table with no policy would be sealed shut by the FORCE below. Give it the
        -- same isolation rule every other tenant table already carries.
        IF has_tenant AND policy_count = 0 THEN
            EXECUTE format(
                'CREATE POLICY tenant_isolation ON public.%I
                   USING (tenant_id = (current_setting(''app.current_tenant'', true))::uuid)
                   WITH CHECK (tenant_id = (current_setting(''app.current_tenant'', true))::uuid)',
                r.tbl);
            RAISE NOTICE 'RLS: added tenant_isolation policy to %', r.tbl;
        END IF;

        -- A table with NO tenant_id and NO policy is infrastructure, not tenant data
        -- (scheduled_job_runs is the scheduler's own bookkeeping). Forcing it with no policy
        -- confines it to the superuser — which is already the only role that touches it, since
        -- the scheduler runs on the admin pool. Flagged loudly rather than done quietly, because
        -- a future table landing here by accident should be noticed.
        IF NOT has_tenant AND policy_count = 0 THEN
            RAISE NOTICE 'RLS: % has no tenant_id and no policy — forcing it confines it to the superuser', r.tbl;
        END IF;

        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', r.tbl);
        EXECUTE format('ALTER TABLE public.%I FORCE ROW LEVEL SECURITY', r.tbl);
    END LOOP;
END $$;

-- Re-assert the data-plane grants. A table created by a later migration may never have been
-- granted to qaat_app: 003 granted on ALL TABLES at the time it ran, which says nothing about
-- tables created afterwards. RLS then decides which ROWS are visible; the grant decides whether
-- the table can be addressed at all, and without it the role gets "permission denied" rather
-- than an empty result.
GRANT SELECT, INSERT, UPDATE ON ALL TABLES IN SCHEMA public TO qaat_app;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO qaat_app;

-- The append-only ledger keeps its standing prohibition: corrections are new rows.
REVOKE DELETE ON attendance_logs FROM qaat_app;

