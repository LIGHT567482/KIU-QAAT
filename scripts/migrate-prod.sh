#!/usr/bin/env bash
# Apply every pending database migration to a remote (Render) Postgres.
#
# This script used to carry a HAND-WRITTEN list of three filenames (050, 052, 053). Every migration
# added afterwards had to be remembered and applied by hand — and several were not: 050 and 057-062
# were absent from the running database while the code that needed them shipped, so features such as
# "Schools & Departments" looked present in the app but could not load. The list is gone. The runner
# reads db/migrations itself and records what it applied in a `schema_migrations` ledger, so it is
# always complete, and re-running it only ever applies what is genuinely outstanding.
#
# Usage:
#   ./scripts/migrate-prod.sh "postgres://qaat:...@dpg-...render.com/qaat?sslmode=require"
#   EXTERNAL_URL='postgres://...' ./scripts/migrate-prod.sh
#   ./scripts/migrate-prod.sh --status "postgres://..."   # report only, change nothing
#   ./scripts/migrate-prod.sh --adopt  "postgres://..."   # FIRST run against a hand-migrated DB
#   ./scripts/migrate-prod.sh --yes    "postgres://..."   # skip the confirmation prompt
#
# The URL is the Render dashboard -> qaat-postgres -> Connect -> "External Database URL". It must be
# the OWNER role (`qaat`), not `qaat_app`: migrations create tables, policies and roles, which the
# RLS-confined data-plane role deliberately cannot do.
#
# Run this BEFORE deploying code that depends on a new migration.
set -euo pipefail
cd "$(dirname "$0")/.."

URL=""; YES="no"; MODE="up"; EXTRA=""
for a in "$@"; do
  case "$a" in
    --yes|-y)   YES="yes" ;;
    --status)   MODE="status" ;;
    --adopt)    EXTRA="--adopt" ;;
    --dry-run)  EXTRA="--dry-run" ;;
    -h|--help)  sed -n '2,21p' "$0"; exit 0 ;;
    *)          [[ -z "$URL" ]] && URL="$a" ;;
  esac
done
URL="${URL:-${EXTERNAL_URL:-${DBURL:-}}}"

if [[ -z "${URL:-}" ]]; then
  echo "ERROR: pass the Render Postgres EXTERNAL url (arg 1, or set EXTERNAL_URL/DBURL)." >&2
  echo "  Render dashboard -> qaat-postgres -> Connect -> External Database URL" >&2
  exit 1
fi
command -v go >/dev/null || { echo "ERROR: go not found — it builds the migration runner." >&2; exit 1; }
[[ -d db/migrations ]] || { echo "ERROR: db/migrations not found — run this from the repo." >&2; exit 1; }

MIGRATIONS_DIR="$PWD/db/migrations"
run() { ( cd backend/api-gateway && go run ./cmd/migrate -db "$URL" -dir "$MIGRATIONS_DIR" "$@" ); }

# Always report before touching anything.
run status

if [[ "$MODE" == "status" ]]; then exit 0; fi

echo
echo "Each pending migration is applied in its own transaction and recorded in schema_migrations."
echo "Anything already recorded is skipped, so this is safe to re-run."
echo
echo "NOTE: if 052/053 are among the pending list, they reset EVERY never-signed-in student/lecturer"
echo "      to the default password (\"Student\" / \"Lecturer\") and force a change at first login."
echo "      Already-changed passwords are left alone (last_login_at guard)."
[[ -n "$EXTRA" ]] && echo "Extra flag: $EXTRA"
if [[ "$YES" != "yes" ]]; then
  read -r -p "Proceed? [y/N] " ans
  [[ "${ans:-}" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 1; }
fi

if [[ -n "$EXTRA" ]]; then run up "$EXTRA"; else run up; fi

echo
echo "-> verifying the features that depend on the newest migrations ..."
run status | tail -4
echo
echo "Done. Deploy the code so the services match the schema."
