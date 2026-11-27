#!/usr/bin/env bash
# Infisical secrets backend (self-hosted or cloud).
# Infisical is the ONLY secrets backend for mainframe.
# Requires: infisical CLI (auto-installed if missing)

# Configuration (read from env or mainframe.yml):
#   INFISICAL_HOST       - Infisical instance URL (default: http://localhost:8080 for self-hosted)
#   INFISICAL_PROJECT_ID - Project ID to fetch secrets from
#   INFISICAL_ENV        - Environment (dev, staging, prod; default: dev)
#   INFISICAL_CLIENT_ID  - Machine identity client ID
#   INFISICAL_CLIENT_SECRET - Machine identity client secret

INFISICAL_HOST="${INFISICAL_HOST:-http://localhost:8080}"
INFISICAL_ENV="${INFISICAL_ENV:-dev}"

secrets_available() {
  command -v infisical &>/dev/null
}

secrets_install() {
  if ! command -v infisical &>/dev/null; then
    echo "[secrets:infisical] Installing Infisical CLI..."
    if [[ "$(uname)" == "Darwin" ]]; then
      brew install infisical/get-cli/infisical 2>&1 | tail -2
    elif command -v apt-get &>/dev/null; then
      curl -1sLf 'https://dl.cloudsmith.io/public/infisical/infisical-cli/setup.deb.sh' 2>/dev/null | sudo -E bash 2>/dev/null
      sudo apt-get install -y infisical 2>&1 | tail -2
    elif command -v dnf &>/dev/null; then
      sudo dnf install -y infisical 2>&1 | tail -2
    elif command -v pacman &>/dev/null; then
      sudo pacman -S --noconfirm infisical 2>&1 | tail -2
    else
      echo "[secrets:infisical] Could not install. Install manually: https://infisical.com/docs/cli/overview" >&2
      return 1
    fi
  fi
  echo "[secrets:infisical] Infisical CLI: $(infisical --version 2>/dev/null)"
}

secrets_status() {
  if ! secrets_available; then
    echo "unavailable"
    return 1
  fi

  # Check machine identity
  if [ -n "${INFISICAL_CLIENT_ID:-}" ] && [ -n "${INFISICAL_CLIENT_SECRET:-}" ]; then
    # Try to authenticate
    if infisical login --method=universal-auth \
        --client-id="$INFISICAL_CLIENT_ID" \
        --client-secret="$INFISICAL_CLIENT_SECRET" \
        --domain="$INFISICAL_HOST" \
        --silent 2>/dev/null; then
      echo "authenticated (machine identity)"
    else
      # Try interactive login check
      if infisical user get 2>/dev/null | grep -q "email"; then
        echo "authenticated (interactive)"
      else
        echo "unauthenticated"
      fi
    fi
  else
    # Check if already logged in
    if infisical user get 2>/dev/null | grep -q "email"; then
      echo "authenticated (interactive)"
    else
      echo "unauthenticated"
    fi
  fi
}

secrets_authenticate() {
  if [ -n "${INFISICAL_CLIENT_ID:-}" ] && [ -n "${INFISICAL_CLIENT_SECRET:-}" ]; then
    infisical login --method=universal-auth \
      --client-id="$INFISICAL_CLIENT_ID" \
      --client-secret="$INFISICAL_CLIENT_SECRET" \
      --domain="$INFISICAL_HOST" \
      --silent 2>/dev/null
  else
    echo "[secrets:infisical] No machine identity configured." >&2
    echo "[secrets:infisical] Set INFISICAL_CLIENT_ID and INFISICAL_CLIENT_SECRET, or run 'infisical login' interactively." >&2
    return 1
  fi
}

secrets_get_value() {
  local secret_name="$1"
  local path="${2:-/}"

  if [ -z "${INFISICAL_PROJECT_ID:-}" ]; then
    echo "[secrets:infisical] INFISICAL_PROJECT_ID not set." >&2
    return 1
  fi

  # Ensure authenticated
  secrets_authenticate 2>/dev/null

  infisical secrets get "$secret_name" \
    --env="$INFISICAL_ENV" \
    --path="$path" \
    --projectId="$INFISICAL_PROJECT_ID" \
    --domain="$INFISICAL_HOST" \
    --plain \
    --silent 2>/dev/null
}

secrets_get_note() {
  # In Infisical, secrets are key-value. The "note" is the value itself.
  secrets_get_value "$1" "${2:-/}"
}

secrets_get_field() {
  local secret_name="$1"
  local field="$2"
  # Infisical doesn't have fields like Bitwarden. 
  # Convention: store as SECRET_NAME_FIELD or use path-based organization.
  secrets_get_value "${secret_name}_${field}" "${3:-/}"
}

secrets_export_all() {
  local path="${1:-/}"

  if [ -z "${INFISICAL_PROJECT_ID:-}" ]; then
    echo "[secrets:infisical] INFISICAL_PROJECT_ID not set." >&2
    return 1
  fi

  secrets_authenticate 2>/dev/null

  infisical export \
    --env="$INFISICAL_ENV" \
    --path="$path" \
    --projectId="$INFISICAL_PROJECT_ID" \
    --domain="$INFISICAL_HOST" \
    --format=dotenv \
    --silent 2>/dev/null
}

secrets_unlock_hint() {
  echo "[secrets:infisical] Setup machine identity:"
  echo "  1. Create a machine identity in Infisical (Settings > Machine Identities)"
  echo "  2. Assign it to your project with read access"
  echo "  3. Set environment variables:"
  echo "     export INFISICAL_HOST='https://your-infisical-instance.com'"
  echo "     export INFISICAL_PROJECT_ID='your-project-id'"
  echo "     export INFISICAL_CLIENT_ID='your-client-id'"
  echo "     export INFISICAL_CLIENT_SECRET='your-client-secret'"
  echo "  Or run 'infisical login' interactively for development."
}
