-- QAAT — Initial Schema
-- Migration: 001_init_schema.sql
-- Run order: 1 of 4

-- ─── Extensions ──────────────────────────────────────────────────────────────
CREATE EXTENSION IF NOT EXISTS "pgcrypto";  -- gen_random_uuid()

-- ─── ENUMs ───────────────────────────────────────────────────────────────────

CREATE TYPE user_role_enum AS ENUM (
    'COORDINATOR', 'QA_OFFICER', 'DQA_DIRECTOR', 'VC', 'ADMIN'
);

CREATE TYPE session_status_enum AS ENUM (
    'PENDING_LECTURER', 'ACTIVE', 'CLOSED', 'AUTO_CLOSED'
);

CREATE TYPE sync_status_enum AS ENUM (
    'PENDING', 'UPLOADING', 'SYNCED', 'FAILED'
);

CREATE TYPE entry_method_enum AS ENUM (
    'QR_SCAN', 'MANUAL_OVERRIDE'
);

CREATE TYPE enrollment_status_enum AS ENUM (
    'ACTIVE', 'SUSPENDED', 'GRADUATED', 'WITHDRAWN'
);

CREATE TYPE intake_session_enum AS ENUM (
    'Morning', 'Evening', 'Weekend', 'Distance'
);

CREATE TYPE beacon_format_enum AS ENUM (
    'IBEACON', 'EDDYSTONE_UID'
);

-- ─── Tables ───────────────────────────────────────────────────────────────────

CREATE TABLE tenants (
    tenant_id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name                     VARCHAR(200) NOT NULL,
    domain                   VARCHAR(100) UNIQUE NOT NULL,
    rsa_key_id               VARCHAR(100) NOT NULL,
    attendance_threshold     SMALLINT    NOT NULL DEFAULT 75  CHECK (attendance_threshold BETWEEN 1 AND 100),
    checkin_window_minutes   SMALLINT    NOT NULL DEFAULT 120 CHECK (checkin_window_minutes > 0),
    auto_kill_minutes        SMALLINT    NOT NULL DEFAULT 180 CHECK (auto_kill_minutes > 0),
    rssi_threshold_dbm       SMALLINT    NOT NULL DEFAULT -65,
    logo_url                 TEXT,
    brand_color              VARCHAR(7),
    is_active                BOOLEAN     NOT NULL DEFAULT true,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Auth: user store (bcrypt passwords, TOTP, device binding key)
CREATE TABLE users (
    user_id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    email                    VARCHAR(255) NOT NULL,
    password_hash            VARCHAR(72)  NOT NULL,           -- bcrypt output
    role                     user_role_enum NOT NULL,
    full_name                VARCHAR(200) NOT NULL,
    is_active                BOOLEAN     NOT NULL DEFAULT true,
    -- TOTP (mandatory for VC + DQA_DIRECTOR)
    totp_secret_enc          TEXT,                            -- AES-256-GCM encrypted TOTP secret
    totp_enabled             BOOLEAN     NOT NULL DEFAULT false,
    totp_backup_codes_enc    TEXT,                            -- encrypted JSON array of 8 codes
    -- Device binding key issued to Coordinator PWA on first login
    device_binding_key_enc   TEXT,                            -- AES-256 key, encrypted at rest
    -- Audit
    last_login_at            TIMESTAMPTZ,
    failed_login_count       SMALLINT    NOT NULL DEFAULT 0,
    locked_until             TIMESTAMPTZ,
    created_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (tenant_id, email)
);

CREATE TABLE venues (
    venue_id                 VARCHAR(50)  PRIMARY KEY,
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    name                     VARCHAR(200) NOT NULL,
    building                 VARCHAR(100),
    floor                    SMALLINT,
    capacity                 SMALLINT,
    gps_latitude             DECIMAL(10, 8),
    gps_longitude            DECIMAL(11, 8),
    geofence_radius_meters   SMALLINT    NOT NULL DEFAULT 50
);

CREATE TABLE ble_beacons (
    beacon_uuid              UUID        PRIMARY KEY,
    tenant_id                UUID        NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    venue_id                 VARCHAR(50) NOT NULL REFERENCES venues(venue_id),
    format                   beacon_format_enum NOT NULL,
    major                    SMALLINT,
    minor                    SMALLINT,
    battery_level            SMALLINT,
    last_seen                TIMESTAMPTZ
);

CREATE TABLE courses (
    course_id                VARCHAR(50)  PRIMARY KEY,
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    name                     VARCHAR(200) NOT NULL,
    coordinator_id           VARCHAR(50)  NOT NULL,
    department               VARCHAR(100),
    school                   VARCHAR(100)
);

CREATE TABLE course_units (
    unit_id                  VARCHAR(50)  PRIMARY KEY,
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    course_id                VARCHAR(50)  NOT NULL REFERENCES courses(course_id),
    name                     VARCHAR(200) NOT NULL,
    year                     SMALLINT,
    semester                 SMALLINT     CHECK (semester IN (1, 2)),
    academic_year            VARCHAR(20)
);

CREATE TABLE students_extended (
    student_id               VARCHAR(50)  PRIMARY KEY,
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    full_name                VARCHAR(200) NOT NULL,
    email                    VARCHAR(255) NOT NULL,
    course_id                VARCHAR(50)  NOT NULL REFERENCES courses(course_id),
    intake_session           intake_session_enum,
    current_year             SMALLINT     CHECK (current_year BETWEEN 1 AND 6),
    semester                 SMALLINT     CHECK (semester IN (1, 2)),
    academic_year            VARCHAR(20)  NOT NULL,
    enrollment_status        enrollment_status_enum NOT NULL DEFAULT 'ACTIVE',
    hardware_fingerprint     VARCHAR(128),                    -- AES-256 encrypted
    rebind_count             SMALLINT     NOT NULL DEFAULT 0  CHECK (rebind_count <= 2),
    last_rebind_date         TIMESTAMPTZ,
    qr_public_key_hash       VARCHAR(128),
    qr_serial_number         VARCHAR(64)  UNIQUE,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at               TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    session_id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    coordinator_id           VARCHAR(50)  NOT NULL,
    unit_id                  VARCHAR(50)  NOT NULL REFERENCES course_units(unit_id),
    lecturer_id              VARCHAR(50),
    venue_id                 VARCHAR(50)  REFERENCES venues(venue_id),
    session_date             DATE         NOT NULL,
    gate_open_time           TIMESTAMPTZ,
    gate_close_time          TIMESTAMPTZ,
    checkin_window_start     TIMESTAMPTZ,
    checkin_window_end       TIMESTAMPTZ,
    coordinator_end_time     TIMESTAMPTZ,
    auto_close_time          TIMESTAMPTZ,
    session_status           session_status_enum NOT NULL DEFAULT 'PENDING_LECTURER',
    warden_id                VARCHAR(50),
    sync_status              sync_status_enum    NOT NULL DEFAULT 'PENDING',
    audit_flags              TEXT[]       NOT NULL DEFAULT '{}',
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE attendance_logs (
    log_id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    session_id               UUID         NOT NULL REFERENCES sessions(session_id),
    student_id               VARCHAR(50)  NOT NULL,
    checkin_timestamp        TIMESTAMPTZ  NOT NULL,
    device_fingerprint_hash  VARCHAR(128),
    beacon_rssi              SMALLINT,
    sequence_number          INTEGER      NOT NULL,
    entry_method             entry_method_enum NOT NULL DEFAULT 'QR_SCAN',
    override_officer_id      VARCHAR(50),
    override_reason          TEXT,
    audit_flags              TEXT[]       NOT NULL DEFAULT '{}'
    -- No DELETE on this table. Corrections use entry_method = MANUAL_OVERRIDE.
);

CREATE TABLE lecturer_attendance_logs (
    log_id                   UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    session_id               UUID         NOT NULL REFERENCES sessions(session_id),
    lecturer_id              VARCHAR(50)  NOT NULL,
    gate_open_time           TIMESTAMPTZ  NOT NULL,
    gate_close_time          TIMESTAMPTZ,
    contact_hours            DECIMAL(4,2),
    unit_id                  VARCHAR(50)  NOT NULL,
    venue_id                 VARCHAR(50),
    session_date             DATE         NOT NULL
);

CREATE TABLE hardware_vault (
    student_id               VARCHAR(50)  PRIMARY KEY,
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    fingerprint_hash         VARCHAR(128) NOT NULL,
    first_bound_at           TIMESTAMPTZ  NOT NULL,
    last_verified_at         TIMESTAMPTZ,
    academic_year            VARCHAR(20)  NOT NULL
);

CREATE TABLE admin_audit_log (
    audit_id                 UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    actor_id                 VARCHAR(50)  NOT NULL,
    actor_role               VARCHAR(30)  NOT NULL,
    action                   VARCHAR(100) NOT NULL,
    target_type              VARCHAR(50),
    target_id                VARCHAR(100),
    payload                  JSONB,
    ip_address               INET,
    occurred_at              TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Chunked sync upload state (Sync Receiver)
CREATE TABLE sync_uploads (
    upload_id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id                UUID         NOT NULL REFERENCES tenants(tenant_id) ON DELETE CASCADE,
    coordinator_id           VARCHAR(50)  NOT NULL,
    session_ids              UUID[]       NOT NULL,
    total_chunks             SMALLINT     NOT NULL,
    received_chunks          INTEGER[]    NOT NULL DEFAULT '{}',
    package_checksum         VARCHAR(64)  NOT NULL,
    status                   sync_status_enum NOT NULL DEFAULT 'PENDING',
    chunk_size_bytes         INTEGER      NOT NULL DEFAULT 65536,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    completed_at             TIMESTAMPTZ
);
