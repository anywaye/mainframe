package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
	Use:   "new [name]",
	Short: "Create a new mainframe project directory",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := "mainframe"
		if len(args) > 0 {
			name = args[0]
		}

		dir := name
		if _, err := os.Stat(dir); err == nil {
			return fmt.Errorf("directory %s already exists", dir)
		}

		dirs := []string{dir, dir + "/projects/example"}
		for _, d := range dirs {
			if err := os.MkdirAll(d, 0755); err != nil {
				return err
			}
		}

		files := map[string]string{
			dir + "/mainframe.yml":             mainframeYML,
			dir + "/packages.conf":             packagesConf,
			dir + "/firewall.conf":             firewallConf,
			dir + "/.env.template":             envTemplate,
			dir + "/.gitignore":                gitignore,
			dir + "/README.md":                 readmeMD,
			dir + "/projects/example/setup.sh": exampleSetup,
		}
		for path, content := range files {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}
		}
		os.Chmod(dir+"/projects/example/setup.sh", 0755)

		fmt.Printf("Created %s/\n", dir)
		fmt.Println("Next steps:")
		fmt.Printf("  cd %s\n", dir)
		fmt.Println("  cp .env.template .env")
		fmt.Println("  mf lock")
		fmt.Println("  sudo mf provision --mode dev")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(newCmd)
}

const mainframeYML = `mode: dev

tailscale:
  enabled: true

firewall:
  mode: tailscale-only
  extra_ports: []

hardening:
  ssh_crypto: true
  bbr: true
  apparmor: true
  unattended: true
  needrestart: true
  swap: true
  journald: true
  etckeeper: true
  deadman: true
  fail2ban: true
  fido2: false
  totp: false
  auditd: false
  aide: false
  noexec_shm: false
  core_dumps: false

secrets:
  backend: bitwarden

packages: packages.conf
`

const packagesConf = `# packages.conf - list of packages to install.
# One package per line. Lines starting with # are comments.
# Version pins use colon: package:version (e.g., go:1.26.6)
# OS packages (no version) are installed from the OS repo.
# Toolchains (with version) are downloaded and checksum-verified.

# --- Core ---
docker              # Container runtime
git                 # Version control
curl                # HTTP client
wget                # File downloader
jq                  # JSON processor
unzip               # Archive extraction
build-essential     # Compilers and build tools

# --- Development toolchains ---
go:1.26.6           # Go toolchain
node:22             # Node.js
python3             # Python
python3-venv        # Python virtual environments

# --- Database clients ---
postgresql-client   # psql for migrations
redis-tools         # redis-cli for debugging

# --- CLI tools ---
gh                  # GitHub CLI
helm:4.3.0          # Kubernetes package manager

# --- Optional (uncomment if needed) ---
# bw                # Bitwarden CLI (secret management)
# osv-scanner       # Vulnerability scanner
# tmux              # Terminal multiplexer
# htop              # Process monitor
# ncdu              # Disk usage analyzer
`

const firewallConf = `# firewall.conf - custom firewall rules.
# Used when FIREWALL_MODE=custom.
# One rule per line. Lines starting with # are comments.
`

const envTemplate = `# Environment variables for mainframe.
# Copy to .env and fill in your values.
# Source with: source .env

# Tailscale (required for private network access)
export TAILSCALE_AUTHKEY="tskey-auth-..."

# GitHub (required for cloning repos and pushing)
export GH_TOKEN="ghp_..."

# Bitwarden (optional, for secret management)
# export BW_EMAIL="your@email.com"

# VPS name (default: mainframe)
# export VPS_NAME="mainframe"

# Uptime monitoring (healthchecks.io, Better Stack, etc.)
# export HEALTHCHECK_URL="https://hc-ping.com/your-uuid"

# Base code directory (default: /code)
# export BASE_DIR="/code"
`

const gitignore = `# Secrets - never commit
.env
*.key
*.pem
*.secret
secrets/

# OS
.DS_Store
Thumbs.db

# Build artifacts
*.tar.gz
*.tgz
`

const readmeMD = "# Mainframe project\n\nProvisioned with mf. See https://github.com/anywaye/mainframe for docs.\n"

const exampleSetup = `#!/usr/bin/env bash
# Example project setup script.
set -euo pipefail

PROJECT_NAME="example"
BASE_DIR="${BASE_DIR:-/code}"
CODE_DIR="$BASE_DIR/$PROJECT_NAME"

echo "=== Setting up project: $PROJECT_NAME ==="

mkdir -p "$CODE_DIR"
cd "$CODE_DIR"

# Clone your repos here
# git clone "https://\$GH_TOKEN@github.com/your-org/your-repo.git" your-repo

echo "Project $PROJECT_NAME set up in $CODE_DIR"
`