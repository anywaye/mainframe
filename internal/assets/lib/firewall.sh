#!/usr/bin/env bash
# Firewall rules. Usage: source firewall.sh <dev|prod> [tailscale-only|public|custom] [extra_ports]
set -euo pipefail

MODE="${1:-dev}"
FIREWALL_MODE="${2:-}"

# Validate mode
if [ "$MODE" != "dev" ] && [ "$MODE" != "prod" ]; then
  echo "[firewall] ERROR: invalid mode '$MODE'. Use 'dev' or 'prod'." >&2
  exit 1
fi

# Validate firewall mode
if [ -n "$FIREWALL_MODE" ]; then
  case "$FIREWALL_MODE" in
    tailscale-only|public|custom) ;;
    *)
      echo "[firewall] ERROR: invalid firewall mode '$FIREWALL_MODE'. Use 'tailscale-only', 'public', or 'custom'." >&2
      exit 1
      ;;
  esac
fi
FIREWALL_EXTRA_PORTS="${3:-}"
ALLOW_DB_FROM="${4:-${ALLOW_DB_FROM:-}}"

# --- Existing firewall detection ---
# Check if ufw already has rules (existing VPS)
EXISTING_RULES=""
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "ALLOW\|DENY"; then
  EXISTING_RULES=$(ufw status numbered 2>/dev/null | grep -E "ALLOW|DENY" || true)
fi

FORCE_FIREWALL="${FORCE_FIREWALL:-false}"
if [ -n "$EXISTING_RULES" ] && [ "$FORCE_FIREWALL" != "true" ]; then
  echo "[firewall] Existing firewall rules detected:"
  echo "$EXISTING_RULES" | while IFS= read -r line; do echo "  $line"; done
  echo "[firewall] Skipping firewall reset to preserve existing rules."
  echo "[firewall] To override: set FORCE_FIREWALL=true or use --force-firewall"
  echo "[firewall] Add custom rules to firewall.conf and re-run with FIREWALL_MODE=custom"
  # Save existing rules to a reference file
  if [ -n "${PROJECT_DIR:-}" ]; then
    ufw status numbered > "$PROJECT_DIR/firewall.existing" 2>/dev/null || true
    echo "[firewall] Existing rules saved to $PROJECT_DIR/firewall.existing"
  fi
  exit 0
fi

if [ -z "$FIREWALL_MODE" ]; then
  # Default to "public" (SSH on port 22) for safety.
  # tailscale-only mode blocks all SSH if Tailscale crashes.
  # Use tailscale-only only if you have a working Tailscale connection
  # AND a console password for recovery.
  FIREWALL_MODE="public"
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

# DB port allowlisting (from --allow-db-from)
if [ -n "$ALLOW_DB_FROM" ]; then
  ufw allow from "$ALLOW_DB_FROM" to any port 5432 proto tcp comment "PostgreSQL from $ALLOW_DB_FROM"
  ufw allow from "$ALLOW_DB_FROM" to any port 6432 proto tcp comment "PgBouncer from $ALLOW_DB_FROM"
  echo "[firewall] DB ports allowed from: $ALLOW_DB_FROM"
fi

# Extra ports
if [ -n "$FIREWALL_EXTRA_PORTS" ]; then
  IFS=',' read -ra PORTS <<< "$FIREWALL_EXTRA_PORTS"
  for port in "${PORTS[@]}"; do
    ufw allow "$port" comment "extra port"
  done
  echo "[firewall] Extra ports opened: $FIREWALL_EXTRA_PORTS"
fi

# Safety: verify SSH is working before enabling firewall
# If SSH is not accessible, enabling the firewall could lock us out.
if [ "$FIREWALL_MODE" = "tailscale-only" ]; then
  if ! tailscale status >/dev/null 2>&1; then
    echo "[firewall] WARNING: Tailscale is not connected. tailscale-only mode will block ALL SSH."
    echo "[firewall] Aborting firewall enable. Fix Tailscale first, then re-run."
    exit 1
  fi
  # Verify we can reach the Tailscale IP
  TS_IP=$(tailscale ip -4 2>/dev/null || echo "")
  if [ -z "$TS_IP" ]; then
    echo "[firewall] WARNING: No Tailscale IP. Cannot verify SSH access."
    echo "[firewall] Aborting firewall enable."
    exit 1
  fi
  echo "[firewall] Tailscale IP: $TS_IP — SSH will be allowed via Tailscale"
fi

ufw --force enable
echo "[firewall] Firewall enabled"