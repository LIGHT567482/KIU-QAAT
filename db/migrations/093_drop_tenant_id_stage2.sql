-- 093: Remove tenant_id — stage 2 (the leaf tables).
--
-- See 092 for why this is staged and what the hazard is. In short: a column cannot be dropped until
-- every Go query naming it has been rewritten, and dropping a column CASCADE takes any unique index
-- containing it along too — leaving no constraint at all rather than a smaller one.
--
-- These seven are the leaves: nothing joins to them on tenant, and each is reached through a key
-- that is already unique on its own (a patrol, a lecturer, a student, a credential). They are done
-- first precisely because getting them wrong is cheap to notice and cheap to undo.
--
-- Go rewritten for each, in this order:
--   hardware_vault                 — no query named tenant_id at all; the column was never read
--   lecturer_biometric_templates   — same; only a comment mentioned the table
--   monitor_log_units              — patrol_manual.go insert (column list + placeholders renumbered)
--   sync_uploads                   — clear_semester.go delete
--   lecturer_daily_codes           — manifest.go: two reads and the insert
--   lecturer_webauthn_credentials  — webauthn_lecturer.go read + upsert, lecturer_gate_scan.go count
--   notification_log               — scheduler.go MarkSent (insert AND its ON CONFLICT target)

ALTER TABLE hardware_vault                DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE lecturer_biometric_templates  DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE monitor_log_units             DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE sync_uploads                  DROP COLUMN IF EXISTS tenant_id CASCADE;
ALTER TABLE lecturer_webauthn_credentials DROP COLUMN IF EXISTS tenant_id CASCADE;

-- lecturer_daily_codes carried TWO unique indexes containing tenant_id, and they mean different
-- things. Both are recreated below, so dropping the column cannot quietly relax either.
ALTER TABLE lecturer_daily_codes          DROP COLUMN IF EXISTS tenant_id CASCADE;

-- One code per lecturer per day: what the manifest's read-then-insert-then-read loop relies on to
-- settle on a single code when two requests race for the same lecturer.
CREATE UNIQUE INDEX IF NOT EXISTS uq_lecturer_daily_code_owner
    ON lecturer_daily_codes (lecturer_id, valid_date);
-- And the code itself is unique for the day — the property that makes a four-digit code an
-- identifier at all. Without it two lecturers could be issued 0421 on the same morning and the
-- coordinator typing it in would start the wrong lecture.
CREATE UNIQUE INDEX IF NOT EXISTS uq_lecturer_daily_code_value
    ON lecturer_daily_codes (valid_date, code);

-- notification_log's whole purpose is idempotency: the scheduler may re-run, and this index is what
-- stops a lecturer being emailed twice about the same lecture on the same day. Recreated to match
-- the ON CONFLICT target in scheduler.go MarkSent exactly.
ALTER TABLE notification_log              DROP COLUMN IF EXISTS tenant_id CASCADE;
CREATE UNIQUE INDEX IF NOT EXISTS uq_notification_once
    ON notification_log (kind, subject_key, subject_date);
