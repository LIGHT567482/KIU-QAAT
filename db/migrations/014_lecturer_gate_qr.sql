-- QAAT — Lecturer Gate QR columns
-- Migration: 014_lecturer_gate_qr.sql
-- Adds two columns to lecturer_attendance_logs so we can record when the lecturer
-- physically scanned the coordinator's gate QR (proving they were on-site) and
-- from which device. Ghost-session detection uses these in admin audit queries.

ALTER TABLE lecturer_attendance_logs
  ADD COLUMN IF NOT EXISTS lecturer_scanned_at       TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS lecturer_fingerprint_hash VARCHAR(64);
