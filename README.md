# QAAT — Quality Assurance Attendance Tracker

QAAT is a multi-tenant, **offline-first** SaaS platform for university attendance. It combines signed QR codes, a live rotating room code, **same-LAN (Wi-Fi) proximity**, and one-device-one-person binding to eliminate proxy attendance ("sign-ins for absent students") and ghost lectures.

**Attendance is taken completely offline.** The coordinator's hub — a **Linux laptop**, or a **native Android app** on a phone — is the room's Wi-Fi hotspot *and* the local server + database. Students' and the lecturer's phones join that hotspot and submit over the LAN; every log is written to the hub's database the instant it is accepted. No internet is needed in the room. When the hub later has connectivity, each closed session is sealed and **atomically** synced to the central SaaS database.

> **Capacity reality — one access point ≈ one classroom.** A single hotspot holds a limited number of phones at once (**~10 on a stock Android**, ~20–40 on a laptop). So students **rotate**: each turns Wi-Fi **off** the moment they're marked present (the check-in screen says so explicitly), freeing a slot for the next. Large groups are served by this rotation over time, by several coordinators/APs in parallel, or by putting the hub's server on campus Wi-Fi. The Go server scales to thousands; the **Wi-Fi radio is the real limit**, not the software.

> **Note (proximity model):** earlier builds used BLE beacons/RSSI for proximity. That was **removed** (migration 039). Proximity is now proven by **being on the coordinator's hotspot LAN plus the live rotating room code** — simpler, hardware-free, and works on every phone.

## Features

- **Proxy-resistant check-in** — signed personal QR validated against the roster, a **live rotating room code**, **same-LAN proximity** (must be on the coordinator's hotspot), hardware-fingerprint binding, and **one-device-one-person** enforcement per session.
- **Fully-offline edge server** — the coordinator's laptop runs the stack and is the room hotspot; sessions run with **zero network dependency**; sealed results sync to the cloud afterwards.
- **Passwordless identities** — students are identified by **registration number only** (no email/phone/password); lecturers by **staff ID**. Both get a **permanent QR**; lecturers also have a staff-ID dashboard login.
- **Passwordless student progress portal** — a single page where a student types their reg-no (scoped to their institution) and sees their own attendance % and exam eligibility — no account, no login.
- **Standby coordinator** — if a coordinator is absent, they can pre-authorise an **own-cohort student** with a one-day code to run that day's session as their deputy (coordinator-only; never an admin).
- **Optional QR email dispatch** — an optional email may be supplied for a lecturer or student **solely** to email them their QR on create/import; blank means no email is sent.
- **Multi-tenancy with hard isolation** — PostgreSQL Row-Level Security on every table, enforced per request from the JWT tenant claim.
- **Append-only attendance ledger** — attendance records cannot be deleted; corrections are new rows with a `MANUAL_OVERRIDE` entry method.
- **Resilient, atomic sync** — each closed session is sealed (AES-256-GCM + device-bound HMAC-SHA256 + SHA-256 checksum), chunked, and uploaded all-or-nothing with resume + retries.
- **Custom in-house authentication** — RS256 JWTs, bcrypt password hashing, TOTP MFA (dev-toggleable), and a Redis-backed token blacklist.
- **Role-based dashboards** — VC, DVC, DQA, QA, and Admin views plus the student progress portal.
- **Curriculum model** — a **course** is created independently of level (no level/total-years on the course); **levels** (Certificate/Diploma/Degree/Masters…) are added inside the course, each with its own year × semester unit roadmap.
- **Global cohorts** — a cohort (session · year · semester · level · intake) can be created once and applied across **all** courses at once, rather than per course.
- **Lecturer assignment + attendance tracking** — lecturers are assigned to specific units; `lecturer_attendance_logs` records gate-open/close + contact hours per session (anti-ghost-lecture audit trail), surfaced in the admin and lecturer dashboards.
- **Active semester control + rollover** — admin sets the active academic year/semester; a password-confirmed "advance semester" promotes every student and cohort one step (final level/year → GRADUATED).

## Architecture

```
services/
  auth-service/         Go 1.21 — RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist
  api-gateway/          Go 1.21 — routing, JWT middleware, RBAC, tenant, rate limiter, Prometheus
  qr-generator/         Node.js 20 — RSA-2048 QR signing, PNG generation, QR email delivery (/qr/email-link)
  session-manager/      Go 1.21 — warden delegation, exam clearance tokens
  sync-receiver/        Go 1.21 — chunked AES-256 sealed-package upload, integrity verify, eligibility refresh
  notification-service/ Node.js 20 — SMTP + Web Push notifications

apps/
  coordinator-pwa/      React 18 + TypeScript + Vite + PWA (offline edge server = room hotspot + hub)
  admin-dashboards/     React 18 + TypeScript + React Router (VC / DVC / DQA / QA / Admin + lecturer dashboard)
  student-portal/       React 18 lightweight SPA (passwordless reg-no progress portal)
  super-admin/          React 18 SPA (platform owner: register tenants, branding, billing)

db/
  migrations/           SQL migration files (run in order by Postgres init)
  seeds/                Test tenants + users for RLS isolation testing
  rls-policies/         (inline in 003_rls_policies.sql)

infra/
  docker-compose.yml    Full local dev stack (Postgres, Redis, all services, Mailhog)
  k8s/                  Kubernetes manifests + Kustomize overlays for staging/production

tests/
  security/             Go RLS isolation tests (all tables, append-only guard)
  load/                 k6 load scripts (300 scans/min, 10k sync ops, rate limiter)
  e2e/                  Playwright E2E tests + `run_e2e_test.sh` (shell-based E2E covering session lifecycle, lecturer_attendance_logs, QR, sync)

docs/
  openapi.yaml          OpenAPI 3.1 spec for all endpoints
  PILOT_CHECKLIST.md    Phase 4 pilot checklist
```

## Getting started

Requirements: Docker + Docker Compose, Go 1.21, Node.js 20, and [pnpm](https://pnpm.io).

```bash
make keys          # Generate the RSA-2048 key pair (run once)
cp .env.example .env
make tidy          # go mod tidy across all Go services
make install       # pnpm install across all frontend apps
make up            # docker compose up (full stack)
```

### Default ports

| Service              | Port  |
|----------------------|-------|
| API Gateway          | 8443  |
| Auth service         | 8081  |
| Coordinator PWA      | 3000  |
| Admin dashboards     | 3001  |
| QR generator         | 3002  |
| Student portal       | 3003  |
| Notification service | 3004  |
| Mailhog UI           | 8025  |

> Frontends are served behind Caddy (HTTPS). The student progress portal is opened
> per-institution as `https://<host>:3003/?org=<institution-domain>`.

> **Package manager:** all Node.js/frontend work uses **pnpm** — not npm or yarn.

## Running tests

```bash
# Go unit tests (integration tests skip without DB_URL)
cd services/auth-service && go test ./...

# Integration tests (need running DB + Redis)
DB_URL=postgres://... REDIS_URL=redis://... RSA_PRIVATE_KEY_PATH=keys/auth_private.pem \
  RSA_PUBLIC_KEY_PATH=keys/auth_public.pem go test ./...

# RLS isolation tests
cd tests/security && DB_URL=postgres://... go test ./... -v

# k6 load test
k6 run --env BASE_URL=http://localhost:8443 tests/load/k6-scan-session.js

# Playwright E2E
cd tests/e2e && pnpm install && pnpm test
```

## Key design decisions

### PostgreSQL Row-Level Security
Every query **must** `SET LOCAL app.current_tenant = '<uuid>'` before touching data. The `SetTenant` middleware in [services/api-gateway/internal/middleware/tenant.go](services/api-gateway/internal/middleware/tenant.go) does this automatically from the JWT `tenant_id` claim. This is never bypassed.

### Append-only attendance logs
`attendance_logs` has a RESTRICTIVE RLS policy that blocks DELETE at the database level (`db/migrations/003_rls_policies.sql`). Corrections create new rows with `entry_method = MANUAL_OVERRIDE`, and the `qaat_app` DB role has DELETE revoked.

### Session state machine
Defined in [apps/coordinator-pwa/src/session/state-machine.ts](apps/coordinator-pwa/src/session/state-machine.ts) using XState v5: `IDLE → PENDING_LECTURER → ACTIVE → CLOSED / AUTO_CLOSED`. Timers enforce the T+120 check-in window and T+180 auto-kill.

### Check-in validation (the proof factors)
Identity is proven by the signed QR; presence + uniqueness are proven by the room and the device. A student is recorded `PRESENT` only when **all** hold (enforced on the coordinator's laptop, offline):

1. Signed QR → resolves the student's account (passwordless QR-login)
2. Session is `ACTIVE` and inside the daily window
3. The **live rotating room code** is valid (read off the coordinator's screen)
4. The phone is **on the coordinator's hotspot LAN** (else `NOT_SAME_NETWORK`)
5. **One-device-one-person**: this device hasn't already checked in a different student this session (else `DEVICE_ALREADY_USED`)
6. Not already present (idempotent)

### Chunked sync protocol
`sync-receiver` expects: `POST /sync/init` → `POST /sync/chunk/:id/:idx` → `GET /sync/resume/:id` (on reconnect) → `POST /sync/complete/:id`. Each chunk is stored in Redis (7-day TTL). Complete validates the SHA-256 checksum, writes `attendance_logs`, and triggers `REFRESH MATERIALIZED VIEW CONCURRENTLY student_attendance_summary`.

### JWT security
RS256 (not HS256). The private key never leaves `auth-service`; the public key is shared with `api-gateway` via a mounted file. Every JWT carries a `jti` claim stored in Redis on issuance; logout and refresh blacklist it. Token TTL is 24h.

### Multi-tenancy
A single PostgreSQL cluster with RLS on all tables. `tenant_id` is always a UUID FK to `tenants`. The `student_id_hash` (SHA-256 of the registration number) is stored on the Coordinator PWA — never the raw registration number.

## Continuous integration
`.github/workflows/ci.yml` runs four jobs in parallel: auth-service (build + test), api-gateway (build + test), coordinator-pwa (typecheck + test), and db-migrations (apply + RLS verify). Each Go job runs `go mod tidy` first.

## Adding a new API endpoint
1. Add a handler in `services/api-gateway/internal/handlers/`.
2. Wire it in `services/api-gateway/internal/router/router.go` with `RequireRole()`.
3. Add the RLS `SET LOCAL` call via the `GetDB(ctx)` helper or acquire a connection directly.
4. Document it in `docs/openapi.yaml`.
5. Add an E2E test in `tests/e2e/specs/api.spec.ts`.

## Roadmap

The following are scaffolded or stubbed and not yet complete:

- **Reporting engine** — `GET /api/v1/dashboard/vc/overview` computes inline in handlers; a dedicated reporting service would improve performance at scale.
- **SIS automated pull** — `POST /api/v1/import/trigger` is a stub; the OAuth 2.0 client to each institution's SIS REST API needs to be built per client.
- **WebAuthn lecturer fingerprint** — requires a stable hostname RP ID (not a bare IP); optional, off by default.

## Documentation

- [docs/SYSTEM_TEST_GUIDE.md](docs/SYSTEM_TEST_GUIDE.md) — **test the whole system with real students & devices** (backend → DB → frontends → QR → live offline session → verify).
- [flow.md](flow.md) — **whole-system flowchart** (+ pre-rendered [flow-1-large.png](flow-1-large.png) / [flow-2.png](flow-2.png)).
- [docs/FLOWCHART.md](docs/FLOWCHART.md) — the offline attendance gate, step by step.
- [ARCHITECT.md](ARCHITECT.md) — full system architecture.
- [DEPLOY.md](DEPLOY.md) / [RUN-ANYWHERE.md](RUN-ANYWHERE.md) — deployment + run-on-any-laptop (offline hotspot) guide.
- [docs/CLOUD_DEPLOY_RENDER.md](docs/CLOUD_DEPLOY_RENDER.md) — **move the DB + services to the cloud (Render)**; how clients connect via the gateway (never the DB); offline roster pull.
- [USER_GUIDE.md](USER_GUIDE.md) — end-user guide.
- [docs/API.md](docs/API.md) — API overview.
- [docs/SECURITY_PRIVACY_REVIEW.md](docs/SECURITY_PRIVACY_REVIEW.md) — security & privacy review.
