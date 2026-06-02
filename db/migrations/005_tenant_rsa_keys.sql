-- QAAT — Tenant RSA Key Store
-- Migration: 005_tenant_rsa_keys.sql
-- Run order: 5 of N
--
-- Each tenant has one active RSA-2048 key pair.
-- Private key stored AES-256-CBC encrypted (master key from env KEY_ENCRYPTION_KEY).
-- Annual rotation: insert new row, set revoked_at on old row.

CREATE TABLE tenant_rsa_keys (
    key_id              UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    rsa_private_key_enc TEXT        NOT NULL,   -- AES-256-CBC encrypted PEM
    rsa_public_key_pem  TEXT        NOT NULL,   -- plaintext PEM (public is not secret)
    hmac_secret_enc     TEXT        NOT NULL,   -- AES-256-CBC encrypted hex secret
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at          TIMESTAMPTZ
);

CREATE INDEX idx_tenant_rsa_keys_active ON tenant_rsa_keys(tenant_id, revoked_at)
    WHERE revoked_at IS NULL;

ALTER TABLE tenant_rsa_keys ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON tenant_rsa_keys
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
