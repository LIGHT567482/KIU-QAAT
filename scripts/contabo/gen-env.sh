#!/usr/bin/env bash
# Create or repair .env.production.
# Never overwrites a non-empty secret. Fills empty / change-me keys so Compose
# ${VAR:?} interpolation can run. Preserves UPANEL_API_TOKEN if already set.
#
#   bash scripts/contabo/gen-env.sh              # create if missing, then ensure
#   bash scripts/contabo/gen-env.sh --ensure     # same (used by deploy)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
EXAMPLE="$ROOT/.env.production.example"
DEST="$ROOT/.env.production"
for a in "$@"; do
  case "$a" in
    --ensure) ;;
    -*) echo "unknown flag $a" >&2; exit 1 ;;
    *) DEST="$a" ;;
  esac
done

if [[ ! -f "$EXAMPLE" ]]; then
  echo "missing $EXAMPLE" >&2
  exit 1
fi

hex32() { openssl rand -hex 32; }

umask 077
if [[ ! -f "$DEST" ]]; then
  cp "$EXAMPLE" "$DEST"
  echo "created $DEST from example"
fi

# If the file was replaced with only a token, copy any missing keys from the example.
while IFS= read -r line || [[ -n "$line" ]]; do
  case "$line" in
    ''|'#'*) continue ;;
  esac
  key="${line%%=*}"
  [[ "$key" == "$line" ]] && continue
  if ! grep -qE "^${key}=" "$DEST"; then
    echo "$line" >> "$DEST"
  fi
done < "$EXAMPLE"

fill() {
  local key="$1" file="$2"
  local cur
  cur="$(awk -F= -v k="$key" '$1==k{sub(/^[^=]+=/,""); gsub(/\r/,""); gsub(/^[[:space:]'\''"]+/,""); gsub(/[[:space:]'\''"]+$/,""); print}' "$file" | tail -n 1)"
  if [[ -n "$cur" && "$cur" != "change-me" ]]; then
    return 0
  fi
  local hex
  hex="$(hex32)"
  if grep -qE "^${key}=" "$file"; then
    awk -v k="$key" -v v="$hex" -F= '
      $1==k { print k "=" v; next }
      { print }
    ' "$file" > "${file}.tmp" && mv "${file}.tmp" "$file"
  else
    echo "${key}=${hex}" >> "$file"
  fi
  echo "    filled ${key}"
}

echo "==> Ensuring secrets in $DEST"
fill DB_PASSWORD "$DEST"
fill APP_DB_PASSWORD "$DEST"
fill REDIS_PASSWORD "$DEST"
fill KEY_ENCRYPTION_KEY "$DEST"
fill INTERNAL_SVC_KEY "$DEST"
fill SYNC_SIGN_KEY "$DEST"
fill CRON_SECRET "$DEST"
chmod 600 "$DEST"
echo "ready: $DEST (set UPANEL_API_TOKEN=yourtoken with no space after =)"
