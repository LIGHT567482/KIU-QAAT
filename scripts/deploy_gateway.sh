#!/usr/bin/env bash
# BUILD AND DEPLOY THE GATEWAY, LOUDLY.
#
# WHY THIS EXISTS. During the tenant removal a `docker compose build` failed — Docker Hub was
# briefly unreachable — and the failure went unnoticed for a whole stage. The command was:
#
#     docker compose build ... > /tmp/build.log 2>&1 && docker compose up -d ... ; SMOKE_TESTS
#
# The `&&` did its job and skipped the deploy. The `;` did not: the smoke tests ran anyway, against
# a container still running the PREVIOUS binary. They returned 401 (the token step had been skipped
# too), which read as "gateway still starting"; a re-run with hard-coded values then returned 200s
# from the old code, which read as success. The database had already moved on, so the system was
# actually broken and every signal said it was fine.
#
# The check that would have caught it immediately is not "did the image id change" — it did not,
# because the failed build left the old image in place and compose happily reused it. It is
# "does the RUNNING BINARY contain the code I just wrote".
#
# So this script:
#   1. builds, and aborts on a non-zero exit — no redirection hiding it
#   2. deploys, and aborts on a non-zero exit
#   3. waits for the gateway to actually answer, with a deadline rather than a fixed sleep
#   4. optionally asserts on the CONTENT of the deployed binary: a string that must now be
#      present, and/or one that must now be gone
#
# Usage:
#   scripts/deploy_gateway.sh
#   scripts/deploy_gateway.sh --absent "AND tenant_id = \$2" --present "FROM semester_archives"
set -uo pipefail

cd "$(dirname "$0")/.." || exit 2
COMPOSE=(docker-compose -f infra/docker-compose.yml --env-file .env)
SVC=api-gateway
CONTAINER="${QAAT_GATEWAY_CONTAINER:-infra-api-gateway-1}"
BIN=/app/api-gateway
HEALTH_URL="${QAAT_HEALTH_URL:-https://localhost:8443/api/v1/branding}"
DEADLINE=90

PRESENT=(); ABSENT=()
while [ $# -gt 0 ]; do
  case "$1" in
    --present) PRESENT+=("$2"); shift 2 ;;
    --absent)  ABSENT+=("$2");  shift 2 ;;
    *) echo "unknown argument: $1"; exit 2 ;;
  esac
done

die() { echo; echo "DEPLOY FAILED — $1"; echo "The running gateway has NOT been changed."; exit 1; }

before=$(docker inspect "$CONTAINER" --format '{{.Image}}' 2>/dev/null || echo none)

echo "[1/4] building $SVC"
# Deliberately NOT redirected to a file. A build failure has to be visible where it happens.
if ! "${COMPOSE[@]}" build "$SVC"; then
  die "the image build returned non-zero (network? syntax? see the output above)"
fi

echo "[2/4] deploying $SVC"
if ! "${COMPOSE[@]}" up -d --force-recreate "$SVC"; then
  die "compose up returned non-zero"
fi

echo "[3/4] waiting for the gateway to answer (max ${DEADLINE}s)"
waited=0
until curl -sk -o /dev/null --max-time 3 "$HEALTH_URL" 2>/dev/null; do
  sleep 2; waited=$((waited+2))
  [ $waited -ge $DEADLINE ] && die "the gateway did not answer $HEALTH_URL within ${DEADLINE}s"
done
echo "      answered after ${waited}s"

echo "[4/4] verifying the deployed binary"
after=$(docker inspect "$CONTAINER" --format '{{.Image}}' 2>/dev/null || echo none)
echo "      image ${before:7:12} -> ${after:7:12}$([ "$before" = "$after" ] && echo '  (unchanged — fine only if the source did not change)')"

fail=0
for s in ${PRESENT+"${PRESENT[@]}"}; do
  if docker exec "$CONTAINER" sh -c "strings $BIN 2>/dev/null | grep -qF -- \"$s\""; then
    echo "      PRESENT ok : ${s:0:60}"
  else
    echo "      PRESENT MISSING : ${s:0:60}"; fail=1
  fi
done
for s in ${ABSENT+"${ABSENT[@]}"}; do
  if docker exec "$CONTAINER" sh -c "strings $BIN 2>/dev/null | grep -qF -- \"$s\""; then
    echo "      ABSENT  STILL THERE : ${s:0:60}"; fail=1
  else
    echo "      ABSENT  ok : ${s:0:60}"
  fi
done
[ $fail -ne 0 ] && die "the running binary does not match the source you just built"

echo
echo "DEPLOY OK — gateway is running the code in your working tree."
