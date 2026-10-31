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

tailscale up --authkey="$TAILSCALE_AUTHKEY"
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

# Watchdog: check if tailscaled is actually connected (not just running)
cat > /usr/local/bin/tailscale-watchdog << 'EOF'
#!/bin/bash
# Restart tailscaled if it's not responding
if ! tailscale status >/dev/null 2>&1; then
  systemctl restart tailscaled
  sleep 5
  tailscale up --authkey="${TAILSCALE_AUTHKEY:-}" 2>/dev/null || true
fi
EOF
chmod +x /usr/local/bin/tailscale-watchdog

# Add cron job for watchdog (every minute)
if ! crontab -l 2>/dev/null | grep -q "tailscale-watchdog"; then
  (crontab -l 2>/dev/null; echo "* * * * * /usr/local/bin/tailscale-watchdog") | crontab -
  echo "[tailscale] Watchdog cron job installed (checks every minute)"
fi