#!/usr/bin/env bash
# Secrets management dispatcher.
# Infisical is the ONLY secrets backend for mainframe.
set -euo pipefail

SECRETS_BACKEND="infisical"

# Source the infisical backend module
_backend_module="$(dirname "${BASH_SOURCE[0]}")/secrets/infisical.sh"
if [ -f "$_backend_module" ]; then
  source "$_backend_module"
else
  echo "[secrets] Infisical backend module not found at $_backend_module" >&2
  exit 1
fi

# Generic interface (implemented by infisical backend):
#   secrets_available()     - returns 0 if infisical CLI is installed
#   secrets_install()       - installs infisical CLI
#   secrets_status()        - prints: unauthenticated, authenticated, unavailable
#   secrets_authenticate()  - authenticates via machine identity or interactive login
#   secrets_get_value(name) - prints the secret value
#   secrets_get_note(name)  - prints the secret value (alias for get_value)
#   secrets_get_field(name, field) - prints a specific field (convention: NAME_FIELD)
#   secrets_export_all()    - exports all secrets as dotenv
#   secrets_unlock_hint()   - prints setup instructions
