#!/bin/sh
# Apply pending migrations as the owner role, then align qaat_app's password
# with APP_DB_PASSWORD (migration 003 creates that role with a placeholder).
set -eu

DB_URL="${ADMIN_DB_URL:-${DB_URL:?DB_URL unset}}"

echo "qaat-migrate: applying pending migrations"
/migrate -db "$DB_URL" -dir /migrations up

if [ -n "${APP_DB_PASSWORD:-}" ]; then
  echo "qaat-migrate: setting qaat_app password"
  export PGPASSWORD="${DB_PASSWORD:?DB_PASSWORD unset}"
  psql -h postgres -U "${DB_USER:-qaat}" -d "${DB_NAME:-qaat}" -v ON_ERROR_STOP=1 \
    -c "ALTER ROLE qaat_app WITH LOGIN PASSWORD '${APP_DB_PASSWORD}';"
fi

if [ "${SEED_KIU:-true}" = "true" ]; then
  echo "qaat-migrate: seeding KIU institution (idempotent)"
  export PGPASSWORD="${DB_PASSWORD:?DB_PASSWORD unset}"
  psql -h postgres -U "${DB_USER:-qaat}" -d "${DB_NAME:-qaat}" -v ON_ERROR_STOP=1 \
    -f /seeds/010_kiu_institution.sql
fi

echo "qaat-migrate: done"
