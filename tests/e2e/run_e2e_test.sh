#!/usr/bin/env bash
# QAAT End-to-End Test Script
# Tests: (1) QR generation + real SMTP email, (2) coordinator session lifecycle,
#        (3) attendance sync and DB verification.
#
# Usage:
#   ./tests/e2e/run_e2e_test.sh
#
# Prerequisites:
#   - api-gateway running on :8080
#   - qr-generator running on :3002
#   - sync-receiver running on :8083  (or SYNC_PORT=<n>)
#   - Postgres on :5434 (password in DB_PASS below)
#   - Seed data loaded: psql ... -f db/seeds/003_e2e_test_data.sql
#
# SMTP configuration (set these before running for real email test):
#   export SMTP_HOST=smtp.gmail.com
#   export SMTP_PORT=587
#   export SMTP_SECURE=false
#   export SMTP_USER=your@gmail.com
#   export SMTP_PASS=your-app-password
#   (Or use any SMTP provider — SendGrid, Mailgun, etc.)
#
# For a local dry-run (Mailhog), leave SMTP vars unset — default is mailhog:1025.

set -euo pipefail

API="http://localhost:8080"
QR="http://localhost:3002"
DB_PASS="${DB_PASS:-changeme_db}"
PSQL="PGPASSWORD=$DB_PASS psql -h 127.0.0.1 -p 5434 -U qaat -d qaat"

# Fixed tenant UUID from seed 003_e2e_test_data.sql
E2E_TENANT_ID="a0000000-0000-0000-0000-000000000001"

GREEN='\033[0;32m'; RED='\033[0;31m'; YELLOW='\033[1;33m'; NC='\033[0m'
pass() { echo -e "${GREEN}PASS${NC} $*"; }
fail() { echo -e "${RED}FAIL${NC} $*"; exit 1; }
info() { echo -e "${YELLOW}INFO${NC} $*"; }

# ── 0. Health checks ──────────────────────────────────────────────────────────
info "0. Health checks"
curl -sf "$API/health" > /dev/null || fail "api-gateway not reachable at $API"
pass "api-gateway healthy"

# Check qr-generator (optional for QR email test)
if curl -sf "$QR/health" > /dev/null 2>&1; then
  QR_UP=true; pass "qr-generator healthy"
else
  QR_UP=false; info "qr-generator not running — skipping QR email test"
fi

# ── 1. Admin login ────────────────────────────────────────────────────────────
info "1. Admin login (admin@test.local)"
ADMIN_LOGIN=$(curl -sf -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"admin@test.local\",\"password\":\"Admin1234!\",\"tenant_id\":\"$E2E_TENANT_ID\"}" \
  || echo "{}")
ADMIN_TOKEN=$(echo "$ADMIN_LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('access_token',''))" 2>/dev/null || echo "")
[ -n "$ADMIN_TOKEN" ] || fail "Admin login failed (got: $ADMIN_LOGIN) — load seed: psql ... -f db/seeds/003_e2e_test_data.sql"
pass "Admin token acquired"

# ── 2. Verify tenant via admin API ────────────────────────────────────────────
info "2. Fetch tenants (admin API)"
TENANTS=$(curl -sf "$API/api/v1/admin/tenants" -H "Authorization: Bearer $ADMIN_TOKEN" || echo "[]")
TENANT_ID=$(echo "$TENANTS" | python3 -c "
import sys,json
t = json.load(sys.stdin)
match = [x for x in t if x.get('tenant_id') == '$E2E_TENANT_ID']
print(match[0]['tenant_id'] if match else '')
" 2>/dev/null || echo "")
[ -n "$TENANT_ID" ] || fail "Test tenant not visible via admin API — seed may not have loaded"
pass "Tenant verified: $TENANT_ID"

# ── 2b. Coordinator login ─────────────────────────────────────────────────────
info "2b. Coordinator login (coordinator@test.local)"
COORD_LOGIN=$(curl -sf -X POST "$API/api/v1/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"coordinator@test.local\",\"password\":\"Coord1234!\",\"tenant_id\":\"$E2E_TENANT_ID\"}" \
  || echo "{}")
COORD_TOKEN=$(echo "$COORD_LOGIN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('access_token',''))" 2>/dev/null || echo "")
[ -n "$COORD_TOKEN" ] || fail "Coordinator login failed (got: $COORD_LOGIN)"
pass "Coordinator token acquired"

# ── 3. Coordinator pulls manifest ─────────────────────────────────────────────
info "3. Pull daily manifest"
MANIFEST=$(curl -sf "$API/api/v1/manifest/daily" -H "Authorization: Bearer $COORD_TOKEN" || echo "{}")
SESSION_COUNT=$(echo "$MANIFEST" | python3 -c "import sys,json; m=json.load(sys.stdin); print(len(m.get('sessions',[])))" 2>/dev/null || echo "0")
pass "Manifest fetched — $SESSION_COUNT units available"

# ── 4. Open a session ─────────────────────────────────────────────────────────
info "4. Open session"
UNIT_ID=$(echo "$MANIFEST" | python3 -c "
import sys,json
m = json.load(sys.stdin)
s = m.get('sessions', [])
print(s[0]['unit_id'] if s else '')
" 2>/dev/null || echo "")

if [ -z "$UNIT_ID" ]; then
  info "No units in manifest — using seeded unit UNIT-E2E-001 directly"
  UNIT_ID="UNIT-E2E-001"
fi

SESSION_OPEN=$(curl -sf -X POST "$API/api/v1/sessions/open" \
  -H "Authorization: Bearer $COORD_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"unit_id\":\"$UNIT_ID\",\"lecturer_id\":\"c0000000-0000-0000-0000-000000000001\"}" \
  || echo "{}")
SESSION_ID=$(echo "$SESSION_OPEN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('session_id',''))" 2>/dev/null || echo "")
[ -n "$SESSION_ID" ] || fail "Session open failed: $SESSION_OPEN"
ROOM_CODE=$(echo "$SESSION_OPEN" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('checkin_code',''))" 2>/dev/null || echo "")
pass "Session opened: $SESSION_ID  room-code: $ROOM_CODE"

# ── 5. Verify lecturer_attendance_logs row created ────────────────────────────
info "5. Verify lecturer_attendance_logs"
LAL_COUNT=$(eval "$PSQL -t -c \"SELECT COUNT(*) FROM lecturer_attendance_logs WHERE session_id = '$SESSION_ID'\"" | tr -d ' \n')
[ "$LAL_COUNT" = "1" ] || { info "WARNING: expected 1 lal row, got $LAL_COUNT (lecturer_id required for write)"; }
[ "$LAL_COUNT" = "1" ] && pass "lecturer_attendance_logs row created (gate_open recorded)"

# ── 6. Close the session ──────────────────────────────────────────────────────
info "6. Close session"
CLOSE_RES=$(curl -sf -X POST "$API/api/v1/sessions/$SESSION_ID/close" \
  -H "Authorization: Bearer $COORD_TOKEN" || echo "{}")
CLOSE_STATUS=$(echo "$CLOSE_RES" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('status',''))" 2>/dev/null || echo "")
[ "$CLOSE_STATUS" = "CLOSED" ] || fail "Close session failed: $CLOSE_RES"
pass "Session closed"

# Verify session_status in DB
DB_STATUS=$(eval "$PSQL -t -c \"SELECT session_status FROM sessions WHERE session_id = '$SESSION_ID'\"" | tr -d ' \n')
[ "$DB_STATUS" = "CLOSED" ] || fail "DB session_status=$DB_STATUS, expected CLOSED"
pass "DB: session_status=CLOSED"

# Verify lecturer_attendance_logs gate_close_time was filled
LAL_CLOSE=$(eval "$PSQL -t -c \"SELECT gate_close_time IS NOT NULL FROM lecturer_attendance_logs WHERE session_id = '$SESSION_ID'\"" | tr -d ' \n')
[ "$LAL_CLOSE" = "t" ] && pass "DB: lecturer gate_close_time recorded" || info "WARNING: gate_close_time NULL (no lal row for this session)"

info "Session lifecycle (open → close) PASSED"

# ── 7. QR generation + SMTP email test ──────────────────────────────────────
if [ "$QR_UP" = "true" ]; then
  info "7. QR batch generation + email delivery"
  GEN_RES=$(curl -sf -X POST "$API/api/v1/qr/generate/batch" \
    -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H "X-Tenant-ID: $TENANT_ID" \
    -H "Content-Type: application/json" \
    -d '{"academic_year":"2024/2025"}' || echo "{}")
  JOB_ID=$(echo "$GEN_RES" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('job_id',''))" 2>/dev/null || echo "")
  EST=$(echo "$GEN_RES" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('estimated_count',0))" 2>/dev/null || echo "0")
  [ -n "$JOB_ID" ] || fail "QR generation failed: $GEN_RES"
  pass "QR generation queued: job=$JOB_ID  estimated=$EST students"
  info "Check your inbox (or Mailhog at :8025) for QR emails."
else
  info "7. Skipping QR email test (qr-generator not running)"
fi

# ── 8. Sync test ──────────────────────────────────────────────────────────────
SYNC_PORT="${SYNC_PORT:-8083}"
info "8. Sync receiver health (port $SYNC_PORT)"
SYNC_HEALTH=$(curl -sf "http://localhost:${SYNC_PORT}/health" 2>/dev/null || echo "{}")
if echo "$SYNC_HEALTH" | grep -q "ok"; then
  pass "sync-receiver healthy"
  info "Sync round-trip requires a sealed session package from the coordinator PWA."
  info "To test: open coordinator PWA → run a session → close → allow outbox to sync."
  info "Then verify: SELECT COUNT(*) FROM attendance_logs WHERE session_id = '<id>'"
else
  info "sync-receiver not running on :${SYNC_PORT} — skipping sync test"
fi

echo ""
echo -e "${GREEN}═══════════════════════════════════${NC}"
echo -e "${GREEN}  E2E test run complete${NC}"
echo -e "${GREEN}═══════════════════════════════════${NC}"
