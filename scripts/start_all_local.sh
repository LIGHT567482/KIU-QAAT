#!/usr/bin/env bash
# Start all QAAT backend services for local development / E2E testing.
# Requires: Go 1.21, Node.js 20, pnpm, Postgres on :5434, Redis on :6379.
#
# Usage: ./scripts/start_all_local.sh
# Stop:  kill $(cat /tmp/qaat-pids.txt)

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
KEK=$(grep 'KEY_ENCRYPTION_KEY' "$REPO/.env" | tail -1 | cut -d= -f2)

GREEN='\033[0;32m'; NC='\033[0m'
info() { echo -e "${GREEN}[start]${NC} $*"; }

kill_prev() {
  [ -f /tmp/qaat-pids.txt ] && kill $(cat /tmp/qaat-pids.txt) 2>/dev/null || true
  rm -f /tmp/qaat-pids.txt
}
kill_prev

PIDS=()

# ── auth-service ──────────────────────────────────────────────────────────────
info "Building auth-service..."
(cd "$REPO/services/auth-service" && go build -o auth-service . 2>&1)
DB_URL="postgres://qaat:changeme_db@127.0.0.1:5434/qaat?sslmode=disable" \
REDIS_URL="redis://:changeme_redis@127.0.0.1:6379/0" \
RSA_PRIVATE_KEY_PATH="$REPO/keys/auth_private.pem" \
RSA_PUBLIC_KEY_PATH="$REPO/keys/auth_public.pem" \
JWT_ISSUER="qaat-auth" \
JWT_AUDIENCE="qaat-api" \
KEY_ENCRYPTION_KEY="$KEK" \
PORT="8090" \
"$REPO/services/auth-service/auth-service" &>/tmp/auth-service.log &
PIDS+=($!)
info "auth-service → :8090 (PID ${PIDS[-1]})"

sleep 1

# ── api-gateway ───────────────────────────────────────────────────────────────
info "Building api-gateway..."
(cd "$REPO/services/api-gateway" && go build -o api-gateway . 2>&1)
DB_URL="postgres://qaat:changeme_db@127.0.0.1:5434/qaat?sslmode=disable" \
REDIS_URL="redis://:changeme_redis@127.0.0.1:6379/0" \
RSA_PUBLIC_KEY_PATH="$REPO/keys/auth_public.pem" \
AUTH_SERVICE_URL="http://127.0.0.1:8090" \
QR_GENERATOR_URL="http://127.0.0.1:3002" \
SYNC_RECEIVER_URL="http://127.0.0.1:8083" \
PORT="8080" \
"$REPO/services/api-gateway/api-gateway" &>/tmp/api-gateway.log &
PIDS+=($!)
info "api-gateway → :8080 (PID ${PIDS[-1]})"

# ── sync-receiver ─────────────────────────────────────────────────────────────
info "Building sync-receiver..."
(cd "$REPO/services/sync-receiver" && go build -o sync-receiver . 2>&1)
SIGN_KEY=$(openssl rand -hex 32)
DB_URL="postgres://qaat:changeme_db@127.0.0.1:5434/qaat?sslmode=disable" \
REDIS_URL="redis://:changeme_redis@127.0.0.1:6379/0" \
RSA_PUBLIC_KEY_PATH="$REPO/keys/auth_public.pem" \
JWT_ISSUER="qaat-auth" JWT_AUDIENCE="qaat-api" \
SYNC_SIGN_KEY="$SIGN_KEY" \
PORT="8083" \
"$REPO/services/sync-receiver/sync-receiver" &>/tmp/sync-receiver.log &
PIDS+=($!)
info "sync-receiver → :8083 (PID ${PIDS[-1]})"

# Save PIDs for cleanup
printf '%s\n' "${PIDS[@]}" > /tmp/qaat-pids.txt

sleep 3

echo ""
info "All services started. Health checks:"
for svc in "8090/health:auth-service" "8080/health:api-gateway" "8083/health:sync-receiver"; do
  port="${svc%%/*}"; path="${svc%%:*}"; name="${svc##*:}"
  curl -sf "http://localhost:$port/${path#*/}" > /dev/null \
    && echo -e "  ${GREEN}OK${NC}  $name (:$port)" \
    || echo -e "  FAIL $name (:$port)"
done

echo ""
echo "PIDs saved to /tmp/qaat-pids.txt"
echo "Logs: /tmp/auth-service.log  /tmp/api-gateway.log  /tmp/sync-receiver.log"
echo "Stop: kill \$(cat /tmp/qaat-pids.txt)"
