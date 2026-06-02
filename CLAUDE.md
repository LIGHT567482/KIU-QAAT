# QAAT — Codebase Guide for Claude

## What this is
QAAT (Quality Assurance Attendance Tracker) is a multi-tenant, offline-first SaaS platform for university attendance. The system uses BLE beacons + QR codes + hardware fingerprinting to eliminate proxy attendance and ghost lectures.

**Architecture:** Decentralised edge-cloud hybrid. All session logic runs offline on the Coordinator's PWA (the edge server). The cloud handles pre-session provisioning and post-session sync.

**Key constraint:** Custom auth only — no Auth0/Firebase/Cognito/Clerk ever. See `services/auth-service/`.

## Repo layout

```
services/
  auth-service/        Go 1.21 — RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist
  api-gateway/         Go 1.21 — routing, JWT middleware, RBAC, tenant, rate limiter, Prometheus
  qr-generator/        Node.js 20 — RSA-2048 QR signing, PNG generation, email delivery
  session-manager/     Go 1.21 — warden delegation (GPS geofence), exam clearance tokens
  sync-receiver/       Go 1.21 — chunked AES-256 upload, Vector Clock dedup, eligibility trigger
  notification-service/ Node.js 20 — SMTP + Web Push notifications

apps/
  coordinator-pwa/     React 18 + TypeScript + Vite + PWA (offline edge server)
  admin-dashboards/    React 18 + TypeScript + React Router (VC / DQA / QA / Admin)
  student-portal/      React 18 lightweight SPA (personal attendance %)

db/
  migrations/          001–005 SQL migration files (run in order by Postgres init)
  seeds/               Two test tenants + 5 users for RLS isolation testing
  rls-policies/        (inline in 003_rls_policies.sql)

infra/
  docker-compose.yml   Full local dev stack (Postgres, Redis, all services, Mailhog)
  k8s/                 Kubernetes manifests + Kustomize overlays for staging/production

tests/
  security/            Go RLS isolation tests (all 9 tables, append-only guard)
  load/                k6 load scripts (300 scans/min, 10k sync ops, rate limiter)
  e2e/                 Playwright E2E tests (auth, dashboards, API security headers)

docs/
  openapi.yaml         OpenAPI 3.1 spec for all 22 endpoints
  PILOT_CHECKLIST.md   50-item Phase 4 pilot checklist
```

## Getting started locally

```bash
make keys          # Generate RSA-2048 key pair (run once)
cp .env.example .env
make tidy          # go mod tidy on all Go services
make install       # pnpm install on all frontend apps
make up            # docker compose up (all services)
```

Default ports: API Gateway :8443, Auth :8081, Admin Dashboards :3001, Coordinator PWA :3000, Student Portal :3003, QR Generator :3002, Notification :3004, Mailhog UI :8025.

## Running tests

```bash
# Go unit tests (skip integration without DB_URL)
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

### PostgreSQL RLS
Every query **must** `SET LOCAL app.current_tenant = '<uuid>'` before touching data. The `SetTenant` middleware in `services/api-gateway/internal/middleware/tenant.go` does this automatically from the JWT `tenant_id` claim. Never bypass this.

### Attendance logs are append-only
`attendance_logs` has a RESTRICTIVE RLS policy that blocks DELETE at the DB level (`003_rls_policies.sql`). Corrections create new rows with `entry_method = MANUAL_OVERRIDE`. The `qaat_app` DB role also has DELETE revoked.

### Session state machine
Defined in `apps/coordinator-pwa/src/session/state-machine.ts` using XState v5. Mirrors ARCHITECT.md §5 exactly: IDLE → PENDING\_LECTURER → ACTIVE → CLOSED / AUTO\_CLOSED. Timers enforce T+120 checkin window and T+180 auto-kill.

### QR validation (8 steps)
All 8 steps live in `apps/coordinator-pwa/src/qr/validator.ts`:
1. RSA-2048 signature (SubtleCrypto)
2. Expiry check
3. Tenant ID match
4. Roster lookup (SHA-256 hashed student\_id)
5. BLE RSSI ≥ threshold
6. Hardware fingerprint (bind on first scan)
7. Duplicate scan
8. One-device-per-session

### Chunked sync protocol
`sync-receiver` expects: POST /sync/init → POST /sync/chunk/:id/:idx → GET /sync/resume/:id (on reconnect) → POST /sync/complete/:id. Each chunk stored in Redis (7-day TTL). Complete validates SHA-256 checksum, writes `attendance_logs`, triggers `REFRESH MATERIALIZED VIEW CONCURRENTLY student_attendance_summary`.

### JWT security
RS256 (not HS256). Private key never leaves `auth-service`. Public key shared with `api-gateway` via mounted file. Every JWT has a `jti` claim stored in Redis on issuance; logout/refresh blacklists it. Token TTL = 24h.

### Multi-tenancy
Single PostgreSQL cluster, RLS on all 12 tables. `tenant_id` is always a UUID FK to `tenants`. The `student_id_hash` (SHA-256 of registration number) is stored on the Coordinator PWA — never the raw registration number.

## Package manager
**pnpm** for all Node.js/frontend work. Never npm or yarn.

## CI
`.github/workflows/ci.yml` runs 4 jobs in parallel: auth-service (build + test), api-gateway (build + test), coordinator-pwa (typecheck + test), db-migrations (apply + RLS verify). Each Go job runs `go mod tidy` first.

## Adding a new API endpoint
1. Add handler in `services/api-gateway/internal/handlers/`
2. Wire it in `services/api-gateway/internal/router/router.go` with `RequireRole()`
3. Add the RLS `SET LOCAL` call via the `GetDB(ctx)` helper or acquire a connection directly
4. Document in `docs/openapi.yaml`
5. Add an E2E test in `tests/e2e/specs/api.spec.ts`

## What is NOT built yet
- Coordinator PWA: LAN server is scaffolded but the BroadcastChannel between SW and main thread for student QR submission is not wired
- Reporting Engine: `GET /api/v1/dashboard/vc/overview` computes inline in handlers — a dedicated reporting service would improve performance at scale
- SIS automated pull: `POST /api/v1/import/trigger` is a stub; the OAuth 2.0 client to the SIS REST API per institution needs to be built per-client
- Admin dashboard: beacon RSSI calibration UI not built (done manually via PUT /dqa/thresholds)
