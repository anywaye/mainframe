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

		dirs := []string{
			dir,
			dir + "/shell/zsh",
			dir + "/shell/bash",
			dir + "/git",
			dir + "/editor",
			dir + "/terminal",
			dir + "/ssh",
			dir + "/bin",
			dir + "/os",
			dir + "/mcp",
			dir + "/mcp/servers",
			dir + "/skills/common",
			dir + "/agents",
			dir + "/install",
			dir + "/resources",
		}
		for _, d := range dirs {
			if err := os.MkdirAll(d, 0755); err != nil {
				return err
			}
		}

		files := map[string]string{
			dir + "/mainframe.yml":          mainframeYML,
			dir + "/packages.conf":          packagesConf,
			dir + "/runtimes.conf":          runtimesConf,
			dir + "/firewall.conf":          firewallConf,
			dir + "/.env.template":          envTemplate,
			dir + "/.gitignore":             gitignore,
			dir + "/README.md":              readmeMD,
			dir + "/sync.sh":                syncSh,
			dir + "/mcp/registry.json":      mcpRegistry,
			dir + "/mcp/config.json":        mcpConfig,
			dir + "/sandbox.yml":            sandboxYML,
			dir + "/os.sh":                  osSh,
			dir + "/os/darwin.sh":           darwinDefaults,
			dir + "/shell/zsh/zshrc":        zshrcTemplate,
			dir + "/git/gitconfig":          gitconfigTemplate,
			dir + "/ssh/config":             sshConfigTemplate,
			dir + "/skills/common/.gitkeep": "",
			dir + "/bin/.gitkeep":           "",
			dir + "/editor/.gitkeep":        "",
			dir + "/terminal/.gitkeep":      "",
			dir + "/agents/.gitkeep":        "",
			dir + "/install/.gitkeep":       "",
		}
		for path, content := range files {
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return err
			}
		}
		os.Chmod(dir+"/sync.sh", 0755)

		fmt.Printf("Created %s/\n", dir)
		fmt.Println("Next steps:")
		fmt.Printf("  cd %s\n", dir)
		fmt.Println("  cp .env.template .env")
		fmt.Println("  # Edit mainframe.yml, packages.conf, runtimes.conf")
		fmt.Println("  # Add your shell config to shell/")
		fmt.Println("  # Add your git config to git/")
		fmt.Println("  # Add your editor config to editor/")
		fmt.Println("  mf apply")
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

secrets:
  backend: bitwarden

packages: packages.conf
runtimes: runtimes.conf

dev:
  shell: shell/
  git: git/
  editor: editor/
  terminal: terminal/
  ssh: ssh/
  bin: bin/
  skills: skills/
  mcp:
    registry: mcp/registry.json
    config: mcp/config.json
  os: os/darwin.sh

agents:
  registry: agents/registry.json
  enabled: []
`

const packagesConf = `# packages.conf - system packages
# One package per line. Lines starting with # are comments.
# Version pins use colon: package:version

# --- Core ---
git
curl
wget
jq
unzip

# --- CLI tools ---
# gh
# bw
# mise
# fzf
# ripgrep
# bat

# --- Development toolchains ---
# go:1.26.6
# node:22
# python3
# ruby

# --- macOS (brew) ---
# ghostty
# tmux
# neovim

# --- Optional (uncomment if needed) ---
# osv-scanner
# delta
# zoxide
# starship
# uv
# eza
`

const runtimesConf = `# runtimes.conf - language runtimes via mise
# Format: tool@version (mise format)

# node@22
# ruby@4.0
# python@3.12
# go@1.26
# rust@stable
`

const firewallConf = `# firewall.conf - custom firewall rules.
# Used when FIREWALL_MODE=custom.
# One rule per line. Lines starting with # are comments.
`

const envTemplate = `# Environment variables for mainframe.
# Copy to .env and fill in your values.
# Source with: source .env

# Tailscale (required for private network access)
# export TAILSCALE_AUTHKEY="tskey-auth-..."

# GitHub (required for cloning repos and pushing)
# export GH_TOKEN="ghp_..."

# Bitwarden (optional, for secret management)
# export BW_EMAIL="your@email.com"

# VPS name (default: mainframe)
# export VPS_NAME="mainframe"

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

const syncSh = `#!/usr/bin/env bash
# sync.sh - sync personal dev configs
set -euo pipefail

CONFIG_REPO="${CONFIG_REPO:-https://github.com/your-org/mainframe.git}"
BASE_DIR="${BASE_DIR:-/code}"

echo "=== Syncing dev configs ==="

rm -rf /tmp/devbox-config
git clone "$CONFIG_REPO" /tmp/devbox-config 2>/dev/null || {
    echo "Warning: could not clone config repo. Skipping."
    exit 0
}

# Sync skills
if [ -d /tmp/devbox-config/skills ]; then
    mkdir -p ~/.agents/skills
    cp -r /tmp/devbox-config/skills/* ~/.agents/skills/
    echo "Skills synced."
fi

# Sync MCP config
if [ -f /tmp/devbox-config/mcp/config.json ]; then
    mkdir -p ~/.zcode/cli
    cp /tmp/devbox-config/mcp/config.json ~/.zcode/cli/config.json
    echo "MCP config synced."
fi

# Sync dotfiles (shell, git, editor, terminal)
for dir in shell git editor terminal; do
    if [ -d "/tmp/devbox-config/$dir" ]; then
        echo "Syncing $dir..."
    fi
done

# Pull secrets
if command -v bw &>/dev/null; then
    bw sync 2>/dev/null || true
fi

echo "=== Sync complete ==="
`

const mcpRegistry = `{
  "servers": {},
  "version": 1
}
`

const mcpConfig = `{
  "mcp": {
    "servers": {}
  }
}
`

const sandboxYML = `# sandbox.yml - Dev environment spec
base: ubuntu:24.04
packages: packages.conf
provision: sync.sh
configs:
  skills: common
  mcp: common
  dotfiles: true
secrets:
  provider: bitwarden
  items: []
repos: []
`

const osSh = `# os.sh - OS-specific settings (optional)
# Sourced by mf apply on all platforms.
# Add OS-conditional blocks:

# if [[ "$(uname)" == "Darwin" ]]; then
#   defaults write NSGlobalDomain AppleShowAllExtensions -bool true
# fi

# if [[ "$(uname)" == "Linux" ]]; then
#   sysctl -w net.ipv4.ip_forward=1
# fi
`

const darwinDefaults = `#!/usr/bin/env bash
# macOS defaults - sourced by mf apply on macOS only
# Uncomment and customize the settings you want

# Finder: show all filename extensions
# defaults write NSGlobalDomain AppleShowAllExtensions -bool true

# Show hidden files by default
# defaults write com.apple.Finder AppleShowAllFiles -bool false

# Expand save dialog by default
# defaults write NSGlobalDomain NSNavPanelExpandedStateForSaveMode -bool true

# Show the ~/Library folder in Finder
# chflags nohidden ~/Library

# Enable full keyboard access for all controls
# defaults write NSGlobalDomain AppleKeyboardUIMode -int 3

# Enable tap to click (Trackpad)
# defaults write com.apple.driver.AppleBluetoothMultitouch.trackpad Clicking -bool true

# Set a blazingly fast keyboard repeat rate
# defaults write NSGlobalDomain KeyRepeat -int 1
# defaults write NSGlobalDomain InitialKeyRepeat -int 10
`

const zshrcTemplate = `# Shell configuration managed by mainframe
# Add your zsh config here

export DOTFILES="${MAINFRAME_DIR:-$HOME/.mainframe}"

# Source utils
[[ -f "$DOTFILES/shell/zsh/utils.zsh" ]] && source "$DOTFILES/shell/zsh/utils.zsh"

# Add your aliases, functions, and settings below
`

const gitconfigTemplate = `# Git configuration
[user]
    name = Your Name
    email = your@email.com

[init]
    defaultBranch = main

[core]
    editor = nvim
`

const sshConfigTemplate = `# SSH configuration
# Identity files are managed by mainframe (fetched from password manager)

Host *
    AddKeysToAgent yes
    UseKeychain yes
    IdentitiesOnly yes

# GitHub
Host github.com
    HostName github.com
    User git
    IdentityFile ~/.ssh/id_ed25519
`
