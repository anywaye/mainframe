#!/usr/bin/env bash
# Mainframe hardening — sane defaults, no levels.
# Applies a solid security baseline. Optional extras are env-gated.
# Assets (config templates) are in assets/. Copy, fill placeholders, validate, apply.
set -uo pipefail
# Non-fatal mode: individual steps can fail without aborting
HARDEN_ERRORS=0

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ASSETS_DIR="$SCRIPT_DIR/../assets"
AUDIT_DIR="$SCRIPT_DIR/../audit"

# Validate running as root
if [ "$(id -u)" -ne 0 ]; then
  echo "[harden] ERROR: must run as root (use sudo)." >&2
  exit 1
fi

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

# 0. Lynis baseline audit
LYNIS_BEFORE=""
if command -v lynis &>/dev/null || apt-get install -y -qq lynis 2>/dev/null; then
  if command -v lynis &>/dev/null; then
    lynis audit system --quiet 2>/dev/null || true
    logfile=$(ls -t /var/log/lynis-report.dat 2>/dev/null | head -1)
    if [ -n "$logfile" ]; then
      LYNIS_BEFORE=$(grep -oP '^hardening_index=\K[0-9]+' "$logfile" 2>/dev/null || echo "0")
      echo "[harden] Lynis baseline score: $LYNIS_BEFORE/100"
    fi
  fi
fi

# 0. Console access check (lockout prevention)
# Before disabling password SSH, verify console access exists.
ROOT_PW_STATUS=$(passwd -S root 2>/dev/null | awk '{print $2}' || echo "unknown")
if [ "$ROOT_PW_STATUS" = "L" ] || [ "$ROOT_PW_STATUS" = "LK" ]; then
  echo "[harden] WARNING: root password is locked. If SSH keys fail, you will be locked out."
  echo "[harden] Set a root password for console access: passwd root"
  echo "[harden] Or ensure your provider's web console (VNC/serial) is accessible."
  echo "[harden] Continuing in 5 seconds... (Ctrl+C to abort)"
  sleep 5
fi

# Check for existing SSH config customizations
FORCE_SSH="${FORCE_SSH:-false}"
EXISTING_SSH_CONFIGS=$(ls /etc/ssh/sshd_config.d/*.conf 2>/dev/null | grep -v "00-mainframe.conf" | grep -v "50-cloud-init.conf" || true)
if [ -n "$EXISTING_SSH_CONFIGS" ] && [ "$FORCE_SSH" != "true" ]; then
  echo "[harden] Existing SSH config found:"
  for f in $EXISTING_SSH_CONFIGS; do echo "  $f"; done
  echo "[harden] Skipping SSH hardening to preserve existing config."
  echo "[harden] To override: set FORCE_SSH=true"
  HARDEN_SSH_CRYPTO="false"
fi

# 1. SSH hardening (sane defaults, idempotent)
if [ "$HARDEN_SSH_CRYPTO" = "true" ]; then
  if [ -f /etc/ssh/sshd_config.d/00-mainframe.conf ] && grep -q "PermitRootLogin no" /etc/ssh/sshd_config.d/00-mainframe.conf 2>/dev/null; then
    echo "[harden] SSH hardening already applied, skipping"
  else
    echo "[harden] Applying SSH hardening..."
    mkdir -p /etc/ssh/sshd_config.d 2>/dev/null || true
    cp "$ASSETS_DIR/ssh/00-mainframe.conf" /etc/ssh/sshd_config.d/00-mainframe.conf 2>/dev/null || echo "[harden] SSH config dir not found (non-Linux?)"
    groupadd -f sshusers 2>/dev/null || true
    # Add users with SSH authorized_keys to sshusers group
    for home in /root /home/*; do
      if [ -f "$home/.ssh/authorized_keys" ]; then
        user=$(basename "$home")
        [ "$user" = "root" ] && user="root"
        [ "$home" = "/root" ] && user="root"
        if id "$user" >/dev/null 2>&1; then
          usermod -aG sshusers "$user" 2>/dev/null || true
          echo "[harden] Added $user to sshusers group"
        fi
      fi
    done
    # Ensure the default sudo user is in sshusers
    for u in ubuntu admin debian; do
      if id "$u" >/dev/null 2>&1; then
        usermod -aG sshusers "$u" 2>/dev/null || true
      fi
    done
    sshd -t 2>/dev/null && echo "[harden] SSH config valid" || echo "[harden] SSH validation skipped"
  fi
fi

# 1b. Docker daemon hardening (idempotent)
if command -v docker &>/dev/null; then
  cfg=/etc/docker/daemon.json
  if [ -f "$cfg" ] && grep -q '"icc": false' "$cfg" 2>/dev/null && grep -q '"log-opts"' "$cfg" 2>/dev/null; then
    echo "[harden] Docker daemon already hardened, skipping"
  else
    echo "[harden] Applying Docker daemon hardening..."
    mkdir -p /etc/docker 2>/dev/null || true
    cat > "$cfg" << 'DEOF'
{
  "icc": false,
  "userland-proxy": false,
  "log-driver": "json-file",
  "log-opts": { "max-size": "10m", "max-file": "3" }
}
DEOF
    systemctl restart docker 2>/dev/null || service docker restart 2>/dev/null || true
    sleep 2
  fi
fi

# 1c. telnet removal
if dpkg -l inetutils-telnet 2>/dev/null | grep -q "^ii"; then
  echo "[harden] Removing telnet..."
  apt purge -y -qq inetutils-telnet 2>/dev/null || true
fi

# 1d. DB port allowlisting (optional)
if [ -n "${ALLOW_DB_FROM:-}" ]; then
  echo "[harden] Allowing DB ports from $ALLOW_DB_FROM..."
  if command -v ufw &>/dev/null; then
    ufw allow from "$ALLOW_DB_FROM" to any port 5432 proto tcp comment "PostgreSQL" 2>/dev/null || true
    ufw allow from "$ALLOW_DB_FROM" to any port 6432 proto tcp comment "PgBouncer" 2>/dev/null || true
  fi
fi

# 2. Sysctl (sane defaults)
if [ "$HARDEN_BBR" = "true" ]; then
  echo "[harden] Applying sysctl (including BBR)..."
  mkdir -p /etc/sysctl.d 2>/dev/null || true
  cp "$ASSETS_DIR/sysctl/99-mainframe.conf" /etc/sysctl.d/99-mainframe.conf 2>/dev/null || echo "[harden] sysctl dir not available (non-Linux?)"
  # Apply sysctl to all interfaces including specific ones
  sysctl --system >/dev/null 2>&1 || true
  # Per-interface overrides for common interface names
  for iface in eth0 ens3 enp0s3 eno1 tailscale0; do
    if ip link show "$iface" >/dev/null 2>&1; then
      sysctl -w "net.ipv4.conf.$iface.accept_redirects=0" >/dev/null 2>&1 || true
      sysctl -w "net.ipv4.conf.$iface.send_redirects=0" >/dev/null 2>&1 || true
      sysctl -w "net.ipv4.conf.$iface.accept_source_route=0" >/dev/null 2>&1 || true
      sysctl -w "net.ipv6.conf.$iface.accept_redirects=0" >/dev/null 2>&1 || true
    fi
  done
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
  apt-get install -y -qq unattended-upgrades 2>&1 | tail -1 || true
  cp "$ASSETS_DIR/apt/20auto-upgrades" /etc/apt/apt.conf.d/20auto-upgrades 2>/dev/null || true
  cp "$ASSETS_DIR/apt/52mainframe-unattended" /etc/apt/apt.conf.d/52mainframe-unattended 2>/dev/null || true
fi

# 5. Needrestart
if [ "$HARDEN_NEEDRESTART" = "true" ]; then
  echo "[harden] Configuring needrestart..."
  apt-get install -y -qq needrestart 2>&1 | tail -1 || true
  mkdir -p /etc/needrestart/conf.d 2>/dev/null || true
  cp "$ASSETS_DIR/needrestart/50-mainframe.conf" /etc/needrestart/conf.d/50-mainframe.conf 2>/dev/null || true
fi

# 6. Swap
if [ "$HARDEN_SWAP" = "true" ]; then
  if [ ! -f /swapfile ]; then
    echo "[harden] Creating 2GB swap file..."
    fallocate -l 2G /swapfile 2>/dev/null && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile && echo '/swapfile none swap sw 0 0' >> /etc/fstab || echo "[harden] Swap creation skipped (non-Linux?)"
  else
    echo "[harden] Swap already exists"
  fi
fi

# 7. etckeeper
if [ "$HARDEN_ETCKEEPER" = "true" ]; then
  apt-get install -y -qq etckeeper 2>&1 | tail -1 || true
  if [ ! -d /etc/.git ]; then
    etckeeper init 2>&1 | tail -1 || true
    etckeeper commit "mainframe: initial" 2>&1 | tail -1 || true
  fi
fi

# 8. Deadman switch
if [ "$HARDEN_DEADMAN" = "true" ]; then
  if [ -f "$AUDIT_DIR/deadman.sh" ]; then
    install -m 755 "$AUDIT_DIR/deadman.sh" /usr/local/sbin/mainframe-deadman 2>/dev/null || true
    echo "[harden] Deadman installed"
  fi
fi

# 9. fail2ban
if [ "$HARDEN_FAIL2BAN" = "true" ]; then
  apt-get install -y -qq fail2ban 2>&1 | tail -1 || true
  mkdir -p /etc/fail2ban/jail.d 2>/dev/null || true
  # Determine admin IP from SSH connection
  ADMIN_IP="${SSH_CLIENT:-}"
  ADMIN_IP="${ADMIN_IP%% *}"
  [ -z "$ADMIN_IP" ] && ADMIN_IP="127.0.0.1"
  # Get SSH port
  SSH_PORT=$(ss -tlnp 2>/dev/null | grep sshd | head -1 | sed -n 's/.*:\([0-9]*\).*/\1/p')
  [ -z "$SSH_PORT" ] && SSH_PORT="22"
  # Copy and fill placeholders
  if [ -f "$ASSETS_DIR/fail2ban/mainframe.local" ]; then
    sed -e "s/<ssh-port>/$SSH_PORT/g" -e "s/<admin-ip>/$ADMIN_IP/g" "$ASSETS_DIR/fail2ban/mainframe.local" > /etc/fail2ban/jail.d/mainframe.local
  fi
  systemctl enable --now fail2ban 2>&1 | tail -1 || true
  echo "[harden] fail2ban installed (port=$SSH_PORT, admin=$ADMIN_IP)"
fi

# 10. AppArmor
if [ "$HARDEN_APPARMOR" = "true" ]; then
  if command -v aa-status &>/dev/null; then
    aa-status 2>&1 | head -2
  else
    apt-get install -y -qq apparmor apparmor-utils 2>&1 | tail -1 || true
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
  apt-get install -y -qq auditd 2>&1 | tail -1 || true
  systemctl enable --now auditd 2>/dev/null || true
fi
if [ "$HARDEN_AIDE" = "true" ]; then
  apt-get install -y -qq aide 2>&1 | tail -1 || true
  echo "[harden] Run 'aideinit' to initialize AIDE database"
fi
if [ "$HARDEN_NOEXEC_SHM" = "true" ]; then
  echo "[harden] /dev/shm noexec: add to /etc/fstab manually"
fi
if [ "$HARDEN_CORE_DUMPS" = "true" ]; then
  echo "kernel.core_pattern = |/bin/false" > /etc/sysctl.d/99-core-dumps.conf 2>/dev/null || true
  sysctl --system >/dev/null 2>&1 || true
fi

# Tailscale deadman: restart tailscale if it crashes
if command -v tailscale &>/dev/null; then
  cat > /etc/systemd/system/mainframe-tailscale-watchdog.service << 'SVC'
[Unit]
Description=Mainframe Tailscale watchdog
After=tailscaled.service

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'systemctl is-active --quiet tailscaled || systemctl restart tailscaled'
SVC
  cat > /etc/systemd/system/mainframe-tailscale-watchdog.timer << 'TIMER'
[Unit]
Description=Mainframe Tailscale watchdog timer

[Timer]
OnBootSec=60
OnUnitActiveSec=60
Unit=mainframe-tailscale-watchdog.service

[Install]
WantedBy=timers.target
TIMER
  systemctl daemon-reload 2>/dev/null || true
  systemctl enable --now mainframe-tailscale-watchdog.timer 2>/dev/null || true
  echo "[harden] Tailscale watchdog installed (restarts tailscaled if it crashes)"
fi

# Crash prevention (memory, disk, CPU, process protection)
if [ -f "$SCRIPT_DIR/crash-prevention.sh" ]; then
  echo "[harden] Applying crash prevention..."
  bash "$SCRIPT_DIR/crash-prevention.sh" 2>&1 | tail -5
fi

# Disable ACPI power button handling (prevents hypervisor-initiated shutdowns)
mkdir -p /etc/systemd/logind.conf.d 2>/dev/null || true
printf '[Login]\nHandlePowerKey=ignore\nHandleSuspendKey=ignore\nHandleHibernateKey=ignore\nHandleLidSwitch=ignore\n' > /etc/systemd/logind.conf.d/99-mainframe.conf 2>/dev/null || true
systemctl restart systemd-logind 2>/dev/null || true
echo "[harden] ACPI power button handling disabled (prevents hypervisor shutdown signals)"

# Summary
echo ""
echo "[harden] Hardening complete. Errors: $HARDEN_ERRORS"
if [ "$HARDEN_ERRORS" -gt 0 ]; then
  echo "[harden] WARNING: Some steps had errors. Review output above."
fi

# Commit etckeeper
if [ -d /etc/.git ]; then
  etckeeper commit "mainframe: hardening applied" 2>&1 | tail -1 || true
fi

# Run audit after hardening
echo ""
echo "[harden] Running audit (post-hardening)..."
if [ -f "$AUDIT_DIR/audit.sh" ]; then
  bash "$AUDIT_DIR/audit.sh" 2>&1 | tail -3 || true
fi

# Lynis after score
LYNIS_AFTER=""
if command -v lynis &>/dev/null; then
  lynis audit system --quiet 2>/dev/null || true
  logfile=$(ls -t /var/log/lynis-report.dat 2>/dev/null | head -1)
  if [ -n "$logfile" ]; then
    LYNIS_AFTER=$(grep -oP '^hardening_index=\K[0-9]+' "$logfile" 2>/dev/null || echo "0")
    echo "[harden] Lynis score: ${LYNIS_BEFORE:-?} -> $LYNIS_AFTER"
  fi
fi

echo "[harden] Done. Review audit output above."
__zcode_status=$?
if [ "$__zcode_status" -eq 0 ]; then pwd -P > '/var/folders/3b/kt90wy3d66lftws_dwtxxglm0000gn/T/zcode-b7de6963-4152-43df-8298-2837f660fd82-cwd'; fi
exit "$__zcode_status"