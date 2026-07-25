# Deploying QAAT to the cloud (Render) — DB, services, clients

Answers three things: **where the database goes**, **how every client connects (they never touch the
DB — only the gateway)**, and **how the coordinator app gets the roster for offline use**.

## The golden rule of this architecture
```
   Browsers (admin / super-admin / student portals)  ─┐
   Coordinator app / PWA                              ─┤  HTTPS + JWT
                                                       ▼
                                  ┌───────────────────────────────┐
                                  │  api-gateway  (public URL)     │
                                  │  auth-service · sync-receiver  │  ← only THESE hold DB connections
                                  │  qr-generator · notifications  │
                                  └───────────────┬───────────────┘
                                                  │ DB_URL (private, sslmode=require)
                                   ┌──────────────▼──────────────┐
                                   │ Managed Postgres (Render)    │   + Managed Redis (Render Key Value)
                                   └──────────────────────────────┘
```
**No client ever connects to Postgres.** Phones/browsers only call the **gateway**. This is why the DB
can live privately in the cloud and a phone on a classroom hotspot still works (it cached what it needs).

---

## 1. The database: where it is now → Render
**Now:** Postgres 15 in the `infra-postgres-1` Docker container; data in a Docker **volume** on your
machine; URLs use `sslmode=disable`. Not reachable from the internet.

**Move to Render:**
1. **Create a Render PostgreSQL** instance (pick a region; note Free tier expires). Render gives an
   **Internal** connection string (for services in the same region — use this) and an **External** one
   (for one-off `psql` from your laptop). Render Postgres **requires TLS** → URLs end with `?sslmode=require`.
2. **Create the app role + load the schema** (Render's DB has one owner role; QAAT's RLS needs a second,
   non-superuser `qaat_app` role — migration `009` creates it). From your laptop, using the **External** URL:
   ```bash
   # apply every migration in order, then the seed
   for f in db/migrations/*.sql; do psql "$EXTERNAL_URL" -f "$f"; done
   psql "$EXTERNAL_URL" -f db/seeds/*.sql           # platform tenant + super-admin
   # set a password for the RLS app role created by migration 009
   psql "$EXTERNAL_URL" -c "ALTER ROLE qaat_app WITH PASSWORD '<APP_DB_PASSWORD>';"
   ```
   (Render Postgres has no `docker-entrypoint-initdb.d`, so migrations run manually — once.)
3. **Migrating existing data** (if you have a local DB to keep): `pg_dump` local → restore to Render:
   ```bash
   docker exec infra-postgres-1 pg_dump -U qaat -d qaat --no-owner --no-privileges > qaat.sql
   psql "$EXTERNAL_URL" -f qaat.sql
   ```
4. **Create a Render Key Value (Redis)** instance → note its URL (auth jti blacklist + sync chunks need it).

---

## 2. The backend services on Render
Deploy each as a **Render Web Service from this repo** (each has a Dockerfile): `api-gateway`,
`auth-service`, `sync-receiver`, `qr-generator`, `notification-service`. Set env vars (use Render's
**private** Postgres/Redis URLs so DB traffic stays internal):

| Var | Value |
|---|---|
| `DB_URL` | `postgres://qaat_app:<APP_DB_PASSWORD>@<host>/<db>?sslmode=require` (RLS-enforced role) |
| `ADMIN_DB_URL` *(gateway only)* | `postgres://<owner>:<pw>@<host>/<db>?sslmode=require` (privileged, for cross-tenant admin) |
| `REDIS_URL` | the Render Key Value URL |
| `KEY_ENCRYPTION_KEY` | a real 64-hex secret (NOT `changeme*`) — wraps device keys |
| `INTERNAL_KEY` / `INTERNAL_SVC_KEY` | a real shared secret for service-to-service calls |
| RSA JWT keys | mount `keys/auth_private.pem` + `auth_public.pem` as Render **Secret Files** |
| SMTP / VAPID | real creds if you want QR emails + web push |

> **Security (do NOT skip — these are the H3 production blockers):** replace every `changeme_*` default,
> generate a fresh `KEY_ENCRYPTION_KEY` + VAPID keys, and keep `sslmode=require` everywhere.

**Public entry:** the **api-gateway** is the only service that must be public (Render gives it
`https://qaat-gateway.onrender.com` or your custom domain + free TLS). The others can be **private
services** the gateway reaches internally. (Render terminates HTTPS, so you don't need Caddy in the
cloud — Caddy stays for the offline laptop/hotspot deployments.)

---

## 3. The clients (dashboards + coordinator app) — point them at the gateway
They connect **only to the gateway URL**, never the DB.
- **Dashboards / student portal** (static React): the API base is `VITE_API_URL` (falls back to the
  current host). **Rebuild each with the cloud gateway URL baked in**, then host on **Render Static Sites**:
  ```bash
  VITE_API_URL=https://qaat-gateway.onrender.com pnpm --filter admin-dashboards build   # etc.
  ```
- **Coordinator Android app — ONE build serves BOTH:** the same APK works against your **local server**
  (testing/maintenance) **and** the **cloud** (world), no rebuild:
  - **TLS trusts both** — the phone's normal CA store (your cloud domain's real Let's Encrypt/CA cert)
    *and* the app-embedded self-signed cert (your local LAN server). Hostname checks stay strict for
    public domains and are relaxed only for private LAN IPs.
  - **URL is switchable at runtime** — the login screen's **"Change server"** field points it at a LAN IP
    or your domain; it's remembered. You can also set the *default* per build:
    ```bash
    ./gradlew assembleRelease -Pqaat.apiBase=https://qaat-gateway.onrender.com   # default; still overridable in-app
    ```
  Sign the release APK/AAB and upload to Play.
- **Coordinator PWA:** build with `VITE_API_URL=<gateway>`; host as a Render Static Site.

Login → the client gets a **JWT** → it's sent as `Authorization: Bearer` on every gateway call. That
JWT (not a DB connection) is how the app is "connected".

---

## 4. The offline roster — how the phone works with no DB connection
The phone never connects to the DB; it connects to the **gateway** while online, caches, then runs offline:
1. **Online (before class):** `GET /api/v1/manifest/daily` (Bearer JWT) → the **gateway** queries the
   cloud Postgres → returns the day's roster (student-id **hashes** + QR serials), the institution RSA
   public key, the attendance policy, and the units → the app stores it in **local SQLite** (`ManifestClient`).
2. **Offline (in the room):** the app's embedded server validates check-ins against that **local cache**
   on the phone's own hotspot — zero internet, zero DB.
3. **Online (after class):** the sealed session uploads via `POST /api/v1/sync/*` → the gateway/
   sync-receiver write it to the cloud Postgres (the central session is created from the package).

So the "download for offline use" is exactly step 1 — one authenticated pull through the gateway. The
DB stays private in Render; the phone only ever needed the morning's manifest + the evening's sync.

---

## Checklist
- [ ] Render Postgres created; migrations + seed applied; `qaat_app` role + password set; `sslmode=require`.
- [ ] Render Key Value (Redis) created.
- [ ] 5 services deployed (gateway public, rest private) with real secrets (no `changeme*`).
- [ ] Frontends rebuilt with `VITE_API_URL=<gateway>` and hosted as Static Sites.
- [ ] Android: ONE build works local + cloud (trusts both certs; "Change server" switches URL at runtime); signed for Play.
- [ ] Smoke test: super-admin logs in → create tenant → admin adds a student → coordinator app logs in →
      pulls manifest → opens session → check-in → close → row appears in the cloud DB.
