# Mainframe

**Secure, reproducible VPS bootstrap for developers.**

Take a fresh Ubuntu/Debian VPS to a hardened, configured dev box in one command.

```sh
mf provision --mode dev
```

---

## Table of Contents

- [What it does](#what-it-does)
- [Install](#install)
- [Quick start](#quick-start)
- [Commands](#commands)
- [Security](#security)
- [Configuration](#configuration)
- [SSH recovery](#ssh-recovery)
- [OS support](#os-support)
- [Requirements](#requirements)
- [License](#license)

---

## What it does

1. **Base provisioning** (`mf provision --mode dev|prod`):
   - Detects OS (Debian/Ubuntu, Fedora, Arch, Alpine)
   - Installs packages from `packages.conf` (editable, one per line)
   - Sets up Tailscale (private network, strongly recommended)
   - Runs mainframe hardening (audit → SSH, firewall, fail2ban, sysctl, unattended upgrades)
   - Creates `/code/` directory structure
   - No project-specific code or infra

2. **Project setup** (`projects/<name>/setup.sh`):
   - Clones project repos to `/code/<project>/<repo>`
   - Sets up project-specific infra
   - Starts dev services

## Install

```sh
# On your VPS (as root):
curl -fsSL https://get.mainframe.sh | bash
# Or clone and build:
git clone https://github.com/anywaye/mainframe.git
cd mainframe
make build && make install
```

Verify: `mf version`

## Quick start

### Dev VPS

```sh
# 1. Set environment variables
export TAILSCALE_AUTHKEY="tskey-auth-..."   # from Tailscale admin
export GH_TOKEN="ghp_..."                  # GitHub PAT (repo + packages:write)
export BW_EMAIL="your@email.com"           # optional, for Bitwarden secrets

# 2. Provision
mf provision --mode dev

# 3. Set up your project (optional)
bash projects/<name>/setup.sh
```

### Prod VPS

```sh
export TAILSCALE_AUTHKEY="tskey-auth-..."
export GH_TOKEN="ghp_..."
mf provision --mode prod
# Follow the printed instructions for substrate install
```

## Commands

### Provisioning

| Command | Description |
|---|---|
| `mf provision --mode dev` | Provision a dev VPS |
| `mf provision --mode prod` | Provision a prod VPS |
| `mf provision --dry-run` | Preview without executing |

### Security & SSH

| Command | Description |
|---|---|
| `mf harden` | Apply security hardening (SSH, firewall, fail2ban, sysctl) |
| `mf audit` | Run security audit (before/after scoring) |
| `mf recover` | **Emergency recovery** — restores SSH if hardening locks you out |
| `mf ssh verify` | Health check (daemon, port, keys, permissions, firewall) |
| `mf ssh keygen --emergency` | Generate a break-glass SSH key pair |
| `mf ssh recover` | Emergency recovery with break-glass key support |

### Utilities

| Command | Description |
|---|---|
| `mf status` | Show system status |
| `mf doctor` | Diagnose common issues |
| `mf version` | Print version |
| `mf update` | Check for updates |

## Security

mainframe applies a security baseline automatically:

- **SSH**: Key-only auth, no root login (after hardening), fail2ban
- **Firewall**: Default deny incoming, allow SSH + configured ports
- **Sysctl**: BBR congestion control, redirect/source-route hardening
- **Unattended upgrades**: Automatic security patches
- **Deadman switch**: Auto-rolls back SSH/firewall changes if not disarmed within 5 minutes
- **etckeeper**: Tracks all config changes in git

### Recovery (if you get locked out)

```sh
# 1. VNC into the VPS via Contabo panel
# 2. Log in as root
# 3. Run:
mf recover

# 4. SSH should now work with password
ssh -o PreferredAuthentications=password root@<IP>

# 5. Fix the underlying issue, then re-harden:
mf harden
```

### Break-glass key (recommended)

Generate an emergency key **before** you need it:

```sh
mf ssh keygen --emergency
# Save the private key to Bitwarden immediately!
```

This key is added to `authorized_keys` with a `# BREAK-GLASS - DO NOT REMOVE` comment and survives hardening.

## Configuration

All config via environment variables or `.env` file (from `.env.template`):

| Variable | Default | Description |
|---|---|---|
| `TAILSCALE_AUTHKEY` | (required) | Tailscale auth key |
| `GH_TOKEN` | (required) | GitHub PAT |
| `VPS_NAME` | `mainframe` | Hostname |
| `BASE_DIR` | `/code` | Code directory |
| `BW_EMAIL` | (empty) | Bitwarden email (optional) |
| `TAILSCALE_ENABLED` | `true` | Enable Tailscale |
| `MAINFRAME_ENABLED` | `true` | Enable mainframe hardening |
| `FIREWALL_MODE` | `auto` | tailscale-only, public, or custom |
| `FIREWALL_EXTRA_PORTS` | (empty) | Additional ports to open |
| `PACKAGES_FILE` | `./packages.conf` | Path to packages config |
| `PROJECT` | (empty) | Project to set up after provisioning |

## Packages

Edit `packages.conf` to customize which packages are installed. One package per line,
with comments. Version pins use colon: `go:1.26.6`.

The OS module reads this file and installs only what's listed. If a package isn't
available on your OS, it's skipped with a warning.

## OS support

Multi-OS via `lib/os/<family>.sh` modules:
- **Debian/Ubuntu** (apt) — fully implemented
- **Fedora/RHEL/CentOS** (dnf) — partial
- **Arch** (pacman) — partial
- **Alpine** (apk) — partial

Adding a new OS is just adding a new file that implements the standard interface.

## Requirements

- Fresh Ubuntu 24.04 VPS (Contabo, Hetzner, DigitalOcean, etc.)
- Root SSH access
- Tailscale account (free tier: 100 devices)
- GitHub PAT (repo + packages:write scopes)
- Bitwarden account (optional, for secrets)

## License

MIT
