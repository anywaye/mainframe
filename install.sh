#!/bin/bash
# Install mainframe CLI (mf) binary
set -e

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
esac

BINARY="mf-${OS}-${ARCH}"
REPO="anywaye/mainframe"
VERSION="${MAINFRAME_VERSION:-latest}"

if [ "$VERSION" = "latest" ]; then
  URL="https://github.com/${REPO}/releases/latest/download/${BINARY}"
else
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY}"
fi

echo "Downloading mainframe ($OS/$ARCH)..."
curl -fsSL "$URL" -o /tmp/mf
chmod +x /tmp/mf

# Install to /usr/local/bin or ~/.local/bin
if [ -w /usr/local/bin ]; then
  mv /tmp/mf /usr/local/bin/mf
else
  mkdir -p ~/.local/bin
  mv /tmp/mf ~/.local/bin/mf
  echo "Added to ~/.local/bin — add to PATH if needed"
fi

mf version
echo ""
echo "Next steps:"
echo "  mf init https://github.com/your-org/mainframe.git  # clone your config"
echo "  mf apply                                          # set up this machine"
