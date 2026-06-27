-- 008: Online student check-in.
--
-- Replaces the offline PWA-LAN scan path (which cannot work cross-platform: iOS
-- can't host a server, phone hotspots cap at ~10 clients, and secure-context
-- browser APIs require HTTPS — see plan async-napping-petal). Live check-in now
-- runs over HTTPS against the cloud. Proximity (was BLE RSSI, impossible on iOS
-- Safari) becomes a rotating 6-digit room code derived from a per-session secret.

-- ─── Per-session check-in secret ──────────────────────────────────────────────
-- The secret seeds the rotating room code (HMAC-based TOTP). It is created when
-- the coordinator opens the session and NEVER leaves the server: the coordinator
-- polls the current code over an authenticated endpoint; students only ever see
-- the 6-digit code on the projector.
ALTER TABLE sessions
    ADD COLUMN IF NOT EXISTS checkin_secret BYTEA;

-- ─── Race-safe per-student dedup ──────────────────────────────────────────────
-- One QR_SCAN attendance row per (tenant, session, student). This enforces the
-- "duplicate scan" check at the DB level so two concurrent submissions for the
-- same student can never both land. MANUAL_OVERRIDE rows are excluded so a
-- correction is always a new row (append-only preserved). This complements the
-- existing vector-clock index uq_attendance_vector_clock (007).
CREATE UNIQUE INDEX IF NOT EXISTS uq_attendance_session_student
    ON attendance_logs (tenant_id, session_id, student_id)
    WHERE entry_method = 'QR_SCAN';
