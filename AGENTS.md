# QAAT — Agent guide

## Quick start

```bash
make keys          # RSA-2048 key pair (run once)
cp .env.example .env && make tidy && make install && make up
```

Default super-admin: `superadmin@qaat.platform` / `Super1234!`

## Package manager

**pnpm only** (no npm/yarn). Every frontend app has its own `package.json` — no shared workspace.
CI uses `pnpm install --frozen-lockfile`. All frontends run `pnpm typecheck` before `pnpm test`.

## Architecture

### Microservices (6)

| Service | Lang | Role |
|---------|------|------|
| `auth-service` | Go 1.21 | RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist |
| `api-gateway` | Go 1.21 | Routing, JWT middleware, RBAC, tenant middleware, rate limiter |
| `session-manager` | Go 1.21 | Warden delegation, exam clearance tokens |
| `sync-receiver` | Go 1.21 | Chunked AES-256 sealed-package upload, integrity verify, MV refresh |
| `qr-generator` | Node.js 20 | RSA-2048 QR signing, PNG generation, email delivery |
| `notification-service` | Node.js 20 | SMTP + Web Push (internal only, not via gateway) |

### Frontend apps (5)

- `coordinator-pwa/` — React 18 + Vite PWA (offline edge server)
- `coordinator-android/` — Native Android (phone-as-hub, Ktor server + Room/SQLite, replaces PWA)
- `admin-dashboards/` — React 18 (VC/DVC/DQA/QA/Admin + lecturer dashboard)
- `super-admin/` — React 18 (platform owner: register tenants, branding)
- `student-portal/` — React 18 (passwordless reg-no progress portal)

Frontends are deployed as separate Vercel projects (not in Docker Compose).

## DB connection roles

- `qaat` (superuser) — auth-service, sync-receiver, all migrations/seeds
- `qaat_app` (NOSUPERUSER NOBYPASSRLS) — api-gateway data plane, qr-generator, session-manager
- **Gateway has two DB URLs**: `DB_URL` (qaat_app, RLS-enforced for tenant data) and `ADMIN_DB_URL` (qaat superuser, ADMIN-only cross-tenant handlers).
- All tenant queries **must** `SET LOCAL app.current_tenant = '<uuid>'` (gateway's `SetTenant` middleware does this from JWT `tenant_id` claim).

## Offline-first design

Coordinator's laptop = Wi-Fi hotspot + server + DB. No internet needed for attendance.
Post-session sync: `POST /sync/init` → `/chunk/:id/:idx` → `/resume/:id` → `/complete/:id`.
Chunks stored in Redis (7-day TTL); completion triggers `REFRESH MATERIALIZED VIEW CONCURRENTLY student_attendance_summary`.

## Session state machine (XState v5)

`apps/coordinator-pwa/src/session/state-machine.ts`:
`IDLE → PENDING_LECTURER → ACTIVE → CLOSED / AUTO_CLOSED`
- T+120 min check-in window; T+180 min auto-kill
- Room code rotates every 15s (HOTP, ±1 skew = 45s window)

## Proximity model

Same-LAN (egress IP match) + live rotating room code. BLE/RSSI removed (migration 039).

## Key commands

| Command | What |
|---------|------|
| `make up` / `make down` | docker compose up/down |
| `make logs` | Tail all container logs |
| `make tidy` | `go mod tidy` on all Go services |
| `make install` | `pnpm install` on frontends |
| `make keys` | RSA-2048 key pair in `keys/` |
| `make migrate` | Re-applies only `001_init_schema.sql` |
| `make seed` | Loads `001_test_tenants` + `002_test_users` |
| `make lint` | `golangci-lint run ./...` on auth-service + api-gateway |
| `make dev-pwa` / `make dev-dashboards` | Vite dev servers |
| `make test-auth` / `make test-gateway` / `make test-pwa` | Per-service tests |
| `./scripts/start_all_local.sh` | Start all 4 Go services + QR natively (needs Postgres:5434 + Redis:6379) |

## Testing

- **Go integration tests** skip without `DB_URL` + `REDIS_URL` + `RSA_PRIVATE_KEY_PATH`.
- **RLS isolation suite** (`tests/security/`) **must** connect as `qaat_app` — running as `qaat` passes vacuously (superuser bypasses RLS).
- **E2E** (`tests/e2e/`): Playwright. Prerequisite: `db/seeds/003_e2e_test_data.sql`.
- **k6 load tests** in `tests/load/` need `k6` CLI.
- **CI** (`.github/workflows/ci.yml`): 5 parallel jobs — auth-service, api-gateway, coordinator-pwa, admin-dashboards, db-migrations. Applies migrations 001–009 only.

## DB migrations

48 files in `db/migrations/` (001–049, no 045). Auto-run on first empty-volume boot. `make migrate` re-runs only 001. Full reset: `make down && docker volume rm qaat_pgdata && make up`.

## Key env vars

| Variable | Needed by | Notes |
|----------|-----------|-------|
| `KEY_ENCRYPTION_KEY` | auth-service, qr-generator, sync-receiver | 32-byte hex, `openssl rand -hex 32`, no all-zeros |
| `INTERNAL_SVC_KEY` | auth-service, api-gateway | Shared secret for student token minting after QR verify |
| `SYNC_SIGN_KEY` | sync-receiver | Random 32-byte hex for HMAC |
| `DISABLE_MFA` | auth-service | `true` in dev (bypasses TOTP for VC/DQA) |

`.env.smtp` is optional — sourced by `start_all_local.sh` for SMTP config.

## Important constraints

- **Append-only ledger**: `attendance_logs` has DELETE revoked; corrections = new rows with `entry_method = MANUAL_OVERRIDE`.
- **Students identified by registration number only** (no email/phone/password). QR is their login.
- **One-device-one-person**: hardware fingerprint prevents device reuse by different students in same session.
- Gateway strips inbound `X-Tenant-ID`, `X-User-ID`, `X-Role` before setting its own values.
- QR generator docker port is `3012:3002` (external:internal).

## API

~136 endpoints, all proxied through `api-gateway:8443`. Full spec: `docs/openapi.yaml`.
Key groups: `POST /api/v1/auth/*`, `POST /api/v1/sync/*` (COORD), `POST /api/v1/qr/*` (DQA_DIRECTOR/ADMIN), `/api/v1/admin/tenants/{id}/*` (ADMIN). Notification service is internal-only (`POST /notify/*`).

## Coordinator Android app

`apps/coordinator-android/` — phone-as-hub native app. Crypto-core and engine modules compile as pure JVM and are verified. App module needs Android SDK (not in CI). Guide: `BUILD_AND_TEST.md`.
