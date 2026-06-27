-- QAAT — Lecturer attendance: indexes + safety grants
-- Migration: 013_lecturer_attendance_indexes.sql
--
-- lecturer_attendance_logs has existed since migration 001 but no code wrote to
-- it. This migration adds the query indexes needed for the attendance reporting
-- endpoints, and re-asserts grants so the table is accessible as qaat_app even
-- though it predates the DEFAULT PRIVILEGES in 009.

-- ─── Indexes ─────────────────────────────────────────────────────────────────

-- Primary reporting query: per-lecturer attendance history
CREATE INDEX IF NOT EXISTS idx_lal_lecturer
    ON lecturer_attendance_logs(lecturer_id, tenant_id, session_date DESC);

-- Per-session lookup (used by CloseSession to update gate_close_time)
CREATE INDEX IF NOT EXISTS idx_lal_session
    ON lecturer_attendance_logs(session_id);

-- Per-unit attendance queries
CREATE INDEX IF NOT EXISTS idx_lal_unit
    ON lecturer_attendance_logs(unit_id, tenant_id);

-- ─── Safety grants (idempotent) ───────────────────────────────────────────────
-- 009 ran GRANT … ON ALL TABLES before 011/012 existed, so belt-and-braces:
GRANT SELECT, INSERT, UPDATE ON lecturer_attendance_logs TO qaat_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON lecturers TO qaat_app;
GRANT SELECT, INSERT, UPDATE, DELETE ON lecturer_assignments TO qaat_app;
