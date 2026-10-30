#!/usr/bin/env bash
# Alpine Linux OS module (apk-based)
# TODO: Implement all functions.
set -euo pipefail

install_packages() { echo "[alpine] Not implemented." >&2; return 1; }
install_docker() { apk add docker && rc-update add docker default && service docker start; }
install_go() { echo "[alpine] Not implemented." >&2; return 1; }
install_node() { echo "[alpine] Not implemented." >&2; return 1; }
install_python() { echo "[alpine] Not implemented." >&2; return 1; }
install_helm() { echo "[alpine] Not implemented." >&2; return 1; }
install_gh() { echo "[alpine] Not implemented." >&2; return 1; }
install_bw() { echo "[alpine] Not implemented." >&2; return 1; }
install_osv_scanner() { echo "[alpine] Not implemented." >&2; return 1; }
install_postgresql_client() { apk add postgresql-client; }

echo "[alpine] OS module loaded (partial)"