-- QAAT — Materialized Views
-- Migration: 004_materialized_views.sql
-- Run order: 4 of 4
--
-- Refreshed after each successful atomic sync (triggered by Sync Receiver service).

CREATE MATERIALIZED VIEW student_attendance_summary AS
SELECT
    al.student_id,
    cu.unit_id,
    cu.name                                          AS unit_name,
    cu.course_id,
    al.tenant_id,
    COUNT(DISTINCT s.session_id)                     AS sessions_held,
    COUNT(DISTINCT al.session_id)                    AS sessions_attended,
    ROUND(
        COUNT(DISTINCT al.session_id)::DECIMAL /
        NULLIF(COUNT(DISTINCT s.session_id), 0) * 100,
        2
    )                                                AS attendance_percentage
FROM course_units cu
JOIN sessions s
    ON  s.unit_id = cu.unit_id
    AND s.session_status IN ('CLOSED', 'AUTO_CLOSED')
LEFT JOIN attendance_logs al
    ON  al.session_id = s.session_id
    AND al.tenant_id  = cu.tenant_id
GROUP BY al.student_id, cu.unit_id, cu.name, cu.course_id, al.tenant_id
WITH DATA;

CREATE UNIQUE INDEX ON student_attendance_summary(student_id, unit_id, tenant_id);
CREATE INDEX        ON student_attendance_summary(tenant_id, unit_id);
