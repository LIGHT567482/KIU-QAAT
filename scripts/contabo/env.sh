#!/usr/bin/env bash
# Read KEY from a dotenv file without executing it as shell.
# `source .env` treats "TOKEN= abc" as "run command abc".
env_get() {
  local key="$1" file="${2:-.env.production}"
  [[ -f "$file" ]] || return 0
  awk -F= -v k="$key" '
    $1 == k {
      sub(/^[^=]+=/, "")
      gsub(/\r/, "")
      gsub(/^[[:space:]]+/, "")
      gsub(/[[:space:]]+$/, "")
      if (length($0) >= 2) {
        q = substr($0, 1, 1)
        if ((q == "\"" || q == "'\''") && substr($0, length($0), 1) == q)
          $0 = substr($0, 2, length($0) - 2)
      }
      print
    }
  ' "$file" | tail -n 1
}
