-- QAAT — migration 028
-- (1) Tenant-configurable daily session window (coordinators can only begin a
--     session inside it; default 08:00–17:00, Mon–Sat).
-- (2) Separate fixed passcode gating the admin Users page.
-- (3) New DVC (Deputy Vice-Chancellor) oversight role.
-- (4) Per-lecturer WebAuthn (biometric passkey) credentials for identity proof.

-- ── (3) DVC role ─────────────────────────────────────────────────────────────
-- ADD VALUE is transaction-safe in PG12+ as long as the new label is not used in
-- the same transaction (it is not — only stored from the app later).
ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'DVC';

-- ── (1) Daily session window + (2) Users-page passcode ───────────────────────
ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS session_window_start TIME        NOT NULL DEFAULT '08:00',
    ADD COLUMN IF NOT EXISTS session_window_end   TIME        NOT NULL DEFAULT '17:00',
    -- ISO day-of-week: 1=Mon … 7=Sun. Default Mon–Sat.
    ADD COLUMN IF NOT EXISTS session_active_days  SMALLINT[]  NOT NULL DEFAULT '{1,2,3,4,5,6}',
    -- bcrypt hash of the Users-page passcode; NULL = no passcode set yet.
    ADD COLUMN IF NOT EXISTS users_passcode_hash  TEXT;

-- ── (4) Lecturer WebAuthn credentials ────────────────────────────────────────
CREATE TABLE IF NOT EXISTS lecturer_webauthn_credentials (
    credential_id   TEXT        PRIMARY KEY,                 -- base64url cred id
    tenant_id       UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    lecturer_id     UUID        NOT NULL REFERENCES lecturers(lecturer_id) ON DELETE CASCADE,
    public_key      BYTEA       NOT NULL,                    -- COSE public key
    attestation_type TEXT       NOT NULL DEFAULT '',
    aaguid          BYTEA,
    sign_count      BIGINT      NOT NULL DEFAULT 0,
    transports      TEXT[]      NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at    TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS ix_webauthn_lecturer ON lecturer_webauthn_credentials (tenant_id, lecturer_id);

ALTER TABLE lecturer_webauthn_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_webauthn_credentials FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON lecturer_webauthn_credentials
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON lecturer_webauthn_credentials TO qaat_app;
