# QAAT — Build Progress
**Last updated:** 2026-06-29
**Reference docs:** flow.md · ARCHITECT.md · technicaldoc.md · plan.md  
**Current position:** All 4 phases substantially complete + post-pilot hardening shipped

---

> ### Since session 12 — shipped changes (newest first)
> - **Optional QR email dispatch** (2026-06-28): optional email re-added for lecturers
>   + students, used *only* to email their permanent QR on create/import; blank = no email.
> - **Standby coordinator** (2026-06-28, migration 042): an absent coordinator delegates
>   that day's session to an own-cohort student via a one-day code.
> - **Lean registration** (2026-06-26): passwordless **reg-no** student progress portal;
>   students identified by reg-no only (no email/phone/password); lecturer **staff-ID**
>   login; course is level-independent (levels added inside it); cohorts apply across all courses.
> - **Curriculum + lecturer QR** (2026-06-23): curriculum bulk import; lecturer permanent
>   career QR → passwordless dashboard; 75% attendance-threshold floor.
> - **LAN-only proximity** (migration 039): **BLE/beacon/RSSI removed entirely.** Proximity =
>   on the coordinator's hotspot LAN + live rotating room code. The BLE rows below are HISTORICAL.

---

## Phase 1 — Foundation (Weeks 1–3) ✅ COMPLETE

### Week 1 — Team A (Infrastructure + DB)

| Task | Deliverable | Status |
|---|---|---|
| Docker Compose dev environment (PostgreSQL 15, Redis 7, Mailhog) | `infra/docker-compose.yml` | ✅ |
| GitHub Actions CI pipeline (build + test + migration verify) | `.github/workflows/ci.yml` | ✅ |
| Full PostgreSQL schema — 12 tables, 7 ENUMs | `db/migrations/001_init_schema.sql` | ✅ |
| All indexes (12 composite + tenant indexes) | `db/migrations/002_indexes.sql` | ✅ |
| Row-Level Security on all 12 tenant-scoped tables | `db/migrations/003_rls_policies.sql` | ✅ |
| `attendance_logs` append-only guard (RESTRICTIVE DELETE/UPDATE block) | `db/migrations/003_rls_policies.sql` | ✅ |
| `student_attendance_summary` materialized view | `db/migrations/004_materialized_views.sql` | ✅ |
| Two test tenant seeds + 5 users (one per role) for RLS isolation tests | `db/seeds/001_test_tenants.sql`, `002_test_users.sql` | ✅ |

### Week 2 — Team A (Auth Service Phase 1)

| Task | Deliverable | Status |
|---|---|---|
| Custom Auth Service — user store, bcrypt, `POST /api/v1/auth/login` | `services/auth-service/` | ✅ |
| RS256 JWT issuance (24h TTL, `jti` claim) | `internal/crypto/jwt.go` | ✅ |
| JWT validation middleware — RS256 verify + `jti` Redis blacklist check | `services/api-gateway/internal/middleware/jwt.go` | ✅ |
| RBAC middleware — role claim extraction, per-route role guards | `internal/middleware/rbac.go` | ✅ |
| Account lockout — 5 failed attempts → 15-min lock | `internal/store/user_store.go` | ✅ |

### Week 3 — Team A (Auth Service Phase 2 + Gateway)

| Task | Deliverable | Status |
|---|---|---|
| TOTP MFA enrolment (`POST /api/v1/auth/mfa/enroll`) | `internal/handlers/auth_handler.go` | ✅ |
| TOTP MFA verification (`POST /api/v1/auth/mfa/verify`) | `internal/handlers/auth_handler.go` | ✅ |
| Token refresh — rotates `jti`, blacklists old token | `POST /api/v1/auth/refresh` | ✅ |
| Logout — blacklists `jti` with TTL = remaining lifetime | `POST /api/v1/auth/logout` | ✅ |
| Rate limiting — 50 req/s per coordinator, 200/s global | `internal/middleware/ratelimit.go` | ✅ |
| Multi-tenant middleware — `SET LOCAL app.current_tenant` on every request | `internal/middleware/tenant.go` | ✅ |
| API Gateway skeleton — all routes stubbed with role guards | `services/api-gateway/internal/router/router.go` | ✅ |
| CORS + security headers middleware | `internal/middleware/cors.go` | ✅ |
| `go mod tidy` + `go.sum` generated (auth-service, api-gateway, session-manager, sync-receiver) | all `go.sum` files | ✅ |

### Week 3 — Team C & D (Frontend Scaffolds)

| Task | Deliverable | Status |
|---|---|---|
| Coordinator PWA scaffold — Vite + React 18 + TypeScript + vite-plugin-pwa | `apps/coordinator-pwa/` | ✅ |
| Service Worker (Workbox 7) — precache + Background Sync outbox | `src/sw.ts` | ✅ |
| IndexedDB vault schema (Dexie.js) — 6 stores, all typed | `src/db/vault.ts` | ✅ |
| AES-256-GCM encryption layer (HKDF key derivation) | `src/crypto/vault-crypto.ts` | ✅ |
| Hardware fingerprint engine (SHA-256 of 6 device components) | `src/crypto/fingerprint.ts` | ✅ |
| Session state machine (XState v5) — IDLE → PENDING_LECTURER → ACTIVE → CLOSED/AUTO_CLOSED | `src/session/state-machine.ts` | ✅ |
| Auth store (Zustand + persist) — login, logout, token expiry | `src/store/auth.ts` | ✅ |
| Login page — email/password + TOTP MFA gate | `src/pages/Login.tsx` | ✅ |
| Admin Dashboards scaffold — React + TypeScript + React Router v6 | `apps/admin-dashboards/` | ✅ |
| AuthContext — JWT session management, role extraction | `src/contexts/AuthContext.tsx` | ✅ |
| RoleLayout — per-role sidebar nav + route guard (`<Navigate>`) | `src/layouts/RoleLayout.tsx` | ✅ |
| Login page + role-aware redirect | `src/pages/Login.tsx` | ✅ |
| Stub pages — VC Overview, DQA Thresholds, QA Live Sessions | `src/pages/vc/`, `dqa/`, `qa/` | ✅ |

**Phase 1 Exit Criteria:**

- [x] PostgreSQL schema deployed + RLS on all 12 tables + two tenant seeds
- [x] `POST /auth/login` issues valid RS256 JWT
- [x] TOTP MFA enrol + verify working (mandatory for VC + DQA Director)
- [x] JWT middleware blocks 401 (missing/invalid) and 403 (wrong role)
- [x] Rate limiter at 50 req/s per coordinator
- [x] PWA scaffold installable with Service Worker
- [x] Dashboard scaffold with role-based routing + login page
- [ ] `docker-compose up` smoke test (requires `make keys` to generate RSA key pair first)

---

## Phase 2 — Core Services (Weeks 4–8) 🔄 IN PROGRESS

### Weeks 4–5 — Teams A + B

| Task | Deliverable | Status |
|---|---|---|
| **QR Generation Microservice** (Node.js 20+) | `services/qr-generator/` | ✅ |
| RSA-2048 key pair generation per tenant | `src/crypto/rsa-keys.ts` | ✅ |
| QR payload signing (RSA-SHA256 + HMAC integrity) | `src/crypto/rsa-keys.ts` | ✅ |
| 1024×1024 PNG rendering (`qrcode` library) | `src/crypto/qr-image.ts` | ✅ |
| `POST /api/v1/qr/generate/batch` — async job, 100 emails/s rate-limited delivery | `src/handlers/generate.ts` | ✅ |
| `POST /api/v1/qr/reissue` — invalidate old serial, re-deliver | `src/handlers/generate.ts` | ✅ |
| `tenant_rsa_keys` table — AES-256 encrypted private key storage | `db/migrations/005_tenant_rsa_keys.sql` | ✅ |
| **Daily Manifest service** (Go, in API Gateway) | `internal/handlers/manifest.go` | ✅ |
| Assemble roster (SHA-256 student_id hashes — no PII on device) | `manifest.go:buildManifest` | ✅ |
| Tenant policy + RSA public key included in manifest | `manifest.go:buildManifest` | ✅ |
| Redis cache until midnight UTC | `manifest.go:ManifestDaily` | ✅ |
| `GET /api/v1/manifest/daily` wired into API Gateway router | `internal/router/router.go` | ✅ |
| **Session Manager service** (Go) | `services/session-manager/` | ✅ |
| `POST /api/v1/sessions/warden-link` — crypto token + GPS geofence binding | `internal/handlers/warden.go` | ✅ |
| `POST /api/v1/sessions/warden-validate` — Haversine geofence check | `internal/handlers/warden.go` | ✅ |
| **Sync Receiver service** (Go) | `services/sync-receiver/` | ✅ |
| `POST /api/v1/sync/init` — create upload record in DB | `internal/sync/receiver.go` | ✅ |
| `POST /api/v1/sync/chunk/{id}/{idx}` — store chunk in Redis, HMAC-SHA256 ACK | `internal/sync/receiver.go` | ✅ |
| `GET /api/v1/sync/resume/{id}` — return first missing chunk index | `internal/sync/receiver.go` | ✅ |
| `POST /api/v1/sync/complete/{id}` — reassemble, SHA-256 verify, write attendance_logs | `internal/sync/receiver.go` | ✅ |
| Vector Clock deduplication via `ON CONFLICT (log_id) DO NOTHING` | `internal/sync/receiver.go` | ✅ |
| Async `REFRESH MATERIALIZED VIEW CONCURRENTLY student_attendance_summary` after sync | `internal/sync/receiver.go` | ✅ |

### Weeks 4–8 — Team A/B/C/D additional items (this session)

| Task | Owner | Deliverable | Status |
|---|---|---|---|
| DQA threshold API — `GET/PUT /api/v1/dashboard/dqa/thresholds` | Team A | `handlers/dqa.go` | ✅ |
| Daily Manifest cache bust on threshold change | Team A | `handlers/dqa.go:PutThresholds` | ✅ |
| Admin audit log middleware (all state-mutating requests) | Team A | `middleware/audit.go` | ✅ |
| PWA — Daily Fetch (download manifest, decrypt, store in IndexedDB) | Team C | `hooks/useManifest.ts` | ✅ |
| PWA — BLE scanner (10s weighted RSSI rolling average) | Team C | `ble/scanner.ts` | ✅ |
| PWA — QR Validation Engine (all 8 steps) | Team C | `qr/validator.ts` | ✅ |
| PWA — Session State Machine UI (full IDLE→CLOSED flow) | Team C | `pages/SessionPage.tsx` | ✅ |
| PWA — QR Camera scanner (BarcodeDetector API) | Team C | `components/QRScanner.tsx` | ✅ |
| PWA — Session sealer (AES-256-GCM + HMAC + SHA-256 checksum) | Team C | `sync/sealer.ts` | ✅ |
| PWA — Outbox queue + chunked upload + resume logic | Team C | `sync/outbox.ts` | ✅ |

### Weeks 7–8 — This session (Team A/B/C/D)

| Task | Owner | Deliverable | Status |
|---|---|---|---|
| VC Dashboard — live KPIs + eligibility bar chart (Recharts) | Team D | `pages/vc/VCOverview.tsx` | ✅ |
| DQA Dashboard — threshold form with live GET/PUT | Team D | `pages/dqa/DQAThresholds.tsx` | ✅ |
| DQA Eligibility — student lookup table | Team D | `pages/dqa/DQAEligibility.tsx` | ✅ |
| QA Officer — live sessions (10s poll, anomaly flags) | Team D | `pages/qa/QALiveSessions.tsx` | ✅ |
| QA Officer — device binding reset form | Team D | `pages/qa/QADeviceReset.tsx` | ✅ |
| Shared API client + `useQuery` / `usePoll` hooks | Team D | `lib/api.ts`, `lib/useApi.ts` | ✅ |
| `GET /api/v1/dashboard/vc/overview` | Team A | `handlers/reporting.go` | ✅ |
| `GET /api/v1/dashboard/qa/live-sessions` | Team A | `handlers/reporting.go` | ✅ |
| `GET /api/v1/eligibility/{student_id}` | Team A | `handlers/reporting.go` | ✅ |
| `POST /api/v1/dashboard/qa/device-reset` | Team A | `handlers/reporting.go` | ✅ |
| SIS CSV import (`POST /api/v1/import/csv`) | Team A | `handlers/sis.go` | ✅ |
| SIS trigger endpoint (`POST /api/v1/import/trigger`) | Team A | `handlers/sis.go` | ✅ |
| Exam clearance token generation (`POST /api/v1/eligibility/clearance-token`) | Team B | `session-manager/handlers/clearance.go` | ✅ |
| PWA — Local LAN server (port 8080, SW intercept, student scan page) | Team C | `sync/lan-server.ts`, `sw.ts` | ✅ |

### Phase 2 completion + Phase 3 entry (this session)

| Task | Owner | Deliverable | Status |
|---|---|---|---|
| `GET /api/v1/reports/vc/audit.pdf` — 30-day PDF (fpdf) | Team B | `handlers/exports.go` | ✅ |
| `GET /api/v1/reports/dqa/eligibility.csv` — full roster CSV | Team B | `handlers/exports.go` | ✅ |
| Export buttons wired to VC + DQA dashboards | Team D | `VCOverview.tsx`, `DQAEligibility.tsx` | ✅ |
| Student Status Portal — login + per-unit attendance % + progress bars | Team D | `apps/student-portal/` | ✅ |
| K8s CronJob — SIS nightly pull at 02:00 UTC | Team A | `infra/k8s/sis-cronjob.yaml` | ✅ |
| Full Kubernetes manifests — all 5 services + postgres + redis + cronjob | Team A1 | `infra/k8s/` | ✅ |
| Security middleware — full CSP, HSTS, Permissions-Policy, CORP/COOP/COEP | Team A | `middleware/security.go` | ✅ |
| HTTPS enforcement (X-Forwarded-Proto redirect) | Team A | `middleware/security.go` | ✅ |
| OpenAPI 3.1 spec — all 22 endpoints documented | Team A | `docs/openapi.yaml` | ✅ |

### Phase 4 (this session)

| Task | Deliverable | Status |
|---|---|---|
| Tenant onboarding API — list/create/suspend tenants, create users, register beacons | `handlers/admin.go` (7 endpoints) | ✅ |
| Admin panel — Tenant list + create form + user management | `pages/admin/AdminTenants.tsx`, `AdminUsers.tsx` | ✅ |
| Admin routes wired into dashboard app + sidebar nav | `App.tsx` | ✅ |
| Notification service — SMTP + Web Push (sync overdue, QR reissued, warden data) | `services/notification-service/` | ✅ |
| Playwright E2E tests — auth, dashboards, API security headers, cross-tenant isolation | `tests/e2e/specs/` (4 spec files) | ✅ |
| `CLAUDE.md` — full codebase guide for future Claude sessions | `CLAUDE.md` | ✅ |

### Session 8 — Admin data management + auth fixes

| Task | Deliverable | Status |
|---|---|---|
| Admin: course + unit CRUD (`handlers/admin_data.go`) | `GET/POST /api/v1/admin/tenants/{id}/courses`, `/courses/{id}/units` | ✅ |
| Admin: venue CRUD | `GET/POST /api/v1/admin/tenants/{id}/venues` | ✅ |
| Admin: student registration (users + students_extended, atomic tx) | `GET/POST /api/v1/admin/tenants/{id}/students` | ✅ |
| Admin dashboard pages — Courses, Units, Students, Venues | `AdminCourses.tsx`, `AdminCourseUnits.tsx`, `AdminStudents.tsx`, `AdminVenues.tsx` | ✅ |
| Auth: MFA bypass env var for dev (`DISABLE_MFA=true`) | `auth-service/config.go`, `auth_handler.go` | ✅ |
| Auth: KEY_ENCRYPTION_KEY for coordinator device binding | `auth-service` env | ✅ |
| Online check-in session lifecycle (`OpenSession`, `CheckinCode`) | `handlers/sessions.go` | ✅ |
| Coordinator PWA: full Session state machine UI with roster polling | `pages/SessionPage.tsx` | ✅ |
| Student portal: UUID→email→student_id fallback in GetEligibility | `handlers/reporting.go` | ✅ |
| Student portal moved to port 3005; coordinator PWA on port 3000 | vite config | ✅ |

### Session 9 — Lecturer registration + coordinator roster

| Task | Deliverable | Status |
|---|---|---|
| DB migration 011: `lecturers` table + `lecturer_assignments` table with RLS | `db/migrations/011_lecturers_and_assignments.sql` | ✅ |
| Admin: lecturer CRUD (`ListLecturers`, `CreateLecturer`) | `GET/POST /api/v1/admin/tenants/{id}/lecturers` | ✅ |
| Admin: assignment CRUD (`ListLecturerAssignments`, `CreateLecturerAssignment`, `DeleteLecturerAssignment`) | `GET/POST /api/v1/admin/tenants/{id}/lecturer-assignments`, `DELETE /admin/lecturer-assignments/{id}` | ✅ |
| Coordinator: unit lecturers endpoint (`GetUnitLecturers`) | `GET /api/v1/coordinator/units/{unit_id}/lecturers` | ✅ |
| Admin dashboard: `AdminLecturers.tsx` — register lecturers per tenant | `pages/admin/AdminLecturers.tsx` | ✅ |
| Admin dashboard: `AdminLecturerAssignments.tsx` — assign lecturers to units (cascading dropdowns, year/sem/session) | `pages/admin/AdminLecturerAssignments.tsx` | ✅ |
| AdminTenants nav updated with "Lecturers" + "Assignments" links | `AdminTenants.tsx` | ✅ |
| Coordinator PWA `PendingLecturer` updated with live lecturer dropdown | `pages/SessionPage.tsx` | ✅ |
| Coordinator PWA: "Open Session" button sends `lecturer_id` on session open | `pages/SessionPage.tsx` | ✅ |
| api DELETE method added to admin API client | `lib/api.ts` | ✅ |

### Session 10 — Course roadmap, active semester, optional coordinator

| Task | Deliverable | Status |
|---|---|---|
| DB migration 012: `total_years` on courses, `default_venue_id` on course_units, `active_academic_year`+`active_semester` on tenants, coordinator_id nullable | `db/migrations/012_course_roadmap_and_semester.sql` | ✅ |
| Backend: `UpdateCourse` — dynamic PATCH (name/dept/school/coordinator/total_years) | `PATCH /api/v1/admin/courses/{course_id}` | ✅ |
| Backend: `UpdateCourseUnit` — dynamic PATCH (name/year/semester/academic_year/default_venue_id) | `PATCH /api/v1/admin/courses/{course_id}/units/{unit_id}` | ✅ |
| Backend: `GetCourseRoadmap` — returns `roadmap[year][semester][]unit` | `GET /api/v1/admin/courses/{course_id}/roadmap` | ✅ |
| Backend: `UpdateTenantAcademicPeriod` — sets active_academic_year + active_semester | `PATCH /api/v1/admin/tenants/{tenant_id}/academic-period` | ✅ |
| Manifest fix: pulls from `course_units` (not sessions) filtered by active semester | `handlers/manifest.go` | ✅ |
| Admin: `AdminCourses.tsx` rewritten — inline edit panel, total_years, coordinator optional | `pages/admin/AdminCourses.tsx` | ✅ |
| Admin: `AdminCourseUnits.tsx` rewritten — Year/Semester roadmap grid, add per slot, edit per unit | `pages/admin/AdminCourseUnits.tsx` | ✅ |
| Admin: `AdminTenants.tsx` updated — "Set Semester" button + inline academic period panel + active period badge | `pages/admin/AdminTenants.tsx` | ✅ |
| `joinComma()` helper for dynamic SQL SET clause building | `handlers/admin_data.go` | ✅ |

### Phase 3 completion + Phase 4 prep (session 6)

| Task | Deliverable | Status |
|---|---|---|
| Auth integration tests — 6 cases (login, wrong PW, cross-tenant, refresh/blacklist, logout, lockout) | `auth_integration_test.go` | ✅ |
| RLS isolation tests — all 9 tables, wildcard SELECT, append-only DELETE guard | `tests/security/rls_isolation_test.go` | ✅ |
| k6 load test — 300 scans/min (ramping-arrival-rate, 5 min hold) | `tests/load/k6-scan-session.js` | ✅ |
| k6 load test — 10k sync ops (constant 167/s, 5 min) | `tests/load/k6-sync-upload.js` | ✅ |
| k6 security test — rate limiter verification (100/s burst, expect 429) | `tests/load/k6-rate-limiter.js` | ✅ |
| Prometheus metrics middleware — requests, latency, in-flight, sync counters | `middleware/metrics.go` | ✅ |
| `/metrics` scrape endpoint wired into API Gateway | `router/router.go` | ✅ |
| Prometheus + Grafana K8s deployment config | `infra/k8s/monitoring.yaml` | ✅ |
| Staging K8s overlay (1-replica, staging env) | `infra/k8s/overlays/staging/` | ✅ |
| Production K8s overlay | `infra/k8s/overlays/production/` | ✅ |
| Phase 4 pilot deployment checklist (50 items, 2 weeks) | `docs/PILOT_CHECKLIST.md` | ✅ |

---

## Milestones

| Milestone | Target | Status |
|---|---|---|
| **M1: Dev Environment Ready** | End Week 1 | ✅ Done |
| **M2: Auth Service Complete** | End Week 3 | ✅ Done |
| **M3: First QR Generated & Emailed** | End Week 5 | 🔄 Needs smoke test with real SMTP |
| **M4: First Offline Session** | End Week 7 | 🔄 All code done; needs device test on real hardware |
| **M5: First Cloud Sync** | End Week 8 | 🔄 Full loop done; needs E2E test on staging |
| **M7: Security & Load Tests Passed** | End Week 12 | 🔄 Tests written; run against staging to get results |
| **M8: Pilot Launch** | End Week 13 | 🔄 Checklist ready at docs/PILOT_CHECKLIST.md |
| **M6: Full End-to-End UC-01** | End Week 10 | ⏳ |
| **M7: Security & Load Tests** | End Week 12 | ⏳ |
| **M8: Pilot Launch** | End Week 13 | ⏳ |
| **M9: Full Campus Rollout** | End Week 16 | ⏳ |

---

## Architecture Coverage

### Services built

| Service | Language | Endpoints | Status |
|---|---|---|---|
| Auth Service | Go 1.21 | login, refresh, logout, mfa/enroll, mfa/verify | ✅ Complete |
| API Gateway | Go 1.21 | all routes wired + middleware stack | ✅ Skeleton complete |
| QR Generator | Node.js 20 | generate/batch, reissue | ✅ Complete |
| Session Manager | Go 1.21 | warden-link, warden-validate | ✅ Core complete |
| Sync Receiver | Go 1.21 | init, chunk, resume, complete | ✅ Complete |
| Reporting Engine | Go | — | ⏳ Not started |
| Notification Service | Node.js | — | ⏳ Not started (email via QR service for now) |

### Database

| Component | Status |
|---|---|
| All 12 core tables | ✅ |
| `tenant_rsa_keys` table | ✅ |
| 12 indexes | ✅ |
| RLS on all 12 tables + append-only guard | ✅ |
| `student_attendance_summary` materialized view | ✅ |
| `qaat_app` role with DELETE revoked on attendance_logs | ✅ |

### Security (ARCHITECT.md §6)

| Control | Status |
|---|---|
| RS256 JWT (24h TTL, `jti` replay prevention via Redis) | ✅ |
| bcrypt password hashing | ✅ |
| TOTP MFA (RFC 6238, mandatory for VC + DQA Director) | ✅ |
| Account lockout (5 attempts → 15 min) | ✅ |
| Rate limiting (50 req/s coordinator) | ✅ |
| TLS 1.3 enforcement header (`Strict-Transport-Security`) | ✅ |
| CORS allowlist | ✅ |
| RLS tenant isolation | ✅ |
| AES-256-GCM IndexedDB encryption (PWA) | ✅ Implemented |
| HKDF device key derivation | ✅ Implemented |
| RSA-2048 QR signing | ✅ |
| HMAC payload integrity | ✅ |
| Hardware fingerprint engine | ✅ Implemented |
| BLE proximity enforcement | ✅ PWA BLE scanner built (`ble/scanner.ts`); wired to session |
| Session package sealing (pre-sync) | ✅ Sealer + outbox built (`sync/sealer.ts`, `sync/outbox.ts`) |

---

### Session 10 (continued) — Data completeness fixes

| Task | Deliverable | Status |
|---|---|---|
| `ListTenants` now returns `active_academic_year` + `active_semester` — badge shows correctly in admin UI | `handlers/admin.go` | ✅ |
| `GetCourseRoadmap` now returns `tenant_id` — venues dropdown in roadmap page loads correctly | `handlers/admin_data.go` | ✅ |
| `ListCourses` now returns `total_years` from DB instead of a hardcoded default | `handlers/admin_data.go` | ✅ |
| `CreateCourse` now saves `total_years`; coordinator_id inserted as NULL when blank (`NULLIF`) | `handlers/admin_data.go` | ✅ |
| `CreateCourseUnit` now saves `default_venue_id` (NULLIF for blank string) | `handlers/admin_data.go` | ✅ |
| `AdminCourseUnits.tsx` venues dropdown now fetches real venues via tenant_id from roadmap response | `pages/admin/AdminCourseUnits.tsx` | ✅ |

### Session 11 — Lecturer attendance + E2E test + service startup

| Task | Deliverable | Status |
|---|---|---|
| DB migration 013: indexes on `lecturer_attendance_logs` + grants for 011/012 tables | `db/migrations/013_lecturer_attendance_indexes.sql` | ✅ |
| `OpenSession` now writes gate_open row to `lecturer_attendance_logs` when `lecturer_id` set | `handlers/sessions.go:OpenSession` | ✅ |
| `CloseSession` endpoint — writes `gate_close_time` + calculates `contact_hours` | `POST /api/v1/sessions/{session_id}/close` | ✅ |
| Admin: `GetLecturerAttendanceLogs` — joined query (lecturer + unit + session) | `GET /api/v1/admin/tenants/{id}/lecturer-attendance` | ✅ |
| Admin: `GetLecturerAttendanceSummary` — GROUP BY lecturer with totals/averages | `GET /api/v1/admin/tenants/{id}/lecturer-attendance/summary` | ✅ |
| Admin dashboard: `AdminLecturerAttendance.tsx` — summary cards + detail log table | `pages/admin/AdminLecturerAttendance.tsx` | ✅ |
| Coordinator PWA: `SessionPage.tsx` `onEnd` calls `CloseSession` before state machine transition | `pages/coordinator-pwa/SessionPage.tsx` | ✅ |
| SMTP test setup — `.env.smtp.example` + `scripts/start_qr_generator.sh` | docs + scripts | ✅ |
| E2E seed — fixed-UUID tenant + admin + coordinator + course + unit + lecturer | `db/seeds/003_e2e_test_data.sql` | ✅ |
| `scripts/start_all_local.sh` — one-command startup: builds + starts all 4 services with correct env | `scripts/start_all_local.sh` | ✅ |
| `tests/e2e/run_e2e_test.sh` — full E2E: health→login→manifest→open→close→lal→qr→sync | `tests/e2e/run_e2e_test.sh` | ✅ |
| **E2E test run: ALL 8 CHECKS PASS** — session lifecycle, lecturer_attendance_logs, QR generation, sync-receiver health | verified 2026-06-14 | ✅ |

### Session 12 — SMTP prep, mobile access, sync wiring

| Task | Deliverable | Status |
|---|---|---|
| Added student row to E2E seed — `jzany17@gmail.com`, `ACTIVE`, Year 1, Sem 1 | `db/seeds/003_e2e_test_data.sql` | ✅ |
| QR batch now shows `estimated=1` — seed student is picked up by batch endpoint | verified via E2E script | ✅ |
| `scripts/start_all_local.sh` updated — adds `SYNC_RECEIVER_URL=http://127.0.0.1:8083` to api-gateway | `scripts/start_all_local.sh` | ✅ |
| api-gateway restarted with `SYNC_RECEIVER_URL` — sync endpoints now proxy to local sync-receiver | running PID 452032 | ✅ |
| Coordinator PWA restarted with `VITE_API_URL=http://10.200.6.121:8080 --host` — accessible from mobile at `http://10.200.6.121:3000` | running PID 455408 | ✅ |

**Test status (as of 2026-06-14):**

| Test | Status | Notes |
|------|--------|-------|
| SMTP real email | ⏳ Awaiting credentials | Fill `.env.smtp`, then `source .env.smtp && ./scripts/start_qr_generator.sh`, then run E2E script |
| Offline session on real hardware | ⏳ Ready to run | Open `http://10.200.6.121:3000` on mobile → login as coordinator@test.local / Coord1234! → open session → check in → end |
| Sync round-trip | ✅ Proven (2026-06-29) | Verified live with a real 64-char hash → server resolves hash→reg-no → `attendance_logs` row written (`records_written=1`). Earlier silent failure (hash overflowed `student_id varchar(50)`) is fixed in `sync-receiver`. |

---

## What remains (production blockers)

| Area | What | Priority |
|---|---|---|
| **H3 (security)** | Move secrets out of `.env` into KMS/Vault; rotate KEK + VAPID key; replace all `changeme*`; enable Postgres TLS | High |
| ~~**M5 (sync)**~~ | ✅ DONE 2026-06-29 — real seal→decrypt→write round-trip proven (with a 64-char hash); server-side hash→reg-no resolution fix in `sync-receiver` | — |
| **H4 (design)** | Rotating room code relayable by absent students — consider shorter window or server-side ghost check | High |
| **M1–M4 (medium)** | XFF-spoofable rate limiter; in-process unbounded limiters; TOTP keyed by password hash; public Grafana LB | Medium |
| **CI** | Apply all 13 migrations in CI, seed, run tests/security as `qaat_app` | Medium |
| **Device testing** | First offline session on real mobile hardware (M4) — use `scripts/start_all_local.sh` | Pilot |
| **Real SMTP test** | Fill `.env.smtp` with Gmail/SendGrid credentials, source it, run `scripts/start_qr_generator.sh` | Pilot |
| **Staging deploy** | Run full k8s stack on staging; smoke E2E + k6 load tests | Pilot |
