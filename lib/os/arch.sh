#!/usr/bin/env bash
# Arch Linux OS module (pacman-based)
# TODO: Implement all functions.
set -euo pipefail

install_packages() { echo "[arch] Not implemented." >&2; return 1; }
install_docker() { pacman -S --noconfirm docker && systemctl enable --now docker; }
install_go() { echo "[arch] Not implemented." >&2; return 1; }
install_node() { echo "[arch] Not implemented." >&2; return 1; }
install_python() { echo "[arch] Not implemented." >&2; return 1; }
install_helm() { echo "[arch] Not implemented." >&2; return 1; }
install_gh() { echo "[arch] Not implemented." >&2; return 1; }
install_bw() { echo "[arch] Not implemented." >&2; return 1; }
install_osv_scanner() { echo "[arch] Not implemented." >&2; return 1; }
install_postgresql_client() { pacman -S --noconfirm postgresql; }

echo "[arch] OS module loaded (partial)"