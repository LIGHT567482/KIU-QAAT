-- QAAT — Integrity & Scale hardening
-- Migration: 007_integrity_and_scale.sql
-- Run order: 7 of N
--
-- Closes four review findings:
--   F-09  Vector-clock dedup on attendance_logs (session + coordinator + sequence)
--   F-07  Per-tenant keyed hashing of student ids (replaces unsalted SHA-256)
--   F-08  Per-tenant incremental eligibility summary (no global matview refresh)
--   F-10  Append-only / immutable admin_audit_log

-- ─── F-09: Vector-clock deduplication ─────────────────────────────────────────
-- The Sync Receiver dedups by the vector clock (tenant, session, coordinator,
-- sequence) rather than a client-chosen log_id, so a forged/duplicated upload
-- cannot create a second row for the same logical scan.

ALTER TABLE attendance_logs
    ADD COLUMN IF NOT EXISTS coordinator_id VARCHAR(50);

-- One QR_SCAN row per (tenant, session, coordinator, sequence). MANUAL_OVERRIDE
-- rows are excluded so a correction is always a new row (append-only preserved).
CREATE UNIQUE INDEX IF NOT EXISTS uq_attendance_vector_clock
    ON attendance_logs (tenant_id, session_id, coordinator_id, sequence_number)
    WHERE entry_method = 'QR_SCAN';

-- ─── F-07: Per-tenant keyed student-id hashing ────────────────────────────────
-- An unsalted SHA-256 of a low-entropy registration number is reversible by
-- brute force. We key the hash with a per-tenant secret (delivered to the
-- Coordinator inside the daily manifest over TLS) so stored hashes cannot be
-- reversed without the tenant key.

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS student_hash_key VARCHAR(64);

UPDATE tenants
   SET student_hash_key = encode(gen_random_bytes(32), 'hex')
 WHERE student_hash_key IS NULL;

ALTER TABLE tenants
    ALTER COLUMN student_hash_key SET DEFAULT encode(gen_random_bytes(32), 'hex'),
    ALTER COLUMN student_hash_key SET NOT NULL;

-- ─── F-08: Per-tenant incremental eligibility summary ─────────────────────────
-- The previous design did REFRESH MATERIALIZED VIEW CONCURRENTLY of the WHOLE
-- view on every sync — a global lock that does not scale to many tenants.
-- We replace it with a plain table refreshed for one tenant at a time.

DROP MATERIALIZED VIEW IF EXISTS student_attendance_summary;

CREATE TABLE student_attendance_summary (
    student_id             VARCHAR(50)  NOT NULL,
    unit_id                VARCHAR(50)  NOT NULL,
    unit_name              VARCHAR(200),
    course_id              VARCHAR(50),
    tenant_id              UUID         NOT NULL,
    sessions_held          INTEGER      NOT NULL DEFAULT 0,
    sessions_attended      INTEGER      NOT NULL DEFAULT 0,
    attendance_percentage  NUMERIC(5,2) NOT NULL DEFAULT 0,
    refreshed_at           TIMESTAMPTZ  NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, student_id, unit_id)
);

CREATE INDEX idx_summary_tenant_unit ON student_attendance_summary(tenant_id, unit_id);

ALTER TABLE student_attendance_summary ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON student_attendance_summary
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);

-- Refresh exactly one tenant's rows. SECURITY DEFINER so the Sync Receiver can
-- call it without holding a tenant GUC; the p_tenant filter scopes every write.
CREATE OR REPLACE FUNCTION refresh_attendance_summary(p_tenant UUID)
RETURNS void
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
BEGIN
    DELETE FROM student_attendance_summary WHERE tenant_id = p_tenant;

    INSERT INTO student_attendance_summary
        (student_id, unit_id, unit_name, course_id, tenant_id,
         sessions_held, sessions_attended, attendance_percentage)
    SELECT
        al.student_id,
        cu.unit_id,
        cu.name,
        cu.course_id,
        cu.tenant_id,
        COUNT(DISTINCT s.session_id),
        COUNT(DISTINCT al.session_id),
        ROUND(
            COUNT(DISTINCT al.session_id)::DECIMAL /
            NULLIF(COUNT(DISTINCT s.session_id), 0) * 100, 2)
    FROM course_units cu
    JOIN sessions s
        ON  s.unit_id = cu.unit_id
        AND s.session_status IN ('CLOSED', 'AUTO_CLOSED')
    LEFT JOIN attendance_logs al
        ON  al.session_id = s.session_id
        AND al.tenant_id  = cu.tenant_id
    WHERE cu.tenant_id = p_tenant
    GROUP BY al.student_id, cu.unit_id, cu.name, cu.course_id, cu.tenant_id
    HAVING al.student_id IS NOT NULL;
END;
$$;

REVOKE ALL ON FUNCTION refresh_attendance_summary(UUID) FROM PUBLIC;
GRANT  EXECUTE ON FUNCTION refresh_attendance_summary(UUID) TO qaat_app;
GRANT  SELECT, INSERT, UPDATE ON student_attendance_summary TO qaat_app;

-- ─── F-10: Immutable admin_audit_log ──────────────────────────────────────────
-- The attendance log is append-only; its audit trail must be too.

CREATE POLICY no_delete_audit ON admin_audit_log
    AS RESTRICTIVE FOR DELETE USING (false);

CREATE POLICY no_update_audit ON admin_audit_log
    AS RESTRICTIVE FOR UPDATE USING (false);

REVOKE UPDATE, DELETE ON admin_audit_log FROM qaat_app;
