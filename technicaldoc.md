# QAAT — Technical Documentation
**Document:** technicaldoc.md  
**Version:** 1.0.0  
**Based on:** QAAT-SRS-2025-001 | ARCHITECT.md

---

> ## ⚠️ Current-state addendum (read first)
> This spec predates several shipped changes. Where it conflicts with the items
> below, **the items below win**, and [`flow.md`](flow.md) +
> [`docs/FLOWCHART.md`](docs/FLOWCHART.md) are the authoritative current view.
>
> - **Proximity is LAN-only.** A student is proven in the room by **being on the
>   coordinator's Wi-Fi hotspot LAN** (egress IP must match `sessions.coordinator_ip`)
>   **plus** a **live rotating room code**.
> - **Attendance is fully offline.** The coordinator's laptop is the room hotspot +
>   LAN server + database; every log is written locally the instant it is accepted,
>   then the closed session is **sealed (AES-256-GCM + device-bound HMAC-SHA256 +
>   SHA-256 checksum) and atomically synced** when internet returns.
> - **Passwordless identities.** Students are identified by **registration number**
>   only (no email/phone/password); lecturers by **staff ID**. Both get a **permanent
>   QR**; an **optional** email may be supplied solely to email that QR on
>   create/import (qr-generator `POST /api/v1/qr/email-link`).
> - **Passwordless student progress portal** (`GET /api/v1/student/progress?reg=&org=`):
>   read-only attendance %/eligibility, scoped to one institution.
> - **Standby coordinator** (migration 042): an absent coordinator may pre-authorise
>   an own-cohort student with a one-day code to run that day's session.
> - **Curriculum model:** a course is created independently of level; **levels** are
>   added inside the course (each with its own year × semester unit roadmap).
>   **Cohorts** can be applied across all courses at once.
> - **Threshold floor:** the exam-eligibility attendance threshold is clamped to a
>   **75% minimum**.

---

## 1. Backend API Specification

### 1.1 Base URL and Versioning

```
Production:   https://api.qaat.platform/api/v1/
Staging:      https://staging-api.qaat.platform/api/v1/
```

All endpoints require `Authorization: Bearer <JWT>` unless marked public. All requests/responses use `Content-Type: application/json`. All responses include a `X-Correlation-ID` header for tracing.

---

### 1.2 Authentication Endpoints

> **Architecture note:** The Auth Service is **fully custom-built in Go**. No third-party identity provider (Auth0, Firebase, Cognito, Clerk, etc.) is used. The service owns the full lifecycle: user registration, bcrypt password hashing, TOTP MFA secret management, RS256 JWT issuance, jti replay prevention via Redis blacklist, and token refresh/revocation. The TOTP implementation follows RFC 6238 (30-second TOTP with SHA-1, 6-digit codes).

#### POST /api/v1/auth/login
Authenticate a user and receive a JWT.

**Request:**
```json
{
  "email": "coordinator@university.edu",
  "password": "...",
  "totp_code": "123456",        // required for VC and DQA Director roles
  "tenant_id": "uuid-here"
}
```

**Response 200:**
```json
{
  "access_token": "eyJ...",
  "token_type": "Bearer",
  "expires_in": 86400,
  "jti": "unique-jwt-id",
  "role": "COORDINATOR",
  "coordinator_id": "C-001"
}
```

**Response 401:**
```json
{ "error": "INVALID_CREDENTIALS", "message": "..." }
```

JWT Payload Structure:
```json
{
  "sub": "user-id",
  "role": "COORDINATOR | QA_OFFICER | DQA_DIRECTOR | VC | ADMIN",
  "tenant_id": "uuid",
  "jti": "unique-replay-prevention-id",
  "iat": 1700000000,
  "exp": 1700086400
}
```

#### POST /api/v1/auth/refresh
Refresh a JWT before expiry. Invalidates the old `jti`.

#### POST /api/v1/auth/logout
Adds `jti` to the Redis blacklist.

---

### 1.3 QR Management Endpoints

#### POST /api/v1/qr/generate/batch — `[DQA_DIRECTOR, ADMIN]`
Trigger QR generation for all students in an academic year.

**Request:**
```json
{
  "tenant_id": "uuid",
  "academic_year": "2025/2026",
  "course_id": "BIT"          // optional, omit for all courses
}
```

**Response 202 (Accepted — async job):**
```json
{
  "job_id": "qr-gen-job-uuid",
  "status": "QUEUED",
  "estimated_count": 1200
}
```

#### POST /api/v1/qr/reissue — `[QA_OFFICER]`
Reissue a single student's QR code, invalidating the previous one.

**Request:**
```json
{
  "student_id": "REG-2024-0001",
  "reason_code": "LOST | DEVICE_CHANGE | COMPROMISED",
  "officer_id": "QA-007"
}
```

**Response 200:**
```json
{
  "student_id": "REG-2024-0001",
  "new_serial_number": "SN-20260001",
  "delivery_status": "EMAIL_QUEUED",
  "logged_at": "2026-05-26T10:00:00Z"
}
```

---

### 1.4 Daily Manifest Endpoint

#### GET /api/v1/manifest/daily — `[COORDINATOR]`
Returns the encrypted daily manifest for the requesting Coordinator.

**Headers:**
```
Authorization: Bearer <JWT>
X-Device-Fingerprint: <sha256-hash>
```

**Response 200:**
```json
{
  "manifest_version": "2026-05-26-C001",
  "generated_at": "2026-05-26T05:00:00Z",
  "expires_at": "2026-05-26T23:59:59Z",
  "sessions": [
    {
      "unit_id": "CS-301",
      "unit_name": "Database Systems",
      "venue_id": "LH-05",
      "scheduled_start": "2026-05-26T08:00:00Z",
      "scheduled_end": "2026-05-26T10:00:00Z"
    }
  ],
  "policy": {
    "attendance_threshold": 75,
    "checkin_window_minutes": 120,
    "auto_kill_minutes": 180
  },
  "institution_public_key": "-----BEGIN PUBLIC KEY-----\n...",
  "roster": {
    "CS-301": [
      {
        "student_id_hash": "sha256(REG-2024-0001)",
        "qr_serial_number": "SN-20260001"
      }
    ]
  }
}
```

Note: `student_id_hash` is SHA-256 of the registration number. Full PII is never sent to the PWA.

---

### 1.5 Session Management Endpoints

#### POST /api/v1/sessions/warden-link — `[COORDINATOR]`
Generate a one-time Warden Delegation Link.

**Request:**
```json
{
  "coordinator_id": "C-001",
  "warden_student_id": "REG-2024-0999",
  "session_unit_id": "CS-301",
  "venue_id": "LH-05",
  "session_date": "2026-05-26",
  "time_window_start": "2026-05-26T08:00:00Z",
  "time_window_end": "2026-05-26T10:30:00Z"
}
```

**Response 200:**
```json
{
  "delegation_link": "https://warden.qaat.platform/s/TOKEN",
  "token_hash": "sha256-of-token",
  "expires_at": "2026-05-26T10:35:00Z",
  "geofence": {
    "latitude": -1.2921,
    "longitude": 36.8219,
    "radius_meters": 50
  }
}
```

#### POST /api/v1/sessions/sync — `[COORDINATOR PWA]`
Initiate a chunked sync upload (see Section 4 for full protocol).

---

### 1.6 Synchronisation Endpoints

#### POST /api/v1/sync/init — `[COORDINATOR PWA]`
```json
// Request
{
  "coordinator_id": "C-001",
  "session_ids": ["uuid-1", "uuid-2"],
  "total_chunks": 8,
  "package_checksum": "sha256-of-full-package"
}

// Response 200
{
  "upload_id": "upload-uuid",
  "chunk_size_bytes": 65536,
  "server_timestamp": "2026-05-26T12:00:00Z"
}
```

#### POST /api/v1/sync/chunk/:upload_id/:chunk_index — `[COORDINATOR PWA]`
Body: raw AES-256 encrypted binary chunk

**Response 200:**
```json
{
  "upload_id": "upload-uuid",
  "chunk_index": 0,
  "next_expected": 1,
  "server_ack_signature": "RSA-signature-of-ack"
}
```

#### GET /api/v1/sync/resume/:upload_id — `[COORDINATOR PWA]`
```json
// Response 200
{
  "upload_id": "upload-uuid",
  "resume_from_chunk": 3,
  "received_chunks": [0, 1, 2]
}
```

#### POST /api/v1/sync/complete/:upload_id — `[COORDINATOR PWA]`
```json
// Response 200
{
  "status": "SYNCED",
  "records_written": 287,
  "duplicates_rejected": 0,
  "sync_timestamp": "2026-05-26T12:05:00Z"
}
```

---

### 1.7 Exam Eligibility Endpoints

#### GET /api/v1/eligibility/:student_id — `[QA_OFFICER, DQA_DIRECTOR, VC]`
```json
// Response 200
{
  "student_id": "REG-2024-0001",
  "academic_year": "2025/2026",
  "semester": 1,
  "units": [
    {
      "unit_id": "CS-301",
      "unit_name": "Database Systems",
      "sessions_held": 20,
      "sessions_attended": 17,
      "attendance_percentage": 85.0,
      "threshold": 75.0,
      "status": "ELIGIBLE"
    },
    {
      "unit_id": "CS-302",
      "unit_name": "Software Engineering",
      "sessions_held": 20,
      "sessions_attended": 14,
      "attendance_percentage": 70.0,
      "threshold": 75.0,
      "status": "EXAM_INELIGIBLE",
      "deficit_sessions": 1
    }
  ]
}
```

#### POST /api/v1/eligibility/clearance-token — `[DQA_DIRECTOR]`
Generate RSA-signed exam clearance QR tokens for all eligible students.

---

### 1.8 Dashboard Data Endpoints

All dashboard endpoints support query parameters:
- `school`, `department`, `course_id`, `unit_id`, `lecturer_id`, `student_id`
- `academic_year`, `semester`, `intake`, `session_type`
- `date_from`, `date_to`
- `page`, `page_size` (max 1000)

#### GET /api/v1/dashboard/vc/overview — `[VC]`
Returns IER, ghost lecture count, eligibility summary.

#### GET /api/v1/dashboard/dqa/thresholds — `[DQA_DIRECTOR]`
Read/write institutional policy parameters.

#### PUT /api/v1/dashboard/dqa/thresholds — `[DQA_DIRECTOR]`
```json
{
  "attendance_threshold": 80,
  "checkin_window_minutes": 120,
  "auto_kill_minutes": 180
}
```

#### GET /api/v1/dashboard/qa/live-sessions — `[QA_OFFICER]`
Returns currently ACTIVE sessions with coordinator and live student count.

#### POST /api/v1/dashboard/qa/device-reset — `[QA_OFFICER]`
Reset a student's hardware fingerprint binding.

```json
{
  "student_id": "REG-2024-0001",
  "officer_id": "QA-007",
  "reason_code": "LOST_PHONE | DAMAGED_DEVICE | OTHER",
  "reason_text": "Student changed phone due to damage"
}
```

---

## 2. Database Schema (PostgreSQL 15+)

### 2.1 Row-Level Security Setup

```sql
-- Applied to every table in QAAT schema
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON sessions
  FOR ALL
  USING (tenant_id = current_setting('app.current_tenant')::uuid);

-- API gateway sets this before every query:
SET LOCAL app.current_tenant = '<tenant-uuid-from-jwt>';
```

### 2.2 Core Tables

```sql
CREATE TABLE tenants (
  tenant_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name              VARCHAR(200) NOT NULL,
  domain            VARCHAR(100) UNIQUE NOT NULL,
  rsa_key_id        VARCHAR(100) NOT NULL,   -- reference to HSM key
  attendance_threshold    SMALLINT DEFAULT 75,
  checkin_window_minutes  SMALLINT DEFAULT 120,
  auto_kill_minutes       SMALLINT DEFAULT 180,
  logo_url          TEXT,
  brand_color       VARCHAR(7),
  created_at        TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE students_extended (
  student_id            VARCHAR(50) PRIMARY KEY,
  full_name             VARCHAR(200) NOT NULL,
  email                 VARCHAR(255) NOT NULL,
  course_id             VARCHAR(50) NOT NULL REFERENCES courses(course_id),
  intake_session        VARCHAR(20) CHECK (intake_session IN ('Morning','Evening','Weekend','Distance')),
  current_year          SMALLINT CHECK (current_year BETWEEN 1 AND 6),
  semester              SMALLINT CHECK (semester IN (1, 2)),
  academic_year         VARCHAR(20) NOT NULL,
  enrollment_status     VARCHAR(20) CHECK (enrollment_status IN ('ACTIVE','SUSPENDED','GRADUATED','WITHDRAWN')),
  hardware_fingerprint  VARCHAR(128),         -- AES-256 encrypted
  rebind_count          SMALLINT DEFAULT 0 CHECK (rebind_count <= 2),
  last_rebind_date      TIMESTAMPTZ,
  qr_public_key_hash    VARCHAR(128),
  qr_serial_number      VARCHAR(64) UNIQUE,
  tenant_id             UUID NOT NULL REFERENCES tenants(tenant_id),
  created_at            TIMESTAMPTZ DEFAULT now(),
  updated_at            TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE venues (
  venue_id       VARCHAR(50) PRIMARY KEY,
  name           VARCHAR(200) NOT NULL,
  building       VARCHAR(100),
  floor          SMALLINT,
  capacity       SMALLINT,
  gps_latitude   DECIMAL(10, 8),
  gps_longitude  DECIMAL(11, 8),
  geofence_radius_meters SMALLINT DEFAULT 50,
  tenant_id      UUID NOT NULL REFERENCES tenants(tenant_id)
);

CREATE TABLE courses (
  course_id       VARCHAR(50) PRIMARY KEY,
  name            VARCHAR(200) NOT NULL,
  coordinator_id  VARCHAR(50) NOT NULL,
  department      VARCHAR(100),
  school          VARCHAR(100),
  tenant_id       UUID NOT NULL REFERENCES tenants(tenant_id)
);

CREATE TABLE course_units (
  unit_id         VARCHAR(50) PRIMARY KEY,
  course_id       VARCHAR(50) NOT NULL REFERENCES courses(course_id),
  name            VARCHAR(200) NOT NULL,
  year            SMALLINT,
  semester        SMALLINT,
  academic_year   VARCHAR(20),
  tenant_id       UUID NOT NULL REFERENCES tenants(tenant_id)
);

CREATE TYPE session_status_enum AS ENUM (
  'PENDING_LECTURER', 'ACTIVE', 'CLOSED', 'AUTO_CLOSED'
);

CREATE TYPE sync_status_enum AS ENUM ('PENDING', 'SYNCED', 'FAILED');

CREATE TABLE sessions (
  session_id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  coordinator_id        VARCHAR(50) NOT NULL,
  unit_id               VARCHAR(50) NOT NULL REFERENCES course_units(unit_id),
  lecturer_id           VARCHAR(50),
  venue_id              VARCHAR(50) REFERENCES venues(venue_id),
  session_date          DATE NOT NULL,
  gate_open_time        TIMESTAMPTZ,
  gate_close_time       TIMESTAMPTZ,
  checkin_window_start  TIMESTAMPTZ,
  checkin_window_end    TIMESTAMPTZ,
  coordinator_end_time  TIMESTAMPTZ,
  auto_close_time       TIMESTAMPTZ,
  session_status        session_status_enum NOT NULL DEFAULT 'PENDING_LECTURER',
  warden_id             VARCHAR(50),
  sync_status           sync_status_enum NOT NULL DEFAULT 'PENDING',
  audit_flags           TEXT[] DEFAULT '{}',
  tenant_id             UUID NOT NULL REFERENCES tenants(tenant_id),
  created_at            TIMESTAMPTZ DEFAULT now()
);

CREATE TYPE entry_method_enum AS ENUM ('QR_SCAN', 'MANUAL_OVERRIDE');

CREATE TABLE attendance_logs (
  log_id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id            UUID NOT NULL REFERENCES sessions(session_id),
  student_id            VARCHAR(50) NOT NULL,
  checkin_timestamp     TIMESTAMPTZ NOT NULL,
  device_fingerprint_hash VARCHAR(128),
  sequence_number       INTEGER NOT NULL,
  entry_method          entry_method_enum NOT NULL DEFAULT 'QR_SCAN',
  override_officer_id   VARCHAR(50),
  override_reason       TEXT,
  audit_flags           TEXT[] DEFAULT '{}',
  tenant_id             UUID NOT NULL REFERENCES tenants(tenant_id)
  -- NOTE: No DELETE permission on this table. MANUAL_OVERRIDE creates new rows.
);

CREATE TABLE lecturer_attendance_logs (
  log_id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  session_id      UUID NOT NULL REFERENCES sessions(session_id),
  lecturer_id     VARCHAR(50) NOT NULL,
  gate_open_time  TIMESTAMPTZ NOT NULL,
  gate_close_time TIMESTAMPTZ,
  contact_hours   DECIMAL(4,2),   -- computed: gate_close - gate_open in hours
  unit_id         VARCHAR(50) NOT NULL,
  venue_id        VARCHAR(50),
  session_date    DATE NOT NULL,
  tenant_id       UUID NOT NULL REFERENCES tenants(tenant_id)
);

CREATE TABLE hardware_vault (
  student_id          VARCHAR(50) PRIMARY KEY,
  fingerprint_hash    VARCHAR(128) NOT NULL,   -- SHA-256 of fingerprint
  first_bound_at      TIMESTAMPTZ NOT NULL,
  last_verified_at    TIMESTAMPTZ,
  academic_year       VARCHAR(20) NOT NULL,    -- purged at year end
  tenant_id           UUID NOT NULL REFERENCES tenants(tenant_id)
);

CREATE TABLE admin_audit_log (
  audit_id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  actor_id      VARCHAR(50) NOT NULL,
  actor_role    VARCHAR(30) NOT NULL,
  action        VARCHAR(100) NOT NULL,
  target_type   VARCHAR(50),
  target_id     VARCHAR(100),
  payload       JSONB,
  ip_address    INET,
  occurred_at   TIMESTAMPTZ DEFAULT now(),
  tenant_id     UUID NOT NULL REFERENCES tenants(tenant_id)
);
```

### 2.3 Key Indexes

```sql
-- Attendance lookup performance
CREATE INDEX idx_attendance_session ON attendance_logs(session_id, tenant_id);
CREATE INDEX idx_attendance_student ON attendance_logs(student_id, tenant_id);
CREATE INDEX idx_sessions_coordinator ON sessions(coordinator_id, session_date, tenant_id);
CREATE INDEX idx_sessions_unit ON sessions(unit_id, session_date, tenant_id);

-- Eligibility computation
CREATE INDEX idx_attendance_student_unit ON attendance_logs(student_id, session_id) 
  INCLUDE (checkin_timestamp);

-- Tenant isolation (composite)
CREATE INDEX idx_students_tenant ON students_extended(tenant_id, enrollment_status);
```

### 2.4 Attendance Percentage View

```sql
CREATE MATERIALIZED VIEW student_attendance_summary AS
SELECT
  al.student_id,
  cu.unit_id,
  cu.name AS unit_name,
  COUNT(DISTINCT s.session_id) AS sessions_held,
  COUNT(DISTINCT al.session_id) AS sessions_attended,
  ROUND(
    COUNT(DISTINCT al.session_id)::DECIMAL / 
    NULLIF(COUNT(DISTINCT s.session_id), 0) * 100, 2
  ) AS attendance_percentage,
  al.tenant_id
FROM course_units cu
JOIN sessions s ON s.unit_id = cu.unit_id 
  AND s.session_status IN ('CLOSED', 'AUTO_CLOSED')
LEFT JOIN attendance_logs al ON al.session_id = s.session_id
GROUP BY al.student_id, cu.unit_id, cu.name, al.tenant_id;

-- Refresh triggered after each successful atomic sync
CREATE INDEX ON student_attendance_summary(student_id, unit_id);
```

---

## 3. Coordinator PWA — Technical Specification

### 3.1 Technology Stack

| Component | Technology |
|---|---|
| Framework | React 18 + TypeScript (or Svelte — lightweight preferred) |
| Service Worker | Workbox 7 (Google) — cache strategies + Background Sync |
| Local DB | Dexie.js (IndexedDB wrapper with TypeScript support) |
| Encryption | SubtleCrypto API (Web Crypto — AES-256-GCM) |
| QR Validation | jsrsasign (RSA-2048 + SHA-256 verification) |
| LAN Server | Service Worker intercepts on port 8080 via fetch handler |
| State Management | Zustand or XState (for session state machine) |
| Build | Vite + PWA plugin (vite-plugin-pwa) |

### 3.2 IndexedDB Schema (Dexie.js)

```typescript
const db = new Dexie('QAATVault');

db.version(1).stores({
  // Encrypted daily manifest cache
  daily_manifest: 'manifest_id, date, coordinator_id',
  
  // Session vault
  sessions: 'session_id, status, date, unit_id',
  
  // Attendance records (local, pre-sync)
  attendance_records: 'log_id, session_id, student_id_hash, timestamp',
  
  // Hardware fingerprint bindings (hashed only)
  hardware_vault: 'student_id_hash, fingerprint_hash, academic_year',
  
  // Sync outbox
  outbox_queue: '++id, session_id, status, created_at, last_attempt',
  
  // Policy cache
  policy_cache: 'tenant_id, fetched_at'
});
```

All values stored via `crypto.subtle.encrypt` (AES-256-GCM) with device-specific key derived from server-issued secret.

### 3.3 Hardware Fingerprint Algorithm

```typescript
async function computeHardwareFingerprint(): Promise<string> {
  const components = [
    navigator.userAgent,
    `${screen.width}x${screen.height}`,
    screen.colorDepth.toString(),
    window.devicePixelRatio.toString(),
    navigator.hardwareConcurrency.toString(),
    await getCanvasHash()
  ];
  
  const combined = components.join('|');
  const encoded = new TextEncoder().encode(combined);
  const hashBuffer = await crypto.subtle.digest('SHA-256', encoded);
  return bufferToHex(hashBuffer);
}

async function getCanvasHash(): Promise<string> {
  const canvas = document.createElement('canvas');
  const ctx = canvas.getContext('2d')!;
  ctx.textBaseline = 'top';
  ctx.font = '14px Arial';
  ctx.fillText('QAAT-fingerprint-probe', 2, 2);
  const dataUrl = canvas.toDataURL();
  const encoded = new TextEncoder().encode(dataUrl);
  const hash = await crypto.subtle.digest('SHA-256', encoded);
  return bufferToHex(hash);
}
```

### 3.4 QR Validation Sequence

```typescript
async function validateStudentQR(qrPayload: string, session: ActiveSession): Promise<ValidationResult> {
  // Step 1: RSA-2048 signature verification
  const { valid, data } = verifyRSASignature(qrPayload, session.institutionPublicKey);
  if (!valid) return { status: 'REJECTED', reason: 'INVALID_SIGNATURE' };
  
  // Step 2: Expiry check
  if (new Date(data.expiry_date) < new Date()) 
    return { status: 'REJECTED', reason: 'QR_EXPIRED' };
  
  // Step 3: Tenant ID match
  if (data.tenant_id !== session.tenantId) 
    return { status: 'REJECTED', reason: 'TENANT_MISMATCH' };
  
  // Step 4: Roster lookup (SHA-256 of student_id against cached hashes)
  const studentIdHash = await sha256(data.student_registration_number);
  if (!session.rosterHashes.includes(studentIdHash)) 
    return { status: 'REJECTED', reason: 'NOT_ON_ROSTER' };
  
  // Step 5: proximity check — same LAN as the coordinator + live rotating room code
  if (!onSameLAN(clientIP, session) || !verifyRoomCode(submittedCode, session)) 
    return { status: 'REJECTED', reason: 'PROXIMITY_FAILED' };
  
  // Step 6: Hardware fingerprint check
  const deviceFingerprint = await computeHardwareFingerprint();
  const storedFingerprint = await db.hardware_vault
    .get({ student_id_hash: studentIdHash });
  
  if (storedFingerprint && storedFingerprint.fingerprint_hash !== deviceFingerprint) 
    return { status: 'REJECTED', reason: 'DEVICE_MISMATCH', flag: 'DEVICE_MISMATCH' };
  
  if (!storedFingerprint) {
    // First scan — bind device
    await db.hardware_vault.put({ student_id_hash: studentIdHash, fingerprint_hash: deviceFingerprint, academic_year: session.academicYear });
  }
  
  // Step 7: Duplicate scan check
  const existing = await db.attendance_records
    .where({ session_id: session.sessionId, student_id_hash: studentIdHash }).first();
  if (existing) return { status: 'REJECTED', reason: 'DUPLICATE_SCAN' };
  
  // Step 8: One-device-per-session check
  const deviceUsed = await db.attendance_records
    .where({ session_id: session.sessionId, fingerprint_hash: deviceFingerprint }).first();
  if (deviceUsed) return { status: 'REJECTED', reason: 'DEVICE_ALREADY_USED' };
  
  // All checks passed — record attendance
  await db.attendance_records.add({
    log_id: crypto.randomUUID(),
    session_id: session.sessionId,
    student_id_hash: studentIdHash,
    device_fingerprint_hash: deviceFingerprint,
    timestamp: new Date().toISOString(),
    sequence_number: await getNextSequence(session.sessionId)
  });
  
  return { status: 'PRESENT', student_id_hash: studentIdHash };
}
```

### 3.5 Session Package Sealing (Pre-Sync)

```typescript
async function sealSessionPackage(sessionId: string): Promise<EncryptedPackage> {
  const session = await db.sessions.get(sessionId);
  const records = await db.attendance_records.where({ session_id: sessionId }).toArray();
  
  const payload = JSON.stringify({
    session,
    attendance_records: records,
    sealed_at: new Date().toISOString(),
    coordinator_id: currentCoordinatorId,
    package_version: '1.0'
  });
  
  // Sign with HMAC for integrity
  const hmac = await computeHMAC(payload, deviceSecret);
  
  // Encrypt with AES-256-GCM
  const encrypted = await crypto.subtle.encrypt(
    { name: 'AES-GCM', iv: crypto.getRandomValues(new Uint8Array(12)) },
    await getDeviceKey(),
    new TextEncoder().encode(payload)
  );
  
  // Create JWT wrapper for authentication
  const jwtWrapper = createJWT({ 
    package_id: sessionId, 
    hmac, 
    chunk_count: Math.ceil(encrypted.byteLength / CHUNK_SIZE) 
  }, coordinatorToken);
  
  return { encrypted, hmac, jwtWrapper };
}
```

---

## 4. Proximity Engine — same-LAN + rotating room code

Proximity is proven without any extra radio hardware, using two independent signals
that both require the student to be physically in the room.

### 4.1 Same-LAN check

The coordinator's device is the room's Wi-Fi hotspot, LAN server and database. When a
session opens, the coordinator's address is stamped on the session as
`sessions.coordinator_ip`. Every check-in is accepted only if the submitting device's
egress IP matches that stamp — i.e. it is on this room's access point.

```typescript
function onSameLAN(clientIP: string, session: Session): boolean {
  return normaliseIP(clientIP) === normaliseIP(session.coordinatorIP);
}
```

Caddy is the only front door and is the sole writer of `X-Forwarded-For`, so the
client IP cannot be spoofed by an upstream hop.

### 4.2 Rotating room code

The coordinator displays a 6-digit code on the projector that changes every
`StepSeconds`. It is an HMAC-based TOTP (RFC 6238 style) keyed by a per-session secret
that never leaves the server, so it cannot be precomputed or shared ahead of time — a
student must read it live off the screen.

See `backend/api-gateway/internal/checkin/roomcode.go` for the implementation.

---

## 5. Security Implementation Details

### 5.1 AES-256-GCM Key Derivation (PWA)

```typescript
async function deriveDeviceKey(serverIssuedSecret: string): Promise<CryptoKey> {
  const baseKey = await crypto.subtle.importKey(
    'raw',
    new TextEncoder().encode(serverIssuedSecret),
    { name: 'HKDF' },
    false,
    ['deriveKey']
  );
  
  return crypto.subtle.deriveKey(
    {
      name: 'HKDF',
      hash: 'SHA-256',
      salt: new TextEncoder().encode('QAAT-IndexedDB-Salt-v1'),
      info: new TextEncoder().encode('coordinator-vault-key')
    },
    baseKey,
    { name: 'AES-GCM', length: 256 },
    false,
    ['encrypt', 'decrypt']
  );
}
```

### 5.2 RSA QR Code Signing (Node.js QR Service)

```javascript
const { privateKey, publicKey } = crypto.generateKeyPairSync('rsa', {
  modulusLength: 2048,
  publicKeyEncoding: { type: 'spki', format: 'pem' },
  privateKeyEncoding: { type: 'pkcs8', format: 'pem' }
});

function generateStudentQR(student, tenantPrivateKey, expiryDate) {
  const payload = {
    student_id: student.student_id,
    tenant_id: student.tenant_id,
    academic_year: student.academic_year,
    serial_number: generateCryptoSerialNumber(),
    expiry_date: expiryDate,
    issued_at: new Date().toISOString()
  };
  
  // RSA-2048 SHA-256 signature
  const sign = crypto.createSign('RSA-SHA256');
  sign.update(JSON.stringify(payload));
  payload.signature = sign.sign(tenantPrivateKey, 'base64');
  
  // HMAC integrity check
  payload.hmac = crypto.createHmac('sha256', tenantHmacSecret)
    .update(JSON.stringify(payload))
    .digest('hex');
  
  return QRCode.toBuffer(JSON.stringify(payload), { width: 1024, errorCorrectionLevel: 'H' });
}
```

### 5.3 Rate Limiting (API Gateway)

```go
// Per-coordinator rate limiting: 50 requests/second
var limiter = rate.NewLimiter(rate.Limit(50), 100)

func AttendanceScanMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        coordinatorID := r.Header.Get("X-Coordinator-ID")
        coordinatorLimiter := getLimiterForCoordinator(coordinatorID)
        
        if !coordinatorLimiter.Allow() {
            http.Error(w, `{"error":"RATE_LIMIT_EXCEEDED"}`, http.StatusTooManyRequests)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

---

## 6. Email Service Integration

### 6.1 QR Code Delivery

```javascript
// Bulk delivery with rate limiting (100 emails/second max)
async function deliverQRCodesBatch(students, tenantConfig) {
  const batches = chunk(students, 100); // 100 per second
  
  for (const batch of batches) {
    await Promise.all(batch.map(student => 
      emailService.send({
        from: `noreply@${tenantConfig.domain}`,
        to: student.email,
        subject: `Your ${tenantConfig.name} Attendance QR Code — ${student.academic_year}`,
        html: renderQREmailTemplate(student, tenantConfig),
        attachments: [{
          filename: `qr-${student.student_id}.png`,
          content: student.qrImageBuffer,
          contentType: 'image/png'
        }]
      })
    ));
    
    await sleep(1000); // Enforce rate limit
  }
}
```

---

## 7. Error Codes Reference

| Code | HTTP | Meaning |
|---|---|---|
| `INVALID_SIGNATURE` | 422 | QR RSA signature verification failed |
| `QR_EXPIRED` | 422 | QR expiry date is in the past |
| `TENANT_MISMATCH` | 422 | QR belongs to different institution |
| `NOT_ON_ROSTER` | 403 | Student not enrolled in this Course Unit |
| `PROXIMITY_FAILED` | 422 | Device not on the coordinator's LAN, or wrong room code |
| `DEVICE_MISMATCH` | 403 | Hardware fingerprint does not match stored binding |
| `DUPLICATE_SCAN` | 409 | Student already marked present in this session |
| `DEVICE_ALREADY_USED` | 409 | This device already registered another student |
| `SESSION_NOT_ACTIVE` | 403 | Session is not in ACTIVE state |
| `GATE_NOT_OPEN` | 403 | Lecturer Gate-Open has not been performed |
| `CHECKIN_WINDOW_CLOSED` | 403 | T+120 min check-in window has expired |
| `RATE_LIMIT_EXCEEDED` | 429 | 50 req/s per coordinator exceeded |
| `INVALID_CREDENTIALS` | 401 | Authentication failed |
| `TOKEN_EXPIRED` | 401 | JWT has expired |
| `TOKEN_REVOKED` | 401 | JWT jti is blacklisted |
| `MFA_REQUIRED` | 403 | TOTP code required for this role |
| `GEOFENCE_FAIL` | 403 | Warden device outside registered venue coordinates |
| `SYNC_DEDUP_REJECTED` | 409 | Record already exists (Vector Clock collision) |

---

## 8. Service Worker Sync Logic

```javascript
// sw.js — Outbox processing
self.addEventListener('sync', async (event) => {
  if (event.tag === 'qaat-sync-outbox') {
    event.waitUntil(processOutboxQueue());
  }
});

async function processOutboxQueue() {
  const db = await openQAATVault();
  const pendingPackages = await db.outbox_queue
    .where('status').equals('PENDING')
    .toArray();
  
  for (const pkg of pendingPackages) {
    try {
      await db.outbox_queue.update(pkg.id, { status: 'UPLOADING' });
      await uploadChunked(pkg);
      await db.outbox_queue.update(pkg.id, { status: 'SYNCED', synced_at: new Date() });
    } catch (err) {
      await db.outbox_queue.update(pkg.id, { 
        status: 'FAILED', 
        last_error: err.message,
        attempt_count: (pkg.attempt_count || 0) + 1
      });
      // Re-register sync for retry
      await self.registration.sync.register('qaat-sync-outbox');
    }
  }
}
```

---

## 9. Deployment Configuration

### 9.1 Docker Compose (Development)

```yaml
version: '3.9'
services:
  api-gateway:
    build: ./services/api-gateway
    ports: ["8443:8443"]
    environment:
      - DB_URL=postgresql://qaat:secret@postgres:5432/qaat
      - REDIS_URL=redis://redis:6379
      - JWT_SECRET=${JWT_SECRET}
    depends_on: [postgres, redis]

  qr-service:
    build: ./services/qr-generator
    environment:
      - HSM_ENDPOINT=${HSM_ENDPOINT}
      - SMTP_HOST=${SMTP_HOST}

  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: qaat
      POSTGRES_USER: qaat
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - ./db/init:/docker-entrypoint-initdb.d
      - pgdata:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    command: redis-server --requirepass ${REDIS_PASSWORD}

  coordinator-pwa:
    build: ./apps/coordinator-pwa
    ports: ["3000:3000"]

  admin-dashboards:
    build: ./apps/admin-dashboards
    ports: ["3001:3001"]

volumes:
  pgdata:
```

### 9.2 Kubernetes Resource Limits

```yaml
resources:
  api-gateway:
    requests: { cpu: "500m", memory: "512Mi" }
    limits: { cpu: "2000m", memory: "2Gi" }
  qr-service:
    requests: { cpu: "250m", memory: "256Mi" }
    limits: { cpu: "1000m", memory: "1Gi" }
  sync-receiver:
    requests: { cpu: "500m", memory: "512Mi" }
    limits: { cpu: "2000m", memory: "2Gi" }
```

### 9.3 Environment Variables

| Variable | Service | Description |
|---|---|---|
| `JWT_SECRET` | API Gateway | 256-bit secret for JWT signing |
| `DB_URL` | All backend | PostgreSQL connection string |
| `REDIS_URL` | All backend | Redis connection string |
| `HSM_ENDPOINT` | QR Service | Hardware Security Module endpoint |
| `SMTP_HOST` / `SMTP_PORT` | Notification | Email relay configuration |
| `SENDGRID_API_KEY` | Notification | SendGrid API key (if used) |
| `CDN_MANIFEST_BUCKET` | Manifest Service | S3/GCS bucket for Daily Manifest CDN |
| `TENANT_DEFAULT_THRESHOLD` | API Gateway | Default attendance threshold (75) |
