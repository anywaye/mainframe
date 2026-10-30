#!/usr/bin/env bash
# Bitwarden CLI setup for secret management (optional)
set -euo pipefail

BW_EMAIL="${BW_EMAIL:-}"

if [ -z "$BW_EMAIL" ]; then
  echo "[secrets] BW_EMAIL not set. Skipping Bitwarden setup."
  echo "[secrets] To use Bitwarden: export BW_EMAIL=your@email.com and re-run."
  return 0
fi

if ! command -v bw &>/dev/null; then
  npm install -g @bitwarden/cli 2>&1 | tail -1
fi

echo "[secrets] Bitwarden CLI: $(bw --version 2>/dev/null)"
echo "[secrets] Configure with: bw login $BW_EMAIL"
echo "[secrets] Then: export BW_SESSION=\$(bw unlock --raw)"