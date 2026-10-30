#!/usr/bin/env bash
# Firewall rules. Usage: source firewall.sh <dev|prod> [tailscale-only|public|custom] [extra_ports]
set -euo pipefail

MODE="${1:-dev}"
FIREWALL_MODE="${2:-}"
FIREWALL_EXTRA_PORTS="${3:-}"

if [ -z "$FIREWALL_MODE" ]; then
  if [ "$MODE" = "dev" ]; then
    FIREWALL_MODE="tailscale-only"
  else
    FIREWALL_MODE="public"
  fi
fi

if ! command -v ufw &>/dev/null; then
  apt-get install -y -qq ufw
fi

ufw --force reset >/dev/null 2>&1
ufw default deny incoming
ufw default allow outgoing

case "$FIREWALL_MODE" in
  tailscale-only)
    ufw allow 41641/udp comment 'Tailscale'
    ufw allow from 100.64.0.0/10 to any port 22 comment 'SSH via Tailscale'
    echo "[firewall] Mode: tailscale-only (no public ports)"
    ;;
  public)
    ufw allow 80/tcp comment 'HTTP'
    ufw allow 443/tcp comment 'HTTPS'
    ufw allow 22/tcp comment 'SSH (break-glass)'
    ufw allow 41641/udp comment 'Tailscale'
    echo "[firewall] Mode: public (80/443 + SSH + Tailscale)"
    ;;
  custom)
    CONF="${SCRIPT_DIR:-.}/firewall.conf"
    if [ -f "$CONF" ]; then
      while IFS= read -r line; do
        line="${line%%#*}"
        line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
        [ -z "$line" ] && continue
        # Parse: allow|deny <port/proto> [from <source>] [comment '<text>']
        ufw $line 2>&1 | tail -1
      done < "$CONF"
      echo "[firewall] Mode: custom (from $CONF)"
    else
      echo "[firewall] custom mode but no firewall.conf found. Denying all."
    fi
    ;;
esac

# Extra ports
if [ -n "$FIREWALL_EXTRA_PORTS" ]; then
  IFS=',' read -ra PORTS <<< "$FIREWALL_EXTRA_PORTS"
  for port in "${PORTS[@]}"; do
    ufw allow "$port" comment "extra port"
  done
  echo "[firewall] Extra ports opened: $FIREWALL_EXTRA_PORTS"
fi

ufw --force enable
echo "[firewall] Firewall enabled"