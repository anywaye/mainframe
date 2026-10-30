#!/usr/bin/env bash
# Install packages from packages.conf
# Usage: install_from_conf <path-to-packages.conf>
set -euo pipefail

install_from_conf() {
  local conf="${1:-packages.conf}"
  if [ ! -f "$conf" ]; then
    echo "[packages] No packages.conf found at $conf. Skipping."
    return 0
  fi

  echo "[packages] Reading $conf..."

  # Collect package names (strip versions, comments, blank lines)
  local pkgs=""
  while IFS= read -r line; do
    # Strip inline comments
    line="${line%%#*}"
    # Trim whitespace
    line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [ -z "$line" ] && continue
    # Extract package name (before colon for version pins)
    local pkg="${line%%:*}"
    pkgs="$pkgs $pkg"
  done < "$conf"

  if [ -z "$pkgs" ]; then
    echo "[packages] No packages to install."
    return 0
  fi

  # Source OS module if not already loaded
  if ! type install_packages &>/dev/null; then
    echo "[packages] OS module not loaded. Run detect_os first."
    return 1
  fi

  # Install via OS module
  install_packages "$conf"
  echo "[packages] Done."
}

# If sourced directly, run with default conf
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
  source "$(dirname "${BASH_SOURCE[0]}")/detect.sh"
  detect_os
  source "$(dirname "${BASH_SOURCE[0]}")/os/$OS_FAMILY.sh"
  install_from_conf "${1:-packages.conf}"
fi