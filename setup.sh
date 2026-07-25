#!/usr/bin/env bash
# QAAT — one-command bring-up for ANY laptop.
#
#   ./setup.sh
#
# Prereqs: Docker + Docker Compose (v1 or v2). Everything else is self-contained
# in this folder: source, RSA keys, infra/.env, TLS cert, DB migrations + the
# platform/super-admin seed (auto-applied on first boot).
set -euo pipefail
cd "$(dirname "$0")"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
say(){ echo -e "${GREEN}[setup]${NC} $*"; }
warn(){ echo -e "${YELLOW}[setup]${NC} $*"; }

# 1) Pick the compose command (v2 'docker compose' or v1 'docker-compose').
if docker compose version >/dev/null 2>&1; then COMPOSE="docker compose";
elif command -v docker-compose >/dev/null 2>&1; then COMPOSE="docker-compose";
else echo "ERROR: Docker Compose not found. Install Docker + Compose first."; exit 1; fi
COMPOSE="$COMPOSE -f infra/docker-compose.yml"
say "Using: $COMPOSE"

# 2) Ensure the auth-service RSA keys exist (generated once; they travel with the folder).
if [ ! -f keys/auth_private.pem ]; then
  say "Generating auth RSA keys…"
  mkdir -p keys
  openssl genrsa -out keys/auth_private.pem 2048
  openssl rsa -in keys/auth_private.pem -pubout -out keys/auth_public.pem
fi

# 3) Ensure a TLS cert exists (self-signed, covers the current LAN IP + hotspot IP +
#    localhost + qaat.local). The LAN IP is detected so phones on the home Wi-Fi don't
#    hit a hostname mismatch when DHCP changes it. Re-generate anytime with:
#      QAAT_REGEN_CERT=1 ./setup.sh   (or delete infra/certs/qaat.crt first)
LAN_IP=$(ip -4 addr show 2>/dev/null | grep -oE 'inet (192\.168|10|172\.(1[6-9]|2[0-9]|3[01]))\.[0-9.]+' | awk '{print $2}' | grep -v '^127\.' | grep -v '^10\.42\.0\.1$' | head -1)
if [ ! -f infra/certs/qaat.crt ] || [ -n "$QAAT_REGEN_CERT" ]; then
  say "Generating self-signed TLS cert (LAN IP: ${LAN_IP:-none detected})…"
  mkdir -p infra/certs
  SAN="IP:10.42.0.1,IP:127.0.0.1,DNS:localhost,DNS:qaat.local"
  [ -n "$LAN_IP" ] && SAN="IP:${LAN_IP},${SAN}"
  openssl req -x509 -newkey rsa:2048 -nodes -keyout infra/certs/qaat.key -out infra/certs/qaat.crt -days 825 \
    -subj "/CN=qaat-local" \
    -addext "subjectAltName=${SAN}"
fi

# 4) Build images + start. The DB auto-runs all migrations + the platform seed on
#    its FIRST boot (db/migrations is mounted into /docker-entrypoint-initdb.d).
say "Building images (first run downloads base images — give it a few minutes)…"
$COMPOSE build
say "Starting the stack…"
$COMPOSE up -d

# 5) Wait for the gateway to answer.
say "Waiting for the API gateway…"
for i in $(seq 1 60); do
  curl -sk -o /dev/null https://localhost:8443/health 2>/dev/null && { say "Gateway is up."; break; }
  sleep 2
done

cat <<EOF

${GREEN}=========================================================${NC}
 QAAT is running on this laptop.

 Dashboards (on this laptop):
   Coordinator : https://localhost:3000
   Admin       : https://localhost:3001
   Super-Admin : https://localhost:3002
   Student     : https://localhost:3003
   API health  : https://localhost:8443/health

 Default Super-Admin login (change it after first login):
   email: superadmin@qaat.platform    password: Super1234!
   (Super-Admin → register your institution/tenant + its admin.)

 To use it from PHONES, fully offline, run the hotspot:
   echo 'address=/qaat.local/10.42.0.1' | sudo tee /etc/NetworkManager/dnsmasq-shared.d/qaat.conf
   sudo nmcli device wifi hotspot ifname wlan0 ssid QAAT-Attendance password qaat12345
   → phones join Wi-Fi "QAAT-Attendance", then open https://10.42.0.1:3000 (etc.)

 Self-signed cert → on each device tap Advanced ▸ Proceed once per port.
${GREEN}=========================================================${NC}
EOF
