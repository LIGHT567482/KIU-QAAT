-- 063: Retire the QR subsystem.
--
-- Students check in by typing their registration number on the coordinator's in-room hub, and
-- lecturers start/end a session from the app with their staff ID. Nothing issues, prints, scans or
-- verifies a personal QR any more, so the columns and the key store that backed them are dead
-- weight. The `qr-generator` service (their only writer) is gone with this migration.
--
-- Deliberately KEPT:
--   • attendance_logs.entry_method = 'QR_SCAN' — this enum value is the ledger's "normal check-in"
--     marker. Two partial unique indexes (migrations 007 + 008) and the sync-receiver's conflict
--     clause key on it, and attendance_logs is append-only with DELETE revoked, so renaming it
--     would rewrite history rather than clean it up. It is a label, not a QR.
--   • lecturer_attendance_logs.lecturer_scanned_at — despite the name this is simply "when the
--     lecturer started the lecture", the verification gate every dashboard depends on.

ALTER TABLE students_extended DROP COLUMN IF EXISTS qr_public_key_hash;
ALTER TABLE students_extended DROP COLUMN IF EXISTS qr_serial_number;

-- The per-tenant RSA key pair existed solely to sign and verify student QR payloads.
DROP TABLE IF EXISTS tenant_rsa_keys;
