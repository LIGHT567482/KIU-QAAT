#!/usr/bin/env bash
# Start the qr-generator service.
# Source .env.smtp first for real email delivery, or leave unset for Mailhog.
# Usage:
#   source .env.smtp && ./scripts/start_qr_generator.sh   # real SMTP
#   ./scripts/start_qr_generator.sh                        # Mailhog (default)

set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
KEK=$(grep 'KEY_ENCRYPTION_KEY' "$REPO/.env" | tail -1 | cut -d= -f2)

cd "$REPO/services/qr-generator"

DB_URL="postgresql://qaat:changeme_db@127.0.0.1:5434/qaat?sslmode=disable" \
RSA_PUBLIC_KEY_PATH="$REPO/keys/auth_public.pem" \
KEY_ENCRYPTION_KEY="$KEK" \
SMTP_HOST="${SMTP_HOST:-localhost}" \
SMTP_PORT="${SMTP_PORT:-1025}" \
SMTP_SECURE="${SMTP_SECURE:-false}" \
SMTP_USER="${SMTP_USER:-}" \
SMTP_PASS="${SMTP_PASS:-}" \
PORT="3002" \
pnpm dev
