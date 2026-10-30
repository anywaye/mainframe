#!/usr/bin/env bash
# provision.sh — provision a VPS as a dev or prod box.
#
# Usage:
#   sudo bash provision.sh --mode dev   [--project <name>]
#   sudo bash provision.sh --mode prod
#
# Modes:
#   dev   Secure dev box: OS, packages, Tailscale, hardening, /code dir.
#         No public services. For active development.
#   prod  Secure prod box: base hardening + k3s substrate + observability.
#         Public ingress via Traefik. For production workloads.
#
# Configuration: set env vars or create .env from .env.template
#   TAILSCALE_AUTHKEY   Tailscale auth key (strongly recommended)
#   GH_TOKEN            GitHub PAT (repo + packages:write scopes)
#   VPS_NAME            Hostname (default: mainframe)
#   BASE_DIR            Code directory (default: /code)
#   BW_EMAIL            Bitwarden email (optional)
#   PROJECT             Project name to set up after provisioning
#   PACKAGE_PROFILE     Package profile: (default: custom, reads packages.conf)
#   PACKAGES_FILE       Path to packages.conf (default: ./packages.conf)
#   TAILSCALE_ENABLED   Enable Tailscale (default: true)
#   HARDEN_ENABLED   Enable mainframe hardening (default: true)
#   FIREWALL_MODE       Firewall mode: tailscale-only, public, custom (default: auto)
#   FIREWALL_EXTRA_PORTS  Additional ports to open (comma-separated)
#
# Sane defaults (all enabled by default, disable individually if needed):
#   HARDEN_SSH_CRYPTO=true      Modern KEX/ciphers/MACs, ed25519 host key
#   HARDEN_BBR=true             BBR TCP congestion control
#   HARDEN_APPARMOR=true        AppArmor check
#   HARDEN_UNATTENDED=true      Unattended security updates
#   HARDEN_NEEDRESTART=true     Auto-restart services after updates
#   HARDEN_SWAP=true            2GB swap file
#   HARDEN_JOURNALD=true        Cap journald at 500MB
#   HARDEN_ETCKEEPER=true       Track /etc in git
#   HARDEN_DEADMAN=true         Auto-rollback safety net
#   HARDEN_FAIL2BAN=true        Brute-force protection
#
# Optional extras (disabled by default, enable if needed):
#   HARDEN_FIDO2=false          FIDO2 hardware keys
#   HARDEN_TOTP=false           TOTP 2FA
#   HARDEN_VPN_ONLY=false       SSH only via VPN
#   HARDEN_AUDITD=false         auditd file integrity
#   HARDEN_AIDE=false           AIDE file integrity
#   HARDEN_NOEXEC_SHM=false     /dev/shm noexec
#   HARDEN_CORE_DUMPS=false     Disable core dumps
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

MODE=""
PROJECT=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --mode)    MODE="$2"; shift 2 ;;
    --project) PROJECT="$2"; shift 2 ;;
    -h|--help) echo "Usage: $0 --mode dev|prod [--project <name>]"; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

if [ "$MODE" != "dev" ] && [ "$MODE" != "prod" ]; then
  echo "ERROR: --mode must be 'dev' or 'prod'" >&2
  exit 1
fi

if [ -f "$SCRIPT_DIR/.env" ]; then
  source "$SCRIPT_DIR/.env"
fi

TAILSCALE_AUTHKEY="${TAILSCALE_AUTHKEY:-}"
GH_TOKEN="${GH_TOKEN:-}"
BASE_DIR="${BASE_DIR:-/code}"
VPS_NAME="${VPS_NAME:-mainframe}"
PACKAGE_PROFILE="${PACKAGE_PROFILE:-custom}"
PACKAGES_FILE="${PACKAGES_FILE:-$SCRIPT_DIR/packages.conf}"
TAILSCALE_ENABLED="${TAILSCALE_ENABLED:-true}"
HARDEN_ENABLED="${HARDEN_ENABLED:-true}"
FIREWALL_MODE="${FIREWALL_MODE:-}"
FIREWALL_EXTRA_PORTS="${FIREWALL_EXTRA_PORTS:-}"

if [ -z "$FIREWALL_MODE" ]; then
  if [ "$MODE" = "dev" ]; then
    FIREWALL_MODE="tailscale-only"
  else
    FIREWALL_MODE="public"
  fi
fi

export DEBIAN_FRONTEND=noninteractive
export NEEDRESTART_MODE=a

echo "=== Provisioning $VPS_NAME (mode: $MODE) ==="
echo "Base dir: $BASE_DIR"
echo "Firewall: $FIREWALL_MODE"
echo "Tailscale: $TAILSCALE_ENABLED"
echo "mainframe: $HARDEN_ENABLED"

# BASE: OS detection
source "$SCRIPT_DIR/lib/detect.sh"
detect_os

OS_MODULE="$SCRIPT_DIR/lib/os/$OS_FAMILY.sh"
if [ ! -f "$OS_MODULE" ]; then
  echo "ERROR: OS module not found: $OS_MODULE" >&2
  exit 1
fi
source "$OS_MODULE"

# BASE: Packages from packages.conf
source "$SCRIPT_DIR/lib/packages.sh"
install_from_conf "$PACKAGES_FILE"

# BASE: Tailscale (optional but recommended)
if [ "$TAILSCALE_ENABLED" = "true" ]; then
  if [ -z "$TAILSCALE_AUTHKEY" ]; then
    echo "ERROR: TAILSCALE_ENABLED=true but TAILSCALE_AUTHKEY is not set." >&2
    exit 1
  fi
  source "$SCRIPT_DIR/lib/tailscale.sh"
else
  echo "[tailscale] Skipped (TAILSCALE_ENABLED=false)"
  echo "[tailscale] WARNING: Without Tailscale, SSH is exposed to the internet."
fi

# BASE: mainframe hardening (optional)
if [ "$HARDEN_ENABLED" = "true" ]; then
  source "$SCRIPT_DIR/lib/harden.sh"
else
  echo "[harden] Skipped (HARDEN_ENABLED=false)"
  echo "[harden] WARNING: Without hardening, the VPS is not secured."
fi

# BASE: Firewall
source "$SCRIPT_DIR/lib/firewall.sh" "$MODE" "$FIREWALL_MODE" "$FIREWALL_EXTRA_PORTS"

# BASE: Secrets (optional)
source "$SCRIPT_DIR/lib/secrets.sh"

# BASE: Directory structure
mkdir -p "$BASE_DIR"
mkdir -p "$BASE_DIR/worktrees"
if [ ! -e /data/workspace ]; then
  mkdir -p /data
  ln -s "$BASE_DIR" /data/workspace
fi

# BASE: Audit (show score)
if [ "$HARDEN_ENABLED" = "true" ] && [ -f "$SCRIPT_DIR/audit/audit.sh" ]; then
  echo ""
  echo "=== [audit] Running baseline audit ==="
  bash "$SCRIPT_DIR/audit/audit.sh" 2>&1 | tail -5
fi

# MODE: dev or prod
if [ "$MODE" = "dev" ]; then
  source "$SCRIPT_DIR/modes/dev.sh"
fi
if [ "$MODE" = "prod" ]; then
  source "$SCRIPT_DIR/modes/prod.sh"
fi

# Project setup (optional)
PROJECT="${PROJECT:-}"
if [ -n "$PROJECT" ]; then
  if [ -f "$SCRIPT_DIR/projects/$PROJECT/setup.sh" ]; then
    echo ""
    echo "=== Setting up project: $PROJECT ==="
    bash "$SCRIPT_DIR/projects/$PROJECT/setup.sh"
  else
    echo "WARNING: project setup not found: $SCRIPT_DIR/projects/$PROJECT/setup.sh"
  fi
fi

echo ""
echo "=== Provisioning complete (mode: $MODE) ==="
echo "VPS IP: $(hostname -I 2>/dev/null | awk '{print $1}')"
echo "Tailscale IP: $(tailscale ip -4 2>/dev/null || echo 'not connected')"
echo "Done!"
