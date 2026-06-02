# QAAT — Build Progress
**Last updated:** 2026-05-31 (session 7)
**Reference docs:** ARCHITECT.md · technicaldoc.md · plan.md  
**Current position:** All 4 phases substantially complete — ready for pilot execution

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
| BLE proximity enforcement | ⏳ PWA BLE scanner not yet wired |
| Session package sealing (pre-sync) | ⏳ PWA outbox not yet built |

---

## What to build next (priority order)

1. **PWA Daily Fetch flow** — download manifest, decrypt, store in IndexedDB (unblocks BLE + QR scanner work)
2. **PWA BLE scanner** — Web Bluetooth, 10s weighted RSSI average
3. **PWA QR Validation Engine** — all 8 steps (RSA → roster → BLE → fingerprint → duplicate → one-device)
4. **DQA Threshold API** + policy propagation to Daily Manifest
5. **PWA session sealing + outbox** — completes the offline → sync loop
6. **Dashboard live data** — connect all 3 dashboards to real API endpoints
