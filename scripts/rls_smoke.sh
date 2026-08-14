#!/usr/bin/env bash
# RLS VISIBILITY SMOKE TEST
#
# The bug this exists to catch does not raise an error. Dropping tenant_id CASCADE removes a
# table's tenant_isolation POLICY but leaves ROW LEVEL SECURITY switched on, and a table with RLS
# enabled and no policy denies every row to any role that is not the owner. Reads come back empty;
# writes are refused. Nothing logs anything.
#
# It is invisible to an endpoint smoke test because most admin handlers run on adminPool, which
# connects as the owner and bypasses RLS entirely. So this checks the thing directly: for every
# table, how many rows does the OWNER see, and how many does qaat_app see on a connection scoped
# exactly the way the application scopes it?
#
#   owner N, app N   → fine
#   owner N, app 0   → RLS is hiding the table from the application
#
# Usage:  scripts/rls_smoke.sh            (uses the KIU tenant automatically)
#         scripts/rls_smoke.sh <tenant>   (explicit tenant uuid)
set -uo pipefail

CONTAINER="${QAAT_PG_CONTAINER:-infra-postgres-1}"
PSQL="docker exec -i $CONTAINER psql -U qaat -d qaat -t -A"

TENANT="${1:-$($PSQL -c "SELECT tenant_id FROM tenants ORDER BY created_at LIMIT 1;" | tr -d ' ')}"
if [ -z "$TENANT" ]; then echo "could not resolve a tenant id"; exit 2; fi
echo "RLS visibility check — tenant $TENANT"
echo

TABLES=$($PSQL -c "SELECT c.relname FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
                   WHERE n.nspname='public' AND c.relkind='r' ORDER BY 1;")

fail=0
checked=0
for t in $TABLES; do
  [ -z "$t" ] && continue
  owner=$($PSQL -c "SELECT count(*) FROM \"$t\";" 2>/dev/null | tr -d ' ')
  # The application's exact posture: the confined role, with the tenant GUC set the way
  # middleware.SetTenantConn sets it.
  app=$($PSQL -c "SET ROLE qaat_app; SET app.current_tenant = '$TENANT'; SELECT count(*) FROM \"$t\";" 2>/dev/null | tail -1 | tr -d ' ')
  [ -z "$owner" ] && continue
  checked=$((checked+1))
  # Only rows that EXIST can go missing, so a table with none tells us nothing either way.
  if [ "$owner" != "0" ] && [ "$app" = "0" ]; then
    rls=$($PSQL -c "SELECT c.relrowsecurity::text || '/' || (SELECT count(*) FROM pg_policy p WHERE p.polrelid=c.oid)::text
                     FROM pg_class c WHERE c.relname='$t';" | tr -d ' ')
    printf "  HIDDEN  %-32s owner=%-6s app=%-6s (rls_on/policies = %s)\n" "$t" "$owner" "$app" "$rls"
    fail=1
  fi
done

echo
echo "checked $checked tables"
if [ $fail -eq 0 ]; then
  echo "PASS — every non-empty table is visible to the application role"
else
  echo "FAIL — the tables above return rows to the owner and nothing to qaat_app."
  echo "       If tenant_id was just dropped from one, it needs DISABLE ROW LEVEL SECURITY."
fi
exit $fail
