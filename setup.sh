#!/usr/bin/env bash
# QAAT — one-command bring-up for ANY laptop.
#
#   ./setup.sh
#
# Prereqs: Docker + Docker Compose (v1 or v2). Everything else is self-contained
# in this folder: source, RSA keys, infra/field.env, TLS cert, DB migrations +
# the seed (auto-applied on first boot).
set -euo pipefail
cd "$(dirname "$0")"

GREEN='\033[0;32m'; YELLOW='\033[1;33m'; NC='\033[0m'
say(){ echo -e "${GREEN}[setup]${NC} $*"; }
warn(){ echo -e "${YELLOW}[setup]${NC} $*"; }

# 1) Pick the compose command (v2 'docker compose' or v1 'docker-compose').
if docker compose version >/dev/null 2>&1; then COMPOSE="docker compose";
elif command -v docker-compose >/dev/null 2>&1; then COMPOSE="docker-compose";
else echo "ERROR: Docker Compose not found. Install Docker + Compose first."; exit 1; fi
# The env file is named EXPLICITLY. It used to be infra/.env, which compose loaded
# on its own — and which therefore also attached itself to any other compose command
# run against this file, handing a locally-initialised database credentials it had
# never had. Naming it here means this script says which environment it means, and
# nothing else silently inherits that answer.
ENV_FILE="infra/field.env"
if [ ! -r "$ENV_FILE" ]; then
  echo "ERROR: $ENV_FILE is missing. It ships with this folder and holds the"
  echo "       database/Redis passwords for this deployment. Without it the stack"
  echo "       cannot start. (The repo root .env is the LOCAL dev config, not this.)"
  exit 1
fi
COMPOSE="$COMPOSE -f infra/docker-compose.yml --env-file $ENV_FILE"
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
   Admin / staff : https://localhost:3001
   Student       : https://localhost:3003
   API health    : https://localhost:8443/health
   (Coordinators use the Android app, not a browser. Ports 3000 and 3002 were
    the coordinator PWA and the Super-Admin console; neither is served now.)

 FIRST LOGIN on a brand-new database: there is none yet. Migration 038 seeds a
 platform-owner account and migration 064 deletes it again, because that role
 was removed when QAAT became single-institution. Insert the institution's first
 ADMIN into "users" directly (password_hash = bcrypt cost 12), then create every
 other account from the Admin dashboard.

 To use it from PHONES, fully offline, run the hotspot:
   echo 'address=/qaat.local/10.42.0.1' | sudo tee /etc/NetworkManager/dnsmasq-shared.d/qaat.conf
   sudo nmcli device wifi hotspot ifname wlan0 ssid QAAT-Attendance password qaat12345
   → phones join Wi-Fi "QAAT-Attendance", then open https://10.42.0.1:3000 (etc.)

 Self-signed cert → on each device tap Advanced ▸ Proceed once per port.
${GREEN}=========================================================${NC}
EOF
