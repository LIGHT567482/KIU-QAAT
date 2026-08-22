#!/usr/bin/env bash
# First-time and day-two deploy on Contabo. Same as U-Panel's
# scripts/contabo/deploy-web-on-server.sh for this repo.
exec "$(cd "$(dirname "$0")" && pwd)/deploy-web-on-server.sh"
