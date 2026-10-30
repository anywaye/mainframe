#!/usr/bin/env bash
# Fedora/RHEL/CentOS OS module (dnf-based)
# Implements the standard OS interface.
# TODO: Implement all functions for dnf-based systems.
set -euo pipefail

install_packages() {
  echo "[fedora] Package installation not yet implemented. Install manually."
  return 0
}

install_docker() {
  if command -v docker &>/dev/null; then
    echo "[fedora] Docker already installed"
    return 0
  fi
  dnf -y install dnf-plugins-core
  dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo
  dnf install -y docker-ce docker-ce-cli containerd.io
  systemctl enable --now docker
  echo "[fedora] Docker installed"
}

install_go() { echo "[fedora] Not implemented. Install Go manually." >&2; return 1; }
install_node() { echo "[fedora] Not implemented. Install Node manually." >&2; return 1; }
install_python() { echo "[fedora] Not implemented. Install Python manually." >&2; return 1; }
install_helm() { echo "[fedora] Not implemented. Install Helm manually." >&2; return 1; }
install_gh() { echo "[fedora] Not implemented. Install gh manually." >&2; return 1; }
install_bw() { echo "[fedora] Not implemented. Install Bitwarden CLI manually." >&2; return 1; }
install_osv_scanner() { echo "[fedora] Not implemented. Install osv-scanner manually." >&2; return 1; }
install_postgresql_client() { dnf install -y postgresql 2>&1 | tail -1; }

echo "[fedora] OS module loaded (partial)"