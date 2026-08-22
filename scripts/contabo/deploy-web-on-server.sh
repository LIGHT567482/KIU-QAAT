#!/usr/bin/env bash
# Pull latest main, rebuild dashboard/portal/proxy images (Vite is baked in at
# image build), restart the QAAT stack. Does not touch /opt/upanel.
#
# IMPORTANT: `docker compose up -d` alone does NOT update the JS the browser
# loads. admin-dashboards and student-portal bake `pnpm build` into the image —
# run THIS script after every merge (same role as U-Panel's deploy-web-on-server.sh).
#
# Run ON the Contabo server as root:
#   ssh -p 443 -i ~/.ssh/id_ed25519 root@169.58.135.136
#   cd /opt/qaat && bash scripts/contabo/deploy-web-on-server.sh

set -euo pipefail

APP_DIR="${QAAT_APP_DIR:-/opt/qaat}"
COMPOSE=(docker compose -f infra/docker-compose.prod.yml --env-file .env.production)
ORIGIN_PORT="${QAAT_PUBLISH_PORT:-9080}"

if [[ ! -d "${APP_DIR}/.git" ]]; then
  echo "ERROR: ${APP_DIR} is not a git repository." >&2
  echo "" >&2
  echo "First-time:" >&2
  echo "  git clone <this-repo> ${APP_DIR}" >&2
  echo "  cd ${APP_DIR} && bash scripts/contabo/gen-env.sh && nano .env.production" >&2
  echo "  bash scripts/contabo/deploy-web-on-server.sh" >&2
  exit 1
fi

cd "${APP_DIR}"

if [[ ! -f .env.production ]]; then
  echo "ERROR: ${APP_DIR}/.env.production is missing." >&2
  echo "  bash scripts/contabo/gen-env.sh" >&2
  echo "  nano .env.production   # set UPANEL_API_TOKEN=... with NO space after =" >&2
  exit 1
fi

# Do not `source` .env.production — a space after "=" makes bash run the value
# as a command ("6cc9…: command not found"). Compose reads it via --env-file.
# shellcheck disable=SC1091
. scripts/contabo/env.sh
ORIGIN_PORT="$(env_get QAAT_PUBLISH_PORT)"
ORIGIN_PORT="${ORIGIN_PORT:-9080}"

echo "==> Filling any empty secrets in .env.production (keeps UPANEL_API_TOKEN)"
bash scripts/contabo/gen-env.sh --ensure .env.production

if [[ ! -f keys/auth_private.pem || ! -f keys/auth_public.pem ]]; then
  echo "==> Generating RSA key pair into keys/"
  mkdir -p keys
  openssl genrsa -out keys/auth_private.pem 2048
  openssl rsa -in keys/auth_private.pem -pubout -out keys/auth_public.pem
  chmod 600 keys/auth_private.pem
fi

echo "==> Git state before deploy"
git log -1 --oneline || true

echo "==> Pull latest main"
git fetch origin main
git checkout main
if git rev-parse -q --verify MERGE_HEAD >/dev/null 2>&1; then
  echo "    Aborting in-progress merge"
  git merge --abort
fi
# .env.production and keys/ are gitignored — reset will not delete them.
git reset --hard origin/main
echo "    now at: $(git log -1 --oneline)"

echo "==> Rebuild images (Vite is compiled inside Dockerfile.prod) and restart"
"${COMPOSE[@]}" build --no-cache admin-dashboards student-portal proxy
"${COMPOSE[@]}" up -d --build --remove-orphans

echo "==> Health checks (wait for containers)"
sleep 8
if ! curl -sf "http://127.0.0.1:${ORIGIN_PORT}/health" >/dev/null; then
  echo "ERROR: /health did not return 200 on 127.0.0.1:${ORIGIN_PORT}" >&2
  echo "Check: docker compose -f infra/docker-compose.prod.yml --env-file .env.production logs api-gateway proxy" >&2
  exit 1
fi
echo " /health OK"

DASH_STATUS=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:${ORIGIN_PORT}/")
echo " / HTTP ${DASH_STATUS}"
if [[ "${DASH_STATUS}" != "200" ]]; then
  echo "ERROR: / did not return 200. Check: docker compose logs proxy admin-dashboards" >&2
  exit 1
fi

echo "==> Purge Cloudflare edge cache (prevents stale dashboard JS for up to 4h)"
bash scripts/contabo/purge-cloudflare-cache.sh || true

echo ""
echo "Deploy OK. Verify from your machine:"
echo "  curl -sf http://169.58.135.136:${ORIGIN_PORT}/health"
echo "  curl -sI https://qaat.kiu.orion13.us/ | head"
echo ""
echo "If the browser still shows the old UI:"
echo "  1. Hard refresh: Ctrl+Shift+R (or Cmd+Shift+R on Mac)"
echo "  2. Or: DevTools → Application → Clear site data for qaat.kiu.orion13.us"
echo "  3. Or: open in a private/incognito window"
echo ""
echo "Dashboards: https://qaat.kiu.orion13.us/"
echo "Portal:     https://students.qaat.kiu.orion13.us/"
echo "Origin:     http://169.58.135.136:${ORIGIN_PORT}/"
