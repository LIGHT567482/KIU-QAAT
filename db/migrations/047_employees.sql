-- Migration 047: Employees (non-teaching staff) + their tablet check-in/out log.
--
-- Separate from lecturers and students: general employees are tracked by an
-- existing physical check-in TABLET. Admins register employees (their own way,
-- with a bulk import/export template) and import the tablet's punch log; the
-- employee-attendance report pairs each day's IN/OUT and auto-generates a comment
-- (date, times, and the running count of days the employee actually worked).

-- ── Employee registry ────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS employees (
    employee_pk  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    staff_id     VARCHAR(60) NOT NULL,          -- badge / tablet ID (unique per tenant)
    title        VARCHAR(40),
    full_name    TEXT        NOT NULL,
    department   VARCHAR(160),
    job_title    VARCHAR(160),
    email        VARCHAR(200),
    phone        VARCHAR(40),
    is_active    BOOLEAN     NOT NULL DEFAULT true,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, staff_id)
);

ALTER TABLE employees ENABLE ROW LEVEL SECURITY;
ALTER TABLE employees FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON employees
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON employees TO qaat_app;
CREATE INDEX IF NOT EXISTS ix_employees_tenant ON employees (tenant_id);

-- ── Tablet check-in / check-out punches ──────────────────────────────────────
-- Soft-linked to employees by (tenant_id, staff_id): punches can be imported even
-- for a staff_id not yet registered (a stub employee is created on import). One
-- row per distinct punch — the UNIQUE key makes re-importing the same export
-- idempotent (no duplicate days).
CREATE TABLE IF NOT EXISTS employee_attendance_logs (
    log_id      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    staff_id    VARCHAR(60) NOT NULL,
    event_time  TIMESTAMPTZ NOT NULL,
    event_type  VARCHAR(8)  NOT NULL DEFAULT 'PUNCH',  -- 'IN' | 'OUT' | 'PUNCH' (inferred)
    source      VARCHAR(20) NOT NULL DEFAULT 'TABLET',
    device_id   VARCHAR(80),
    comment     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, staff_id, event_time, event_type)
);

ALTER TABLE employee_attendance_logs ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee_attendance_logs FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON employee_attendance_logs
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON employee_attendance_logs TO qaat_app;
CREATE INDEX IF NOT EXISTS ix_emp_att_tenant_staff ON employee_attendance_logs (tenant_id, staff_id, event_time);
