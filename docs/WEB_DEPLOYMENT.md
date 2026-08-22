# Deploy QAAT web app

| What | URL |
|------|-----|
| **Dashboards** | https://qaat.orion13.us/ |
| **Student portal** | https://students.orion13.us/ |
| **API** | https://qaat.orion13.us/api/ |
| **U-Panel (same VPS)** | https://kiu.orion13.us/ |

## SSL (read this if the browser says ERR_SSL_VERSION_OR_CIPHER_MISMATCH)

This VPS has **SSH on port 443** and **HTTP on port 80**. QAAT itself speaks
plain HTTP on **9080**. HTTPS exists only at Cloudflare, same as U-Panel.

Two things will produce `ERR_SSL_VERSION_OR_CIPHER_MISMATCH`:

1. **DNS-only (grey cloud)** — the browser talks TLS to `:443`, which is SSH, not nginx.
2. **Hostname `qaat.kiu.orion13.us`** — Cloudflare Universal SSL covers `*.orion13.us`
   (`kiu.orion13.us`, `qaat.orion13.us`) but **not** `*.kiu.orion13.us`. There is no
   certificate for that name.

Use **`https://qaat.orion13.us`** (one label under the zone). In Cloudflare:

| Setting | Value |
|---------|--------|
| A record `qaat` | `169.58.135.136`, **Proxied** (orange cloud) |
| A record `students` | `169.58.135.136`, **Proxied** |
| SSL/TLS | **Flexible** (Cloudflare HTTPS → origin HTTP) |
| Origin Rule | hostname `qaat.orion13.us` or `students.orion13.us` → destination port **9080** |

Origin without Cloudflare (from the VPS): `curl -sf http://127.0.0.1:9080/health`

Do not open `https://169.58.135.136` or `https://qaat.orion13.us:443` against the
server itself — that is SSH.

```
QAAT is a **separate Compose project** at `/opt/qaat`. It does not share U-Panel's
Postgres or bind :80 / :443 (U-Panel nginx owns 80; SSH owns 443). Public HTTP
is **qaat-proxy** on host port **9080**.

## How production updates (two steps)

Merging to `main` only **lands the code in git**. It does **not** update
https://qaat.orion13.us by itself.

The dashboards are a Vite build **baked into the `admin-dashboards` Docker image**
at image-build time (same idea as U-Panel baking `main.dart.js` into nginx).

| Step | What happens |
|------|----------------|
| 1. **GitHub** | Push / merge to `main`. CI may run; nothing on the VPS changes yet. |
| 2. **Contabo server** | Run `bash scripts/contabo/deploy-web-on-server.sh` on the VPS. It pulls `main`, rebuilds the dashboard/portal/proxy images, and restarts the QAAT stack. |

**Without step 2**, users keep seeing the old JS hashed into the previous image.

### Automatic server sync (recommended)

Add repo secret **`CONTABO_SSH_PRIVATE_KEY`** (root SSH private key). Optional:
`CONTABO_HOST` (default `169.58.135.136`), `CONTABO_SSH_PORT` (default `443`).

Workflow **`Sync web to Contabo server`** SSHs in after each push to `main`
(and on **Run workflow**).

### Manual server deploy (SSH)

```bash
ssh -p 443 -i ~/.ssh/id_ed25519 root@169.58.135.136
cd /opt/qaat && bash scripts/contabo/deploy-web-on-server.sh
```

First time on a new checkout: create `.env.production` and keys first (see
[First-time VPS](#first-time-vps) below).

Verify the origin is up:

```bash
curl -sf http://127.0.0.1:9080/health
curl -sI http://127.0.0.1:9080/ | head
curl -sI https://qaat.orion13.us/ | grep -iE 'HTTP/|content-length'
```

Hard-refresh the browser (Ctrl+Shift+R) after deploy.

### Cloudflare cache (important)

Cloudflare can cache `/` and hashed assets at the edge for **up to 4 hours** even
after the server is updated. Symptoms: deploy script succeeds but browsers still
show the old UI.

**Fix on deploy:** add to `/opt/qaat/.env.production`:

```env
CLOUDFLARE_API_TOKEN=your-api-token
CLOUDFLARE_ZONE_ID=your-zone-id
```

`deploy-web-on-server.sh` runs `purge-cloudflare-cache.sh` automatically.

**Manual purge:** Cloudflare dashboard → **Caching** → **Purge by URL**:

- `https://qaat.orion13.us/`
- `https://qaat.orion13.us/index.html`
- `https://students.orion13.us/`
- `https://students.orion13.us/index.html`

**On your device:** hard refresh (Ctrl+Shift+R) or clear site data for
`qaat.orion13.us`.

## First-time VPS

`/opt/qaat` does not exist until you clone it. U-Panel stays at `/opt/upanel`.

```bash
ssh -p 443 -i ~/.ssh/id_ed25519 root@169.58.135.136
```

**If this repo is already on GitHub `main`:**

```bash
curl -fsSL https://raw.githubusercontent.com/LIGHT567482/KIU-QAAT/main/scripts/contabo/bootstrap-server.sh | bash
```

That clones https://github.com/LIGHT567482/KIU-QAAT.git into `/opt/qaat`, writes
`.env.production`, then stops so you can paste `UPANEL_API_TOKEN`. Then:

```bash
nano /opt/qaat/.env.production
# UPANEL_API_TOKEN=thetoken   ← no space after =

cd /opt/qaat && bash scripts/contabo/deploy-web-on-server.sh
```

**Manual clone (same thing):**

```bash
git clone https://github.com/LIGHT567482/KIU-QAAT.git /opt/qaat
cd /opt/qaat
bash scripts/contabo/gen-env.sh
nano .env.production          # paste UPANEL_API_TOKEN; optional SMTP + Cloudflare purge
ufw allow 9080/tcp comment qaat-proxy
bash scripts/contabo/deploy-web-on-server.sh
```

If `scripts/contabo/deploy-web-on-server.sh` is missing, `main` on GitHub is older
than this laptop — **push this repo first**, then clone again (or `git pull` in
`/opt/qaat`).

Default admin after seed: `admin@kiu.ac.ug` / `Admin1234!` — change it immediately.

DNS (Cloudflare, same VPS IP as U-Panel, **Proxied**):

| Type | Name | Content |
|------|------|---------|
| A | `qaat` | `169.58.135.136` |
| A | `students` | `169.58.135.136` |

SSL/TLS stays **Flexible**. Add an **Origin Rule**: if hostname is
`qaat.orion13.us` **or** `students.orion13.us` → destination port
**9080**. Without that rule Cloudflare hits U-Panel on :80 and QAAT never sees
the request.

Optional: skip the origin-port rule by attaching U-Panel nginx to `qaat_internal`
and including `infra/nginx/upanel-qaat-vhost.conf` — see [docs/CONTABO.md](CONTABO.md).

## Windows (push main — optional)

There is no `website/` folder to commit. Docker builds the Vite apps **on the
server**.

```powershell
cd C:\path\to\KIU-QAAT

git add -A
git commit -m "Publish QAAT"
git push origin main
```

Then complete **step 2** on the server (or enable automatic sync above).

## Manual build (dev machine)

You do not need this for production. The server image runs `pnpm build` inside
`Dockerfile.prod`. Locally:

```powershell
cd frontend\admin-dashboards
$env:VITE_API_URL=""   # same-origin /api via qaat-proxy
pnpm install
pnpm build
```

```powershell
cd frontend\student-portal
$env:VITE_API_URL=""
pnpm install
pnpm build
```

Push to `main`, then run step 2 on Contabo.

## Backend CORS (Contabo)

In `/opt/qaat/.env.production`:

```env
API_CORS_ORIGINS=https://qaat.orion13.us,https://students.orion13.us,http://169.58.135.136:9080
```

Empty `VITE_API_URL` means the browser calls `/api` on the same host, so CORS
is unused for the dashboards. Keep the list for the Android / coordinator apps.

Restart:

```bash
cd /opt/qaat
docker compose -f infra/docker-compose.prod.yml --env-file .env.production up -d
```
