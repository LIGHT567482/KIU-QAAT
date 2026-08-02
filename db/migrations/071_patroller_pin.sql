-- 071: A second factor for the QA patroller — a PIN they set on first sign-in and enter every time.
--
-- WHY THE PASSWORD IS NOT ENOUGH. A patrol tick is an accusation: it states that a named lecturer
-- was or was not teaching, and QA reports weigh it against the coordinator's own log precisely
-- because it comes from an independent observer. The account password is the weakest part of that
-- chain — it is the thing that gets shared "just to help cover the rounds this week", and once
-- shared, anyone can mark any lecturer absent from anywhere.
--
-- Migration 069 bound the account to ONE handset, which stops the round moving to another phone.
-- This closes the other half: it stops someone else using THAT phone. Together they mean a tick
-- requires the right device AND a secret the patroller never types in front of anyone.
--
-- WHY A PIN AND NOT TOTP. Patrollers work in corridors on cheap handsets, frequently with no data
-- and a clock that drifts (the same wrong-clock problem that breaks TLS on SIM-less phones here).
-- TOTP would fail exactly when the round is happening. A PIN verifies against a stored hash and
-- needs neither a second app nor a correct clock.
--
-- Stored as bcrypt, never in the clear, and never returned by any endpoint — `pin_set` is the only
-- thing the app is ever told. Failed attempts are counted so a stolen-and-unlocked handset cannot
-- be brute-forced through 10 000 combinations; an admin clears the lockout, which is audited.

CREATE TABLE IF NOT EXISTS patroller_pins (
    user_id          UUID        PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
    tenant_id        UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    pin_hash         TEXT        NOT NULL,
    set_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_verified_at TIMESTAMPTZ,
    -- Consecutive failures since the last success. Reset to 0 on every correct entry.
    failed_attempts  INT         NOT NULL DEFAULT 0,
    -- Set when failed_attempts crosses the limit; the gateway refuses verification until it passes.
    locked_until     TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_patroller_pins_tenant ON patroller_pins (tenant_id);

ALTER TABLE patroller_pins ENABLE ROW LEVEL SECURITY;
ALTER TABLE patroller_pins FORCE  ROW LEVEL SECURITY;
-- DROP-then-CREATE: `CREATE POLICY` has no IF NOT EXISTS, so a database where this table
-- was created by hand (the "ragged" case the migrate package exists for) would fail here
-- and leave the migration half-applied.
DROP POLICY IF EXISTS "tenant_isolation" ON patroller_pins;
CREATE POLICY "tenant_isolation" ON patroller_pins
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON patroller_pins TO qaat_app;
