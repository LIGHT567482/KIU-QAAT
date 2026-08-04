-- 077: The biometric software's daily sheet, stored as it arrives.
--
-- WHAT THIS IS. The attendance terminal exports one row per employee per day with 29 columns —
-- Emp No., AC-No., Name, Date, Timetable, On duty, Off duty, Clock In, Clock Out, Normal, Real
-- time, Late, Early, Absent, OT Time, Work Time, Exception, Must C/In, Must C/Out, Department,
-- NDays, WeekEnd, Holiday, ATT_Time, and the three OT breakdowns. That sheet is the source of
-- truth: it is what the institution's HR already works from, and it already encodes the shift
-- rules (a "GENERAL" timetable of 08:00–17:00), the overtime arithmetic and the holiday calendar.
--
-- WHY STORE IT VERBATIM RATHER THAN RECOMPUTE IT. The alternative is to keep ingesting raw punches
-- and derive these columns ourselves, which means reimplementing a shift model, an OT policy and a
-- holiday calendar that already exist and are already agreed. Every one of those is a place to
-- disagree with the payroll the institution actually runs on. Storing the sheet means the system
-- and HR cannot drift; filtering and reporting sit on top.
--
-- The existing employee_attendance_logs (raw punches, migration 047) stays. It answers a different
-- question — the individual clock events — and the tablet importer still writes it.
--
-- IDEMPOTENT RE-UPLOAD. HR re-exports and re-uploads the same month routinely, with corrections.
-- UNIQUE (tenant_id, ac_no, work_date) means a re-upload updates in place rather than doubling
-- every row, which is the difference between "upload it again" being safe and being a data-loss
-- incident.

CREATE TABLE IF NOT EXISTS employee_attendance_days (
    day_id        UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id     UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,

    -- Identity as the terminal reports it. ac_no is the terminal's own enrolment number and is the
    -- stable key; emp_no is HR's, and is frequently blank or repeated in real exports.
    emp_no        VARCHAR(40),
    ac_no         VARCHAR(40)  NOT NULL,
    seq_no        VARCHAR(40),                 -- the sheet's third column, "No."
    full_name     TEXT         NOT NULL DEFAULT '',
    auto_assign   TEXT,
    department    VARCHAR(160),

    work_date     DATE         NOT NULL,
    timetable     VARCHAR(60),                 -- shift name, e.g. GENERAL

    -- Shift boundaries and what actually happened. Stored as TEXT in "HH:MM" because the export
    -- leaves them blank for a day nobody worked, and a blank is meaningful: NULL time is "no
    -- record", which is exactly what an absence looks like.
    on_duty       VARCHAR(10),
    off_duty      VARCHAR(10),
    clock_in      VARCHAR(10),
    clock_out     VARCHAR(10),

    -- The terminal's own computed columns. Numeric where the export is numeric, text where it is a
    -- duration like "01:58", boolean where it is True/blank.
    normal        NUMERIC(8,2),
    real_time     NUMERIC(8,2),
    late          VARCHAR(10),
    early         VARCHAR(10),
    absent        BOOLEAN      NOT NULL DEFAULT false,
    ot_time       VARCHAR(10),
    work_time     VARCHAR(10),
    exception     TEXT,
    must_cin      BOOLEAN      NOT NULL DEFAULT false,
    must_cout     BOOLEAN      NOT NULL DEFAULT false,

    ndays         NUMERIC(8,2),
    weekend       NUMERIC(8,2),
    holiday       NUMERIC(8,2),
    att_time      VARCHAR(10),
    ndays_ot      NUMERIC(8,2),
    weekend_ot    NUMERIC(8,2),
    holiday_ot    NUMERIC(8,2),

    -- Derived on ingest, not by the terminal: whether clock_in was after on_duty and clock_out
    -- before off_duty. Stored because the notification jobs filter on it every minute and the
    -- comparison is on TEXT times that would otherwise need parsing per row per sweep.
    checked_in_late   BOOLEAN NOT NULL DEFAULT false,
    checked_out_early BOOLEAN NOT NULL DEFAULT false,

    source_file   TEXT,
    imported_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    UNIQUE (tenant_id, ac_no, work_date)
);

-- The report filters: by department, by date range, and by the exception flags. All three are the
-- first thing anyone narrows by.
CREATE INDEX IF NOT EXISTS idx_emp_days_tenant_date ON employee_attendance_days (tenant_id, work_date DESC);
CREATE INDEX IF NOT EXISTS idx_emp_days_department  ON employee_attendance_days (tenant_id, btrim(lower(department)));
CREATE INDEX IF NOT EXISTS idx_emp_days_name        ON employee_attendance_days (tenant_id, btrim(lower(full_name)));
CREATE INDEX IF NOT EXISTS idx_emp_days_flags       ON employee_attendance_days (tenant_id, work_date)
    WHERE absent OR checked_in_late OR checked_out_early;

ALTER TABLE employee_attendance_days ENABLE ROW LEVEL SECURITY;
ALTER TABLE employee_attendance_days FORCE  ROW LEVEL SECURITY;
-- DROP-then-CREATE: CREATE POLICY has no IF NOT EXISTS, so a hand-built database would fail here
-- and leave the migration half-applied.
DROP POLICY IF EXISTS "tenant_isolation" ON employee_attendance_days;
CREATE POLICY "tenant_isolation" ON employee_attendance_days
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON employee_attendance_days TO qaat_app;
