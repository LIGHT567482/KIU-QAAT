-- 006: Store the device-bound HMAC of each uploaded session package.
--
-- The Coordinator PWA computes HMAC-SHA256 over the (base64) encrypted payload
-- using a key derived from its device binding key (HKDF). sync-receiver verifies
-- this HMAC before decrypting, so a tampered or forged package is rejected
-- (package_checksum alone is only a transport-integrity check, not authenticity).

ALTER TABLE sync_uploads
    ADD COLUMN IF NOT EXISTS package_hmac VARCHAR(64);
