-- 089: one observed lecture, several course units — and a compensation that says WHICH lecture.
--
-- ── WHY A LECTURE CAN HAVE MORE THAN ONE UNIT ────────────────────────────────────────────────
--
-- The same hour, the same room, the same lecturer, one class in front of them — and two or three
-- different course units, because the same taught content is coded differently by each programme
-- that requires it. "Research Methods" is CSC 3103 to the computer scientists and BIT 3110 to the
-- information-technology students, and the departments that own those codes are usually different
-- even when the college is the same.
--
-- Until now a monitor recording that lecture had to pick ONE. Whichever they picked, every student
-- on the other code had a lecture that QA has no record of, and the lecturer's teaching record
-- carried one hour where they taught two units' worth of obligations. The extra units are therefore
-- not a note in the remarks — remarks cannot be counted — but rows that can be grouped and reported.
--
-- A CHILD TABLE, not a JSON column or three more columns on the log. "Which lectures covered
-- CSC 3103" is an ordinary QA question, and it has to be answerable for a unit that was the second
-- one on the record exactly as it is for the first. That is a join, so it is a table.
--
-- The PRIMARY unit stays on lecturer_patrol_logs. Every existing report, export and uniqueness rule
-- keys off it, and a change that made the primary unit optional would ripple through all of them for
-- no gain: there is always a unit the monitor named first.

CREATE TABLE IF NOT EXISTS monitor_log_units (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    patrol_id    UUID NOT NULL REFERENCES lecturer_patrol_logs(patrol_id) ON DELETE CASCADE,
    unit_id      VARCHAR(50)  NOT NULL,
    unit_name    VARCHAR(200),
    course_code  VARCHAR(50),
    class_group  TEXT,
    -- School AND department, both, and both nullable. Two codes for one lecture are often in the
    -- same college and almost always in different departments of it — which is exactly the pair a
    -- reader needs to see to understand who owes whom the teaching.
    school       TEXT,
    department   TEXT,
    -- Whether this unit was picked from the curriculum or typed. A typed unit is the case the
    -- manual form exists for, and a report that cannot tell the two apart cannot tell a curriculum
    -- gap from a spelling mistake.
    resolved     BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (patrol_id, unit_id)
);

CREATE INDEX IF NOT EXISTS ix_monitor_log_units_log  ON monitor_log_units (patrol_id);
CREATE INDEX IF NOT EXISTS ix_monitor_log_units_unit ON monitor_log_units (tenant_id, unit_id);

ALTER TABLE monitor_log_units ENABLE ROW LEVEL SECURITY;
ALTER TABLE monitor_log_units FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS tenant_isolation ON monitor_log_units;
CREATE POLICY tenant_isolation ON monitor_log_units
    USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON monitor_log_units TO qaat_app;

COMMENT ON TABLE monitor_log_units IS
    'The ADDITIONAL course units covered by one observed lecture. The same hour of teaching often '
    'satisfies several unit codes belonging to different programmes and departments; the first is '
    'on lecturer_patrol_logs, the rest are here, so every one of them can be counted.';

-- ── The department of the lecture itself ─────────────────────────────────────────────────────
--
-- school was added in 084; department was not, and it is the more useful of the two. Two unit codes
-- for one lecture are frequently in the same college — so a school column alone collapses them and
-- loses the fact that different departments are involved.
ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS department TEXT;

COMMENT ON COLUMN lecturer_patrol_logs.department IS
    'The department owning the primary course unit. Inherited from the curriculum when the unit was '
    'picked, typed when it was not.';

-- ── WHEN the lecture being compensated for was ───────────────────────────────────────────────
--
-- compensation_for (083) is a DATE, and a date is not enough. A lecturer misses a Tuesday on which
-- they were timetabled twice; "compensating for Tuesday" does not say which of the two, so it
-- cannot be matched against the missed lecture and cannot be counted as making it good. The time is
-- what makes a compensation checkable rather than merely claimed.
--
-- The old DATE column is kept and kept in step, so every existing report, export and query reading
-- compensation_for continues to work unchanged. New writes fill both.
ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS compensation_for_at TIMESTAMPTZ;

COMMENT ON COLUMN lecturer_patrol_logs.compensation_for_at IS
    'The date AND START TIME of the lecture this one makes good. Required whenever '
    'is_compensation is true — without the time a compensation cannot be matched to the lecture it '
    'claims to replace. compensation_for holds its date, for readers written before this existed.';

UPDATE lecturer_patrol_logs
   SET compensation_for_at = compensation_for::timestamptz
 WHERE is_compensation AND compensation_for IS NOT NULL AND compensation_for_at IS NULL;

-- ── The end of the lecture ───────────────────────────────────────────────────────────────────
--
-- scheduled_time is when the monitor observed the lecture, and it doubles as the key that keeps two
-- real lectures of one unit on one day apart. It says nothing about how long the class ran. A
-- monitor standing in the room knows the timetabled span — "10:00 to 12:00" — and recording only
-- the first half of it loses the contact hours the lecture is worth.
ALTER TABLE lecturer_patrol_logs
    ADD COLUMN IF NOT EXISTS end_time TEXT;

COMMENT ON COLUMN lecturer_patrol_logs.end_time IS
    'HH:MM the lecture was due to end. scheduled_time is when it began; the pair is what the monitor '
    'is shown and what contact-hour reporting needs. Nullable: older records have no end.';
