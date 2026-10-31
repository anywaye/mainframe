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
  systemctl enable --now earlyoom 2>/dev/null || true
  echo "[crash-prevention] earlyoom enabled"
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

echo "[crash-prevention] All crash prevention measures applied."
