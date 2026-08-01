# QAAT — Agent guide

## Quick start

```bash
make keys          # RSA-2048 key pair (run once)
cp .env.example .env && make tidy && make install && make up
```

Default super-admin: `superadmin@qaat.platform` / `Super1234!`

## Directory layout

| Path | Contents |
|------|----------|
| `backend/` | 4 Go services (auth-service, api-gateway, session-manager, sync-receiver) + 2 Node.js services (qr-generator, notification-service) |
| `frontend/` | 4 apps (admin-dashboards, coordinator-pwa, student-portal, coordinator-android) |
| `db/migrations/` | 60 SQL files (001–062, no 045 or 051). Auto-run on empty PG volume |
| `db/seeds/` | `001_test_tenants.sql`, `002_test_users.sql`, `003_e2e_test_data.sql` |
| `tests/security/` | Go RLS isolation tests (must connect as `qaat_app`) |
| `tests/e2e/` | Playwright + shell-based E2E |
| `tests/load/` | k6 scripts |
| `infra/` | Docker Compose + K8s manifests + Caddy config |

## ⚠️ Stale CI/Makefile

CI workflow (`.github/workflows/ci.yml`), Makefile, and `scripts/start_all_local.sh` reference **`services/`** and **`apps/`** which **do not exist** in current layout. Actual paths are `backend/` and `frontend/`. These scripts will fail without manual path correction.

## Package manager

**pnpm only** (no npm/yarn). Each frontend/Node app has its own `package.json` + lockfile — no shared workspace.
CI uses `pnpm@9.1.0`; project files declare `pnpm@9.15.9`.

## Backend services

| Service | Lang | Framework | Notes |
|---------|------|-----------|-------|
| `auth-service` | Go 1.21 | chi/v5 | RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist |
| `api-gateway` | Go 1.25.0 | chi/v5 | JWT middleware, RBAC, tenant middleware, rate limiter, Prometheus |
| `session-manager` | Go 1.21 | stdlib | No chi. Warden delegation, exam clearance tokens |
| `sync-receiver` | Go 1.21 | chi/v5 | Chunked AES-256 sealed-package upload, integrity verify, MV refresh |
| `qr-generator` | Node.js/Express | tsx watch | RSA-2048 QR signing, PNG, email. Has vitest tests |
| `notification-service` | Node.js/Express | tsx watch | SMTP + Web Push. No tests. Internal-only (not via gateway) |

Node.js services run via `pnpm dev` (tsx watch). Both support `pnpm typecheck`.

## Frontend apps

| App | Stack | Notes |
|-----|-------|-------|
| `coordinator-pwa` | React 18 + Vite + PWA + XState v5 | Offline edge server = room hotspot + hub. Session state machine: `IDLE → PENDING_LECTURER → ACTIVE → CLOSED / AUTO_CLOSED` (T+120 check-in, T+180 auto-kill). Room code rotates every 15s (HOTP, ±1 skew = 45s window) |
| `admin-dashboards` | React 18 + React Router | VC/DVC/DQA/QA/Admin + lecturer dashboard |
| `student-portal` | React 18 | Passwordless reg-no progress portal |
| `coordinator-android` | Native Android, Ktor + Room/SQLite | Phone-as-hub. See `BUILD_AND_TEST.md` |

All use `pnpm typecheck` before `pnpm test` (CI enforces this). `make dev-pwa` / `make dev-dashboards` for Vite dev servers.

## DB connection roles

- `qaat` (superuser) — auth-service, sync-receiver, all migrations/seeds, gateway's ADMIN_DB_URL
- `qaat_app` (NOSUPERUSER NOBYPASSRLS) — api-gateway data plane, qr-generator, session-manager
- Gateway has **two DB URLs**: `DB_URL` (qaat_app, RLS-enforced) and `ADMIN_DB_URL` (qaat superuser, ADMIN-only cross-tenant handlers)
- All tenant queries **must** `SET LOCAL app.current_tenant = '<uuid>'` (gateway's `SetTenant` middleware does this from JWT `tenant_id` claim)

## Key env vars

| Variable | Needed by | Notes |
|----------|-----------|-------|
| `KEY_ENCRYPTION_KEY` | auth-service, qr-generator, sync-receiver | 32-byte hex, `openssl rand -hex 32`, no all-zeros |
| `INTERNAL_SVC_KEY` | auth-service, api-gateway | Shared secret for student token minting after QR verify |
| `SYNC_SIGN_KEY` | sync-receiver | Random 32-byte hex for HMAC |
| `DISABLE_MFA` | auth-service | `true` in dev (bypasses TOTP for VC/DQA) |

`.env.smtp` is optional — sourced by `start_all_local.sh` for SMTP config.

## Commands

| Command | Action |
|---------|--------|
| `make up` / `make down` | `docker-compose -f infra/docker-compose.yml up -d` / down |
| `make build` | Build all images |
| `make logs` | Tail all container logs |
| `make tidy` | `go mod tidy` on all Go services (run first on fresh clone) |
| `make install` | `pnpm install` on coordinator-pwa + admin-dashboards |
| `make keys` | Generate RSA-2048 key pair in `keys/` |
| `make migrate` | Re-applies only `001_init_schema.sql` |
| `make seed` | Loads test tenants + users |
| `make lint` | `golangci-lint` on auth-service + api-gateway only |
| `make test-auth` / `test-gateway` / `test-pwa` | Per-service tests |
| `./scripts/start_all_local.sh` | Start auth-service (:8090) + api-gateway (:8080) + qr-generator (:3002) + sync-receiver (:8083) natively. Needs PG:5434 + Redis:6379. Logs in `/tmp/*.log` |

## Tests

| Suite | Command | Prerequisites |
|-------|---------|---------------|
| Go unit (auth) | `cd backend/auth-service && go test ./... -race -count=1` | DB_URL + REDIS_URL + RSA keys |
| Go unit (gateway) | `cd backend/api-gateway && go test ./... -race -count=1` | Same |
| Go unit (session-mgr) | `cd backend/session-manager && go test ./... -race -count=1` | Same |
| PWA | `cd frontend/coordinator-pwa && pnpm test` | None |
| Admin dashboards | `cd frontend/admin-dashboards && pnpm test` | None |
| RLS isolation | `cd tests/security && go test ./... -count=1 -v` | **Must connect as `qaat_app`** (superuser passes vacuously) |
| E2E | `cd tests/e2e && pnpm test` (Playwright) | Seeded via `003_e2e_test_data.sql`; needs full stack |
| Load | `k6 run tests/load/...` | Needs `k6` CLI |

Go integration tests skip without DB_URL. QR generator also has vitest: `cd backend/qr-generator && pnpm test`.

## Infrastructure

- Docker Compose: `infra/docker-compose.yml` via `docker-compose` (hyphenated)
- External ports: PG `5434:5432`, Redis `6380:6379`, QR `3012:3002`, Mailhog `8025`
- HTTPS proxy: Caddy with self-signed TLS (`infra/certs/qaat.crt`)
- K8s: `infra/k8s/` with Kustomize overlays for staging/production

## Key constraints

- **Append-only ledger**: `attendance_logs` has DELETE revoked; corrections = new rows with `entry_method = MANUAL_OVERRIDE`
- **Students identified by registration number only** (no email/phone/password). QR is their login
- **One-device-one-person**: hardware fingerprint prevents device reuse by different students in same session
- Gateway strips inbound `X-Tenant-ID`, `X-User-ID`, `X-Role` before setting its own values
- `make tidy` **must** run before `go build` or `go test` on a fresh clone
- QR generator docker port mapping: `3012:3002` (external:internal)
- All endpoints proxied through `api-gateway:8443`. Notification service is internal-only (`POST /notify/*`)
- RLS enforced via `qaat_app` role (NOSUPERUSER NOBYPASSRLS). Every query SET LOCAL app.current_tenant
- Proximity: same-LAN (egress IP match) + live rotating room code. BLE/RSSI removed (migration 039)
