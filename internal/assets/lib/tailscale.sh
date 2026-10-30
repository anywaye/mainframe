#!/usr/bin/env bash
# Install and configure Tailscale (mandatory — private network access)
set -euo pipefail

TAILSCALE_AUTHKEY="${TAILSCALE_AUTHKEY:-}"

if [ -z "$TAILSCALE_AUTHKEY" ]; then
  echo "ERROR: TAILSCALE_AUTHKEY is not set." >&2
  exit 1
fi

if ! command -v tailscale &>/dev/null; then
  curl -fsSL https://tailscale.com/install.sh | sh
fi

tailscale up --authkey="$TAILSCALE_AUTHKEY" --ssh
echo "Tailscale: connected"
echo "  Tailscale IP: $(tailscale ip -4 2>/dev/null || echo 'check tailscale status')"