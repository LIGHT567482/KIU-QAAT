-- 058: QA patroller role + lecturer patrol logs (Phase 3 of the QA subsystem).
--
-- A QA patroller walks room-to-room and records whether the TIMETABLED lecturer is actually
-- teaching. The patroller mobile app infers unit/lecturer/room/time from the cached daily timetable;
-- the patroller only ticks TAUGHT / NOT TAUGHT. Every log carries the patroller's identity (name +
-- staff id) and an automatic timestamp, and is idempotent per (unit, date, scheduled time).

ALTER TYPE user_role_enum ADD VALUE IF NOT EXISTS 'QA_PATROLLER';

-- QA/support staff (incl. patrollers) can carry a staff ID on their user account.
ALTER TABLE users ADD COLUMN IF NOT EXISTS staff_id VARCHAR(50);

CREATE TABLE IF NOT EXISTS lecturer_patrol_logs (
    patrol_id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    unit_id             VARCHAR(50) NOT NULL,
    unit_name           VARCHAR(200),
    course_code         VARCHAR(50),
    lecturer_id         VARCHAR(50),          -- the lecturer's staff_id (matches the manifest key)
    lecturer_name       VARCHAR(200),
    room                TEXT,
    session_date        DATE        NOT NULL,
    scheduled_time      TEXT,                 -- "HH:MM" from the timetable slot
    taught              BOOLEAN     NOT NULL,
    patroller_id        UUID        NOT NULL,
    patroller_name      VARCHAR(200),
    patroller_staff_id  VARCHAR(50),
    taken_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    entry_method        TEXT        NOT NULL DEFAULT 'PATROL',
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, unit_id, session_date, scheduled_time)
);

CREATE INDEX IF NOT EXISTS idx_patrol_logs_lecturer ON lecturer_patrol_logs(tenant_id, lecturer_id, session_date);
CREATE INDEX IF NOT EXISTS idx_patrol_logs_date     ON lecturer_patrol_logs(tenant_id, session_date);

ALTER TABLE lecturer_patrol_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE lecturer_patrol_logs FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON lecturer_patrol_logs
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON lecturer_patrol_logs TO qaat_app;
