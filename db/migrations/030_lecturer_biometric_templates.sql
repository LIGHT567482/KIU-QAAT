-- QAAT — migration 030
-- Forward-looking store for EXTERNAL-READER fingerprint templates (the future
-- "compare to the admin-enrolled template, on any device" path). WebAuthn passkeys
-- live in lecturer_webauthn_credentials; this table holds raw biometric templates
-- captured by a dedicated scanner SDK (e.g. ISO 19794-2). Verification (offline
-- 1:1 match on the coordinator device) is wired when the reader/SDK is chosen.
CREATE TABLE IF NOT EXISTS lecturer_biometric_templates (
    template_id     UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    lecturer_id     UUID        NOT NULL REFERENCES lecturers(lecturer_id) ON DELETE CASCADE,
    template        BYTEA       NOT NULL,            -- raw biometric template
    template_format TEXT        NOT NULL DEFAULT 'ISO_19794_2',
    finger_position SMALLINT,                        -- ANSI/NIST finger code (optional)
    reader_model    TEXT,                            -- which scanner captured it
    enrolled_by     VARCHAR(50),                     -- admin user_id who enrolled
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS ix_bio_tmpl_lecturer ON lecturer_biometric_templates (tenant_id, lecturer_id);

ALTER TABLE lecturer_biometric_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_biometric_templates FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON lecturer_biometric_templates
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON lecturer_biometric_templates TO qaat_app;
