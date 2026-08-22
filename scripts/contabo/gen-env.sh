#!/usr/bin/env bash
# Write .env.production with fresh secrets if it does not already exist.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
DEST="${1:-"$ROOT/.env.production"}"
EXAMPLE="$ROOT/.env.production.example"

if [[ -f "$DEST" ]]; then
  echo "exists: $DEST (not overwritten)"
  exit 0
fi
if [[ ! -f "$EXAMPLE" ]]; then
  echo "missing $EXAMPLE" >&2
  exit 1
fi

hex32() { openssl rand -hex 32; }

umask 077
sed \
  -e "s/^DB_PASSWORD=.*/DB_PASSWORD='$(hex32)'/" \
  -e "s/^APP_DB_PASSWORD=.*/APP_DB_PASSWORD='$(hex32)'/" \
  -e "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD='$(hex32)'/" \
  -e "s/^KEY_ENCRYPTION_KEY=.*/KEY_ENCRYPTION_KEY='$(hex32)'/" \
  -e "s/^INTERNAL_SVC_KEY=.*/INTERNAL_SVC_KEY='$(hex32)'/" \
  -e "s/^SYNC_SIGN_KEY=.*/SYNC_SIGN_KEY='$(hex32)'/" \
  -e "s/^CRON_SECRET=.*/CRON_SECRET='$(hex32)'/" \
  "$EXAMPLE" > "$DEST"

echo "wrote $DEST — set UPANEL_API_TOKEN=yourtoken with NO space after the equals sign"
