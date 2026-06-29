-- Migration 042: Emergency standby coordinator.
--
-- If a coordinator is absent for a session, they (and only they) can pre-authorise
-- a student of their OWN cohort to act as the coordinator for the rest of the day.
-- The student exchanges a short code + their reg-no for a COORDINATOR token minted
-- for the absent coordinator's own identity, scoped to that one offering and capped
-- to end of day. This table is the delegation record + audit trail.

CREATE TABLE IF NOT EXISTS coordinator_delegations (
    delegation_id     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    offering_id       UUID        NOT NULL REFERENCES course_offerings(offering_id) ON DELETE CASCADE,
    coordinator_id    VARCHAR(50) NOT NULL,           -- granting coordinator's user_id
    deputy_student_id VARCHAR(50) NOT NULL,           -- the standby student's reg-no
    deputy_name       TEXT,
    code              VARCHAR(16) NOT NULL,           -- standby code handed to the student
    expires_at        TIMESTAMPTZ NOT NULL,
    revoked           BOOLEAN     NOT NULL DEFAULT false,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at      TIMESTAMPTZ,
    UNIQUE (tenant_id, code)
);

ALTER TABLE coordinator_delegations ENABLE ROW LEVEL SECURITY;
ALTER TABLE coordinator_delegations FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON coordinator_delegations
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON coordinator_delegations TO qaat_app;

CREATE INDEX IF NOT EXISTS ix_coord_deleg_code ON coordinator_delegations (code);
CREATE INDEX IF NOT EXISTS ix_coord_deleg_coordinator ON coordinator_delegations (coordinator_id);
