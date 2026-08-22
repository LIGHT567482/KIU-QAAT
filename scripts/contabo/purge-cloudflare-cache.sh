#!/usr/bin/env bash
# Purge Cloudflare edge cache for the QAAT dashboard / portal shells after deploy.
#
# Without this, Cloudflare can keep serving an old index.html for hours
# even after deploy-web-on-server.sh updates the origin.
#
# Usage (on server or locally):
#   CLOUDFLARE_API_TOKEN=... CLOUDFLARE_ZONE_ID=... bash scripts/contabo/purge-cloudflare-cache.sh
#
# Or add to /opt/qaat/.env.production:
#   CLOUDFLARE_API_TOKEN=...
#   CLOUDFLARE_ZONE_ID=...

set -euo pipefail

DASH_ORIGIN="${QAAT_WEB_ORIGIN:-https://qaat.kiu.orion13.us}"
PORTAL_ORIGIN="${QAAT_PORTAL_ORIGIN:-https://students.qaat.kiu.orion13.us}"
TOKEN="${CLOUDFLARE_API_TOKEN:-}"
ZONE="${CLOUDFLARE_ZONE_ID:-}"

if [[ -f .env.production ]]; then
  # shellcheck disable=SC1091
  set -a
  source .env.production
  set +a
  TOKEN="${CLOUDFLARE_API_TOKEN:-$TOKEN}"
  ZONE="${CLOUDFLARE_ZONE_ID:-$ZONE}"
  DASH_ORIGIN="${QAAT_WEB_ORIGIN:-https://${QAAT_HOST:-qaat.kiu.orion13.us}}"
  PORTAL_ORIGIN="${QAAT_PORTAL_ORIGIN:-https://${QAAT_PORTAL_HOST:-students.qaat.kiu.orion13.us}}"
fi

if [[ -z "$TOKEN" || -z "$ZONE" ]]; then
  echo "WARN: CLOUDFLARE_API_TOKEN and CLOUDFLARE_ZONE_ID not set — skipping purge." >&2
  echo "      Purge manually: Cloudflare → Caching → Purge by URL:" >&2
  echo "        ${DASH_ORIGIN}/" >&2
  echo "        ${DASH_ORIGIN}/index.html" >&2
  echo "        ${PORTAL_ORIGIN}/" >&2
  echo "        ${PORTAL_ORIGIN}/index.html" >&2
  exit 0
fi

DASH="${DASH_ORIGIN%/}"
PORTAL="${PORTAL_ORIGIN%/}"
FILES=(
  "${DASH}/"
  "${DASH}/index.html"
  "${PORTAL}/"
  "${PORTAL}/index.html"
)

JSON_FILES=$(printf '"%s",' "${FILES[@]}")
JSON_FILES="[${JSON_FILES%,}]"

echo "==> Purging Cloudflare cache for ${#FILES[@]} QAAT shell URLs"
RESP=$(curl -sf -X POST \
  "https://api.cloudflare.com/client/v4/zones/${ZONE}/purge_cache" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  --data "{\"files\":${JSON_FILES}}")

if echo "$RESP" | grep -q '"success":true'; then
  echo "    Cloudflare purge OK"
else
  echo "ERROR: Cloudflare purge failed: $RESP" >&2
  exit 1
fi
