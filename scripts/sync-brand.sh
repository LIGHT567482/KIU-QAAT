#!/usr/bin/env bash
# Single source of truth = ./brand.json (repo root). This copies it into every app's build
# context so the HYBRID branding (bundled default) stays in sync. Run after editing brand.json,
# then rebuild/redeploy the backend + frontends + Android APK.
set -euo pipefail
cd "$(dirname "$0")/.."
cp brand.json backend/api-gateway/internal/handlers/brand.json
cp brand.json frontend/admin-dashboards/src/brand.json
cp brand.json frontend/student-portal/src/brand.json
cp brand.json frontend/coordinator-android/app/src/main/assets/brand.json
echo "brand.json synced to backend + 3 web apps + Android."
