-- 048: Semester archives — a compressed (zip) snapshot of the attendance data that
-- an end-of-semester CLEAR is about to delete, kept so it can be downloaded later
-- from the Reports feature. Clearing is now INTAKE-scoped: a semester can end for one
-- intake (e.g. August) while another (e.g. May) is still studying, so the admin never
-- wipes the whole institution at once — they pick which intake(s)/period to clear, and
-- every clear first writes an archive here recording exactly what it covered.

CREATE TABLE IF NOT EXISTS semester_archives (
    archive_id        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    label             TEXT        NOT NULL,             -- human label e.g. "August · 2024/2025 · Sem 2"
    intakes           TEXT[]      NOT NULL DEFAULT '{}',-- which intake(s) this archive covered
    academic_year     TEXT,                             -- optional period filter used
    semester          INT,                              -- optional period filter used
    filename          TEXT        NOT NULL,             -- suggested download filename (.zip)
    content           BYTEA       NOT NULL,             -- the zip (CSVs of attendance/sessions/lecturer logs)
    size_bytes        BIGINT      NOT NULL DEFAULT 0,
    attendance_rows   INT         NOT NULL DEFAULT 0,
    session_rows      INT         NOT NULL DEFAULT 0,
    lecturer_rows     INT         NOT NULL DEFAULT 0,
    created_by        TEXT,                             -- admin who ran the clear
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_semester_archives_tenant
    ON semester_archives (tenant_id, created_at DESC);

ALTER TABLE semester_archives ENABLE ROW LEVEL SECURITY;
ALTER TABLE semester_archives FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON semester_archives
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, DELETE ON semester_archives TO qaat_app;
