# QAAT — Deployment Runbook (`light.md`)

Everything about how QAAT is deployed to the cloud: the topology, the exact resources
created, the one‑time database bootstrap, the secrets, the redeploy flow, the Android
build, and the fixes that were applied to make it all work.

> **Secrets are never written into this file.** Real values live in
> `infra/render/secrets.generated.env` (git‑ignored). The Render API key and Vercel
> token used during setup should be **rotated** after any handover.

---

## 1. Architecture at a glance

```
  Browsers (admin / super-admin / student portals)  ─┐
  Coordinator PWA / Android app                      ─┤  HTTPS + JWT
                                                      ▼
                              ┌───────────────────────────────────────┐
   Vercel (static SPAs)       │  api-gateway  (PUBLIC, onrender.com)   │  Render
   • admin-dashboards         │  auth · session · sync · qr · notify   │  (Docker web services)
   • coordinator-pwa          └───────────────┬───────────────────────┘
   • student-portal                           │ DB_URL (internal)
   • super-admin               ┌──────────────▼──────────────┐
                               │ Managed Postgres  +  Redis   │  Render
                               └──────────────────────────────┘
```

- **Only `api-gateway` is public.** Every client talks to it; it proxies to the internal
  services and holds the DB/Redis connections.
- **Clients never touch Postgres.** Browsers/phones call the gateway with a JWT.
- Hosting split: **frontends on Vercel**, **backend + data on Render**, **Android built locally**.

---

## 2. Live endpoints

### Frontends (Vercel — stable aliases)
| App | URL |
|-----|-----|
| Admin dashboards | https://admin-dashboards-gamma.vercel.app |
| Coordinator PWA | https://coordinator-pwa.vercel.app |
| Student portal | https://student-portal-xi-ruddy.vercel.app |
| Super-admin | https://super-admin-seven-gamma.vercel.app |

> Always use the **alias** above. Each `vercel --prod` also mints an immutable
> `…-<hash>-…vercel.app` snapshot URL that never updates — don't bookmark those.

### Backend (Render)
| Service | Kind | ID | URL |
|---------|------|----|-----|
| qaat-gateway | web (public) | `srv-d9j04furnols73802mn0` | https://qaat-gateway.onrender.com |
| qaat-auth | web | `srv-d9j03l37uimc73c8r9v0` | https://qaat-auth.onrender.com |
| qaat-session | web | `srv-d9j04bn41pts73bu1h5g` | https://qaat-session.onrender.com |
| qaat-sync | web | `srv-d9j04cernols73802g0g` | https://qaat-sync.onrender.com |
| qaat-qr | web | `srv-d9j04d7lk1mc73fc9480` | https://qaat-qr.onrender.com |
| qaat-notify | web | `srv-d9j04f4vikkc73d8i0p0` | https://qaat-notify.onrender.com |
| Postgres 15 | database | `dpg-d9ivn03tthos73c16ph0-a` | internal host `dpg-d9ivn03tthos73c16ph0-a` |
| Redis (Key Value) | keyvalue | `red-d9ivn3jtqb8s739h3gog` | `redis://red-d9ivn3jtqb8s739h3gog:6379` |

- **Render workspace:** `My Workspace` (`tea-d9ivd0beo5us73ajddfg`).
- **GitHub repo:** `LIGHT567482/QAAT` (public — Render builds Docker services from it).
- **Default super-admin login:** `superadmin@qaat.platform` / `Super1234!`

> **Free tier caveats:** services **sleep after ~15 min idle** (first request cold-starts
> ~5–50 s, then fast); the **free Postgres is deleted ~30 days after creation** — upgrade
> to a paid instance before then to keep data.

---

## 3. Frontend deployment (Vercel)

Each app is its own Vercel project (created on first deploy), built remotely by Vercel.

**The API URL is editable via a committed env file.** Each app has
`frontend/<app>/.env.production` with:
```
VITE_API_URL=https://qaat-gateway.onrender.com
```
Vite bakes it into the build; edit + redeploy to repoint an app at a different backend.

**Deploy / redeploy one app:**
```bash
cd frontend/<app>            # admin-dashboards | coordinator-pwa | student-portal | super-admin
npx --yes vercel@latest --prod --yes --token=$VERCEL_TOKEN
```

**Post-deploy hardening that was required (do it once per project):**
- **Disable Deployment Protection** (Vercel's SSO login wall is ON by default and would
  block all end users). Done via the API:
  ```bash
  curl -X PATCH -H "Authorization: Bearer $VERCEL_TOKEN" -H "Content-Type: application/json" \
    -d '{"ssoProtection": null}' "https://api.vercel.com/v9/projects/<project-name>"
  ```
- **`vercel.json`** must not contain `comment` keys inside `headers[]` (Vercel's schema
  rejects them). `coordinator-pwa` and `super-admin` are PWAs and use `Cache-Control:
  max-age=0, must-revalidate` on `sw.js` / `index.html` so new builds aren't stuck behind
  the service-worker cache. `admin-dashboards` is also a PWA (autoUpdate) — after a deploy,
  a hard reload (Ctrl/Cmd+Shift+R) may be needed once to clear the old worker.

**CORS:** the gateway's `CORS_ORIGINS` lists the four Vercel aliases above, so the browsers
are allowed to call it.

---

## 4. Backend deployment (Render)

### 4.1 What exists / how it was created
Everything was created against the Render API with a `RENDER_API_KEY`. A **Blueprint**
equivalent is committed at **`render.yaml`** (Postgres + Redis + 6 services) for a
dashboard "New → Blueprint" launch, but the live stack was created via API calls:

- Postgres: `POST /v1/postgres` (plan `free`, region `oregon`, db `qaat`, user `qaat`).
- Redis: `POST /v1/key-value` (plan `free`, `noeviction`).
- Services: `POST /v1/services` — `type: web_service`, `runtime: docker`, `rootDir:
  backend/<svc>`, `dockerfilePath: ./Dockerfile`, `autoDeploy: yes`, `healthCheckPath: /health`.

> On the **free tier there are no private services**, so all six are `web_service`s (each
> gets a public URL, gated by `INTERNAL_SVC_KEY` + JWT). The gateway reaches the others
> over their **public** `*.onrender.com` URLs (`AUTH_SERVICE_URL`, `QR_GENERATOR_URL`,
> `SESSION_MANAGER_URL`, `SYNC_RECEIVER_URL`).

### 4.2 Environment variables (per service)
| Var | Where | Value / source |
|-----|-------|----------------|
| `PORT` | all | service's port (gateway 8443, auth 8081, session 8082, sync 8083, qr 3002, notify 3004) |
| `DB_URL` | gateway, qr | `postgres://qaat_app:<APP_DB_PASSWORD>@dpg-…-a/qaat` (RLS role) |
| `DB_URL` | auth, session, sync | owner internal URL (privileged; see §5) |
| `ADMIN_DB_URL` | gateway | owner internal connection string (cross-tenant admin) |
| `REDIS_URL` | gateway, auth, session, sync | `redis://red-…:6379` |
| `KEY_ENCRYPTION_KEY` | auth, sync, qr | from `secrets.generated.env` |
| `INTERNAL_SVC_KEY` | gateway, auth | from `secrets.generated.env` (must match across services) |
| `SYNC_SIGN_KEY` | sync | from `secrets.generated.env` |
| `JWT_ISSUER` / `JWT_AUDIENCE` | all | `qaat-auth` / `qaat-api` |
| `RSA_PUBLIC_KEY_PATH` | all verifiers | `/etc/secrets/auth_public.pem` |
| `RSA_PRIVATE_KEY_PATH` | auth | `/etc/secrets/auth_private.pem` |
| `CORS_ORIGINS` | gateway | the 4 Vercel aliases |
| `WEBAUTHN_RP_ID` / `_ORIGINS`, `*_CHECKIN_BASE_URL` | gateway | `qaat-gateway.onrender.com` |
| `STUDENT_PORTAL_URL` | qr | student-portal Vercel URL (so student QRs open the portal) |
| `SMTP_*`, `VAPID_*` | qr / notify | **not set** — add real creds for QR email + web push |

### 4.3 RSA JWT keys (Render Secret Files)
`auth-service` signs JWTs with an RSA private key; every other service verifies with the
public key. The Dockerfiles do **not** bake keys, so they're provided at runtime as
**Render Secret Files** (settable via API — this was automated):
```bash
curl -X PUT -H "Authorization: Bearer $RENDER_KEY" -H "Content-Type: application/json" \
  -d "{\"name\":\"auth_public.pem\",\"content\":<PEM>}" \
  "https://api.render.com/v1/services/<serviceId>/secret-files/auth_public.pem"
```
- `qaat-auth` gets **both** `auth_public.pem` and `auth_private.pem`.
- gateway / session / sync / qr get `auth_public.pem` only. (Local keys: `keys/*.pem`.)

---

## 5. Database bootstrap (one-time) — **Render-adapted**

Render's managed Postgres gives **no superuser**, which the schema assumed. Two
adaptations are baked into `infra/render/bootstrap_db.sh`:

1. **`ALTER ROLE qaat_app NOSUPERUSER …`** is dropped (only a superuser can run it;
   `qaat_app` is already non-super/non-bypass by default).
2. **`FORCE ROW LEVEL SECURITY` → `ENABLE`** (whitespace-robust). Tenant isolation is still
   enforced via the non-owner `qaat_app` role; the **owner-based privileged services**
   (auth, sync, gateway admin handlers) rely on the owner **bypassing** RLS, which only
   works when tables are *not* force-secured.

> A subtle bug during setup: several migrations wrote `FORCE  ROW LEVEL SECURITY` (double
> space), so a single-space filter missed ~9 tables (`course_offerings`, `timetable_slots`,
> `employees`, …). Symptom: admin writes (e.g. adding a cohort) failed with
> `SQLSTATE 42501 … row-level security policy`. Fixed by `ALTER TABLE … NO FORCE ROW LEVEL
> SECURITY;` on those tables and making the bootstrap filter whitespace-robust.

**Run it once (from a machine with `psql`), against the External URL:**
```bash
# Temporarily open the DB's external allow-list (services use the INTERNAL URL and are
# unaffected), then close it again afterwards:
curl -X PATCH -H "Authorization: Bearer $RENDER_KEY" -H "Content-Type: application/json" \
  -d '{"ipAllowList":[{"cidrBlock":"0.0.0.0/0","description":"bootstrap"}]}' \
  "https://api.render.com/v1/postgres/dpg-d9ivn03tthos73c16ph0-a"

EXTERNAL_URL='postgres://qaat:<owner-pw>@dpg-…-a.oregon-postgres.render.com/qaat?sslmode=require' \
APP_DB_PASSWORD='<pick-strong>' \
./infra/render/bootstrap_db.sh          # applies all migrations + seed 004 + sets qaat_app pw

# Re-close external access:
curl -X PATCH -H "Authorization: Bearer $RENDER_KEY" -H "Content-Type: application/json" \
  -d '{"ipAllowList":[]}' "https://api.render.com/v1/postgres/dpg-d9ivn03tthos73c16ph0-a"
```

Verification performed after bootstrap: 25 tables, super-admin + platform tenant present,
`qaat_app` sees **0** users without the tenant GUC and **1** with it (isolation enforced),
and a full super-admin login returned a valid RS256 JWT end-to-end.

---

## 6. Secrets

`infra/render/secrets.generated.env` (git-ignored) holds the generated values:
`KEY_ENCRYPTION_KEY`, `INTERNAL_SVC_KEY`, `SYNC_SIGN_KEY`, `APP_DB_PASSWORD`, and VAPID keys.
RSA keypair: `keys/auth_private.pem` (git-ignored) + `keys/auth_public.pem`.

**Rotate after handover:** the `RENDER_API_KEY` and `VERCEL_TOKEN` used during setup were
pasted into a chat transcript. Regenerate them at
`dashboard.render.com/u/settings` and `vercel.com/account/tokens`.

---

## 7. Redeploy flow (how updates go live)

- **Frontend change** → `cd frontend/<app> && npx vercel --prod --yes --token=$VERCEL_TOKEN`
  (uploads the local build; does **not** require a git push).
- **Backend change** → Render builds from **GitHub `main`**, so it must be **pushed first**:
  ```bash
  git push origin main
  # then trigger a redeploy of the affected service(s):
  curl -X POST -H "Authorization: Bearer $RENDER_KEY" -H "Content-Type: application/json" \
    -d '{"clearCache":"do_not_clear"}' "https://api.render.com/v1/services/<serviceId>/deploys"
  ```
  Check status: `GET /v1/services/<serviceId>/deploys/<deployId>` → `status: live`.
- **Android change** → rebuild the APK locally (§8); no push/redeploy needed for the app
  to work (the URL/roots are baked at build time).

---

## 8. Android (coordinator) app

Native app in `frontend/coordinator-android/`. Built locally (needs the Android SDK).

**Toolchain:** `ANDROID_HOME=~/Android/Sdk`, AGP 8.5.2, Kotlin 2.0.21, Gradle 9.3.0. Build
with **JDK 21** (the system default JDK 25 is too new for AGP):
```bash
cd frontend/coordinator-android
export JAVA_HOME=/usr/lib/jvm/java-21-openjdk-amd64
export PATH=$JAVA_HOME/bin:$PATH
./gradlew :app:assembleDebug          # installable debug APK
```
**Output:** `app/build/outputs/apk/debug/app-debug.apk` → install with `adb install -r app-debug.apk`.

- **Default backend URL** is set in `app/build.gradle.kts` (`qaat.apiBase`, defaults to
  `https://qaat-gateway.onrender.com`). Override per build: `-Pqaat.apiBase=https://…`, or
  switch at runtime via the login "Change server" field.
- For a **Play Store** build you need a signing keystore: `./gradlew assembleRelease
  -Pqaat.apiBase=https://…` and sign the AAB. (Debug APK is for sideloading/pilot.)

---

## 9. Fixes applied along the way (why the repo differs from stock)

**Backend**
- `api-gateway`: passwordless lecturer dashboard login — `POST /api/v1/auth/lecturer-login`
  (institution + staff ID → read-only LECTURER token).
- `qr-generator`: student QR URL now carries `&org=<tenant domain>` so the portal can
  resolve the institution for the attendance lookup when scanned off any hotspot.
- DB: RLS `FORCE → ENABLE` adaptation for managed Postgres (see §5).

**Frontends**
- Student registration auto-fills level/year/semester/intake from the chosen **cohort**,
  and academic year from the institution's **active academic year** (all greyed/locked).
- Lecturer sign-in wired to the passwordless staff-ID flow.
- `vercel.json` fixes (invalid `comment` keys removed; PWA cache headers).

**Android**
- Default `apiBase` → live Render URL (was a LAN laptop IP).
- **TLS:** embedded **GTS Root R4 + GlobalSign Root CA** into the app trust bundle
  (`res/raw/qaat_ca.crt`) so the `onrender.com` cert validates on **older phones** whose OS
  CA store lacks the Google Trust Services roots (fixes "Trust anchor … not found").
- **Foreground service:** `START_STICKY → START_NOT_STICKY`, refuse background auto-restart,
  bail if it can't foreground — fixes the Android-14 `ForegroundServiceDidNotStartInTime`
  crash (~6 s after launch).
- **Networking:** reuse **one** `HttpClient` instead of building a new OkHttp engine per
  call (was leaking sockets/fds).
- **Crash reporting:** a global uncaught-exception handler persists the trace; the next
  launch shows a **copyable dialog** so silent closes are diagnosable.
- **Main-thread Room queries fixed:** `DashboardScreen.loadOnline` (offline fallback) and
  `ManifestClient.fetchAndStore` (login roster write) now run on `Dispatchers.IO` — this was
  the actual "closes ~6 s after login" crash.

---

## 10. How offline attendance works (verified)

1. **Online, before class:** `GET /api/v1/manifest/daily` (cohort-scoped to the coordinator)
   → roster hashes + QR serials + institution public key + policy, cached in encrypted Room.
2. **Offline, in the room:** the phone runs an embedded Ktor server on its LocalOnlyHotspot;
   the on-device `CheckinValidator` validates check-ins against the cached roster and writes
   append-only rows to `attendance_logs` (SQLCipher). Zero internet, zero DB.
3. **Online, after class:** `POST /api/v1/sync/*` uploads the sealed session package; the
   gateway/sync-receiver write it to cloud Postgres. Sessions queue as `PENDING_SYNC` until
   uploaded.

---

## 11. Quick smoke test

```bash
# Gateway health
curl https://qaat-gateway.onrender.com/health

# Super-admin login (full chain: gateway → auth → Postgres → RS256 JWT)
curl -X POST https://qaat-gateway.onrender.com/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"superadmin@qaat.platform","password":"Super1234!","tenant_id":"00000000-0000-0000-0000-000000000000"}'

# Lecturer-login endpoint present (expect 400 INVALID_REQUEST, not 404)
curl -X POST https://qaat-gateway.onrender.com/api/v1/auth/lecturer-login -d '{}'
```

Full flow: super-admin logs in → create tenant → admin adds a student → coordinator app
logs in → pulls manifest → opens session → student checks in on the hotspot → close → sync
→ row appears in cloud Postgres.
