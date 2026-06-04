# QAAT — Quality Assurance Attendance Tracker

QAAT is a multi-tenant, offline-first SaaS platform for university attendance. It combines BLE beacons, signed QR codes, and hardware fingerprinting to eliminate proxy attendance ("sign-ins for absent students") and ghost lectures.

The architecture is a decentralised edge–cloud hybrid: all live session logic runs **offline** on the Coordinator's PWA (the edge server), while the cloud handles pre-session provisioning and post-session sync.

## Features

- **Proxy-resistant check-in** — RSA-2048 signed QR codes validated against a roster, BLE/RSSI proximity, hardware fingerprint binding, and one-device-per-session enforcement.
- **Offline-first edge server** — sessions run entirely on the Coordinator PWA with no network dependency; results sync to the cloud afterwards.
- **Multi-tenancy with hard isolation** — PostgreSQL Row-Level Security on every table, enforced per request from the JWT tenant claim.
- **Append-only attendance ledger** — attendance records cannot be deleted; corrections are new rows with a `MANUAL_OVERRIDE` entry method.
- **Resilient chunked sync** — resumable AES-256 chunked upload with vector-clock deduplication and SHA-256 integrity checks.
- **Custom in-house authentication** — RS256 JWTs, bcrypt password hashing, TOTP MFA, and a Redis-backed token blacklist.
- **Role-based dashboards** — VC, DQA, QA, and Admin views plus a lightweight student attendance portal.

## Architecture

```
services/
  auth-service/         Go 1.21 — RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist
  api-gateway/          Go 1.21 — routing, JWT middleware, RBAC, tenant, rate limiter, Prometheus
  qr-generator/         Node.js 20 — RSA-2048 QR signing, PNG generation, email delivery
  session-manager/      Go 1.21 — warden delegation (GPS geofence), exam clearance tokens
  sync-receiver/        Go 1.21 — chunked AES-256 upload, vector-clock dedup, eligibility trigger
  notification-service/ Node.js 20 — SMTP + Web Push notifications

apps/
  coordinator-pwa/      React 18 + TypeScript + Vite + PWA (offline edge server)
  admin-dashboards/     React 18 + TypeScript + React Router (VC / DQA / QA / Admin)
  student-portal/       React 18 lightweight SPA (personal attendance %)

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
  e2e/                  Playwright E2E tests (auth, dashboards, API security headers)

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

### QR validation (8 steps)
All eight steps live in [apps/coordinator-pwa/src/qr/validator.ts](apps/coordinator-pwa/src/qr/validator.ts):

1. RSA-2048 signature (SubtleCrypto)
2. Expiry check
3. Tenant ID match
4. Roster lookup (SHA-256 hashed student ID)
5. BLE RSSI ≥ threshold
6. Hardware fingerprint (bound on first scan)
7. Duplicate scan
8. One-device-per-session

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

- **Coordinator PWA LAN server** — the BroadcastChannel between the service worker and main thread for student QR submission is not yet wired.
- **Reporting engine** — `GET /api/v1/dashboard/vc/overview` computes inline in handlers; a dedicated reporting service would improve performance at scale.
- **SIS automated pull** — `POST /api/v1/import/trigger` is a stub; the OAuth 2.0 client to each institution's SIS REST API needs to be built per client.
- **Beacon RSSI calibration UI** — currently done manually via `PUT /dqa/thresholds`.

## Documentation

- [ARCHITECT.md](ARCHITECT.md) — full system architecture.
- [DEPLOY.md](DEPLOY.md) — deployment guide.
- [USER_GUIDE.md](USER_GUIDE.md) — end-user guide.
- [docs/openapi.yaml](docs/openapi.yaml) — API specification.
