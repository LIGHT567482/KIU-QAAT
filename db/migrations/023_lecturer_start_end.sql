-- 023 — Lecturer scan-to-start / scan-to-end (next.txt #27/#28)
--
-- The lecturer scans the coordinator's live QR to BEGIN the lecture and again to
-- END it, each time entering the 10s rotating digit code shown on the coordinator
-- PWA. lecturer_scanned_at already records the START; add the END marker + its
-- own fingerprint so we can compute the lecturer's real contact time from their
-- two physical-presence proofs.

ALTER TABLE lecturer_attendance_logs
    ADD COLUMN IF NOT EXISTS lecturer_ended_at timestamptz,
    ADD COLUMN IF NOT EXISTS lecturer_end_fingerprint_hash varchar(128);
