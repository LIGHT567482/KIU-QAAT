-- Migration 041: Multi-slot timetable.
--
-- The old offering_unit_schedules holds ONE slot per (offering, unit) with no
-- room. Real timetables (see KIU PDFs) run the same unit on several days, each in
-- a different room. timetable_slots allows many slots per unit, each with a day,
-- start time, duration, room, and (optionally) a specific lecturer — and is what
-- the redesigned weekly-grid timetable + timetable import read and write.
--
-- offering_unit_schedules is kept for the existing set-once "planned start" used
-- by OpenSession; the weekly grid is authoritative for display + per-day rooms.

CREATE TABLE IF NOT EXISTS timetable_slots (
    slot_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id        UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    offering_id      UUID        NOT NULL REFERENCES course_offerings(offering_id) ON DELETE CASCADE,
    unit_id          VARCHAR(50) NOT NULL REFERENCES course_units(unit_id) ON DELETE CASCADE,
    day_of_week      SMALLINT    NOT NULL CHECK (day_of_week BETWEEN 1 AND 7),
    start_time       TIME        NOT NULL,
    duration_minutes INTEGER     NOT NULL DEFAULT 60,
    room             TEXT,
    lecturer_id      UUID        REFERENCES lecturers(lecturer_id) ON DELETE SET NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (offering_id, unit_id, day_of_week, start_time)
);

ALTER TABLE timetable_slots ENABLE ROW LEVEL SECURITY;
ALTER TABLE timetable_slots FORCE  ROW LEVEL SECURITY;
CREATE POLICY "tenant_isolation" ON timetable_slots
    FOR ALL USING (tenant_id = current_setting('app.current_tenant', true)::uuid);
GRANT SELECT, INSERT, UPDATE, DELETE ON timetable_slots TO qaat_app;

CREATE INDEX IF NOT EXISTS ix_timetable_slots_offering ON timetable_slots (offering_id);

-- Backfill the existing single slots so nothing is lost.
INSERT INTO timetable_slots (tenant_id, offering_id, unit_id, day_of_week, start_time, duration_minutes)
SELECT tenant_id, offering_id, unit_id, day_of_week, session_start, COALESCE(session_duration_minutes, 60)
FROM offering_unit_schedules
WHERE day_of_week IS NOT NULL AND session_start IS NOT NULL
ON CONFLICT (offering_id, unit_id, day_of_week, start_time) DO NOTHING;
