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
  if tailscale up --authkey="$TAILSCALE_AUTHKEY" 2>&1; then
    break
  fi
  echo "[tailscale] Retry $i/3..."
  sleep 2
done

echo "Tailscale: connected"
echo "  Tailscale IP: $(tailscale ip -4 2>/dev/null || echo 'check tailscale status')"

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
# Restart tailscaled if it's not responding (safety net only)
# Primary protection: OOMScoreAdjust + earlyoom + memory limits
if ! tailscale status >/dev/null 2>&1; then
  logger -t tailscale-watchdog "Tailscale not responding, restarting..."
  systemctl restart tailscaled
  sleep 5
  tailscale up --authkey="${TAILSCALE_AUTHKEY:-}" 2>/dev/null || true
fi
EOF
chmod +x /usr/local/bin/tailscale-watchdog
echo "[tailscale] Watchdog script installed (systemd timer runs every 5 min)"