-- 050: Global one-device-one-student binding for the native student attendance app.
--
-- The coordinator's in-room engine already enforces the PER-LECTURE device lock (one device =
-- one student within a session). This table adds the GLOBAL guard used at the student app's
-- one-time onboarding (POST /api/v1/student/register-device): a device may belong to exactly one
-- student, and a student to exactly one device, across all lectures.
--
-- student_id is the registration number (students_extended PK, VARCHAR(50)) — NOT a UUID.

CREATE TABLE IF NOT EXISTS student_device_bindings (
    student_id              VARCHAR(50)  PRIMARY KEY
                                REFERENCES students_extended(student_id) ON DELETE CASCADE,
    tenant_id               UUID         NOT NULL
                                REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    device_fingerprint_hash VARCHAR(128) NOT NULL,
    -- After a self-rebind (device switch) the new device is paused from taking attendance for a
    -- cooldown window (12h). Delivered to the app at onboarding; enforced client-side.
    attend_block_until      TIMESTAMPTZ,
    created_at              TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);
-- Idempotent for existing deployments where 050 already ran without the column.
ALTER TABLE student_device_bindings ADD COLUMN IF NOT EXISTS attend_block_until TIMESTAMPTZ;

-- One device <-> one student: a given device fingerprint maps to at most one student.
CREATE UNIQUE INDEX IF NOT EXISTS uq_device_one_student
    ON student_device_bindings (device_fingerprint_hash);

CREATE INDEX IF NOT EXISTS idx_sdb_tenant ON student_device_bindings (tenant_id);
