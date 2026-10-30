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

# Use --reset if already connected, otherwise plain up
if tailscale status >/dev/null 2>&1; then
  tailscale up --authkey="$TAILSCALE_AUTHKEY" --ssh --reset --accept-risk=lose-ssh 2>/dev/null || tailscale up --authkey="$TAILSCALE_AUTHKEY" --ssh --accept-risk=lose-ssh
else
  tailscale up --authkey="$TAILSCALE_AUTHKEY" --ssh
fi
echo "Tailscale: connected"
echo "  Tailscale IP: $(tailscale ip -4 2>/dev/null || echo 'check tailscale status')"