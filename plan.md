# QAAT — Project Management Plan
**Document:** plan.md  
**Role:** Project Manager View  
**Version:** 1.0.0  
**Based on:** QAAT-SRS-2025-001 | ARCHITECT.md | technicaldoc.md

---

## 1. Executive Summary

QAAT requires **8 major workstreams** split across **4 parallel teams**. The system has a hard critical path: Infrastructure and Database must be stable before Auth is built; Auth must be complete before any feature service can be integrated end-to-end. The Coordinator PWA is the single most complex deliverable and requires the most senior frontend resources.

**Total estimated build duration:** 16 weeks (4 phases)  
**Minimum team size:** 8 developers + 1 DevOps + 1 QA engineer  
**No third-party auth provider** — the Auth Service is built in-house (see Section 3 Team A)

---

## 2. Work Breakdown Structure (WBS)

### WBS Overview

```
QAAT Platform
├── W1  Infrastructure & DevOps
├── W2  Database & Schema
├── W3  Custom Authentication Service     ← CRITICAL PATH
├── W4  QR Generation & Email Service
├── W5  Coordinator PWA (Edge Server)     ← MOST COMPLEX
├── W6  Session & Sync Backend Services
├── W7  Administrative Dashboards
├── W8  Student Portal & Exam Eligibility
└── W9  Legacy SIS Integration & Multi-Tenant Engine
```

---

## 3. Team Structure

### Team A — Backend Core (3 developers)
**Focus:** Infrastructure, Database, Custom Auth, API Gateway

| Member | Primary Role |
|---|---|
| A1 | DevOps / Infrastructure lead |
| A2 | Database architect + Auth Service |
| A3 | API Gateway + Rate Limiting + RBAC |

### Team B — Backend Features (3 developers)
**Focus:** QR Service, Session Manager, Sync Receiver, Reporting Engine

| Member | Primary Role |
|---|---|
| B1 | QR Generation Microservice (Node.js) |
| B2 | Session Manager + Warden Delegation (Go) |
| B3 | Sync Receiver + Eligibility Engine (Go) |

### Team C — Frontend PWA (2 developers)
**Focus:** Coordinator PWA — the offline-first edge server application

| Member | Primary Role |
|---|---|
| C1 | PWA architecture, Service Worker, IndexedDB, BLE integration |
| C2 | QR validation engine, session state machine UI, sync queue UI |

### Team D — Frontend Dashboards (2 developers)
**Focus:** All administrative dashboards and student portal

| Member | Primary Role |
|---|---|
| D1 | VC Dashboard, DQA Dashboard (charts, filters, PDF export) |
| D2 | QA Officer Dashboard, Coordinator View, Student Status Portal |

---

## 4. Dependency Map

```
W1 Infrastructure
    │
    ├──► W2 Database & Schema
    │         │
    │         ├──► W3 Custom Auth Service  ←──── CRITICAL PATH
    │         │         │
    │         │         ├──► W4 QR Service (can start with stub auth)
    │         │         ├──► W6 Session & Sync Backend
    │         │         ├──► W7 Admin Dashboards (can start with mock auth)
    │         │         └──► W8 Student Portal
    │         │
    │         └──► W9 SIS Integration (can start after DB schema)
    │
    └──► W5 Coordinator PWA (can start UI scaffolding in parallel)
              │
              └─ Depends on W3 (auth), W4 (QR), W6 (sync endpoint)
                 for end-to-end integration
```

**Hard dependencies (cannot be skipped):**
1. W1 must complete before W2 begins integration
2. W2 must complete before W3 can be fully tested
3. W3 must be done before any service ships to staging
4. W4 (QR service) must be done before Coordinator PWA can do end-to-end QR validation
5. W6 Sync Receiver must be done before PWA sync can be tested end-to-end

**Soft dependencies (can proceed in parallel with mocks):**
- W5 (PWA) can build UI and local logic using mock data and a local JWT stub
- W7 (Dashboards) can build UI components with hardcoded fixture data
- W9 (SIS integration) can build import/export logic against a sample SIS schema

---

## 5. Phase Plan

### Phase 1 — Foundation (Weeks 1–3)
**Goal:** Stable infrastructure, database, and authentication that every other team can build on.

| Week | Team | Task | Deliverable |
|---|---|---|---|
| 1 | A1 | Docker Compose dev environment, Kubernetes manifests, PostgreSQL + Redis provisioning | Running dev environment |
| 1 | A1 | CI/CD pipeline (GitHub Actions / GitLab CI) with test + build stages | Automated pipeline |
| 1 | A2 | Full PostgreSQL schema (all tables, ENUMs, indexes) | Migration files |
| 1 | A2 | Row-Level Security policies per tenant on all tables | RLS verified |
| 1 | A3 | API Gateway skeleton (Go): routing, middleware, CORS, health check | `/api/v1/health` responding |
| 2 | A2 | **Custom Auth Service — Phase 1:** User table, password hashing (bcrypt), login endpoint, JWT issuance (RS256, 24h TTL, jti claim) | `POST /api/v1/auth/login` working |
| 2 | A2 | JWT validation middleware (verify RS256 signature, check jti blacklist in Redis, check expiry) | Auth middleware ready |
| 2 | A3 | RBAC middleware (role claim extraction, per-endpoint role guards: VC / DQA / QA_OFFICER / COORDINATOR / ADMIN) | Role guards on all routes |
| 3 | A2 | **Custom Auth Service — Phase 2:** TOTP MFA (mandatory for VC + DQA Director), TOTP enrolment flow, TOTP verification endpoint | `POST /api/v1/auth/mfa/enroll` + `/verify` |
| 3 | A2 | Token refresh (`POST /api/v1/auth/refresh`), logout with Redis jti blacklist (`POST /api/v1/auth/logout`) | Full auth lifecycle complete |
| 3 | A3 | Rate limiting middleware (50 req/s per coordinator, global limits per endpoint) | Rate limiter integrated |
| 3 | A1 | Multi-tenant middleware: extract `tenant_id` from JWT, set `app.current_tenant` on every DB connection | RLS active in all requests |
| 3 | C1 | PWA project scaffold (Vite + React/Svelte + TypeScript + vite-plugin-pwa + Workbox) | PWA installable shell |
| 3 | D1 | Dashboard project scaffold (React + TypeScript, routing, auth context, role-based layout) | Dashboard login page working |

**Phase 1 Exit Criteria:**
- [ ] PostgreSQL schema deployed and all RLS policies tested with two tenant seeds
- [ ] `POST /auth/login` issues valid JWT
- [ ] TOTP MFA works for VC-role account
- [ ] JWT middleware blocks unauthorised requests (401) and wrong-role requests (403)
- [ ] All team members can run the full stack locally via `docker-compose up`

---

### Phase 2 — Core Services (Weeks 4–8)
**Goal:** All backend services functional; PWA local session logic complete; dashboards rendering real data.

#### Team A (Weeks 4–8)
| Week | Task | Deliverable |
|---|---|---|
| 4–5 | Multi-tenant policy engine: `PUT /api/v1/dashboard/dqa/thresholds`, policy propagation to Daily Manifest | Policy config API working |
| 4–5 | Daily Manifest generation service: assemble roster + policy + public key, encrypt, cache in Redis + CDN | `GET /api/v1/manifest/daily` returning encrypted manifest |
| 6–7 | SIS import pipeline: CSV upload parser, field validation, upsert into `students_extended` | `POST /api/v1/import/csv` working |
| 6–7 | SIS automated pull: configurable nightly CRON job, OAuth 2.0 client to SIS REST API | Nightly import job running |
| 8 | Admin audit log: middleware capturing all DQA/QA/Admin actions with actor, target, IP, timestamp | Audit log populated on every admin action |

#### Team B (Weeks 4–8)
| Week | Task | Deliverable |
|---|---|---|
| 4–5 | QR Generation Microservice: RSA-2048 key pair per tenant, sign student JSON payload, render 1024×1024 PNG | QR PNGs generating correctly |
| 4–5 | QR delivery via email (SMTP/SendGrid/Mailgun): batch delivery, 100 emails/sec rate limiter, bounce handling | `POST /api/v1/qr/generate/batch` sending emails |
| 4–5 | QR reissuance: rotate serial number, invalidate old, re-deliver | `POST /api/v1/qr/reissue` working |
| 6–7 | Session Manager: Warden Delegation Link generation (cryptographic token, GPS geofence, time-window binding) | `POST /api/v1/sessions/warden-link` working |
| 6–7 | Session Manager: exam clearance token generation (RSA-signed QR per eligible student) | Clearance QR generated |
| 7–8 | Sync Receiver: chunked upload endpoint, per-chunk signed ACK, resume endpoint, Vector Clock deduplication | Full chunked sync protocol working |
| 8 | Sync Receiver: post-sync eligibility recomputation trigger, refresh materialized view | Attendance percentages update after sync |

#### Team C (Weeks 4–8)
| Week | Task | Deliverable |
|---|---|---|
| 4 | IndexedDB schema (Dexie.js): all stores, AES-256-GCM encryption wrapper using SubtleCrypto | Encrypted local storage working |
| 4–5 | Daily Fetch flow: download + decrypt + store manifest, UI screen with sync status | Daily Fetch screen working |
| 5–6 | Session State Machine: IDLE → PENDING_LECTURER → ACTIVE → CLOSED/AUTO_CLOSED, all timers (T+15 Gate-Open TTL, T+120 check-in window, T+180 auto-kill) | Full state machine working locally |
| 5–6 | BLE Scanner Module: Web Bluetooth API, 10-second weighted RSSI rolling average, session initialisation gate | BLE proximity check working |
| 6–7 | QR Validation Engine (full 5-step sequence): RSA verify → roster lookup → BLE check → fingerprint binding → duplicate check | QR scan validates or rejects correctly |
| 6–7 | Hardware Fingerprint Engine: compute fingerprint, bind on first scan, compare on subsequent, DEVICE_MISMATCH flag | Fingerprint binding working |
| 7–8 | Local LAN server (port 8080): student scan URL generated per session, served by Service Worker | Students can scan from their phones |
| 8 | Session sealing: AES-256 encrypt + HMAC sign session package, place in IndexedDB outbox queue | Session package sealed on close |

#### Team D (Weeks 4–8)
| Week | Task | Deliverable |
|---|---|---|
| 4–5 | VC Dashboard: filtering UI (all dimensions), data tables with sort/filter, chart components (bar, line, heatmap) | VC dashboard rendering with fixture data |
| 4–5 | DQA Dashboard: threshold config panel, exam eligibility list, attendance trend graphs | DQA dashboard rendering |
| 6–7 | QA Officer Dashboard: live session monitor (polling/WebSocket), anomaly alert feed (DEVICE_MISMATCH, etc.), device reset form | QA Officer dashboard rendering |
| 6–7 | Coordinator View: session status display, live student count, presence list, sync queue status panel | Coordinator view rendering |
| 7–8 | Connect all dashboards to real API endpoints (replace fixture data) | All dashboards pulling live data |
| 8 | PDF export (VC audit report), CSV export (eligibility list) | Export functions working |

**Phase 2 Exit Criteria:**
- [ ] QR codes generated, signed, and emailed to test students
- [ ] Coordinator PWA can scan a real student QR and accept/reject correctly (offline)
- [ ] BLE beacon proximity check gates session correctly
- [ ] Session lifecycle (all states + auto-timers) working end-to-end
- [ ] Chunked sync uploads a sealed session package to the cloud
- [ ] All dashboards display real data from the API

---

### Phase 3 — Integration & End-to-End Testing (Weeks 9–12)
**Goal:** All components integrated; full end-to-end flow verified; security hardened.

| Week | Team | Task |
|---|---|---|
| 9 | All | **Integration Sprint:** Connect PWA Daily Fetch → real manifest endpoint |
| 9 | All | Connect PWA sync → real Sync Receiver; verify Vector Clock deduplication |
| 9 | All | Connect Warden Delegation: Coordinator generates link → Warden opens → GPS geofence → session runs → auto-submits → Coordinator approves |
| 10 | A | Exam eligibility computation end-to-end: sync → materialized view refresh → student portal updates within 60 seconds |
| 10 | B | Clearance token flow: DQA generates → student shows QR → invigilator scans → verified without dashboard access |
| 10 | A+B | Multi-tenant isolation testing: verify RLS prevents cross-tenant data leakage with two active test tenants |
| 11 | All | **Security Review:** RSA key management audit, JWT replay attack test, rate limiter stress test (>50 req/s per coordinator), BLE spoofing resistance test |
| 11 | A | TLS 1.3 enforcement, CSP headers, CORS lockdown on all API endpoints |
| 11 | C | Offline resilience test: kill internet mid-session, verify no data loss, verify sync resumes correctly on reconnection |
| 12 | QA | Load testing: 300 concurrent student scans/minute on PWA; 500 concurrent students in single venue; 10,000 concurrent sync operations on API |
| 12 | All | Bug fix sprint — address all P1/P2 issues from integration testing |

**Phase 3 Exit Criteria:**
- [ ] Full UC-01 (Standard Session) passes end-to-end with real devices
- [ ] Full UC-02 (Warden Delegation) passes end-to-end
- [ ] Full UC-03 (Device Binding Reset) passes end-to-end
- [ ] Cross-tenant RLS leakage test: zero cross-tenant records returned in any query
- [ ] 300 concurrent scans/minute handled without data loss
- [ ] Offline → sync recovery produces zero data loss

---

### Phase 4 — Pilot Deployment & Hardening (Weeks 13–16)
**Goal:** Deploy to staging, run controlled pilot, fix production issues, prepare for full rollout.

| Week | Activities |
|---|---|
| 13 | Staging environment deployment (Kubernetes); BLE beacon UUID registration for pilot venues; Coordinator PWA distributed to pilot Coordinators |
| 14 | Controlled pilot in one department: live sessions, real students, real QR scans — monitor sync pipeline, fraud detection, anomaly flags |
| 15 | Collect pilot feedback; resolve P1 bugs; RSSI threshold calibration per venue; Warden delegation drill |
| 16 | Full campus rollout preparation: all tenant onboarding, all Coordinator training (target: <2 hours), VC and DQA dashboard activation, live monitoring |

---

## 6. Parallel Work Summary

The following workstreams can proceed **simultaneously** without blocking each other, provided they use mock interfaces where real dependencies don't exist yet:

```
Week 1-3:
┌─────────────────────┐  ┌─────────────────────┐  ┌──────────────────────┐
│  Team A             │  │  Team C              │  │  Team D              │
│  Infrastructure     │  │  PWA Scaffold        │  │  Dashboard Scaffold  │
│  DB Schema          │  │  (mock auth)         │  │  (mock auth)         │
│  Auth Service       │  │                      │  │                      │
└─────────────────────┘  └─────────────────────┘  └──────────────────────┘

Week 4-8 (after Auth is done):
┌─────────────────────┐  ┌─────────────────────┐  ┌──────────────────────┐  ┌──────────────────┐
│  Team A             │  │  Team B              │  │  Team C              │  │  Team D          │
│  Policy Engine      │  │  QR Service          │  │  IndexedDB vault     │  │  VC Dashboard    │
│  SIS Integration    │  │  Email Delivery      │  │  Session State Mach. │  │  DQA Dashboard   │
│  Manifest Service   │  │  Sync Receiver       │  │  BLE Engine          │  │  QA Dashboard    │
│  Audit Log          │  │  Eligibility Engine  │  │  QR Validator        │  │  Student Portal  │
└─────────────────────┘  └─────────────────────┘  └──────────────────────┘  └──────────────────┘
```

---

## 7. Critical Path

The following tasks are on the **critical path** — any delay here delays the entire project:

```
[W1: Dev Environment] (Week 1)
         ↓
[W2: Database Schema + RLS] (Week 1)
         ↓
[W3: Custom Auth Service — Login + JWT] (Week 2)
         ↓
[W3: TOTP MFA + Refresh + Logout] (Week 3)
         ↓
[W5-C: PWA QR Validation Engine + BLE] (Weeks 6–7)
         ↓
[W6-B: Sync Receiver (chunked upload + dedup)] (Weeks 7–8)
         ↓
[Phase 3: End-to-End Integration] (Weeks 9–10)
         ↓
[Phase 3: Security + Load Testing] (Weeks 11–12)
         ↓
[Phase 4: Pilot Deployment] (Weeks 13–14)
```

**If Auth slips by 1 week → every downstream team loses 1 week of integrated testing time.**  
**If the QR Validation Engine slips → PWA cannot be tested end-to-end.**  
**If the Sync Receiver slips → full-session loop cannot be validated.**

---

## 8. Component Ownership Matrix

| Component | Owner | Reviewer | Depends On |
|---|---|---|---|
| Docker/K8s Infrastructure | A1 | A2 | — |
| PostgreSQL Schema + Migrations | A2 | A3 | A1 |
| Row-Level Security Policies | A2 | A3 | A2 (schema) |
| **Custom Auth Service** (JWT, TOTP, RBAC) | A2 + A3 | All Leads | A1, A2 |
| API Gateway + Middleware | A3 | A2 | A1, Auth |
| Multi-Tenant Policy Engine | A3 | A2 | Auth, DB |
| Daily Manifest Service | A3 | B2 | DB, Policy Engine |
| Admin Audit Log | A3 | A2 | Auth, DB |
| **QR Generation Microservice** | B1 | A2 | DB (tenant keys) |
| Email Notification Service | B1 | A1 | QR Service |
| Session Manager Service | B2 | B3 | Auth, DB |
| Warden Delegation Module | B2 | A3 | Session Manager |
| **Sync Receiver Service** | B3 | A2 | Auth, DB |
| Eligibility Engine | B3 | B2 | Sync Receiver, DB |
| Exam Clearance Token Generator | B2 | B3 | Eligibility Engine |
| SIS Import Pipeline (CSV + CRON) | A3 | A2 | DB |
| **Coordinator PWA — Core** | C1 | C2 | Auth (JWT) |
| PWA BLE Engine | C1 | C2 | PWA Core |
| PWA QR Validator | C2 | C1 | PWA Core, BLE |
| PWA Session State Machine | C2 | C1 | QR Validator |
| PWA Sync Queue (Service Worker) | C1 | C2 | Sync Receiver |
| VC Dashboard | D1 | D2 | Auth, Reporting API |
| DQA Dashboard | D1 | D2 | Auth, Policy Engine, Eligibility |
| QA Officer Dashboard | D2 | D1 | Auth, Anomaly Alerts |
| Coordinator View (PWA UI) | D2 | C1 | PWA Core |
| Student Status Portal | D2 | B3 | Eligibility Engine |

---

## 9. Risk Register

| Risk | Probability | Impact | Mitigation |
|---|---|---|---|
| **Web Bluetooth API not supported** on some Coordinator devices (iOS Safari restriction) | High | High | Test early on target devices; provide fallback GPS-only mode for unsupported browsers; document iOS PWA limitations |
| **Auth Service slips** and blocks all teams | Medium | Critical | Auth is Phase 1 Week 2–3 priority; teams A2+A3 dedicated entirely to it; daily standups on auth progress |
| **BLE RSSI signal unreliable** in large lecture halls (reflection/interference) | Medium | High | Phase 4 pilot includes RSSI calibration per venue; weighted rolling average in code reduces noise |
| **IndexedDB quota exceeded** on low-storage Coordinator devices | Low | Medium | Monitor storage usage; 500MB minimum requirement; implement storage quota warnings |
| **SIS API non-compliant** or unavailable at client institution | High | Medium | CSV upload fallback always available; integration adaptor per institution |
| **TOTP MFA friction** causes VC/DQA Director adoption resistance | Medium | Low | One-time enrolment flow; backup codes; recovery via Platform Admin |
| **Session data loss** if Coordinator device lost before 7-day archive sync | Low | High | 7-day local archive; SYNC_OVERDUE alert at 48h; QA Officer notified |
| **GPS geofence inaccurate** in multi-storey buildings (Warden delegation) | Medium | Medium | Allow DQA to manually bypass geofence with approval code; log GPS_GEOFENCE_FAIL for audit |

---

## 10. Definition of Done (per workstream)

A workstream is considered **done** when:
1. Unit tests written and passing (minimum 80% coverage on business logic)
2. Integration tests passing against real PostgreSQL + Redis instances
3. No P1 or P2 bugs open
4. Code reviewed and merged to main branch
5. API endpoints documented (OpenAPI spec updated)
6. Feature tested on target devices (mobile browser for PWA items)
7. RLS isolation verified (if component touches DB)

---

## 11. Milestones

| Milestone | Target Week | Description |
|---|---|---|
| **M1: Dev Environment Ready** | End of Week 1 | All developers running full stack locally |
| **M2: Auth Service Complete** | End of Week 3 | JWT + TOTP MFA + RBAC fully working |
| **M3: First QR Generated & Emailed** | End of Week 5 | Real student QR delivered to test inbox |
| **M4: First Offline Session** | End of Week 7 | Full session run on PWA with BLE + QR scan, no internet |
| **M5: First Cloud Sync** | End of Week 8 | Session data synced from PWA to PostgreSQL |
| **M6: Full End-to-End UC-01** | End of Week 10 | Standard session — all steps verified with real devices |
| **M7: Security & Load Tests Passed** | End of Week 12 | All NFRs verified under load |
| **M8: Pilot Launch** | End of Week 13 | Controlled pilot in one department live |
| **M9: Full Campus Rollout Ready** | End of Week 16 | All tenants onboarded, all dashboards live |

---

## 12. Repository Structure (Recommended)

```
qaat/
├── services/
│   ├── api-gateway/          # Go 1.21+ — routing, auth middleware, rate limiting
│   ├── auth-service/         # Go — JWT, TOTP MFA, RBAC (custom, no third-party)
│   ├── qr-generator/         # Node.js 20+ — RSA signing, PNG generation
│   ├── session-manager/      # Go — session lifecycle, warden delegation
│   ├── sync-receiver/        # Go — chunked sync, deduplication, eligibility trigger
│   ├── reporting-engine/     # Go — dashboard aggregations, PDF/CSV export
│   └── notification-service/ # Node.js — email delivery, push notifications
├── apps/
│   ├── coordinator-pwa/      # React/Svelte TypeScript PWA — Team C
│   ├── admin-dashboards/     # React TypeScript SPA — Team D
│   └── student-portal/       # Lightweight React SPA — Team D
├── db/
│   ├── migrations/           # Numbered SQL migration files
│   ├── seeds/                # Test tenant + student seed data
│   └── rls-policies/         # Row-Level Security policy definitions
├── infra/
│   ├── docker-compose.yml    # Local development
│   ├── k8s/                  # Kubernetes manifests
│   └── terraform/            # Cloud infrastructure as code
├── docs/
│   ├── ARCHITECT.md
│   ├── technicaldoc.md
│   └── plan.md               # This file
└── .github/workflows/        # CI/CD pipelines
```
