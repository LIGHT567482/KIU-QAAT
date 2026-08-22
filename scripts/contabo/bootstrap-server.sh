#!/usr/bin/env bash
# One-time create of /opt/qaat on the Contabo VPS (alongside /opt/upanel).
# Preserves .env.production and keys/ if you re-run later.
#
# Run ON the Contabo server as root:
#   curl -fsSL https://raw.githubusercontent.com/LIGHT567482/KIU-QAAT/main/scripts/contabo/bootstrap-server.sh | bash
#
# Or:
#   git clone https://github.com/LIGHT567482/KIU-QAAT.git /opt/qaat
#   cd /opt/qaat && bash scripts/contabo/bootstrap-server.sh

set -euo pipefail

APP_DIR="${QAAT_APP_DIR:-/opt/qaat}"
REPO="${QAAT_REPO:-https://github.com/LIGHT567482/KIU-QAAT.git}"
BRANCH="${QAAT_BRANCH:-main}"
BACKUP_DIR="/root/qaat-bootstrap-backup-$(date +%Y%m%d-%H%M%S)"

echo "==> QAAT server bootstrap"
echo "    App dir: ${APP_DIR}"
echo "    Repo:    ${REPO}"

mkdir -p "${BACKUP_DIR}"

if [[ -d "${APP_DIR}/.git" ]]; then
  echo "==> ${APP_DIR} already a git clone — pulling ${BRANCH}"
  cd "${APP_DIR}"
  git fetch origin "${BRANCH}"
  git checkout "${BRANCH}"
  git reset --hard "origin/${BRANCH}"
elif [[ -d "${APP_DIR}" ]]; then
  echo "==> Backing up existing ${APP_DIR} to ${BACKUP_DIR}/qaat-old"
  for f in .env.production .env keys; do
    if [[ -e "${APP_DIR}/${f}" ]]; then
      cp -a "${APP_DIR}/${f}" "${BACKUP_DIR}/"
      echo "    saved ${f}"
    fi
  done
  if [[ -f "${APP_DIR}/infra/docker-compose.prod.yml" ]]; then
    (cd "${APP_DIR}" && docker compose -f infra/docker-compose.prod.yml --env-file .env.production down 2>/dev/null) || true
  fi
  mv "${APP_DIR}" "${BACKUP_DIR}/qaat-old"
  echo "==> Cloning repository"
  git clone --branch "${BRANCH}" --depth 1 "${REPO}" "${APP_DIR}"
  cd "${APP_DIR}"
else
  echo "==> Cloning repository into ${APP_DIR}"
  git clone --branch "${BRANCH}" --depth 1 "${REPO}" "${APP_DIR}"
  cd "${APP_DIR}"
fi

if [[ -f "${BACKUP_DIR}/.env.production" ]]; then
  cp -a "${BACKUP_DIR}/.env.production" "${APP_DIR}/.env.production"
  echo "==> Restored .env.production from backup"
elif [[ ! -f "${APP_DIR}/.env.production" ]]; then
  echo "==> Creating .env.production with generated secrets"
  bash scripts/contabo/gen-env.sh "${APP_DIR}/.env.production"
  echo "    EDIT BEFORE DEPLOY: nano ${APP_DIR}/.env.production"
  echo "    Set UPANEL_API_TOKEN (and optional CLOUDFLARE_* / SMTP_*)."
  echo "    Then:  cd ${APP_DIR} && bash scripts/contabo/deploy-web-on-server.sh"
  echo "    Backup dir: ${BACKUP_DIR}"
  exit 0
fi

if [[ -d "${BACKUP_DIR}/keys" && ! -f "${APP_DIR}/keys/auth_private.pem" ]]; then
  mkdir -p "${APP_DIR}/keys"
  cp -a "${BACKUP_DIR}/keys/." "${APP_DIR}/keys/"
  echo "==> Restored keys/ from backup"
fi

if ! grep -q '^UPANEL_API_TOKEN=.\+' "${APP_DIR}/.env.production" 2>/dev/null; then
  echo "==> UPANEL_API_TOKEN is empty — not deploying yet"
  echo "    nano ${APP_DIR}/.env.production"
  echo "    cd ${APP_DIR} && bash scripts/contabo/deploy-web-on-server.sh"
  exit 0
fi

echo "==> Deploy QAAT stack"
bash scripts/contabo/deploy-web-on-server.sh

echo ""
echo "Bootstrap complete."
echo "  Backup of old files: ${BACKUP_DIR}"
echo "  Origin:   http://169.58.135.136:9080/health"
echo "  After DNS+Cloudflare origin port 9080: https://qaat.kiu.orion13.us/"
