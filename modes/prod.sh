#!/usr/bin/env bash
# Prod mode: base hardening + k3s substrate + observability.
# Public ingress via Traefik. For production workloads.
set -euo pipefail

echo "[prod] Setting up prod environment..."

# k3s is installed by the substrate install.sh
# Run the anywaye substrate installer:
#   cd /tmp
#   curl -fsSL https://raw.githubusercontent.com/anywaye/kit/master/templates/substrate/install.sh | bash
#
# Or from a checkout:
#   cd /code/anywaye/kit
#   export KIT_SUBSTRATE_ACME_EMAIL=your@email.com
#   export KIT_SUBSTRATE_PULL_SECRET=$GH_TOKEN
#   bash templates/substrate/install.sh

echo "[prod] Substrate install deferred. Run manually:"
echo "  cd /code/anywaye/kit"
echo "  export KIT_SUBSTRATE_ACME_EMAIL=your@email.com"
echo "  export KIT_SUBSTRATE_PULL_SECRET=\$GH_TOKEN"
echo "  bash templates/substrate/install.sh"
echo ""
echo "[prod] After substrate install, apply internal manifests:"
echo "  kubectl apply -f /code/anywaye/kit/templates/internal/..."
echo ""
echo "[prod] Set up Tailscale on your laptop: tailscale up"
echo "[prod] Run mainframe audit + hardened manually."