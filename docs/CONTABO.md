# QAAT on Contabo (alongside U-Panel)

Operator playbook (URLs, two-step deploy, Cloudflare, CORS): **[WEB_DEPLOYMENT.md](WEB_DEPLOYMENT.md)**.

This page is the VPS layout: own Compose project, ports, and optional U-Panel nginx vhost.
QAAT is a **separate Compose project** (`qaat`) at `/opt/qaat` with its own Postgres,
Redis, and app containers. It does not join U-Panel's compose file.

| Host | App |
|------|-----|
| `https://kiu.orion13.us` | U-Panel |
| `https://qaat.orion13.us` | QAAT dashboards + `/api` |
| `https://students.orion13.us` | Student portal + `/api` |

Public HTTP for QAAT is **qaat-proxy** on host port **9080**. Cloudflare Flexible SSL
still talks HTTPS to browsers; the origin is HTTP, same pattern as U-Panel.

## 1. On the VPS

```bash
# clone or copy this repo
git clone <this-repo> /opt/qaat
cd /opt/qaat

bash scripts/contabo/gen-env.sh
nano .env.production          # set UPANEL_API_TOKEN (U-Panel attendance API token)
bash scripts/contabo/deploy.sh
curl -sf http://127.0.0.1:9080/health
```

First login after seed: `admin@kiu.ac.ug` / `Admin1234!` — change it immediately.

`UPANEL_API_URL` defaults to `https://kiu.orion13.us` so QAAT can pull lecturer/student
attendance from the U-Panel stack on the same machine.

## 2. DNS + Cloudflare

In the `orion13.us` zone:

| Type | Name | Content | Proxy |
|------|------|---------|-------|
| A | `qaat` | `169.58.135.136` | Proxied |
| A | `students` | `169.58.135.136` | Proxied |

SSL/TLS mode stays **Flexible**. Add an **Origin Rule** (or Transform → origin port):

- If hostname is `qaat.orion13.us` **or** `students.orion13.us`
- Then destination port **9080**

Without that rule Cloudflare will hit U-Panel on :80 and QAAT never sees the request.

## 3. Optional: serve QAAT on :80 via U-Panel nginx

Avoids the origin-port rule. After QAAT is up:

```bash
docker network connect qaat_internal "$(docker ps -qf name=upanel-nginx)"
```

Append `infra/nginx/upanel-qaat-vhost.conf` to U-Panel's
`config/nginx/upanel-docker.conf`, rebuild U-Panel nginx, and reload. Cloudflare
can then keep origin **80** and route by `Host`.

## 4. Day-two commands

```bash
cd /opt/qaat
docker compose -f infra/docker-compose.prod.yml --env-file .env.production ps
docker compose -f infra/docker-compose.prod.yml --env-file .env.production logs -f api-gateway
docker compose -f infra/docker-compose.prod.yml --env-file .env.production up -d --build
```

Migrations run automatically on every `up` via the one-shot `migrate` container.
Re-running is safe. The KIU institution seed is idempotent (`SEED_KIU=true`).

## 5. What is not in this stack

- Coordinator PWA / Android — room-edge apps, not hosted here
- Mailhog, published Postgres/Redis ports
- Sharing U-Panel's Postgres (QAAT has its own `qaat` database and `qaat_app` role)

## 6. Firewall

```bash
ufw allow 9080/tcp comment qaat-proxy
```

Keep 80 (U-Panel) and 443 (SSH) as they are.
