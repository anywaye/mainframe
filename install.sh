#!/usr/bin/env bash
# Mainframe installer — downloads the mf binary and runs provision.
# Usage: curl -fsSL https://raw.githubusercontent.com/anywaye/mainframe/master/install.sh | bash
# Or:    curl -fsSL ... | bash -s -- --mode dev --project anywaye
set -euo pipefail

REPO="anywaye/mainframe"
BINARY="mf"
INSTALL_DIR="/usr/local/bin"

# Detect OS and arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  arm64)   ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

case "$OS" in
  linux)  PLATFORM="linux" ;;
  darwin) PLATFORM="darwin" ;;
  *) echo "Unsupported OS: $OS" >&2; exit 1 ;;
esac

# Get latest release tag
TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null | grep '"tag_name"' | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
if [ -z "$TAG" ]; then
  TAG="v0.1.0"
fi

echo "Installing mainframe $TAG for $PLATFORM/$ARCH..."

# Download binary
URL="https://github.com/$REPO/releases/download/$TAG/$BINARY-$PLATFORM-$ARCH"
TMP=$(mktemp)
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP" "$INSTALL_DIR/$BINARY"
fi

echo "Installed: $(mf version)"
echo ""
echo "Next: mf provision --mode dev (or --mode prod)"
echo "Docs: https://github.com/$REPO"