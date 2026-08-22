#!/usr/bin/env bash
# Point U-Panel nginx (host :80) at QAAT for qaat.orion13.us / students.orion13.us
# so Cloudflare Flexible does not serve U-Panel's `/` → `/download/` catch-all.
#
# Safe to re-run. The drop-in lives only in the running nginx container — call
# this after every U-Panel nginx recreate, and from deploy-web-on-server.sh.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TEMPLATE="${ROOT}/infra/nginx/upanel-qaat-vhost.conf"
ORIGIN_PORT="${QAAT_PUBLISH_PORT:-9080}"

NGINX="$(docker ps -qf name=^upanel-nginx --format '{{.Names}}' | head -n 1)"
if [[ -z "${NGINX}" ]]; then
  echo "attach-upanel-nginx: no upanel-nginx container — skip" >&2
  exit 0
fi

GW="$(docker inspect "${NGINX}" --format '{{(index .NetworkSettings.Networks "upanel_default").Gateway}}')"
if [[ -z "${GW}" || "${GW}" == "<no value>" ]]; then
  GW="$(docker inspect "${NGINX}" --format '{{range .NetworkSettings.Networks}}{{.Gateway}}{{end}}' | awk '{print $1}')"
fi
if [[ -z "${GW}" ]]; then
  echo "attach-upanel-nginx: cannot find docker gateway for ${NGINX}" >&2
  exit 1
fi

ORIGIN="${GW}:${ORIGIN_PORT}"
echo "==> Attach QAAT ${ORIGIN} on ${NGINX} for qaat.orion13.us"

if ! docker exec "${NGINX}" wget -q -O /dev/null --timeout=5 "http://${ORIGIN}/health"; then
  echo "attach-upanel-nginx: ${NGINX} cannot reach http://${ORIGIN}/health" >&2
  exit 1
fi

tmp="$(mktemp)"
sed "s|__QAAT_ORIGIN__|${ORIGIN}|g" "${TEMPLATE}" > "${tmp}"
docker cp "${tmp}" "${NGINX}:/etc/nginx/conf.d/qaat.conf"
rm -f "${tmp}"

docker exec "${NGINX}" nginx -t
docker exec "${NGINX}" nginx -s reload
echo "    reloaded ${NGINX}"
