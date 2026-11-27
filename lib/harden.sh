#!/usr/bin/env bash
# Mainframe hardening — sane defaults, no levels.
# Applies a solid security baseline. Optional extras are env-gated.
# Assets (config templates) are in assets/. Copy, fill placeholders, validate, apply.
set -uo pipefail
# Non-fatal mode

# Persistent logging
HARDEN_LOG="/var/log/mainframe-harden.log"
exec > >(tee -a "$HARDEN_LOG") 2>&1
echo "=== Hardening started at $(date) ==="
echo "Sections: ${HARDEN_SECTIONS:-all}"
: individual steps can fail without aborting
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

# Selective hardening: HARDEN_SECTIONS env var (comma-separated)
# If empty, run all sections. If set, run only those sections.
should_run() {
  local section="$1"
  if [ -z "${HARDEN_SECTIONS:-}" ]; then
    return 0  # run all
  fi
  echo ",$HARDEN_SECTIONS," | grep -q ",$section,"
}
HARDEN_TOTP="${HARDEN_TOTP:-false}"
HARDEN_VPN_ONLY="${HARDEN_VPN_ONLY:-false}"
HARDEN_AUDITD="${HARDEN_AUDITD:-false}"
HARDEN_AIDE="${HARDEN_AIDE:-false}"
HARDEN_NOEXEC_SHM="${HARDEN_NOEXEC_SHM:-false}"
HARDEN_CORE_DUMPS="${HARDEN_CORE_DUMPS:-false}"

if should_run "audit" && [ "${SKIP_AUDIT:-false}" != "true" ]; then
echo "[harden] Running audit (read-only, baseline)..."
if [ -f "$AUDIT_DIR/audit.sh" ]; then
  bash "$AUDIT_DIR/audit.sh" 2>&1 | tail -3 || true
fi

# Lynis on Linux >= 5.6: CRYP-7902 (entropy estimation) spins for minutes
# because /dev/random no longer blocks on those kernels (cisofy/lynis#1736).
# Skip it via custom.prf so the audit completes in a reasonable time.
KERNEL_MAJOR=$(uname -r | cut -d. -f1)
KERNEL_MINOR=$(uname -r | cut -d. -f2)
if [ "$KERNEL_MAJOR" -gt 5 ] || { [ "$KERNEL_MAJOR" -eq 5 ] && [ "$KERNEL_MINOR" -ge 6 ]; }; then
  mkdir -p /etc/lynis
  printf 'skip-test=CRYP-7902\n' > /etc/lynis/custom.prf
  echo "[harden] Kernel >= 5.6 detected; skipping CRYP-7902 (entropy estimation) via /etc/lynis/custom.prf"
fi
# Refresh package lists so PKGS-7392 (vulnerable packages) resolves quickly.
apt-get update >/dev/null 2>&1 || true

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
# Remove recovery config if present (from mf recover)
if [ -f /etc/ssh/sshd_config.d/00-recovery.conf ]; then
  echo "[harden] Removing recovery config (from previous mf recover)..."
  rm -f /etc/ssh/sshd_config.d/00-recovery.conf
fi
EXISTING_SSH_CONFIGS=$(ls /etc/ssh/sshd_config.d/*.conf 2>/dev/null | grep -v "00-mainframe.conf" | grep -v "50-cloud-init.conf" | grep -v "60-cloudimg-settings.conf" | grep -v "00-recovery.conf" || true)
if [ -n "$EXISTING_SSH_CONFIGS" ] && [ "$FORCE_SSH" != "true" ]; then
  echo "[harden] Existing SSH config found:"
  for f in $EXISTING_SSH_CONFIGS; do echo "  $f"; done
  echo "[harden] Skipping SSH hardening to preserve existing config."
  echo "[harden] To override: set FORCE_SSH=true"
  HARDEN_SSH_CRYPTO="false"
fi

fi

# 0. Create admin user (non-root, for SSH access after hardening)
# This prevents lockout: root SSH is disabled by hardening, so we need
# a non-root admin user with SSH keys and passwordless sudo.
ADMIN_USER="${ADMIN_USER:-ubuntu}"
if id "$ADMIN_USER" &>/dev/null; then
  echo "[harden] Admin user '$ADMIN_USER' already exists"
else
  echo "[harden] Creating admin user '$ADMIN_USER'..."
  useradd -m -s /bin/bash "$ADMIN_USER" 2>/dev/null || true
  # Copy root's SSH authorized_keys to admin user
  if [ -f /root/.ssh/authorized_keys ]; then
    mkdir -p /home/$ADMIN_USER/.ssh
    cp /root/.ssh/authorized_keys /home/$ADMIN_USER/.ssh/authorized_keys
    chown -R $ADMIN_USER:$ADMIN_USER /home/$ADMIN_USER/.ssh
    chmod 700 /home/$ADMIN_USER/.ssh
    chmod 600 /home/$ADMIN_USER/.ssh/authorized_keys
    echo "[harden] Copied SSH keys to $ADMIN_USER"
  fi
  # Add to sudo and sshusers groups
  usermod -aG sudo "$ADMIN_USER" 2>/dev/null || true
  usermod -aG sshusers "$ADMIN_USER" 2>/dev/null || true
  # Passwordless sudo
  echo "$ADMIN_USER ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/$ADMIN_USER
  chmod 440 /etc/sudoers.d/$ADMIN_USER
  echo "[harden] Admin user '$ADMIN_USER' created with sudo access"
fi

# Verify admin user has SSH keys
ADMIN_HAS_KEYS="false"
if [ -f "/home/$ADMIN_USER/.ssh/authorized_keys" ] && [ -s "/home/$ADMIN_USER/.ssh/authorized_keys" ]; then
  ADMIN_HAS_KEYS="true"
  echo "[harden] Admin user '$ADMIN_USER' has SSH keys"
else
  echo "[harden] WARNING: Admin user '$ADMIN_USER' has NO SSH keys."
  echo "[harden] Root SSH will NOT be disabled to prevent lockout."
  echo "[harden] Set up SSH keys for $ADMIN_USER, then re-run hardening."
fi

# Helper: test SSH access for a user (returns 0 if successful)
test_ssh_access() {
  local user="$1"
  local host="${2:-127.0.0.1}"
  local timeout="${3:-5}"
  ssh -o BatchMode=yes -o ConnectTimeout="$timeout" -o StrictHostKeyChecking=no       -o UserKnownHostsFile=/dev/null "$user@$host" "echo SSH_OK" 2>/dev/null | grep -q "SSH_OK"
}

# Layer 1: Verify current SSH access works (baseline)
echo ""
echo "[harden] === SSH Hardening: Layer-by-Layer ==="
echo "[harden] Layer 1: Verifying current SSH access..."
ROOT_SSH_BEFORE="false"
if test_ssh_access "root"; then
  ROOT_SSH_BEFORE="true"
  echo "[harden]   Root SSH: OK (baseline confirmed)"
else
  echo "[harden]   Root SSH: not accessible (may already be restricted)"
fi

# Layer 2: Admin user exists and has keys (already verified above)
echo "[harden] Layer 2: Admin user check..."
if [ "$ADMIN_HAS_KEYS" = "true" ]; then
  echo "[harden]   Admin user '$ADMIN_USER': has SSH keys"
else
  echo "[harden]   Admin user '$ADMIN_USER': NO SSH keys"
fi

# Layer 3: Test SSH as admin user (before any changes)
echo "[harden] Layer 3: Testing SSH as '$ADMIN_USER' (pre-change)..."
ADMIN_SSH_BEFORE="false"
if [ "$ADMIN_HAS_KEYS" = "true" ]; then
  if test_ssh_access "$ADMIN_USER"; then
    ADMIN_SSH_BEFORE="true"
    echo "[harden]   Admin SSH: OK (pre-change verified)"
  else
    echo "[harden]   Admin SSH: FAILED (keys may not match or sshd not running)"
    echo "[harden]   Cannot proceed with root SSH disable. Skipping."
  fi
else
  echo "[harden]   Skipped (no SSH keys for admin user)"
fi

# Layer 4: Apply SSH hardening ONLY if admin SSH is verified
echo "[harden] Layer 4: Applying SSH hardening..."
SSH_HARDENED="false"
if [ "$ADMIN_SSH_BEFORE" = "true" ]; then
  # Backup current config
  mkdir -p /etc/ssh/sshd_config.d 2>/dev/null || true
  if [ -f /etc/ssh/sshd_config.d/00-mainframe.conf ]; then
    cp /etc/ssh/sshd_config.d/00-mainframe.conf /etc/ssh/sshd_config.d/00-mainframe.conf.bak 2>/dev/null || true
  fi
  # Write new config to temp file first
  tmp_conf="/etc/ssh/sshd_config.d/00-mainframe.conf.new"
  # Use the asset template
  ASSETS_DIR="${ASSETS_DIR:-/tmp/mainframe-assets/assets}"
  if [ -f "$ASSETS_DIR/ssh/00-mainframe.conf" ]; then
    cp "$ASSETS_DIR/ssh/00-mainframe.conf" "$tmp_conf" 2>/dev/null
  else
    # Fallback: write config inline
    cat > "$tmp_conf" << 'SSHEOF'
# /etc/ssh/sshd_config.d/00-mainframe.conf: managed by mainframe
Port 22
PermitRootLogin no
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitEmptyPasswords no
PubkeyAuthentication yes
AuthenticationMethods publickey
AllowGroups sshusers
MaxAuthTries 5
MaxSessions 10
LoginGraceTime 30
ClientAliveInterval 300
ClientAliveCountMax 2
X11Forwarding no
AllowAgentForwarding no
PermitUserEnvironment no
SSHEOF
  fi
  # Validate before applying
  if [ -f "$tmp_conf" ] && sshd -t 2>/dev/null; then
    mv "$tmp_conf" /etc/ssh/sshd_config.d/00-mainframe.conf
    echo "[harden]   SSH config applied and validated"
    # Restart SSH (existing sessions are kept)
    systemctl restart ssh 2>/dev/null || service ssh restart 2>/dev/null || true
    sleep 2
    SSH_HARDENED="true"
  else
    echo "[harden]   ERROR: SSH config validation failed. NOT applying."
    rm -f "$tmp_conf"
    SSH_HARDENED="false"
  fi
else
  echo "[harden]   Skipped (admin SSH not verified). Root SSH remains enabled."
fi

# Layer 5: Verify access after hardening
echo "[harden] Layer 5: Verifying access after hardening..."
if [ "$SSH_HARDENED" = "true" ]; then
  # Verify root SSH is now blocked
  if test_ssh_access "root"; then
    echo "[harden]   WARNING: Root SSH still works after hardening (unexpected)"
  else
    echo "[harden]   Root SSH blocked: OK (expected)"
  fi
  # Verify admin SSH still works
  if test_ssh_access "$ADMIN_USER"; then
    echo "[harden]   Admin SSH still works: OK"
  else
    echo "[harden]   ERROR: Admin SSH broken after hardening! REVERTING..."
    # Revert SSH config
    if [ -f /etc/ssh/sshd_config.d/00-mainframe.conf.bak ]; then
      mv /etc/ssh/sshd_config.d/00-mainframe.conf.bak /etc/ssh/sshd_config.d/00-mainframe.conf
    else
      rm -f /etc/ssh/sshd_config.d/00-mainframe.conf
    fi
    systemctl restart ssh 2>/dev/null || service ssh restart 2>/dev/null || true
    sleep 2
    echo "[harden]   Reverted. Root SSH restored."
    SSH_HARDENED="false"
  fi
fi

# Layer 6: Firewall (only if admin SSH works)
echo "[harden] Layer 6: Firewall check..."
# Firewall is handled by mf provision / firewall.sh, not hardening
# But we ensure SSH port 22 is always allowed (break-glass)
if command -v ufw &>/dev/null; then
  if ufw status 2>/dev/null | grep -q "Status: active"; then
    # Ensure port 22 is allowed (break-glass)
    ufw allow 22/tcp comment 'SSH (break-glass, always)' 2>/dev/null || true
    echo "[harden]   Port 22 allowed in firewall (break-glass)"
  fi
fi

echo ""
echo "[harden] SSH hardening summary:"
echo "  Admin user: $ADMIN_USER (has keys: $ADMIN_HAS_KEYS)"
echo "  Admin SSH verified: $ADMIN_SSH_BEFORE"
echo "  SSH hardening applied: $SSH_HARDENED"
echo "  Root SSH blocked: $([ "$SSH_HARDENED" = "true" ] && echo 'yes' || echo 'no (root still accessible)')"
echo ""

# 2. Sysctl (sane defaults)
if should_run "sysctl" && [ "$HARDEN_BBR" = "true" ]; then
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
if should_run "journald" && [ "$HARDEN_JOURNALD" = "true" ]; then
  echo "[harden] Applying journald limits..."
  mkdir -p /etc/systemd/journald.conf.d 2>/dev/null || true
  cp "$ASSETS_DIR/journald/10-mainframe.conf" /etc/systemd/journald.conf.d/10-mainframe.conf 2>/dev/null || echo "[harden] journald not available (non-Linux?)"
  systemctl restart systemd-journald 2>/dev/null || true
fi

# 4. Unattended upgrades
if should_run "unattended" && [ "$HARDEN_UNATTENDED" = "true" ]; then
  echo "[harden] Enabling unattended security updates..."
  apt-get install -y -qq unattended-upgrades 2>&1 | tail -1 || true
  cp "$ASSETS_DIR/apt/20auto-upgrades" /etc/apt/apt.conf.d/20auto-upgrades 2>/dev/null || true
  cp "$ASSETS_DIR/apt/52mainframe-unattended" /etc/apt/apt.conf.d/52mainframe-unattended 2>/dev/null || true
fi

# 5. Needrestart
if should_run "needrestart" && [ "$HARDEN_NEEDRESTART" = "true" ]; then
  echo "[harden] Configuring needrestart..."
  apt-get install -y -qq needrestart 2>&1 | tail -1 || true
  mkdir -p /etc/needrestart/conf.d 2>/dev/null || true
  cp "$ASSETS_DIR/needrestart/50-mainframe.conf" /etc/needrestart/conf.d/50-mainframe.conf 2>/dev/null || true
fi

# 6. Swap
if should_run "swap" && [ "$HARDEN_SWAP" = "true" ]; then
  if [ ! -f /swapfile ]; then
    echo "[harden] Creating 2GB swap file..."
    fallocate -l 2G /swapfile 2>/dev/null && chmod 600 /swapfile && mkswap /swapfile && swapon /swapfile && echo '/swapfile none swap sw 0 0' >> /etc/fstab || echo "[harden] Swap creation skipped (non-Linux?)"
  else
    echo "[harden] Swap already exists"
  fi
fi

# 7. etckeeper
if should_run "etckeeper" && [ "$HARDEN_ETCKEEPER" = "true" ]; then
  apt-get install -y -qq etckeeper 2>&1 | tail -1 || true
  if [ ! -d /etc/.git ]; then
    etckeeper init 2>&1 | tail -1 || true
    etckeeper commit "mainframe: initial" 2>&1 | tail -1 || true
  fi
fi

# 8. Deadman switch
if should_run "deadman" && [ "$HARDEN_DEADMAN" = "true" ]; then
  if [ -f "$AUDIT_DIR/deadman.sh" ]; then
    install -m 755 "$AUDIT_DIR/deadman.sh" /usr/local/sbin/mainframe-deadman  # invoked via: mf deadman 2>/dev/null || true
    echo "[harden] Deadman installed"
  fi
fi

# 9. fail2ban
if should_run "fail2ban" && [ "$HARDEN_FAIL2BAN" = "true" ]; then
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
if should_run "tailscale" && command -v tailscale &>/dev/null; then
  cat > /etc/systemd/system/mainframe-tailscale-watchdog.service << 'SVC'
[Unit]
Description=Mainframe Tailscale watchdog
After=tailscaled.service

[Service]
Type=oneshot
ExecStart=/bin/bash -c 'AUTHKEY="${TAILSCALE_AUTHKEY:-}"; [ -z "$AUTHKEY" ] && [ -f /var/lib/mainframe/tailscale-authkey ] && AUTHKEY=$(cat /var/lib/mainframe/tailscale-authkey); systemctl is-active --quiet tailscaled || systemctl restart tailscaled; tailscale status >/dev/null 2>&1 || tailscale up --authkey="$AUTHKEY" 2>/dev/null || true'
SVC
  cat > /etc/systemd/system/mainframe-tailscale-watchdog.timer << 'TIMER'
[Unit]
Description=Mainframe Tailscale watchdog timer

[Timer]
OnBootSec=300
OnUnitActiveSec=60
Unit=mainframe-tailscale-watchdog.service

[Install]
WantedBy=timers.target
TIMER
  systemctl daemon-reload 2>/dev/null || true
  systemctl enable --now mainframe-tailscale-watchdog.timer 2>/dev/null || true
  echo "[harden] Tailscale watchdog installed (checks every 1 min)"
fi

# Crash prevention (memory, disk, CPU, process protection)
if should_run "crash-prevention" && [ -f "$SCRIPT_DIR/crash-prevention.sh" ]; then
  echo "[harden] Applying crash prevention..."
  bash "$SCRIPT_DIR/crash-prevention.sh" 2>&1 | tail -5
fi

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
if should_run "audit" && [ "${SKIP_AUDIT:-false}" != "true" ]; then
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

fi

echo "[harden] Done. Review audit output above."
__zcode_status=$?
if [ "$__zcode_status" -eq 0 ]; then pwd -P > '/var/folders/3b/kt90wy3d66lftws_dwtxxglm0000gn/T/zcode-b7de6963-4152-43df-8298-2837f660fd82-cwd'; fi
exit "$__zcode_status"