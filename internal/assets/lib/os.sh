#!/usr/bin/env bash
# Install system packages: Docker, Go, Node, Python, psql, gh, bw, etc.
set -euo pipefail

BASE_DIR="${BASE_DIR:-/code}"

echo "[os] Installing system packages..."

apt-get update -qq
apt-get install -y -qq \
  curl wget git jq unzip tar gzip \
  build-essential \
  postgresql-client \
  redis-tools \
  ca-certificates gnupg \
  2>&1 | tail -2

# Docker
if ! command -v docker &>/dev/null; then
  curl -fsSL https://get.docker.com | sh
  systemctl enable --now docker
fi
echo "[os] Docker: $(docker --version)"

# Go
if ! command -v go &>/dev/null; then
  GO_VERSION="1.26.6"
  curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -o /tmp/go.tar.gz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tar.gz
  rm /tmp/go.tar.gz
  echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
fi
export PATH=$PATH:/usr/local/go/bin
echo "[os] Go: $(go version)"

# Node.js
if ! command -v node &>/dev/null; then
  curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
  apt-get install -y -qq nodejs
fi
echo "[os] Node: $(node --version)"

# Python + uv
if ! command -v python3 &>/dev/null; then
  apt-get install -y -qq python3 python3-venv
fi
if ! command -v uv &>/dev/null; then
  curl -LsSf https://astral.sh/uv/install.sh | sh
  export PATH="$HOME/.local/bin:$PATH"
fi
echo "[os] Python: $(python3 --version)"

# Helm
if ! command -v helm &>/dev/null; then
  curl -fsSL https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash -s -- --version v4.3.0
fi

# gh CLI
if ! command -v gh &>/dev/null; then
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg 2>/dev/null
  chmod go+r /usr/share/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" > /etc/apt/sources.list.d/github-cli.list
  apt-get update -qq && apt-get install -y -qq gh
fi

# osv-scanner
if ! command -v osv-scanner &>/dev/null; then
  curl -sL -o /tmp/osv-scanner https://github.com/google/osv-scanner/releases/download/v2.7.0/osv-scanner_linux_amd64
  mv /tmp/osv-scanner /usr/local/bin/osv-scanner
  chmod +x /usr/local/bin/osv-scanner
fi

echo "[os] All packages installed"