#!/usr/bin/env bash
# Debian/Ubuntu OS module (apt-based)
# Implements the standard OS interface.
set -euo pipefail

install_docker() {
  if command -v docker &>/dev/null; then
    echo "[debian] Docker already installed"
    return 0
  fi
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
  echo "[debian] Docker installed"
}

install_go() {
  local version="${1:-1.26.6}"
  if command -v go &>/dev/null; then
    echo "[debian] Go already installed: $(go version)"
    return 0
  fi
  curl -fsSL "https://go.dev/dl/go${version}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
  export PATH=$PATH:/usr/local/go/bin
  echo "[debian] Go $version installed"
}

install_node() {
  local version="${1:-22}"
  if command -v node &>/dev/null; then
    echo "[debian] Node already installed: $(node --version)"
    return 0
  fi
  curl -fsSL "https://deb.nodesource.com/setup_${version}.x" | bash -
  apt-get install -y -qq nodejs
  echo "[debian] Node $version installed"
}

install_python() {
  if ! command -v python3 &>/dev/null; then
    apt-get install -y -qq python3 python3-venv
  fi
  if ! command -v uv &>/dev/null; then
    curl -LsSf https://astral.sh/uv/install.sh | sh
    export PATH="$HOME/.local/bin:$PATH"
  fi
  echo "[debian] Python + uv installed"
}

install_helm() {
  local version="${1:-v4.3.0}"
  if command -v helm &>/dev/null; then
    echo "[debian] Helm already installed"
    return 0
  fi
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash -s -- --version "$version"
  echo "[debian] Helm $version installed"
}

install_gh() {
  if command -v gh &>/dev/null; then
    echo "[debian] gh already installed"
    return 0
  fi
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg 2>/dev/null
  chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list
  apt-get update -qq && apt-get install -y -qq gh
  echo "[debian] gh installed"
}

install_bw() {
  if command -v bw &>/dev/null; then
    echo "[debian] bw already installed"
    return 0
  fi
  npm install -g @bitwarden/cli 2>&1 | tail -1
  echo "[debian] Bitwarden CLI installed"
}

install_osv_scanner() {
  if command -v osv-scanner &>/dev/null; then
    echo "[debian] osv-scanner already installed"
    return 0
  fi
  curl -sL -o /tmp/osv-scanner https://github.com/google/osv-scanner/releases/download/v2.7.0/osv-scanner_linux_amd64
  mv /tmp/osv-scanner /usr/local/bin/osv-scanner
  chmod +x /usr/local/bin/osv-scanner
  echo "[debian] osv-scanner installed"
}

install_postgresql_client() {
  if command -v psql &>/dev/null; then
    echo "[debian] psql already installed"
    return 0
  fi
  apt-get install -y -qq postgresql-client
  echo "[debian] psql installed"
}

echo "[debian] OS module loaded"

# Dispatch: install a single package (with optional version)
install_one() {
  local entry="$1"
  local pkg="${entry%%:*}"
  local ver="${entry#*:}"
  [ "$ver" = "$entry" ] && ver=""
  case "$pkg" in
    docker) install_docker ;;
    go) install_go "${ver:-1.26.6}" ;;
    node) install_node "${ver:-22}" ;;
    helm) install_helm "v${ver:-4.3.0}" ;;
    gh) install_gh ;;
    bw) install_bw ;;
    osv-scanner) install_osv_scanner ;;
    postgresql-client|psql) install_postgresql_client ;;
    python3|python3-venv|uv) install_python ;;
    *) 
      if ! dpkg -l "$pkg" 2>/dev/null | grep -q "^ii"; then
        apt-get install -y -qq "$pkg" 2>&1 | tail -1
      fi
      ;;
  esac
}

# Main: read packages.conf and install all
install_packages() {
  local conf="${1:-/code/mainframe/packages.conf}"
  if [ ! -f "$conf" ]; then
    echo "[debian] No packages.conf found at $conf. Skipping."
    return 0
  fi
  apt-get update -qq
  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [ -z "$line" ] && continue
    install_one "$line"
  done < "$conf"
  echo "[debian] All packages installed from $conf"
}
