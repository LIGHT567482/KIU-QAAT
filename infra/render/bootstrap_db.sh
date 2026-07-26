#!/usr/bin/env bash
# QAAT — one-time Render Postgres bootstrap.
# Applies all migrations + seeds, then sets the RLS app-role password so services
# can connect as `qaat_app` (created by migration 009).
#
# Run ONCE from your laptop against the Render *External* connection string:
#   EXTERNAL_URL='postgres://qaat:...@...oregon-postgres.render.com/qaat?sslmode=require' \
#   APP_DB_PASSWORD='<pick-a-strong-password>' \
#   ./infra/render/bootstrap_db.sh
#
# Afterwards, set DB_URL on every service (the `sync: false` ones in render.yaml) to:
#   postgres://qaat_app:<APP_DB_PASSWORD>@<INTERNAL_HOST>/qaat?sslmode=require
# (use the Render *Internal* host so DB traffic stays private).
set -euo pipefail

: "${EXTERNAL_URL:?set EXTERNAL_URL to the Render Postgres External connection string}"
: "${APP_DB_PASSWORD:?set APP_DB_PASSWORD to the password you want for the qaat_app role}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

# Render's managed Postgres gives NO superuser: the owner role can neither run
# `ALTER ROLE ... NOSUPERUSER` nor be subject-then-bypass FORCE'd RLS. So we
# (a) drop the superuser-only ALTER ROLE line, and (b) rewrite every
# `... FORCE ROW LEVEL SECURITY` (any whitespace) to ENABLE — tenant isolation is
# still enforced via the non-owner qaat_app role; the owner-based privileged
# services (auth, sync, gateway admin handlers) rely on the owner bypassing RLS,
# which only works when the table is NOT force-secured.
echo "→ applying $(ls db/migrations/*.sql | wc -l) migrations (Render-adapted: no FORCE RLS)…"
for f in db/migrations/*.sql; do
  echo "   $f"
  sed -E -e 's/FORCE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY/ENABLE ROW LEVEL SECURITY/gI' \
         -e '/ALTER ROLE qaat_app[[:space:]]+NOSUPERUSER/d' "$f" \
    | psql "$EXTERNAL_URL" -v ON_ERROR_STOP=1 -q
done

echo "→ applying seeds (platform tenant + super-admin)…"
# 004_super_admin is the production-critical seed; the 001-003/005 are test data.
psql "$EXTERNAL_URL" -v ON_ERROR_STOP=1 -q -f db/seeds/004_super_admin.sql

echo "→ setting qaat_app role password…"
psql "$EXTERNAL_URL" -v ON_ERROR_STOP=1 -c "ALTER ROLE qaat_app WITH LOGIN PASSWORD '${APP_DB_PASSWORD}';"

echo "✓ bootstrap complete."
echo "  Now set DB_URL on the services to:"
echo "  postgres://qaat_app:${APP_DB_PASSWORD}@<INTERNAL_HOST>/qaat?sslmode=require"
