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
  # `NO FORCE ROW LEVEL SECURITY` IS PARKED FIRST, and put back last.
  #
  # Rewriting FORCE→ENABLE with a single expression also hit the `NO FORCE` in migrations 060 and
  # 082, producing `ALTER TABLE schools NO ENABLE ROW LEVEL SECURITY` — not a weaker statement but
  # a syntax error, which stopped the whole bootstrap at migration 060 with the schema half built.
  # Parking the phrase behind a placeholder no pattern matches leaves those statements exactly as
  # written, which is what we want: they UNFORCE, and on Render nothing was forced to begin with,
  # so they are correct no-ops.
  sed -E -e 's/NO[[:space:]]+FORCE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY/@@NOFORCE@@/gI' \
         -e 's/FORCE[[:space:]]+ROW[[:space:]]+LEVEL[[:space:]]+SECURITY/ENABLE ROW LEVEL SECURITY/gI' \
         -e 's/@@NOFORCE@@/NO FORCE ROW LEVEL SECURITY/g' \
         -e '/ALTER ROLE qaat_app[[:space:]]+NOSUPERUSER/d' "$f" \
    | psql "$EXTERNAL_URL" -v ON_ERROR_STOP=1 -q
done

echo "→ seeding the institution + first administrator…"
# The migrations leave an empty database: 038 seeds a platform super-admin and 064 deletes it, and
# nothing since inserts an institution or an account. Without this the deploy comes up healthy and
# cannot be logged into. The old 004_super_admin seed this script used to run was retired along with
# the SUPER_ADMIN role — calling a file that no longer exists is what used to abort the run here,
# after the migrations and BEFORE the qaat_app password below, leaving a database no service could
# authenticate against.
psql "$EXTERNAL_URL" -v ON_ERROR_STOP=1 -q -f db/seeds/010_kiu_institution.sql

echo "→ setting qaat_app role password…"
psql "$EXTERNAL_URL" -v ON_ERROR_STOP=1 -c "ALTER ROLE qaat_app WITH LOGIN PASSWORD '${APP_DB_PASSWORD}';"

echo "✓ bootstrap complete."
echo "  Now set DB_URL on the services to:"
echo "  postgres://qaat_app:${APP_DB_PASSWORD}@<INTERNAL_HOST>/qaat?sslmode=require"
