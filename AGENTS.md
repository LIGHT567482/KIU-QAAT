# QAAT — Agent guide

## Quick start

```bash
make keys          # RSA-2048 key pair (run once)
cp .env.example .env && make tidy && make install && make up
```

Default super-admin: `superadmin@qaat.platform` / `Super1234!`

## Package manager

**pnpm only** (no npm/yarn). Every frontend/app has its own `package.json` — no shared workspace.
CI uses `pnpm install --frozen-lockfile` with `pnpm@9.1.0` (project package.json files declare `pnpm@9.15.9`).
All frontends run `pnpm typecheck` before `pnpm test`.

## Architecture

### Go services (5)

| Service | Go | Role |
|---------|----|------|
| `auth-service` | 1.21 | RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist |
| `api-gateway` | 1.25.0 | Routing, JWT middleware, RBAC, tenant middleware, rate limiter, Prometheus |
| `session-manager` | 1.21 | Warden delegation, exam clearance tokens |
| `sync-receiver` | 1.21 | Chunked AES-256 sealed-package upload, integrity verify, MV refresh |

All four use `github.com/go-chi/chi/v5`.

### Node.js services (2)

| Service | Role |
|---------|------|
| `qr-generator` | Express, RSA-2048 QR signing, PNG generation, email delivery |
| `notification-service` | Express, SMTP + Web Push, internal only (not via gateway) |

Run via `pnpm dev` (tsx watch). QR generator has test/vitest; notification-service does not.

### Frontend apps (5)

- `coordinator-pwa/` — React 18 + Vite PWA (offline edge server)
- `coordinator-android/` — Native Android (phone-as-hub, Ktor server + Room/SQLite)
- `admin-dashboards/` — React 18 (VC/DVC/DQA/QA/Admin + lecturer dashboard)
- `super-admin/` — React 18 (platform owner: register tenants, branding)
- `student-portal/` — React 18 (passwordless reg-no progress portal)

Frontends are deployed as separate Vercel projects (not in Docker Compose).

`make install` + `make dev-*` cover only `coordinator-pwa` and `admin-dashboards`.

## Infrastructure

| What | Details |
|------|---------|
| Docker Compose | `infra/docker-compose.yml`, invoked via `docker-compose` (hyphenated) |
| External ports | PG `5434:5432`, Redis `6380:6379`, QR `3012:3002`, Mailhog `8025` |
| HTTPS proxy | Caddy in front of all services with self-signed TLS (`infra/certs/qaat.crt`) |
| K8s | `infra/k8s/` — manifests + Kustomize overlays for staging/production |

## DB connection roles

- `qaat` (superuser) — auth-service, sync-receiver, all migrations/seeds
- `qaat_app` (NOSUPERUSER NOBYPASSRLS) — api-gateway data plane, qr-generator, session-manager
- **Gateway has two DB URLs**: `DB_URL` (qaat_app, RLS-enforced) and `ADMIN_DB_URL` (qaat superuser, ADMIN-only cross-tenant handlers).
- All tenant queries **must** `SET LOCAL app.current_tenant = '<uuid>'` (gateway's `SetTenant` middleware does this from JWT `tenant_id` claim).

## Tests

| Suite | Location | Run command | Prerequisites |
|-------|----------|-------------|---------------|
| Go unit (auth) | `services/auth-service/` | `go test ./... -race -count=1` | DB_URL + REDIS_URL + RSA keys (or CI containers) |
| Go unit (gateway) | `services/api-gateway/` | `go test ./... -race -count=1` | Same as above |
| Go unit (session-mgr) | `services/session-manager/` | `go test ./... -race -count=1` | Same |
| PWA unit | `apps/coordinator-pwa/` | `pnpm test` (vitest) | None |
| Admin dashboards | `apps/admin-dashboards/` | `pnpm test` | None |
| RLS isolation | `tests/security/` | `go test ./... -count=1 -v` | **Must connect as `qaat_app`** (superuser passes vacuously) |
| E2E | `tests/e2e/` | `pnpm test` (Playwright) | Seeded via `db/seeds/003_e2e_test_data.sql`; needs full stack |
| Load | `tests/load/` | `k6 run ...` | Needs `k6` CLI |

- The shell-based E2E runner `tests/e2e/run_e2e_test.sh` tests session lifecycle + QR email + sync w/ real `curl`.
- Playwright E2E specs in `specs/` with chromium + mobile-android projects.

## DB migrations

48 files in `db/migrations/` (001–049, no 045). Auto-run on first empty-volume boot.
`make migrate` re-runs only `001_init_schema.sql`. CI applies migrations 001–009.
Full reset: `make down && docker volume rm qaat_pgdata && make up`.

## Key env vars

| Variable | Needed by | Notes |
|----------|-----------|-------|
| `KEY_ENCRYPTION_KEY` | auth-service, qr-generator, sync-receiver | 32-byte hex, `openssl rand -hex 32`, no all-zeros |
| `INTERNAL_SVC_KEY` | auth-service, api-gateway | Shared secret for student token minting after QR verify |
| `SYNC_SIGN_KEY` | sync-receiver | Random 32-byte hex for HMAC |
| `DISABLE_MFA` | auth-service | `true` in dev (bypasses TOTP for VC/DQA) |

`.env.smtp` is optional — sourced by `start_all_local.sh` for SMTP config.

## Key commands

| Command | What |
|---------|------|
| `make up` / `make down` | docker compose up/down |
| `make build` | `docker-compose build` |
| `make logs` | Tail all container logs |
| `make ps` | `docker-compose ps` |
| `make tidy` | `go mod tidy` on all Go services |
| `make install` | `pnpm install` on coordinator-pwa + admin-dashboards only |
| `make keys` | RSA-2048 key pair in `keys/` |
| `make migrate` | Re-applies only `001_init_schema.sql` |
| `make seed` | Loads `001_test_tenants` + `002_test_users` |
| `make lint` | `golangci-lint run ./...` on auth-service + api-gateway only (not session-mgr or sync-receiver) |
| `make dev-pwa` / `make dev-dashboards` | Vite dev servers |
| `make test-auth` / `make test-gateway` / `make test-pwa` | Per-service tests |
| `./scripts/start_all_local.sh` | Start auth-service (:8090) + api-gateway (:8080) + qr-generator (:3002) + sync-receiver (:8083) natively. Does **not** start session-manager or notification-service. Needs PG:5434 + Redis:6379. Logs: `/tmp/{auth-service,api-gateway,qr-generator,sync-receiver}.log` |

## Important constraints

- **Append-only ledger**: `attendance_logs` has DELETE revoked; corrections = new rows with `entry_method = MANUAL_OVERRIDE`.
- **Students identified by registration number only** (no email/phone/password). QR is their login.
- **One-device-one-person**: hardware fingerprint prevents device reuse by different students in same session.
- Gateway strips inbound `X-Tenant-ID`, `X-User-ID`, `X-Role` before setting its own values.
- QR generator docker port is `3012:3002` (external:internal).
- `make tidy` **must** be run before `go build` or `go test` on a fresh clone.

## Session state machine (XState v5)

`apps/coordinator-pwa/src/session/state-machine.ts`:
`IDLE → PENDING_LECTURER → ACTIVE → CLOSED / AUTO_CLOSED`
- T+120 min check-in window; T+180 min auto-kill
- Room code rotates every 15s (HOTP, ±1 skew = 45s window)

## Proximity model

Same-LAN (egress IP match) + live rotating room code. BLE/RSSI removed (migration 039).

## API

All endpoints proxied through `api-gateway:8443`. Full spec: `docs/openapi.yaml`.
Key groups: `POST /api/v1/auth/*`, `POST /api/v1/sync/*` (COORD), `POST /api/v1/qr/*` (DQA_DIRECTOR/ADMIN), `/api/v1/admin/tenants/{id}/*` (ADMIN). Notification service is internal-only (`POST /notify/*`).

## Coordinator Android app

`apps/coordinator-android/` — phone-as-hub native app. Crypto-core and engine modules compile as pure JVM and are verified. App module needs Android SDK (not in CI). Guide: `BUILD_AND_TEST.md`.
