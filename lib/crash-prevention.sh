#!/usr/bin/env bash
# Crash prevention — memory, disk, CPU, and process protection.
# Prevents the machine from crashing due to resource exhaustion.
set -uo pipefail

echo "[crash-prevention] Applying crash prevention measures..."

# --- 1. OOM protection for critical services ---
echo "[crash-prevention] Protecting critical services from OOM killer..."

for svc in ssh sshd tailscaled docker containerd; do
  if systemctl list-unit-files "$svc.service" >/dev/null 2>&1; then
    mkdir -p "/etc/systemd/system/$svc.service.d" 2>/dev/null || true
    printf '[Service]\nOOMScoreAdjust=-1000\nRestart=always\nRestartSec=5\n' > "/etc/systemd/system/$svc.service.d/oom-protection.conf"
    systemctl daemon-reload 2>/dev/null || true
    echo "[crash-prevention] OOM protection enabled for $svc"
  fi
done

# --- 2. Early OOM killer ---
if ! command -v earlyoom >/dev/null 2>&1; then
  echo "[crash-prevention] Installing earlyoom..."
  apt-get install -y -qq earlyoom 2>&1 | tail -1 || true
fi
if command -v earlyoom >/dev/null 2>&1; then
  # Configure earlyoom to avoid killing critical services
  cat > /etc/default/earlyoom << 'EOF'
EARLYOOM_ARGS="-r 60 --avoid '(^|/)(init|sshd|tailscaled|dockerd|containerd)$'"
EOF
  systemctl restart earlyoom 2>/dev/null || true
  systemctl enable --now earlyoom 2>/dev/null || true
  echo "[crash-prevention] earlyoom enabled (avoids sshd, tailscaled, docker)"
fi

# --- 3. Docker cleanup cron ---
mkdir -p /etc/cron.daily 2>/dev/null || true
cat > /etc/cron.daily/docker-cleanup << 'CRONEOF'
#!/bin/bash
docker system prune -f --filter "until=24h" 2>/dev/null || true
CRONEOF
chmod +x /etc/cron.daily/docker-cleanup
echo "[crash-prevention] Docker cleanup cron installed"

# --- 4. Log rotation ---
mkdir -p /etc/logrotate.d 2>/dev/null || true
cat > /etc/logrotate.d/mainframe << 'LOGEOF'
/var/log/mainframe*.log {
    daily
    rotate 7
    compress
    missingok
    notifempty
    create 0644 root root
}
LOGEOF
echo "[crash-prevention] Log rotation configured"

# --- 5. Temp file cleanup ---
mkdir -p /etc/tmpfiles.d 2>/dev/null || true
printf 'd /tmp 1777 root root 7d\nd /var/tmp 1777 root root 30d\n' > /etc/tmpfiles.d/mainframe.conf
echo "[crash-prevention] Temp file cleanup configured"

# --- 6. Swap protection ---
if ! swapon --show 2>/dev/null | grep -q .; then
  if [ -f /swapfile ]; then
    swapon /swapfile 2>/dev/null || true
  fi
fi
sysctl -w vm.swappiness=10 >/dev/null 2>&1 || true
echo "vm.swappiness = 10" > /etc/sysctl.d/99-swappiness.conf
echo "[crash-prevention] Swap protection configured (swappiness=10)"

# --- 7. Process limits ---
mkdir -p /etc/security/limits.d 2>/dev/null || true
printf '* soft nproc 4096\n* hard nproc 8192\n* soft nofile 65536\n* hard nofile 65536\n' > /etc/security/limits.d/99-mainframe.conf
echo "[crash-prevention] Process limits configured"

# --- 8. Resource monitoring ---
cat > /usr/local/bin/mainframe-resource-monitor << 'MONEOF'
#!/bin/bash
MEM_PCT=$(free 2>/dev/null | awk '/Mem:/ {printf "%.0f", $3/$2 * 100}')
DISK_PCT=$(df / 2>/dev/null | awk 'NR==2 {gsub(/%/,""); print $5}')
CPU_LOAD=$(uptime 2>/dev/null | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',')
CPU_CORES=$(nproc 2>/dev/null || echo 1)

if [ -n "$MEM_PCT" ] && [ "$MEM_PCT" -gt 90 ] 2>/dev/null; then
  echo "WARNING: Memory usage at ${MEM_PCT}%"
  ps aux --sort=-%mem 2>/dev/null | head -6
fi

if [ -n "$DISK_PCT" ] && [ "$DISK_PCT" -gt 85 ] 2>/dev/null; then
  echo "WARNING: Disk usage at ${DISK_PCT}%"
  if [ "$DISK_PCT" -gt 95 ]; then
    echo "CRITICAL: Cleaning up disk space..."
    docker system prune -f 2>/dev/null || true
    journalctl --vacuum-size=100M 2>/dev/null || true
    apt-get clean 2>/dev/null || true
  fi
fi

CPU_INT=$(echo "$CPU_LOAD" | cut -d. -f1)
if [ -n "$CPU_INT" ] && [ "$CPU_INT" -gt "$CPU_CORES" ] 2>/dev/null; then
  echo "WARNING: CPU load ($CPU_LOAD) exceeds cores ($CPU_CORES)"
fi
MONEOF
chmod +x /usr/local/bin/mainframe-resource-monitor

# Add cron job
if ! crontab -l 2>/dev/null | grep -q "mainframe-resource-monitor"; then
  (crontab -l 2>/dev/null || true; echo "*/5 * * * * /usr/local/bin/mainframe-resource-monitor") | crontab - 2>/dev/null || true
  echo "[crash-prevention] Resource monitor installed (checks every 5 minutes)"
fi

# Shutdown logger (capture details before any shutdown)
cat > /usr/local/bin/mainframe-shutdown-logger << 'LOGGER'
#!/bin/bash
LOGFILE=/var/log/mainframe-shutdown.log
TS=$(date '+%Y-%m-%d %H:%M:%S')
{
  echo "========================================"
  echo "SHUTDOWN EVENT at $TS"
  echo "========================================"
  echo "Uptime: $(uptime)"
  echo "Load: $(cat /proc/loadavg 2>/dev/null || echo 'N/A')"
  echo "Memory: $(free -h 2>/dev/null | head -2 | tail -1 || echo 'N/A')"
  echo "Disk: $(df -h / | tail -1)"
  echo ""
  echo "--- Triggering process ---"
  systemctl list-jobs 2>/dev/null | head -10
  echo ""
  echo "--- Recent journal (last 2 min) ---"
  journalctl --since "2 minutes ago" 2>/dev/null | tail -30
  echo ""
  echo "--- ACPI events ---"
  dmesg 2>/dev/null | grep -i "acpi\|power\|shutdown" | tail -5
  echo ""
  echo "--- Running processes (top 10 by CPU) ---"
  ps aux --sort=-%cpu 2>/dev/null | head -11
  echo ""
  echo "--- System info ---"
  echo "Hostname: $(hostname)"
  echo "IP: $(hostname -I 2>/dev/null || hostname -i)"
  echo "========================================"
  echo ""
} >> "$LOGFILE"
logger -t mainframe-shutdown "Shutdown triggered at $TS"
LOGGER
chmod +x /usr/local/bin/mainframe-shutdown-logger

cat > /etc/systemd/system/mainframe-shutdown-logger.service << 'SVC'
[Unit]
Description=Mainframe Shutdown Logger
DefaultDependencies=no
Before=shutdown.target reboot.target halt.target poweroff.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/mainframe-shutdown-logger
RemainAfterExit=yes
TimeoutStartSec=10

[Install]
WantedBy=multi-user.target
SVC
systemctl daemon-reload 2>/dev/null || true
systemctl enable mainframe-shutdown-logger.service 2>/dev/null || true

# Install acpid for ACPI event logging
if ! command -v acpid >/dev/null 2>&1; then
  apt-get install -y -qq acpid 2>&1 | tail -1 || true
fi
if command -v acpid >/dev/null 2>&1; then
  printf 'event=button/power\naction=/usr/local/bin/mainframe-shutdown-logger\n' > /etc/acpi/events/powerbtn 2>/dev/null || true
  systemctl enable --now acpid 2>/dev/null || true
fi

echo "[crash-prevention] Shutdown logger installed (captures details before any shutdown)"

# OS idle/sleep prevention (prevents systemd from suspending the system)
# This is the #1 cause of "VPS stops when idle" issues on Ubuntu.
systemctl mask sleep.target suspend.target hibernate.target hybrid-sleep.target 2>/dev/null || true
echo "[crash-prevention] Disabled systemd sleep/suspend/hibernate (prevents idle shutdown)"

# SSH keepalive (prevent connection drops on idle)
if ! grep -q "ClientAliveInterval" /etc/ssh/sshd_config 2>/dev/null; then
  echo "ClientAliveInterval 60" >> /etc/ssh/sshd_config
  echo "ClientAliveCountMax 3" >> /etc/ssh/sshd_config
  systemctl restart ssh 2>/dev/null || true
  echo "[crash-prevention] SSH keepalive configured (60s interval)"
fi

echo "[crash-prevention] All crash prevention measures applied."
