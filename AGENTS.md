# QAAT — Agent guide

## Quick start

```bash
make keys          # RSA-2048 key pair (run once)
cp .env.example .env && make tidy && make install && make up
```

Default institution admin: `admin@kiu.ac.ug` / `Admin1234!` (change after first login).
The SUPER_ADMIN platform-owner role was removed in migration 064 — this is a
single-institution build and every admin is confined to their own tenant.

## Directory layout

| Path | Contents |
|------|----------|
| `backend/` | 4 Go services (auth-service, api-gateway, session-manager, sync-receiver) + 1 Node.js service (notification-service) |
| `frontend/` | 4 apps (admin-dashboards, coordinator-pwa, student-portal, coordinator-android) |
| `db/migrations/` | 67 SQL files (001–069, no 045 or 051). Auto-run on an **empty** PG volume; on an existing database use `make migrate` (see below) |
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

## Migrations

Migrations are applied by `backend/api-gateway/cmd/migrate`, which records each one in a
`schema_migrations` ledger and applies only what is outstanding, each in its own transaction.

```bash
make migrate-status                     # what is applied / pending
make migrate                            # apply everything pending (local)
./scripts/migrate-prod.sh "postgres://…@…render.com/qaat?sslmode=require"   # remote
```

**Adopt mode.** Docker only auto-runs migrations on an *empty* PG volume, and production used to be
updated by a hand-written list in `scripts/migrate-prod.sh`. Databases predating the ledger are
therefore *ragged* — some later migrations applied, some earlier ones missed, no record of which. A
plain `up` refuses such a database rather than guess. `--adopt` runs each migration
statement-by-statement and steps over only the statements that fail with a Postgres
duplicate-object SQLSTATE (plus `DROP … IF EXISTS` hitting `42809`/`2BP01`), so genuinely new
statements still land. Use it **once**; afterwards the ledger makes plain `up` correct.

Connect as the **owner** role (`ADMIN_DB_URL` / the `qaat` user), never `qaat_app` — migrations
create tables, policies and roles, which the RLS-confined data-plane role cannot do.

Tests: `go test ./internal/migrate/` (unit, no DB). Set `MIGRATE_TEST_DB_URL` to also run the
database-backed tests, which build a deliberately ragged database, adopt it, and assert the result
contains every object of a from-scratch build.

## DB connection roles

- `qaat` (superuser) — auth-service, sync-receiver, all migrations/seeds, gateway's ADMIN_DB_URL
- `qaat_app` (NOSUPERUSER NOBYPASSRLS) — api-gateway data plane, session-manager
- Gateway has **two DB URLs**: `DB_URL` (qaat_app, RLS-enforced) and `ADMIN_DB_URL` (qaat superuser, ADMIN-only cross-tenant handlers)
- All tenant queries **must** `SET LOCAL app.current_tenant = '<uuid>'` (gateway's `SetTenant` middleware does this from JWT `tenant_id` claim)

## Key env vars

| Variable | Needed by | Notes |
|----------|-----------|-------|
| `KEY_ENCRYPTION_KEY` | auth-service, sync-receiver | 32-byte hex, `openssl rand -hex 32`, no all-zeros |
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
| `make migrate` | Applies every **pending** migration, ledger-tracked. Safe to re-run |
| `make migrate-status` | Lists what is applied and what is pending |
| `make migrate-adopt` | First run against a database previously migrated by hand |
| `make seed` | Loads test tenants + users |
| `make lint` | `golangci-lint` on auth-service + api-gateway only |
| `make test-auth` / `test-gateway` / `test-pwa` | Per-service tests |
| `./scripts/start_all_local.sh` | Start auth-service (:8090) + api-gateway (:8080) + sync-receiver (:8083) natively. Needs PG:5434 + Redis:6379. Logs in `/tmp/*.log` |

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

Go integration tests skip without DB_URL.

## Reports

Every report downloads as **Excel, CSV or PDF**, built from the filters currently applied on screen:
student attendance, both lecturer-attendance records, employee attendance and the QA
lecturer-teaching rollup. `writeReport(w, r, pool, format, stem, table)` dispatches the three
formats; the exporters re-invoke the report's own handler and render its JSON
(`internal/handlers/report_exports.go`), so a download can never drift from the on-screen table and
inherits its role guards and org scoping unchanged.

## Lecturer attendance is recorded TWICE

A lecture has two independent witnesses, and they are deliberately **never merged**:

| Record | Table | Written by | Shows |
|--------|-------|-----------|-------|
| Coordinator | `lecturer_attendance_logs` | the session itself (lecturer START/END) | contact hours |
| QA patrol | `lecturer_patrol_logs` | a patroller walking the room | taught / not taught |

"Lecturer Attendance" is therefore one feature with two pages — a tab strip (`RecordTabs`) over
`CoordinatorRecord` and `PatrolLecturerAttendance`, in both the admin console and the oversight
dashboards. Where the two disagree (a session the coordinator logged but the patroller found empty)
is exactly the finding QA exists to surface, so merging them would destroy the signal.

## Android release signing

Both APKs are signed with **v1 + v2 + v3** (`enableV1Signing/V2/V3` in each module's
`signingConfigs`). Gradle defaults to v2-only for `minSdk >= 24`, which produces an APK with no
`META-INF/CERT.RSA` — and the on-device scanners that still read the v1 JAR manifest, MIUI/HyperOS's
installer above all, then reject it as **"this app may be infected by a virus"**. Do not turn v1
back off. Verify with `apksigner verify --min-sdk-version 21 -v <apk>`; all three must say true.

## Org hierarchy

Schools contain ACADEMIC departments. **SUPPORT departments — Finance, Admissions, Bursary, Library,
ICT, Estates — belong to no school** (`departments.school_id` IS NULL, migration 066) and are managed
in their own section of Schools & Departments. Do not reintroduce a synthetic "Support Services"
school: it pollutes every school list and the dean/school dashboards. A partial unique index
(`ux_departments_standalone_name`) keeps standalone names unique, since SQL treats each NULL
`school_id` as distinct and the original `UNIQUE (tenant_id, school_id, name)` cannot.

## Infrastructure

- Docker Compose: `infra/docker-compose.yml` via `docker-compose` (hyphenated)
- External ports: PG `5434:5432`, Redis `6380:6379`, QR `3012:3002`, Mailhog `8025`
- HTTPS proxy: Caddy with self-signed TLS (`infra/certs/qaat.crt`)
- K8s: `infra/k8s/` with Kustomize overlays for staging/production

## Key constraints

- **Append-only ledger**: `attendance_logs` has DELETE revoked; corrections = new rows with `entry_method = MANUAL_OVERRIDE`
- **Students identified by registration number only** (no email/phone/password). They type it to check in
- **One-device-one-person**: hardware fingerprint prevents device reuse by different students in same session
- Gateway strips inbound `X-Tenant-ID`, `X-User-ID`, `X-Role` before setting its own values
- `make tidy` **must** run before `go build` or `go test` on a fresh clone
- QR generator docker port mapping: `3012:3002` (external:internal)
- All endpoints proxied through `api-gateway:8443`. Notification service is internal-only (`POST /notify/*`)
- RLS enforced via `qaat_app` role (NOSUPERUSER NOBYPASSRLS). Every query SET LOCAL app.current_tenant
- Proximity: same-LAN (egress IP match) + live rotating room code
- **No QR anywhere.** Personal student/lecturer/coordinator QRs, the signed-QR cloud check-in and the
  `qr-generator` service were all removed (migration 063). Students type their reg-no on the in-room
  hub; lecturers sign in with their staff ID. `attendance_logs.entry_method = 'QR_SCAN'` survives as
  the ledger's "normal check-in" label only — two partial unique indexes key on it and the table is
  append-only, so it is deliberately not renamed
- **Cohort isolation in every log.** A unit shared across study sessions (Day/Evening/Weekend) is one
  `course_offerings` row per session. Session logs and attendance summaries join on `offering_id`, so
  a Weekend student never appears against a Day session. Anyone who genuinely checked in is still
  shown, flagged as a guest. This depends entirely on `sessions.offering_id` being right: both
  `OpenSession` and sync-receiver stamp it from the coordinator (who owns exactly one offering);
  migration 065 repairs rows that migration 024's arbitrary backfill got wrong
