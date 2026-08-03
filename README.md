# KIU QAAT — Quality Assurance Attendance Tracker

QAAT is an **offline-first** university attendance system. It combines a **live rotating room code**, **same-LAN (Wi-Fi) proximity**, and **one-device-one-person** binding to eliminate proxy attendance ("sign-ins for absent students") and ghost lectures.

**Attendance is taken completely offline.** The coordinator's hub — the **KIU QAAT Android app** on a phone, or a **Linux laptop** — is the room's Wi-Fi hotspot *and* the local server + database. Students' and the lecturer's phones join that hotspot and submit over the LAN; every log is written to the hub's database the instant it is accepted. No internet is needed in the room. When the hub later has connectivity, each closed session is sealed and **atomically** synced to the central database.

> **Capacity reality — one access point ≈ one classroom.** A single hotspot holds a limited number of phones at once (**~10 on a stock Android**, ~20–40 on a laptop). So students **rotate**: each turns Wi-Fi **off** the moment they're marked present (the check-in screen says so explicitly), freeing a slot for the next. Large groups are served by this rotation over time, by several coordinators/APs in parallel, or by putting the hub's server on campus Wi-Fi. The Go server scales to thousands; the **Wi-Fi radio is the real limit**, not the software.

> **Proximity model:** proximity is proven by **being on the coordinator's hotspot LAN plus the live rotating room code** — hardware-free, and works on every phone.

## How a session actually runs

1. **Timetable** says a unit runs today, so it appears on the coordinator's dashboard in the app.
2. **Coordinator opens the session** on the phone. The phone becomes the Wi-Fi hotspot, the HTTP server (`:8080`) and the database. One open session at a time, inside the daily window. A **room code rotates every few seconds** on screen.
   - If the coordinator is absent they can pre-authorise a **standby** — a student *of their own cohort* — with a one-day code. It mints a `COORDINATOR` token scoped to that cohort, expiring at end of day.
3. **Lecturer START gate.** The lecturer scans the **gate QR** displayed on the coordinator's screen — an HMAC-signed rotating token — and passes staff ID + live room code + on-the-hotspot-LAN (+ WebAuthn fingerprint if enrolled). This is what proves a lecture was actually taught.
4. **Students check in.** Each signs in to the KIU QAAT app with their **registration number**, joins the hotspot, and submits the room code. All gates are evaluated on the hub, offline. On success the screen says *"✓ done — turn Wi-Fi OFF now"*, freeing a slot.
5. **Lecturer END gate**, requiring a student quorum, records contact hours.
6. **Close** → the session is sealed (AES-256-GCM + device-bound HMAC-SHA256 + SHA-256 checksum) into an offline outbox.
7. **Sync** — the only step that needs internet. Chunked, resumable, all-or-nothing, retried until acknowledged.

Separately, a **QA patroller** walks room to room and independently records whether the timetabled lecturer is really teaching. Those ticks are weighed against the coordinator's own log precisely because they come from an outside observer.

> **The personal-QR subsystem was retired** (migration 063). Students no longer carry a signed personal QR and there is no public captive-portal check-in; they sign in to the app and check in with the room code. The **coordinator's gate QR that the lecturer scans is a different mechanism and is still in use.**

### The check-in gates

A student is recorded `PRESENT` only when **all** hold — enforced on the coordinator's hub, offline:

1. Student is authenticated in the app, and resolves to this cohort's roster
2. Session is `ACTIVE` and inside the daily window
3. The **live rotating room code** is valid (read off the coordinator's screen)
4. The phone is **on the coordinator's hotspot LAN** (else `NOT_SAME_NETWORK`)
5. **One-device-one-person** — this handset hasn't already checked in a different student this session (else `DEVICE_ALREADY_USED`)
6. Not already present (idempotent)

## Who uses it

There is **no tier above `ADMIN`**: `SUPER_ADMIN` was removed in migration 064. The institution's own administrator creates every account.

| Stakeholder | Role | Surface | Sign-in |
|---|---|---|---|
| IT administrator | `ADMIN` | Admin dashboard | Email + password |
| Vice-Chancellor | `VC` | VC dashboard | Email + password **+ TOTP** |
| Deputy Vice-Chancellor | `DVC` | VC dashboard | Email + password |
| Director of Quality Assurance | `DQA_DIRECTOR` | DQA dashboard | Email + password **+ TOTP** |
| QA Officer | `QA_OFFICER` | QA dashboard | Email + password |
| QA School Handler | `QA_SCHOOL_HANDLER` | QA school dashboard | Email + password |
| QA Department Rep | `QA_DEPT_REP` | QA department dashboard | Email + password |
| QA Patroller | `QA_PATROLLER` | KIU QAAT (Android) | Staff ID / email + password **+ patrol PIN** |
| Dean | `DEAN` | Dean dashboard | Email + password |
| Head of Department | `HOD` | HOD dashboard | Email + password |
| Course Coordinator | `COORDINATOR` | KIU QAAT (Android) | Password or coordinator code |
| Lecturer | `LECTURER` | Lecturer dashboard / gate scan | Passwordless (staff ID) or password |
| Student | `STUDENT` | Student portal / KIU QAAT | Reg-no (portal) or password (app) |
| Employee (non-teaching) | *no account* | Biometric tablet | Fingerprint / staff ID |

MFA is mandatory for `VC` and `DQA_DIRECTOR` only, bypassable in development with `DISABLE_MFA=true`.

**One APK serves four roles** — `COORDINATOR`, `LECTURER`, `STUDENT`, `QA_PATROLLER`. Every other role signs in successfully but is told their work is on the web dashboard, rather than falling through to the coordinator's in-room hub.

See [stakeholders.md](stakeholders.md) for the full account of every role.

## Features

- **Proxy-resistant check-in** — live rotating room code, same-LAN proximity, hardware-fingerprint device binding, one-device-one-person per session.
- **Fully-offline edge server** — the coordinator's phone or laptop runs the stack and is the room hotspot; sessions run with **zero network dependency**; sealed results sync afterwards.
- **Anti-ghost-lecture audit trail** — `lecturer_attendance_logs` records gate-open/close + contact hours per session, cross-checked by independent QA patrol ticks.
- **Patroller integrity controls** — one patroller to one handset (`patroller_device_bindings`), a server-verified **patrol PIN**, and no silent re-login.
- **Passwordless student progress portal** — a student types their reg-no and sees their own attendance % and exam eligibility. No account, no login.
- **Multi-tenancy with hard isolation** — PostgreSQL Row-Level Security on every table, enforced per request from the JWT tenant claim.
- **Append-only attendance ledger** — attendance records cannot be deleted; corrections are new rows with a `MANUAL_OVERRIDE` entry method.
- **Resilient, atomic sync** — chunked, resumable, all-or-nothing upload with integrity verification.
- **Custom in-house authentication** — RS256 JWTs, bcrypt hashing, TOTP MFA, Redis-backed token blacklist.
- **Org hierarchy** — schools and departments are **chosen from the org tree, never typed**: choosing a department fills in and locks its school, and a support department (`school_id IS NULL`) clears it.
- **Lecturers belong to the units they teach** — there is no department column on a lecturer. Departments and schools are derived by joining through their unit assignments, so a lecturer teaching across two colleges appears under each.
- **Curriculum model** — a **course** is level-independent; **levels** (Certificate/Diploma/Degree/Masters…) are added inside it, each with its own year × semester unit roadmap.
- **Global cohorts** — a cohort (session · year · semester · level · intake) can be created once and applied across **all** courses at once.
- **Active semester control + rollover** — a password-confirmed "advance semester" promotes every student and cohort one step (final level/year → GRADUATED), whole-institution or per-intake.
- **Employee attendance** — non-teaching staff check in on a biometric tablet; no-shows are reported to QA each morning.

## Repository layout

```
backend/
  auth-service/         Go — RS256 JWT, bcrypt, TOTP MFA, Redis jti blacklist
  api-gateway/          Go — routing, JWT middleware, RBAC, tenant RLS, rate limiter, Prometheus
  session-manager/      Go — warden delegation, exam clearance tokens
  sync-receiver/        Go — chunked AES-256 sealed-package upload, integrity verify
  notification-service/ Node.js — SMTP + Web Push notifications

frontend/
  admin-dashboards/     React + TypeScript + Vite (ADMIN / VC / DVC / DQA / QA / DEAN / HOD / lecturer)
  student-portal/       React + Vite (passwordless reg-no progress portal)
  coordinator-android/  Native Android (Kotlin) — the KIU QAAT app: hotspot + LAN server + DB hub

apps/
  coordinator-android/  Shared check-in/session engine module

db/
  migrations/           SQL migrations, applied in order by a ledger-tracked runner
  seeds/                Test tenants + users for RLS isolation testing

infra/
  docker-compose.yml    Full local dev stack (Postgres, Redis, all services, Caddy, Mailhog)
  render/               Render bootstrap script
  k8s/                  Kubernetes manifests
  Caddyfile             HTTPS reverse proxy for local dev

tests/
  security/             Go RLS isolation tests (all tables, append-only guard)
  load/                 k6 load scripts
  e2e/                  Playwright E2E tests
```

> The former coordinator **PWA** and the standalone **QR generator** service have been removed; the Android app supersedes the PWA, and QR signing went with the retired QR subsystem.

## Getting started

Requirements: Docker + Docker Compose, Go, Node.js, and [pnpm](https://pnpm.io).

```bash
make keys          # Generate the RSA-2048 key pair (run once)
cp .env.example .env
make tidy          # go mod tidy across all Go services
make install       # pnpm install across all frontend apps
make up            # docker compose up (full stack)
make migrate       # apply every pending DB migration (safe to re-run)
```

`make help` lists every target. Use `make migrate-adopt` for the **first** run against a database that was migrated by hand.

### Default ports

| Service              | Port  |
|----------------------|-------|
| API Gateway (Caddy)  | 8443  |
| Auth service         | 8081  |
| Admin dashboards     | 3001  |
| Student portal       | 3003  |
| Notification service | 3004  |
| Postgres             | 5434  |
| Redis                | 6380  |
| Mailhog UI           | 8025  |

> Frontends are served behind Caddy (HTTPS). The student portal is opened per-institution as `https://<host>:3003/?org=<institution-domain>`.

> **Package manager:** all frontend work uses **pnpm** — not npm or yarn.

## Running tests

```bash
# Go unit tests (integration tests skip without DB_URL)
cd backend/auth-service && go test ./...

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

Or via make: `make test-auth`, `make test-gateway`, `make test-dashboards`, `make lint`.

## Key design decisions

### PostgreSQL Row-Level Security
Every query **must** `SET LOCAL app.current_tenant = '<uuid>'` before touching data. The `SetTenant` middleware in [backend/api-gateway/internal/middleware/tenant.go](backend/api-gateway/internal/middleware/tenant.go) does this automatically from the JWT `tenant_id` claim. This is never bypassed. Clients never talk to Postgres — all traffic goes through the api-gateway.

### Append-only attendance logs
`attendance_logs` has a RESTRICTIVE RLS policy that blocks DELETE at the database level ([db/migrations/003_rls_policies.sql](db/migrations/003_rls_policies.sql)). Corrections create new rows with `entry_method = MANUAL_OVERRIDE`, and the `qaat_app` DB role has DELETE revoked.

### Two database roles
`qaat_app` is the RLS-confined data plane. The privileged owner role is used only by auth-service, sync-receiver and the gateway's cross-tenant `ADMIN_DB_URL` handlers. Migrations run as the owner.

### Session state machine
`IDLE → PENDING_LECTURER → ACTIVE → CLOSED / AUTO_CLOSED`. Timers enforce the check-in window and auto-kill.

### The patroller's second factor
A patrol tick accuses a named lecturer, so a shared password is not enough. The PIN (4–8 digits) is verified **server-side** — a secret a stolen handset could check for itself is a delay, not a factor — so a round cannot be opened offline. Five wrong attempts trigger a 15-minute lockout. The handset binding proves *which phone*; the PIN proves *who is holding it*. An admin can clear a PIN but can never set or read one.

### Sign-out is a handover
One handset is passed between coordinators and lent to students, so sign-out **refuses while a session is open** and warns when sealed sessions have not reached the server. Signing out drops the device-binding key, which is the only thing able to seal a closed session — so signing out early would strand the room's check-ins on the phone forever.

### Chunked sync protocol
`POST /sync/init` → `POST /sync/chunk/:id/:idx` → `GET /sync/resume/:id` (on reconnect) → `POST /sync/complete/:id`. Chunks are staged in Redis. Complete validates the SHA-256 checksum, writes `attendance_logs`, and refreshes the attendance summary materialized view.

### JWT security
RS256, not HS256. The private key never leaves `auth-service`; the public key is shared with the other services via a mounted file. Every JWT carries a `jti` stored in Redis on issuance; logout and refresh blacklist it.

## Deployment

- **Backend + database → Render.** [render.yaml](render.yaml) is a Blueprint provisioning Postgres, Redis and all five services. Three things Render requires out-of-band are documented at the top of that file: uploading the RSA keys as Secret Files, running [infra/render/bootstrap_db.sh](infra/render/bootstrap_db.sh) and pasting the resulting `qaat_app` `DB_URL`, and pasting the `sync: false` secrets.
- **Frontends → Vercel.** Two projects, root dirs [frontend/admin-dashboards](frontend/admin-dashboards) and [frontend/student-portal](frontend/student-portal). Each has a `vercel.json` with the SPA rewrite already in place; set `VITE_API_URL` to the deployed gateway URL.
- After both halves exist, set `CORS_ORIGINS` on the gateway to the two Vercel URLs.

Full detail in [DEPLOY.md](DEPLOY.md) and [docs/CLOUD_DEPLOY_RENDER.md](docs/CLOUD_DEPLOY_RENDER.md). For running the offline hub on any laptop, see [RUN-ANYWHERE.md](RUN-ANYWHERE.md).

## Continuous integration

[.github/workflows/ci.yml](.github/workflows/ci.yml) runs four jobs in parallel: auth-service (build + test), api-gateway (build + test), admin-dashboards (typecheck + test), and db-migrations (apply all migrations, then verify RLS is ENABLED **and** FORCED on every tenant table).

## Adding a new API endpoint

1. Add a handler in `backend/api-gateway/internal/handlers/`.
2. Wire it in `backend/api-gateway/internal/router/router.go` with the appropriate role guard.
3. Add the RLS `SET LOCAL` call via the `GetDB(ctx)` helper, or acquire a connection directly.
4. Document it in [docs/openapi.yaml](docs/openapi.yaml).
5. Add an E2E test in `tests/e2e/specs/`.

## Roadmap

Scaffolded or stubbed, not yet complete:

- **Reporting engine** — dashboard overviews compute inline in handlers; a dedicated reporting service would improve performance at scale.
- **SIS automated pull** — `POST /api/v1/import/trigger` is a stub; the OAuth 2.0 client to each institution's SIS REST API needs building per client.
- **Coordinator web landing** — web login still redirects `COORDINATOR` to `/coordinator`, a route that no longer exists in `admin-dashboards`; coordinators should use the Android app.

## Documentation

- [stakeholders.md](stakeholders.md) — **every role, what they can do, and how they sign in**.
- [flow.md](flow.md) — **whole-system flowchart** (+ pre-rendered [flow-1-large.png](flow-1-large.png) / [flow-2.png](flow-2.png)).
- [docs/FLOWCHART.md](docs/FLOWCHART.md) — the offline attendance gate, step by step.
- [docs/SYSTEM_TEST_GUIDE.md](docs/SYSTEM_TEST_GUIDE.md) — test the whole system with real students & devices.
- [docs/DEVICE_TESTING.md](docs/DEVICE_TESTING.md) — on-device testing notes.
- [ARCHITECT.md](ARCHITECT.md) — full system architecture.
- [DEPLOY.md](DEPLOY.md) / [RUN-ANYWHERE.md](RUN-ANYWHERE.md) — deployment + run-on-any-laptop guide.
- [docs/CLOUD_DEPLOY_RENDER.md](docs/CLOUD_DEPLOY_RENDER.md) — moving the DB + services to Render.
- [USER_GUIDE.md](USER_GUIDE.md) — end-user guide.
- [docs/API.md](docs/API.md) — API overview.
- [docs/NOTIFICATIONS_BACKEND.md](docs/NOTIFICATIONS_BACKEND.md) — notification delivery.
- [docs/SECURITY_PRIVACY_REVIEW.md](docs/SECURITY_PRIVACY_REVIEW.md) — security & privacy review.
