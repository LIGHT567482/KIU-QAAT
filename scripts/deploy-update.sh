#!/usr/bin/env bash
# One-shot deploy of the KIU single-institution update:
#   • Frontends (Vercel): green branding, KIU logo, no Institution-ID field, no lecturer link.
#   • Backend (Render): api-gateway rebuilt so /branding serves the new brand.json (green footer).
#   • Removes the super-admin app from Vercel.
#   • Runs the prod DB cleanup (default admin + delete super-admin) on Render Postgres.
#
# Credentials (export before running — do NOT commit them; rotate after use):
#   export VERCEL_TOKEN=...        # vercel.com/account/tokens
#   export RENDER_API_KEY=...      # dashboard.render.com/u/settings
#   export PROD_DB_URL='postgres://qaat:<owner-pw>@dpg-…-a.oregon-postgres.render.com/qaat?sslmode=require'
#
# Usage:  ./scripts/deploy-update.sh          (all steps)
#         ./scripts/deploy-update.sh frontends | super-admin | render | db   (one step)
set -euo pipefail
cd "$(dirname "$0")/.."

GW_SRV="srv-d9j04furnols73802mn0"   # qaat-gateway (from light.md)

need() { [ -n "${!1:-}" ] || { echo "!! missing env: $1"; exit 1; }; }

deploy_frontends() {
  need VERCEL_TOKEN
  bash scripts/sync-brand.sh
  for app in admin-dashboards student-portal; do
    echo "── deploying $app ──"
    ( cd "frontend/$app" && npm ci && npm run build \
        && npx --yes vercel@latest deploy --prod --yes --token="$VERCEL_TOKEN" )
  done
}

remove_super_admin_vercel() {
  need VERCEL_TOKEN
  echo "── removing super-admin Vercel project ──"
  # Project name per light.md alias super-admin-seven-gamma.vercel.app
  npx --yes vercel@latest remove super-admin --yes --token="$VERCEL_TOKEN" \
    || echo "(super-admin project already gone or named differently — check vercel projects ls)"
}

redeploy_render_gateway() {
  need RENDER_API_KEY
  echo "── pushing backend to main + redeploying qaat-gateway ──"
  echo "   (Render builds from GitHub main — make sure the brand.json/handler changes are on main)"
  curl -fsS -X POST -H "Authorization: Bearer $RENDER_API_KEY" -H "Content-Type: application/json" \
    -d '{"clearCache":"clear"}' "https://api.render.com/v1/services/${GW_SRV}/deploys" \
    | python3 -c "import sys,json;d=json.load(sys.stdin);print('deploy:', d.get('id'), d.get('status'))"
}

run_db_cleanup() {
  need PROD_DB_URL
  echo "── prod DB: default admin + delete super-admin ──"
  psql "$PROD_DB_URL" -f scripts/prod-kiu-cleanup.sql
}

case "${1:-all}" in
  frontends)    deploy_frontends ;;
  super-admin)  remove_super_admin_vercel ;;
  render)       redeploy_render_gateway ;;
  db)           run_db_cleanup ;;
  all)          deploy_frontends; remove_super_admin_vercel; redeploy_render_gateway; run_db_cleanup ;;
  *) echo "usage: $0 [frontends|super-admin|render|db|all]"; exit 1 ;;
esac
echo "✓ done: ${1:-all}"
