#!/usr/bin/env bash
# Generic secrets management dispatcher.
# Sources the appropriate backend module based on SECRETS_BACKEND.
# Supported backends: bitwarden, 1password, infisical, env
set -euo pipefail

SECRETS_BACKEND="${SECRETS_BACKEND:-bitwarden}"

# Source the backend module
_backend_module="$(dirname "${BASH_SOURCE[0]}")/secrets/${SECRETS_BACKEND}.sh"
if [ -f "$_backend_module" ]; then
  source "$_backend_module"
else
  echo "[secrets] Backend '$_SECRETS_BACKEND' not supported. Available: bitwarden, 1password, infisical, env" >&2
  # Fall back to env backend
  source "$(dirname "${BASH_SOURCE[0]}")/secrets/env.sh"
fi

# Generic interface (implemented by each backend):
#   secrets_available()  - returns 0 if backend CLI is installed
#   secrets_status()     - prints: unauthenticated, locked, unlocked
#   secrets_get_note(name) - prints the note content of an item
#   secrets_get_field(name, field) - prints a specific field value
