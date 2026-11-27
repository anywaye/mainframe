#!/usr/bin/env bash
# Bitwarden secrets backend.
# Requires: bw CLI (auto-installed if missing)

secrets_available() {
  command -v bw &>/dev/null
}

secrets_install() {
  if ! command -v bw &>/dev/null; then
    echo "[secrets:bw] Installing Bitwarden CLI..."
    npm install -g @bitwarden/cli 2>&1 | tail -1
  fi
  echo "[secrets:bw] Bitwarden CLI: $(bw --version 2>/dev/null)"
}

secrets_status() {
  if ! secrets_available; then
    echo "unavailable"
    return 1
  fi
  bw status 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "unknown"
}

secrets_get_note() {
  local item_name="$1"
  if [ -z "${BW_SESSION:-}" ]; then
    echo "[secrets:bw] BW_SESSION not set. Run 'bw unlock --raw' first." >&2
    return 1
  fi
  bw get item "$item_name" 2>/dev/null | python3 -c "
import json, sys
data = json.load(sys.stdin)
print(data.get('notes', ''))
" 2>/dev/null
}

secrets_get_field() {
  local item_name="$1"
  local field="$2"
  if [ -z "${BW_SESSION:-}" ]; then
    echo "[secrets:bw] BW_SESSION not set." >&2
    return 1
  fi
  bw get item "$item_name" 2>/dev/null | python3 -c "
import json, sys
data = json.load(sys.stdin)
for f in data.get('fields', []):
    if f.get('name') == '$field':
        print(f.get('value', ''))
        break
" 2>/dev/null
}

secrets_unlock_hint() {
  echo "[secrets:bw] To unlock: bw login <email> && export BW_SESSION=\$(bw unlock --raw)"
}
