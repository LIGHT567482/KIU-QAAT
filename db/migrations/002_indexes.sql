-- QAAT — Indexes
-- Migration: 002_indexes.sql
-- Run order: 2 of 4

-- Users
CREATE INDEX idx_users_tenant_email   ON users(tenant_id, email);
CREATE INDEX idx_users_tenant_role    ON users(tenant_id, role) WHERE is_active = true;

-- Attendance lookup performance
CREATE INDEX idx_attendance_session   ON attendance_logs(session_id, tenant_id);
CREATE INDEX idx_attendance_student   ON attendance_logs(student_id, tenant_id);

-- Eligibility computation
CREATE INDEX idx_attendance_student_unit ON attendance_logs(student_id, session_id)
    INCLUDE (checkin_timestamp);

-- Session lookups
CREATE INDEX idx_sessions_coordinator ON sessions(coordinator_id, session_date, tenant_id);
CREATE INDEX idx_sessions_unit        ON sessions(unit_id, session_date, tenant_id);
CREATE INDEX idx_sessions_status      ON sessions(tenant_id, session_status);

-- Tenant isolation
CREATE INDEX idx_students_tenant      ON students_extended(tenant_id, enrollment_status);
CREATE INDEX idx_courses_tenant       ON courses(tenant_id);
CREATE INDEX idx_units_course         ON course_units(course_id, tenant_id);

-- Sync
CREATE INDEX idx_sync_uploads_coord   ON sync_uploads(coordinator_id, tenant_id, status);

-- Audit log
CREATE INDEX idx_audit_tenant_actor   ON admin_audit_log(tenant_id, actor_id, occurred_at DESC);
