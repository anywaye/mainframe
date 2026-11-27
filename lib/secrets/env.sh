#!/usr/bin/env bash
# Environment variable secrets backend.
# Reads secrets from environment variables.
# Item names are converted to env var names: "GitHub SSH Key" → "GITHUB_SSH_KEY"

secrets_available() {
  return 0  # always available
}

secrets_install() {
  echo "[secrets:env] No installation needed (uses environment variables)."
}

secrets_status() {
  echo "unlocked"  # always unlocked
}

secrets_get_note() {
  local item_name="$1"
  local var_name=$(echo "$item_name" | tr '[:lower:]' '[:upper:]' | tr ' -' '_')
  echo "${!var_name:-}"
}

secrets_get_field() {
  local item_name="$1"
  local field="$2"
  local var_name=$(echo "${item_name}_${field}" | tr '[:lower:]' '[:upper:]' | tr ' -' '_')
  echo "${!var_name:-}"
}

secrets_unlock_hint() {
  echo "[secrets:env] Set environment variables for your secrets."
}
