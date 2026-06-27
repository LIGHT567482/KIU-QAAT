-- QAAT — migration 029
-- Store the full serialized go-webauthn Credential (JSON) so sign-count and
-- authenticator data round-trip cleanly. public_key alone is insufficient.
ALTER TABLE lecturer_webauthn_credentials
    ADD COLUMN IF NOT EXISTS credential JSONB;
