#!/usr/bin/env bash
# One-shot production migration for the single-institution + unified-app changes.
# Applies db/migrations/050, 052, 053 to the Render Postgres, IN ORDER. All three are
# idempotent (IF NOT EXISTS / last_login_at guards), so it is safe to re-run.
#
# IMPORTANT: run this BEFORE you deploy the new code (push main). The new auth-service
# reads users.force_password_change (added by 053) and register-device needs the table
# from 050 — deploying the code first would break logins until these run.
#
# Usage:
#   ./scripts/migrate-prod.sh "postgres://qaat:...@dpg-...-a.oregon-postgres.render.com/qaat?sslmode=require"
#   EXTERNAL_URL='postgres://...' ./scripts/migrate-prod.sh          # or via env
#   ./scripts/migrate-prod.sh --yes "postgres://..."                 # skip the prompt
#
# The URL is the Render dashboard -> qaat-postgres -> Connect -> "External Database URL".
set -euo pipefail
cd "$(dirname "$0")/.."

# ── parse args: first non-flag is the URL; --yes/-y skips the confirmation ──
URL=""; YES="no"
for a in "$@"; do
  case "$a" in
    --yes|-y) YES="yes" ;;
    *) [[ -z "$URL" ]] && URL="$a" ;;
  esac
done
URL="${URL:-${EXTERNAL_URL:-${DBURL:-}}}"

if [[ -z "${URL:-}" ]]; then
  echo "ERROR: pass the Render Postgres EXTERNAL url (arg 1, or set EXTERNAL_URL/DBURL)." >&2
  echo "  Render dashboard -> qaat-postgres -> Connect -> External Database URL" >&2
  exit 1
fi
command -v psql >/dev/null || { echo "ERROR: psql not found — install the postgresql-client package." >&2; exit 1; }

MIGRATIONS=(
  db/migrations/050_student_device_bindings.sql
  db/migrations/052_seed_default_passwords.sql
  db/migrations/053_force_password_change.sql
)
for f in "${MIGRATIONS[@]}"; do
  [[ -f "$f" ]] || { echo "ERROR: missing $f — run this from the repo root." >&2; exit 1; }
done

echo "-> testing connection ..."
psql "$URL" -v ON_ERROR_STOP=1 -tAc 'SELECT 1' >/dev/null
echo "   connected."

echo
echo "About to apply, IN ORDER:"
for f in "${MIGRATIONS[@]}"; do echo "   - $f"; done
echo
echo "WARNING: 052/053 reset EVERY never-signed-in student/lecturer to the default password"
echo "         (\"Student\" / \"Lecturer\") and force a change on first login. Already-changed"
echo "         passwords are left untouched (last_login_at guard). Safe to re-run."
if [[ "$YES" != "yes" ]]; then
  read -r -p "Proceed? [y/N] " ans
  [[ "${ans:-}" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 1; }
fi

for f in "${MIGRATIONS[@]}"; do
  echo "-> applying $f"
  psql "$URL" -v ON_ERROR_STOP=1 -q -f "$f"
done

echo
echo "-> verifying schema ..."
psql "$URL" -v ON_ERROR_STOP=1 -tAc \
  "SELECT 'student_device_bindings table: ' || (to_regclass('public.student_device_bindings') IS NOT NULL)
   || '  | users.force_password_change: ' ||
   EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='users' AND column_name='force_password_change')
   || '  | attend_block_until: ' ||
   EXISTS(SELECT 1 FROM information_schema.columns WHERE table_name='student_device_bindings' AND column_name='attend_block_until')"

echo
echo "Done. Now deploy the code so the services match the schema:"
echo "   git checkout main && git merge --ff-only fix/manifest-crash-timetable-password-hotspot && git push origin main"
