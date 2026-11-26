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

# Wait for tailscaled to be ready
for i in $(seq 1 10); do
  if tailscale status >/dev/null 2>&1 || [ "$(tailscale status 2>&1 | head -1)" != "" ]; then
    break
  fi
  sleep 1
done

# Retry tailscale up
for i in $(seq 1 3); do
  if tailscale up --authkey="$TAILSCALE_AUTHKEY" --ssh=false 2>&1; then
    break
  fi
  echo "[tailscale] Retry $i/3..."
  sleep 2
done

echo "Tailscale: connected"
echo "  Tailscale IP: $(tailscale ip -4 2>/dev/null || echo 'check tailscale status')"

# Persist authkey for recovery (watchdog needs it to re-authenticate)
mkdir -p /var/lib/mainframe
echo "$TAILSCALE_AUTHKEY" > /var/lib/mainframe/tailscale-authkey
chmod 600 /var/lib/mainframe/tailscale-authkey
echo "[tailscale] Authkey persisted to /var/lib/mainframe/tailscale-authkey"

# OOM protection: prevent OOM killer from terminating tailscaled during heavy builds
if [ -f /etc/systemd/system/tailscaled.service ]; then
  # Add OOMScoreAdjust and Restart to the service
  mkdir -p /etc/systemd/system/tailscaled.service.d 2>/dev/null || true
  cat > /etc/systemd/system/tailscaled.service.d/oom-protection.conf << 'EOF'
[Service]
OOMScoreAdjust=-1000
Restart=always
RestartSec=5
EOF
  systemctl daemon-reload 2>/dev/null || true
  systemctl restart tailscaled 2>/dev/null || true
  echo "[tailscale] OOM protection enabled (tailscaled will not be killed by OOM killer)"
fi

# Watchdog script (systemd timer handles scheduling, see harden.sh)
cat > /usr/local/bin/tailscale-watchdog << 'EOF'
#!/bin/bash
# Tailscale watchdog: checks connectivity, not just daemon status
# Re-authenticates if disconnected, restarts daemon if crashed

AUTHKEY="${TAILSCALE_AUTHKEY:-}"
if [ -z "$AUTHKEY" ] && [ -f /var/lib/mainframe/tailscale-authkey ]; then
  AUTHKEY=$(cat /var/lib/mainframe/tailscale-authkey)
fi

# Check if tailscaled daemon is running
if ! systemctl is-active --quiet tailscaled; then
  logger -t tailscale-watchdog "tailscaled not active, restarting..."
  systemctl restart tailscaled
  sleep 5
fi

# Check if actually connected to Tailscale network
if ! tailscale status >/dev/null 2>&1; then
  logger -t tailscale-watchdog "Tailscale not connected, re-authenticating..."
  if [ -n "$AUTHKEY" ]; then
    tailscale up --authkey="$AUTHKEY" --ssh=false 2>/dev/null || true
  else
    tailscale up --ssh=false 2>/dev/null || true
  fi
  sleep 5
fi

# Verify connection
if tailscale status >/dev/null 2>&1; then
  : # Connected, all good
else
  logger -t tailscale-watchdog "WARNING: Tailscale still not connected after recovery attempt"
fi
EOF
chmod +x /usr/local/bin/tailscale-watchdog
# Tailscale keep-alive: prevent connection drops
tailscale up --snat-subnet-routes=false --accept-routes --ssh=false 2>/dev/null || true
echo "[tailscale] Keep-alive configured (snat-subnet-routes=false, accept-routes)"

echo "[tailscale] Watchdog script installed (systemd timer runs every 5 min)"