-- 069: Bind a QA patroller's account to ONE handset.
--
-- A patrol record is an accusation: it states that a named lecturer was or was not teaching, and
-- the QA reports treat it as the independent second record against the coordinator's own log. The
-- only proof behind it is that a trusted patroller stood in the doorway. So the account has to be
-- pinned to the phone that patroller carries — otherwise a token lifted off the device, or an
-- account password shared "just to help cover the rounds", silently becomes the power to mark any
-- lecturer absent from anywhere.
--
-- The binding is claimed on first use (trust-on-first-use, like the student device lock in
-- `student_device_bindings`) and thereafter enforced on every patrol call. Two unique constraints,
-- both deliberate:
--
--   * one row per patroller  → they cannot silently move to a second phone;
--   * one row per handset    → two patrollers cannot share one phone and blur who ticked what.
--
-- Releasing a binding (lost or replaced phone) is an ADMIN action, recorded in admin_audit_log —
-- a rebind is exactly the moment worth being able to look back at.

CREATE TABLE IF NOT EXISTS patroller_device_bindings (
    user_id                 UUID        PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    tenant_id               UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    device_fingerprint_hash VARCHAR(128) NOT NULL,
    bound_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One handset serves exactly one patrol account.
CREATE UNIQUE INDEX IF NOT EXISTS uq_patrol_device_one_patroller
    ON patroller_device_bindings (device_fingerprint_hash);
CREATE INDEX IF NOT EXISTS idx_patrol_bindings_tenant
    ON patroller_device_bindings (tenant_id);

ALTER TABLE patroller_device_bindings ENABLE ROW LEVEL SECURITY;
ALTER TABLE patroller_device_bindings FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON patroller_device_bindings
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON patroller_device_bindings TO qaat_app;

-- Which handset a patrol record came from, kept alongside the record itself. The binding table
-- says where the account is allowed to work today; this says where each individual tick actually
-- came from, which is what an investigation after the fact needs.
ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS patroller_device_hash VARCHAR(128);
