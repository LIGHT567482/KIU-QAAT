# QAAT — Deployment Guide

This document covers deploying QAAT in three environments: **local development** (Docker Compose),
**staging**, and **production** (both Kubernetes via Kustomize).

> Everything in this guide is driven by files already in the repo:
> [infra/docker-compose.yml](infra/docker-compose.yml), [infra/k8s/](infra/k8s/),
> the [Makefile](Makefile), and [.env.example](.env.example).

---

## 1. Components deployed

| Component | Image / Build context | Port | Notes |
|-----------|----------------------|------|-------|
| postgres | `postgres:15-alpine` | 5432 | RLS-enforced, append-only `attendance_logs` |
| redis | `redis:7-alpine` | 6379 | JWT `jti` blacklist + sync chunk store (7-day TTL) |
| auth-service | `services/auth-service` | 8081 | RS256 JWT issuer; holds the **private** RSA key |
| api-gateway | `services/api-gateway` | 8443 | Routing, JWT middleware, RBAC, RLS tenant `SET LOCAL` |
| qr-generator | `services/qr-generator` | 3002 | RSA-2048 QR signing, email delivery |
| session-manager | `services/session-manager` | — | Warden delegation, exam clearance tokens |
| sync-receiver | `services/sync-receiver` | — | Chunked AES-256 upload, vector-clock dedup |
| notification-service | `services/notification-service` | 3004 | SMTP + Web Push |
| student-portal | `apps/student-portal` | 3003 | Passwordless reg-no progress portal |
| mailhog | `mailhog/mailhog` | 1025 / 8025 | **Dev only** — local SMTP catcher |

The three frontends — Coordinator PWA ([apps/coordinator-pwa](apps/coordinator-pwa)), Admin
Dashboards ([apps/admin-dashboards](apps/admin-dashboards)), Student Portal
([apps/student-portal](apps/student-portal)) — are static Vite builds hosted **separately on
Vercel** (see §5), not part of the backend compose/k8s stack.

---

## 2. Prerequisites

- Docker + Docker Compose v2 (`docker compose`)
- `make`, `openssl`
- Go 1.21 and pnpm 9.x (only if building/testing outside containers)
- For staging/production: `kubectl` + `kustomize` (built into recent `kubectl`) and cluster access

---

## 3. Local development (Docker Compose)

```bash
make keys                 # Generate RSA-2048 key pair into keys/ (run once)
cp .env.example .env      # Fill in real secrets — see §3.1
make tidy                 # go mod tidy on all Go services
make install              # pnpm install on frontend apps
make build                # build all service images
make up                   # docker compose up -d
make ps                   # confirm containers are healthy
```

Default ports: API Gateway `:8443`, Auth `:8081`, QR Generator `:3002`,
Notification `:3004`, Student Portal `:3003`, Mailhog UI `:8025`.
The dashboards/PWA dev servers run via `make dev-pwa` / `make dev-dashboards`.

### 3.1 Required secrets in `.env`

Replace every `changeme_*` placeholder before `make up`:

| Variable | How to generate |
|----------|-----------------|
| `DB_PASSWORD` | strong random string |
| `REDIS_PASSWORD` | strong random string |
| `KEY_ENCRYPTION_KEY` | `openssl rand -hex 32` (32-byte hex; AES-256 for tenant RSA keys at rest) |
| RSA key pair | `make keys` → mounted read-only at `/run/secrets/auth_{private,public}.pem` |

> The RSA **private** key is mounted only into `auth-service`. The api-gateway gets the
> **public** key only. Never commit `keys/` or `.env` (both are gitignored).

### 3.2 Database migrations & seeds (local)

Migrations in [db/migrations/](db/migrations/) (`001`–`013`) are mounted into the Postgres
container's `/docker-entrypoint-initdb.d` and run **in order on first boot** of an empty volume.

```bash
make migrate    # re-applies 001_init_schema.sql into a running DB (idempotent-ish; see note)
make seed       # loads the two test tenants + 5 users for RLS isolation testing
# For E2E testing, also apply the E2E seed:
PGPASSWORD=changeme_db psql -h 127.0.0.1 -p 5434 -U qaat -d qaat \
  -f db/seeds/003_e2e_test_data.sql
```

> `make migrate` only re-runs `001`. For a clean slate (re-run all five migrations), drop the
> volume: `make down && docker volume rm qaat_pgdata` then `make up`.

### 3.3 Tear down

```bash
make down                          # stop containers, keep volumes
make down && docker volume rm qaat_pgdata qaat_redisdata   # wipe data
```

### 3.4 Running services natively (without Docker)

If Postgres and Redis are already running (e.g. via Docker Compose for data layer only), start all
backend services with a single script:

```bash
./scripts/start_all_local.sh
# Starts: auth-service :8090, api-gateway :8080, qr-generator :3002, sync-receiver :8083
# Stop:   kill $(cat /tmp/qaat-pids.txt)
```

For real SMTP email delivery (QR generation):

```bash
cp .env.smtp.example .env.smtp
# Edit .env.smtp with Gmail/SendGrid credentials
source .env.smtp && ./scripts/start_qr_generator.sh
```

### 3.5 E2E test

```bash
# 1. Load E2E seed
PGPASSWORD=changeme_db psql -h 127.0.0.1 -p 5434 -U qaat -d qaat \
  -f db/seeds/003_e2e_test_data.sql
# 2. Start services (or have Docker Compose running with native api-gateway/auth-service)
./scripts/start_all_local.sh
# 3. Run tests
./tests/e2e/run_e2e_test.sh
```

Verified passing: all 8 checks (health → login → tenant → manifest → open session → 
`lecturer_attendance_logs` → close session → QR batch → sync-receiver health).

---

## 4. Staging & Production (Kubernetes + Kustomize)

Manifests live in [infra/k8s/](infra/k8s/); environment overlays in
[infra/k8s/overlays/staging/](infra/k8s/overlays/staging/) and
[infra/k8s/overlays/production/](infra/k8s/overlays/production/).

| | Namespace | Name prefix | Replicas (auth/gw/sync) | LOG_LEVEL |
|--|-----------|-------------|--------------------------|-----------|
| staging | `qaat-staging` | `staging-` | 1 / 1 / 1 | debug |
| production | `qaat` | none | 3 (api-gateway), per-manifest | info |

### 4.1 Create secrets (once per cluster)

The committed [infra/k8s/secrets.yaml](infra/k8s/secrets.yaml) is a **template** — do not apply it
with placeholder values. Create the real secrets out-of-band:

```bash
# Application secrets (db/redis/key-encryption + the *-url forms the deployments read)
kubectl create secret generic qaat-secrets -n qaat \
  --from-literal=db-password='...' \
  --from-literal=redis-password='...' \
  --from-literal=key-encryption-key="$(openssl rand -hex 32)" \
  --from-literal=db-url='postgres://qaat:...@postgres:5432/qaat?sslmode=require' \
  --from-literal=redis-url='redis://:...@redis:6379/0'

# RSA key pair (mounted into auth-service + api-gateway)
kubectl create secret generic qaat-rsa -n qaat \
  --from-file=auth_private.pem=keys/auth_private.pem \
  --from-file=auth_public.pem=keys/auth_public.pem
```

> `api-gateway` reads `db-url` and `redis-url` keys from `qaat-secrets`
> (see [infra/k8s/api-gateway.yaml](infra/k8s/api-gateway.yaml)). Make sure those keys exist in
> addition to the password-only keys in the template. For real clusters prefer **Sealed Secrets**
> or **External Secrets Operator** over imperative `kubectl create secret`.
>
> For staging, create the same secrets in the `qaat-staging` namespace.

### 4.2 Build & push images

The k8s deployments reference `qaat/<service>:latest`. Build, tag, and push to your registry,
then either keep the `latest` tag or pin a digest/version via a kustomize `images:` override.

```bash
for svc in auth-service api-gateway qr-generator sync-receiver session-manager; do
  docker build -t <registry>/qaat/$svc:<tag> services/$svc
  docker push  <registry>/qaat/$svc:<tag>
done
```

### 4.3 Deploy

```bash
# Preview the rendered manifests first
kubectl kustomize infra/k8s/overlays/staging
kubectl kustomize infra/k8s/overlays/production

# Apply
kubectl apply -k infra/k8s/overlays/staging
kubectl apply -k infra/k8s/overlays/production
```

This brings up Postgres (StatefulSet — single primary; swap for managed RDS/CloudSQL in prod HA),
Redis, all backend services, the SIS pull CronJob, and the Prometheus/Grafana monitoring stack.

### 4.4 Verify rollout

```bash
kubectl -n qaat get pods
kubectl -n qaat rollout status deploy/api-gateway
kubectl -n qaat port-forward svc/api-gateway 8443:443
curl -k https://localhost:8443/health
```

`api-gateway` exposes a `LoadBalancer` Service on `:443 → 8443` with `/health` liveness and
readiness probes. Point your DNS (`admin.qaat.platform`, `student.qaat.platform`) at the LB.
`CORS_ORIGINS` is already set per-environment in the overlay config maps.

### 4.5 Migrations in k8s

The Postgres StatefulSet mounts the migrations as init scripts (first-boot only), same as local.
For an **existing** database, run migrations explicitly against the primary — e.g.
`kubectl exec` into the postgres pod (or your managed-DB bastion) and apply
`db/migrations/00X_*.sql` in order. Never auto-`DROP` in production.

---

## 5. Frontend hosting (Vercel — pilot)

The three frontends deploy to **Vercel** as **three separate projects** (there's no shared
pnpm workspace — each app has its own `package.json`). Each app ships a `vercel.json` with the
correct build command, SPA rewrite, and cache headers.

> **Plan note:** we start on Vercel's **Hobby (free)** tier. Hobby is licensed for
> *non-commercial* use — fine for an early unpaid pilot with few users, but **move to Vercel Pro
> or Firebase Hosting before QAAT becomes a paid product.** Hobby's usage limits (100 GB/mo
> bandwidth, build minutes) are not a concern at pilot scale.

| App | Vercel project root dir | Suggested domain |
|-----|-------------------------|------------------|
| Admin Dashboards | `apps/admin-dashboards` | `admin.qaat.platform` |
| Student Portal | `apps/student-portal` | `student.qaat.platform` |
| Coordinator PWA | `apps/coordinator-pwa` | `app.qaat.platform` |

### 5.1 Set up each project

Via the Vercel dashboard (recommended): **New Project → import the repo → set _Root Directory_**
to the app folder above. Vercel reads each app's `vercel.json` for build settings. Or via CLI:

```bash
npm i -g vercel
cd apps/admin-dashboards && vercel        # link/create project, then `vercel --prod` to ship
cd ../student-portal     && vercel
cd ../coordinator-pwa    && vercel
```

> Requires the repo to be a git repo pushed to GitHub/GitLab for dashboard imports
> (this repo is **not** a git repo yet — `git init` + push first, or use the CLI).

### 5.2 Coordinator PWA cache rules (important)

The PWA uses `vite-plugin-pwa` with `registerType: 'autoUpdate'`. Its
[vercel.json](apps/coordinator-pwa/vercel.json) caches hashed `/assets/*` forever but forces
`sw.js`, `registerSW.js`, `manifest.webmanifest`, and `index.html` to **revalidate every load**.
Without this the CDN can pin a stale service worker and coordinators never receive PWA
updates mid-semester. Vercel serves real static files before applying the SPA rewrite, so the
service worker and manifest are served directly (not rewritten to `index.html`).

### 5.3 Wire CORS

[api-gateway](infra/k8s/api-gateway.yaml) already allows `https://admin.qaat.platform` and
`https://student.qaat.platform`. You must **add the PWA origin** (and, until custom domains are
attached, the generated `*.vercel.app` preview/prod URLs) to `CORS_ORIGINS` — set in the k8s
overlay config maps, or `API_CORS_ORIGINS` in [.env](.env.example) for local/compose.

---

## 6. CI

[.github/workflows/ci.yml](.github/workflows/ci.yml) runs 4 jobs in parallel on push/PR to
`main`/`develop`: auth-service (build+test), api-gateway (build+test), coordinator-pwa
(typecheck+test), and db-migrations (apply + RLS verify). Each Go job runs `go mod tidy` first.
CI is build/test only — it does **not** deploy. Promote images to staging/production manually
via §4 (or wire a deploy job/GitOps separately).

---

## 7. Production checklist (pre-pilot)

- [ ] All `changeme_*` / `REPLACE_ME` values replaced; no secrets in git
- [ ] `sslmode=require` on `db-url`; Postgres not publicly exposed
- [ ] Redis password set and `appendonly` persistence on
- [ ] RSA private key present **only** in `auth-service`; public key in api-gateway
- [ ] `ENVIRONMENT=production`, `LOG_LEVEL=info`
- [ ] `CORS_ORIGINS` restricted to real admin/student hostnames
- [ ] Real SMTP/SendGrid + VAPID keys configured (Mailhog removed)
- [ ] Postgres backups + (for HA) streaming replica or managed DB
- [ ] Prometheus/Grafana reachable; `/metrics` scraped for gateway/auth/sync
- [ ] Run the 50-item [docs/PILOT_CHECKLIST.md](docs/PILOT_CHECKLIST.md)

---

## 8. Troubleshooting

| Symptom | Likely cause |
|---------|--------------|
| auth-service crashloops on boot | RSA key not mounted / wrong path (`/run/secrets/auth_private.pem`) |
| api-gateway 401 on valid token | public key mismatch, or `JWT_ISSUER`/`JWT_AUDIENCE` differ from auth-service |
| Queries return no rows across tenants | missing `SET LOCAL app.current_tenant` — check tenant middleware / JWT `tenant_id` claim |
| Migrations didn't apply | Postgres volume already initialized; init scripts run on **empty** volume only |
| `make migrate` only ran one file | by design it re-runs `001` only; recreate the volume for a full re-run |
