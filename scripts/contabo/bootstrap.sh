#!/usr/bin/env bash
# Wrapper — same as bootstrap-server.sh.
exec "$(cd "$(dirname "$0")" && pwd)/bootstrap-server.sh"
