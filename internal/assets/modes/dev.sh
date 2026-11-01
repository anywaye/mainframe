#!/usr/bin/env bash
# Dev mode: install dev toolchains, set up /code structure.
# No public services, no observability, no k3s.
set -euo pipefail

# Fix npm permissions (npm may have been run as root during package install)
if [ -d /home/ubuntu/.npm ]; then
  chown -R ubuntu:ubuntu /home/ubuntu/.npm 2>/dev/null || true
fi

BASE_DIR="${BASE_DIR:-/code}"

echo "[dev] Setting up dev environment..."

# Install dev toolchains (already installed by os.sh, but ensure)
# Go, Node, Python, Docker are already installed by lib/os.sh

# Create /code structure
mkdir -p "$BASE_DIR"
mkdir -p "$BASE_DIR/worktrees"

# Backward compat symlink
if [ ! -e /data/workspace ]; then
  mkdir -p /data
  ln -s "$BASE_DIR" /data/workspace
fi

echo "[dev] Dev environment ready."
echo "[dev] Clone repos with: bash projects/<name>/setup.sh"
echo "[dev] Start shared infra: cd /code/<project>/identity && ./bin/dev"