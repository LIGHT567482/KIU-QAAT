# QAAT — System Architecture Design
**Document:** ARCHITECT.md  
**Version:** 1.2.0 (updated 2026-06-14 session 10)
**Based on:** QAAT-SRS-2025-001

---

## 1. Architectural Overview

QAAT follows a **Decentralised Edge-Cloud Hybrid Architecture**. The core principle is that all attendance operations during a live session are **fully offline**, executed on the Coordinator's laptop, which is simultaneously the **room Wi-Fi hotspot**, the **LAN server**, and the **local database**. The cloud is only involved in pre-session data provisioning and post-session synchronisation.

> **Proximity model (current):** BLE beacons / RSSI were **removed** (migration 039). Proximity is now proven by the phone **being on the coordinator's hotspot LAN** plus the **live rotating room code**. In the diagram below, read the "beacon" box as the laptop's hotspot — every phone joins it and submits over the LAN.

```
┌─────────────────────────────────────────────────────────────────────┐
│                        PHYSICAL CLASSROOM                           │
│                                                                     │
│  ┌──────────────┐  joins AP  ┌──────────────┐  LAN (HTTPS:8443)   │
│  │  Phones on   │──────────►│  Coordinator  │◄────────────────────┤
│  │  the laptop  │  same-LAN │  PWA (Edge    │                     │
│  │  Wi-Fi AP    │ + rm code │   Server)     │   ┌──────────────┐  │
│  └──────────────┘           │   IndexedDB   │   │   Students   │  │
│                             │   AES-256     │   │  (Camera QR) │  │
│                             └──────┬────────┘   └──────────────┘  │
│                                    │                                │
└────────────────────────────────────┼────────────────────────────────┘
                                     │ HTTPS TLS 1.3 (Port 443)
                                     │ (Post-session Chunked Sync)
┌────────────────────────────────────▼────────────────────────────────┐
│                        QAAT CLOUD INFRASTRUCTURE                    │
│                                                                     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌──────────┐  │
│  │   Auth      │  │  QR Gen     │  │  Session    │  │  Sync    │  │
│  │  Service    │  │ Microservice│  │  Manager    │  │ Receiver │  │
│  │  (JWT/MFA)  │  │ (RSA/Node)  │  │   (Go)      │  │  (Go)    │  │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └────┬─────┘  │
│         │                │                │               │         │
│  ┌──────▼────────────────▼────────────────▼───────────────▼──────┐ │
│  │              API Gateway (Go 1.21+) — /api/v1/               │ │
│  └──────────────────────────────┬────────────────────────────────┘ │
│                                 │                                   │
│  ┌──────────────────────────────▼────────────────────────────────┐ │
│  │         PostgreSQL 15+ (Primary-Replica, RLS Enabled)        │ │
│  │              Sidecar Schema  |  tenant_id partitioning        │ │
│  └───────────────────────────────────────────────────────────────┘ │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐  │
│  │  Redis 7+    │  │  CDN (Edge   │  │   Reporting Engine       │  │
│  │  (JWT Cache  │  │  Manifest    │  │   (Dashboards / PDF)     │  │
│  │  + Sync Q)   │  │  Delivery)   │  │                          │  │
│  └──────────────┘  └──────────────┘  └──────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 2. Architectural Patterns

| Pattern | Application in QAAT |
|---|---|
| **Edge Computing** | Coordinator PWA acts as the edge server; all session logic runs offline |
| **Offline-First** | IndexedDB as local data vault; sync is opportunistic, not required |
| **Microservices** | Auth, QR Gen, Session Mgmt, Sync Receiver, Reporting run independently |
| **Multi-Tenant SaaS** | PostgreSQL RLS + tenant_id on all tables; per-tenant policy engine |
| **Event Sourcing (append-only)** | Attendance logs are immutable; corrections create ADJUSTMENT entries |
| **CQRS (light)** | Write path: Coordinator PWA → Sync pipeline. Read path: Reporting Engine → Dashboards |
| **State Machine** | Session lifecycle enforced as a strict state machine (see Section 5) |
| **Lecturer Roster** | `lecturers` + `lecturer_assignments` tables link teaching staff to course units per academic period; coordinator selects from a filtered dropdown before opening a session |
| **Course Roadmap** | `courses.total_years` drives a Year × Semester grid of `course_units`; admin populates units per slot; coordinator's manifest is filtered to the active semester only |
| **Active Semester** | `tenants.active_academic_year` + `tenants.active_semester` control which semester's units appear in the coordinator's daily manifest; 0 = show all |

---

## 3. Component Architecture

### 3.1 Coordinator PWA (Edge Server)

The most critical component. Runs entirely in the browser as an installable PWA.

```
┌─────────────────────────────────────────────────────────┐
│                  COORDINATOR PWA                         │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │             SERVICE WORKER                       │   │
│  │  • Background Sync (Outbox Queue processing)     │   │
│  │  • Cache API (offline asset management)          │   │
│  │  • Connectivity monitor                          │   │
│  └──────────────────────┬──────────────────────────┘   │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐   │
│  │             APPLICATION LAYER                    │   │
│  │  ┌─────────────┐  ┌──────────────────────────┐  │   │
│  │  │  Session    │  │   QR Validator Engine     │  │   │
│  │  │  State      │  │   (RSA-2048 + HMAC)       │  │   │
│  │  │  Machine    │  └──────────────────────────┘  │   │
│  │  └─────────────┘  ┌──────────────────────────┐  │   │
│  │  ┌─────────────┐  │   LAN Proximity Engine   │  │   │
│  │  │  Sync       │  │  (same-LAN + room code)  │  │   │
│  │  │  Queue      │  └──────────────────────────┘  │   │
│  │  │  Manager    │  ┌──────────────────────────┐  │   │
│  │  └─────────────┘  │  Hardware Fingerprint    │  │   │
│  │                   │  Binding Engine          │  │   │
│  │                   └──────────────────────────┘  │   │
│  └──────────────────────┬──────────────────────────┘   │
│                         │                               │
│  ┌──────────────────────▼──────────────────────────┐   │
│  │             INDEXEDDB LAYER (AES-256)            │   │
│  │  • session_vault    • attendance_logs            │   │
│  │  • outbox_queue     • hardware_vault             │   │
│  │  • daily_manifest   • policy_cache              │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  LOCAL LAN SERVER (HTTP:8080) ── Student scan endpoint  │
└─────────────────────────────────────────────────────────┘
```

**Key responsibilities:**
- Daily Manifest fetch and encrypted local storage
- Session initialisation on the coordinator's hotspot (captures the coordinator's LAN IP for the same-network proximity gate)
- Lecturer gate-open/close QR code generation and verification
- Student check-in validation (signed QR + roster + live room code + same-LAN proximity + one-device-one-person + duplicate check)
- AES-256 session package sealing and outbox queuing
- Chunked Resume-Sync upload via Service Worker

### 3.2 Cloud Microservices

#### 3.2.1 Authentication Service (Custom — No Third-Party Provider)
> **Decision:** Auth is built in-house using Go. No Auth0, Firebase Auth, AWS Cognito, or any external identity provider is used. The full auth lifecycle (user store, password hashing, JWT issuance, TOTP MFA, token refresh, jti blacklist) is owned by this service.

- Custom user store in PostgreSQL (`users` table with bcrypt-hashed passwords)
- Issues RS256 JWT tokens (24-hour lifetime, `jti` claim stored in Redis for replay prevention)
- TOTP MFA (RFC 6238) enforced for VC and DQA Director accounts — enrolment flow generates TOTP secret, QR code rendered for authenticator app
- Token refresh rotates `jti` and blacklists the old one in Redis
- Logout adds `jti` to Redis blacklist (TTL = remaining token lifetime)
- Manages device binding secrets (AES-256 keys) issued to Coordinator PWA on first login
- Endpoint: `/api/v1/auth/`

#### 3.2.2 QR Generation Microservice (Node.js 20+)
- Manages RSA-2048 key pairs per tenant (stored in HSM/software vault)
- Generates student QR PNG (1024×1024 minimum) with signed JSON payload
- Annual key rotation per tenant
- Integrates with email service for delivery
- Endpoint: `/api/v1/qr/`

#### 3.2.3 Session Management Service (Go)
- Manages session lifecycle API
- Warden delegation link generation and GPS geofence validation
- Exam clearance token generation (RSA-signed QR per eligible student)
- Endpoint: `/api/v1/sessions/`

#### 3.2.4 Synchronisation Receiver Service (Go)
- Accepts chunked, AES-256 encrypted session packages from PWA
- Returns cryptographically signed acknowledgement per chunk
- Server-side deduplication via Vector Clock (Tenant ID + Session ID + Coordinator ID + Sequence Number), enforced by a partial UNIQUE index on `attendance_logs` for `QR_SCAN` rows
- Triggers eligibility recomputation after successful atomic sync
- Endpoint: `/api/v1/sync/`

#### 3.2.5 Reporting Engine
- Aggregates attendance data for multi-role dashboards
- Computes attendance percentages and exam eligibility post-sync
- Generates PDF audit reports and CSV/JSON exports
- Endpoint: `/api/v1/reports/`

#### 3.2.6 Notification Service
- Handles SMTP/SendGrid/Mailgun/AWS SES integration
- Bulk QR delivery (rate-limited: 100 emails/second)
- Push notifications to Coordinator PWA (Warden data received alerts)
- Endpoint: Internal only

### 3.3 Administrative Dashboards (Frontend)

Three role-differentiated SPAs sharing a common component library:

| Dashboard | Role | Key Features |
|---|---|---|
| VC Dashboard | Vice Chancellor | IER, ghost lecture detection, full-scope filtering, PDF audit export |
| DQA Dashboard | DQA Director | Threshold policy management, exam eligibility list, attendance heatmap |
| QA Officer Dashboard | QA Officer | Live session monitor, anomaly alerts, device binding reset, manual corrections |
| Coordinator View | Coordinator | Session status, live count, sync queue — no historical data |
| Student Status Portal | Student | Personal attendance %, eligibility status, offline cache |

---

## 4. Data Architecture

### 4.1 Database Design Principles

- **PostgreSQL 15+** with Row-Level Security (RLS) on all tables
- **Sidecar Schema**: logically isolated from university SIS; linked via `student_id` foreign key
- **Append-only attendance logs**: no DELETE on `attendance_logs` table; corrections are new rows with `entry_method = MANUAL_OVERRIDE`
- **Horizontal sharding by `tenant_id`** for scale to 500+ universities, 1M+ students
- **Hot/Warm/Cold tiering**: active semester (hot), completed semester (warm/compressed), >7 years (purged except aggregates)

### 4.2 Core Schema Relationships

```
tenants (1) ──────────────── (N) users (VC, DQA, QA Officers, Coordinators)
tenants (1) ──────────────── (N) students_extended
tenants (1) ──────────────── (N) courses
courses (1) ───────────────── (N) course_units
courses (1) ───────────────── (1) coordinators
course_units (1) ──────────── (N) sessions
sessions (1) ──────────────── (N) attendance_logs
students_extended (1) ─────── (N) attendance_logs
students_extended (1) ─────── (1) hardware_vault
sessions (0..1) ────────────── (1) lecturer_attendance_logs
tenants (1) ──────────────── (N) venues
sessions (1) ─── stamp ────── coordinator_ip   (same-LAN proximity; ble_beacons dropped — mig 039)
coordinator_delegations (N) ─ (1) course_offerings   (standby coordinator — mig 042)
```

### 4.3 Data Flow — Session Lifecycle

```
[Daily Fetch]
Cloud API ──(HTTPS)──► PWA IndexedDB (hashed roster + policy + public key)

[Session Open — Online]
Coordinator selects unit + lecturer ──► POST /api/v1/sessions/open
                                            ├─ INSERT sessions (session_status=ACTIVE)
                                            └─ INSERT lecturer_attendance_logs (gate_open_time)

[Session Active — Rotating Code]
Server: HOTP code rotates every 15s ──► Coordinator display (6-digit code)
Student: scan personal QR + type code ──► POST /api/v1/checkin
                                            ├─ 8 anti-fraud checks
                                            └─ INSERT attendance_logs (PRESENT / rejection reason)

[Session Close]
Coordinator taps "End Session" ──► POST /api/v1/sessions/{id}/close
                                       ├─ UPDATE sessions (status=CLOSED, gate_close_time)
                                       └─ UPDATE lecturer_attendance_logs
                                              └─ gate_close_time + contact_hours=(close-open)/3600

[Post-Session Sync]
IndexedDB Outbox ──► Service Worker ──(Chunked HTTPS)──► Sync Receiver
                                                              ├─ Deduplication
                                                              ├─ Write to PostgreSQL
                                                              └─ Trigger eligibility compute

[Lecturer Attendance Audit]
Admin ──► GET /api/v1/admin/tenants/{id}/lecturer-attendance/summary
              └─ per-lecturer: total_sessions, total_contact_hours, avg_contact_hours
Admin ──► GET /api/v1/admin/tenants/{id}/lecturer-attendance
              └─ full log: lecturer, unit, date, gate_open, gate_close, contact_hours, status
```

---

## 5. Session State Machine

```
                    ┌─────────────┐
                    │    IDLE     │
                    └──────┬──────┘
                           │ Coordinator selects Course Unit
                           │ + opens session on the hotspot (LAN IP stamped)
                    ┌──────▼──────────────┐
                    │ PENDING_LECTURER    │ ◄── Gate-Open QR displayed (15-min TTL)
                    └──────┬──────────────┘
                           │ Lecturer scans Gate-Open QR
                           │ + Staff QR verified against roster
                    ┌──────▼──────┐
                    │   ACTIVE    │ ◄── Student scans accepted (T+0 to T+120min)
                    └──┬────────┬─┘
                       │        │
          Coordinator  │        │ Auto-expiry triggers:
          presses      │        │  (a) T+120min from gate-open (check-in window)
          "End Session"│        │  (b) T+180min from initialisation
                    ┌──▼────────▼──────┐
                    │     CLOSED       │  ──► Session sealed + placed in Outbox
                    │  or AUTO_CLOSED  │
                    └──────────────────┘
```

---

## 6. Security Architecture

### 6.1 Cryptographic Stack

| Layer | Algorithm | Purpose |
|---|---|---|
| QR Signing | RSA-2048 + SHA-256 | Student/Lecturer identity verification |
| QR Integrity | HMAC | Payload tamper detection |
| Data in Transit | TLS 1.3 (AES-256-GCM) | Cloud ↔ PWA, Cloud ↔ Dashboard |
| Data at Rest (Cloud) | AES-256 (filesystem) | PostgreSQL database encryption |
| Data at Rest (PWA) | AES-256 (IndexedDB) | Local session vault encryption |
| Session Tokens | JWT (jti + 24h TTL) | API authentication + replay prevention |
| MFA | TOTP | VC and DQA Director accounts |
| Exam Clearance | RSA-signed QR | Invigilator eligibility verification |

### 6.2 Privacy-by-Design

- Student PII (name, email) is **never stored** on the Coordinator's device
- Only a **per-tenant keyed HMAC** of the `student_id` (registration number) and a device fingerprint hash persist locally — the keyed HMAC is not reversible to the registration number without the tenant key (which lives server-side and is delivered only inside the AES-encrypted manifest)
- Device fingerprint cannot identify a student independently of their registration number
- Hardware fingerprint data purged annually

### 6.3 Threat Mitigation Map

| Threat | Mitigation |
|---|---|
| Proxy Attendance | **Same-LAN proximity** (the phone must be on the coordinator's hotspot — its egress IP must match the coordinator's `sessions.coordinator_ip`) + a **live rotating room code** read off the coordinator's screen + hardware-fingerprint **one-device-one-person** binding per session. All measured on the student's handset and submitted with the check-in, so they bind the student's device, not the coordinator's |
| QR Code Sharing | QR uniquely bound to student_id + hardware fingerprint on first scan |
| Ghost Lectures | Mandatory Lecturer Gate-Open scan + GHOST_LECTURE_SUSPECTED flag (<5% attendance) |
| Session Replay | JWT `jti` claim + Gate-Open QR 15-min TTL |
| Data Tampering | Append-only logs + AES-256-GCM encrypted sync packages authenticated with a **device-bound HMAC** (HKDF from the coordinator's binding key); the Sync Receiver verifies the HMAC and decrypts server-side before any write — an unverifiable package is never marked SYNCED. (Student QR codes themselves are RSA-2048 signed and verified at scan time.) |
| Cross-Tenant Leakage | PostgreSQL RLS policies on all tables keyed by tenant_id |
| Coordinator Bias | Coordinator has zero read access to attendance percentages |
| Warden Fraud | GPS geofence verification + cryptographically isolated delegation links |

---

## 7. Multi-Tenancy Architecture

```
┌─────────────────────────────────────────────────┐
│              Single PostgreSQL Cluster           │
│                                                 │
│  SET app.current_tenant = 'tenant-uuid-A';      │
│                                                 │
│  CREATE POLICY tenant_isolation ON sessions     │
│    USING (tenant_id = current_setting(          │
│            'app.current_tenant')::uuid);        │
│                                                 │
│  ┌───────────────┐  ┌───────────────┐           │
│  │  University A  │  │  University B │           │
│  │  tenant_id: A  │  │  tenant_id: B │           │
│  │  threshold:75% │  │  threshold:80%│           │
│  │  RSA key: A    │  │  RSA key: B   │           │
│  └───────────────┘  └───────────────┘           │
└─────────────────────────────────────────────────┘
```

Each tenant has isolated:
- RSA key pair (stored in HSM per tenant)
- Policy configuration (attendance threshold (≥75% floor), session window/durations, branding)
- Academic calendar and course hierarchy
- Student roster and attendance records

---

## 8. Offline Synchronisation Architecture

### 8.1 Chunked Resume-Sync Protocol

```
PWA Outbox Queue                    Sync Receiver Service
     │                                      │
     │──── POST /sync/init ────────────────►│
     │◄─── { upload_id, chunk_size } ───────│
     │                                      │
     │──── POST /sync/chunk/0 ─────────────►│ (AES-256 encrypted JSON chunk)
     │◄─── { ack: signed, next: 1 } ────────│
     │                                      │
     │──── POST /sync/chunk/1 ─────────────►│
     │   [CONNECTION DROPS]                 │
     │                                      │
     │──── POST /sync/resume ──────────────►│ { upload_id }
     │◄─── { resume_from: 1 } ──────────────│ (server returns last ack'd index)
     │                                      │
     │──── POST /sync/chunk/1 ─────────────►│ (retransmit from checkpoint)
     │◄─── { ack: signed, next: 2 } ────────│
     │                                      │
     │──── POST /sync/complete ────────────►│
     │◄─── { status: SYNCED } ──────────────│
```

### 8.2 Outbox Queue State Machine

```
PENDING ──► UPLOADING ──► SYNCED
              │
              └──► FAILED ──► SYNC_OVERDUE (>48h) ──► Alert to QA Dashboard
```

---

## 9. Deployment Architecture

```
                         ┌──────────────────────────────────┐
                         │     Kubernetes Cluster (AWS/GCP) │
                         │                                  │
  ┌──────────────┐       │  ┌──────────┐  ┌──────────────┐ │
  │  Cloudflare  │       │  │ Auth Pod │  │  QR Gen Pod  │ │
  │  CDN (Daily  │       │  │ (3 rep.) │  │  (Node.js)   │ │
  │  Manifest)   │       │  └──────────┘  └──────────────┘ │
  └──────────────┘       │  ┌──────────┐  ┌──────────────┐ │
                         │  │ Session  │  │  Sync        │ │
  ┌──────────────┐       │  │ Manager  │  │  Receiver    │ │
  │  Coordinator │──────►│  │  Pod     │  │  Pod         │ │
  │  PWA         │HTTPS  │  └──────────┘  └──────────────┘ │
  └──────────────┘TLS1.3 │  ┌──────────┐  ┌──────────────┐ │
                         │  │Reporting │  │ Notification │ │
  ┌──────────────┐       │  │ Engine   │  │  Service     │ │
  │  Admin       │──────►│  │  Pod     │  │  Pod         │ │
  │  Dashboards  │       │  └──────────┘  └──────────────┘ │
  └──────────────┘       │                                  │
                         │  ┌──────────────────────────┐    │
                         │  │  API Gateway (Go)         │    │
                         │  │  Rate Limiting + Auth     │    │
                         │  └──────────────────────────┘    │
                         └──────────────────────────────────┘
                                      │
                    ┌─────────────────┼──────────────────┐
                    │                 │                  │
             ┌──────▼──────┐  ┌───────▼──────┐  ┌──────▼──────┐
             │ PostgreSQL  │  │   Redis 7+   │  │    HSM      │
             │ Primary +   │  │  (JWT Cache  │  │  (RSA Keys) │
             │  Replica    │  │   + Sync Q)  │  │             │
             └─────────────┘  └──────────────┘  └─────────────┘
```

---

## 10. Technology Stack Summary

| Layer | Technology | Version | Rationale |
|---|---|---|---|
| PWA Runtime | Browser (Chrome/Safari/Firefox) | Chrome 90+, Safari 14+ | No app store dependency |
| PWA Storage | IndexedDB | Browser native | Large offline data capacity |
| PWA Sync | Service Worker + Background Sync | W3C Level 1 | Offline-first sync |
| PWA proximity | Same-LAN (egress IP) + rotating room code | — | Hardware-free proximity enforcement (replaced Web Bluetooth) |
| Backend API | Go (Golang) | 1.21+ | High throughput, low latency |
| QR Service | Node.js | 20+ | Rich cryptography ecosystem |
| Database | PostgreSQL | 15+ | RLS support, JSON support, sharding |
| Cache | Redis | 7+ | JWT invalidation, manifest caching |
| Container | Docker + Kubernetes | Latest stable | Auto-scaling, multi-region |
| CDN | Cloudflare (or equiv.) | — | Daily Manifest edge delivery |
| Email | SendGrid / AWS SES / Mailgun | REST API | Bulk QR delivery |
| Encryption | AES-256 + RSA-2048 + TLS 1.3 | — | As specified in SRS |
| Auth | JWT (RFC 7519) + TOTP MFA | — | Stateless, replay-safe |
