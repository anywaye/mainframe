#!/usr/bin/env bash
# Example project setup script.
# Copy this directory to projects/<your-project>/ and customize.
set -euo pipefail

PROJECT_NAME="example"
BASE_DIR="${BASE_DIR:-/code}"
CODE_DIR="$BASE_DIR/$PROJECT_NAME"

echo "=== Setting up project: $PROJECT_NAME ==="

mkdir -p "$CODE_DIR"
cd "$CODE_DIR"

# Clone your repos here
# git clone "https://$GH_TOKEN@github.com/your-org/your-repo.git" your-repo

# Set up project-specific infra
# e.g., docker compose up -d for databases

# Start dev services
# e.g., ./start-services.sh

echo "Project $PROJECT_NAME set up in $CODE_DIR"