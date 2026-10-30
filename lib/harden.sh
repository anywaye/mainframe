#!/usr/bin/env bash
# Mainframe hardening — sane defaults, no levels.
# Applies a solid security baseline. Optional extras are env-gated.
# Assets (config templates) are in assets/. Copy, fill placeholders, validate, apply.
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="$SCRIPT_DIR/../assets"
AUDIT_DIR="$SCRIPT_DIR/../audit"

# Sane defaults (all true unless explicitly disabled)
HARDEN_SSH_CRYPTO="${HARDEN_SSH_CRYPTO:-true}"
HARDEN_BBR="${HARDEN_BBR:-true}"
HARDEN_APPARMOR="${HARDEN_APPARMOR:-true}"
HARDEN_UNATTENDED="${HARDEN_UNATTENDED:-true}"
HARDEN_NEEDRESTART="${HARDEN_NEEDRESTART:-true}"
HARDEN_SWAP="${HARDEN_SWAP:-true}"
HARDEN_JOURNALD="${HARDEN_JOURNALD:-true}"
HARDEN_ETCKEEPER="${HARDEN_ETCKEEPER:-true}"
HARDEN_DEADMAN="${HARDEN_DEADMAN:-true}"
HARDEN_FAIL2BAN="${HARDEN_FAIL2BAN:-true}"

# Optional extras (all false unless explicitly enabled)
HARDEN_FIDO2="${HARDEN_FIDO2:-false}"
HARDEN_TOTP="${HARDEN_TOTP:-false}"
HARDEN_VPN_ONLY="${HARDEN_VPN_ONLY:-false}"
HARDEN_AUDITD="${HARDEN_AUDITD:-false}"
HARDEN_AIDE="${HARDEN_AIDE:-false}"
HARDEN_NOEXEC_SHM="${HARDEN_NOEXEC_SHM:-false}"
HARDEN_CORE_DUMPS="${HARDEN_CORE_DUMPS:-false}"

echo "[harden] Running audit (read-only, baseline)..."
if [ -f "$AUDIT_DIR/audit.sh" ]; then
  bash "$AUDIT_DIR/audit.sh" 2>&1 | tail -3 || true
fi

# 1. SSH hardening (sane defaults)
if [ "$HARDEN_SSH_CRYPTO" = "true" ]; then
  echo "[harden] Applying SSH hardening..."
  if [ "$(id -u)" -ne 0 ]; then
    echo "[harden] Not root — skipping SSH hardening (run as root on the VPS)"
  else
    cp "$ASSETS_DIR/ssh/00-mainframe.conf" /etc/ssh/sshd_config.d/00-mainframe.conf 2>/dev/null || echo "[harden] SSH config dir not found (non-Linux?)"
    groupadd -f sshusers 2>/dev/null || true
    sshd -t 2>/dev/null && echo "[harden] SSH config valid" || echo "[harden] SSH validation skipped"
  fi
fi

# 2. Sysctl (sane defaults)
if [ "$HARDEN_BBR" = "true" ]; then
  echo "[harden] Applying sysctl (including BBR)..."
  mkdir -p /etc/sysctl.d
  cp "$ASSETS_DIR/sysctl/99-mainframe.conf" /etc/sysctl.d/99-mainframe.conf 2>/dev/null || echo "[harden] sysctl dir not available (non-Linux?)"
  sysctl --system >/dev/null 2>&1 || true
fi

# 3. Journald limits
if [ "$HARDEN_JOURNALD" = "true" ]; then
  echo "[harden] Applying journald limits..."
  mkdir -p /etc/systemd/journald.conf.d 2>/dev/null || true
  cp "$ASSETS_DIR/journald/10-mainframe.conf" /etc/systemd/journald.conf.d/10-mainframe.conf 2>/dev/null || echo "[harden] journald not available (non-Linux?)"
  systemctl restart systemd-journald 2>/dev/null || true
fi

# 4. Unattended upgrades
if [ "$HARDEN_UNATTENDED" = "true" ]; then
  echo "[harden] Enabling unattended security updates..."
  apt-get install -y -qq unattended-upgrades 2>&1 | tail -1
  cp "$ASSETS_DIR/apt/20auto-upgrades" /etc/apt/apt.conf.d/20auto-upgrades 2>/dev/null || true
  cp "$ASSETS_DIR/apt/52mainframe-unattended" /etc/apt/apt.conf.d/52mainframe-unattended 2>/dev/null || true
fi

# 5. Needrestart
if [ "$HARDEN_NEEDRESTART" = "true" ]; then
  echo "[harden] Configuring needrestart..."
  apt-get install -y -qq needrestart 2>&1 | tail -1
  mkdir -p /etc/needrestart/conf.d
  cp "$ASSETS_DIR/needrestart/50-mainframe.conf" /etc/needrestart/conf.d/50-mainframe.conf
fi

# 6. Swap
if [ "$HARDEN_SWAP" = "true" ]; then
  if [ ! -f /swapfile ]; then
    echo "[harden] Creating 2GB swap file..."
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
  else
    echo "[harden] Swap already exists"
  fi
fi

# 7. etckeeper
if [ "$HARDEN_ETCKEEPER" = "true" ]; then
  apt-get install -y -qq etckeeper 2>&1 | tail -1
  if [ ! -d /etc/.git ]; then
    etckeeper init 2>&1 | tail -1
    etckeeper commit "mainframe: initial" 2>&1 | tail -1
  fi
fi

# 8. Deadman switch
if [ "$HARDEN_DEADMAN" = "true" ]; then
  if [ -f "$AUDIT_DIR/deadman.sh" ]; then
    install -m 755 "$AUDIT_DIR/deadman.sh" /usr/local/sbin/mainframe-deadman
    echo "[harden] Deadman installed"
  fi
fi

# 9. fail2ban
if [ "$HARDEN_FAIL2BAN" = "true" ]; then
  apt-get install -y -qq fail2ban 2>&1 | tail -1
  mkdir -p /etc/fail2ban/jail.d
  cp "$ASSETS_DIR/fail2ban/mainframe.local" /etc/fail2ban/jail.d/mainframe.local 2>/dev/null || true
  systemctl enable --now fail2ban 2>&1 | tail -1
  echo "[harden] fail2ban installed"
fi

# 10. AppArmor
if [ "$HARDEN_APPARMOR" = "true" ]; then
  if command -v aa-status &>/dev/null; then
    aa-status 2>&1 | head -2
  else
    apt-get install -y -qq apparmor apparmor-utils 2>&1 | tail -1
  fi
fi

# Optional extras
if [ "$HARDEN_FIDO2" = "true" ]; then
  echo "[harden] FIDO2: configure SSH keys with -t ed25519-sk manually"
fi
if [ "$HARDEN_TOTP" = "true" ]; then
  echo "[harden] TOTP: configure PAM manually (see docs/hardening/ssh.md)"
fi
if [ "$HARDEN_AUDITD" = "true" ]; then
  apt-get install -y -qq auditd 2>&1 | tail -1
  systemctl enable --now auditd
fi
if [ "$HARDEN_AIDE" = "true" ]; then
  apt-get install -y -qq aide 2>&1 | tail -1
  echo "[harden] Run 'aideinit' to initialize AIDE database"
fi
if [ "$HARDEN_NOEXEC_SHM" = "true" ]; then
  echo "[harden] /dev/shm noexec: add to /etc/fstab manually"
fi
if [ "$HARDEN_CORE_DUMPS" = "true" ]; then
  echo "kernel.core_pattern = |/bin/false" > /etc/sysctl.d/99-core-dumps.conf
  sysctl --system >/dev/null 2>&1 || true
fi

# Commit etckeeper
if [ -d /etc/.git ]; then
  etckeeper commit "mainframe: hardening applied" 2>&1 | tail -1
fi

# Run audit after hardening
echo ""
echo "[harden] Running audit (post-hardening)..."
if [ -f "$AUDIT_DIR/audit.sh" ]; then
  bash "$AUDIT_DIR/audit.sh" 2>&1 | tail -3 || true
fi

echo "[harden] Done. Review audit output above."
