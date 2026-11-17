# Mainframe

**Secure, reproducible VPS bootstrap for developers.**

Take a fresh Linux VPS to a hardened, configured box in one command.
Supports both **dev** and **prod** modes.

```sh
# Dev box (interactive, tools, relaxed firewall)
mf provision --mode dev

# Production server (hardened, tailscale-only, k3s-ready)
mf provision --mode prod
```

## Table of Contents

- [When to use mainframe](#when-to-use-mainframe)
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

## When to use mainframe

**You just got a new VPS** and need to:
- Set up SSH, firewall, and security hardening
- Install dev tools (Go, Node, Docker, etc.)
- Join a Tailscale network for private access
- Have a reproducible setup you can re-run or share

**You're managing multiple VPSes** and want:
- Consistent security baseline across all of them
- A deadman switch so you can't lock yourself out
- Easy recovery if something breaks
- Audit scoring to track security posture

**You're NOT looking for:**
- A full PaaS or container orchestrator (use k3s/Docker directly)
- A configuration management tool like Ansible (mainframe is simpler, opinionated)

---

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

### Step 1: Get a VPS

Any provider (Contabo, Hetzner, DigitalOcean, Vultr). Minimum:
- 2 vCPU, 4GB RAM, 40GB SSD
- Ubuntu 24.04 LTS (or Debian, Fedora, Arch, Alpine)
- Root SSH access

### Step 2: Set environment variables

```sh
export TAILSCALE_AUTHKEY="tskey-auth-..."   # from Tailscale admin console
export GH_TOKEN="ghp_..."                  # GitHub PAT (repo + packages:write)
export BW_EMAIL="your@email.com"           # optional, for Bitwarden secrets
```

### Step 3: Provision

```sh
# Dev box - interactive, relaxed firewall, dev tools
mf provision --mode dev

# OR production server - hardened, tailscale-only, no public ports
mf provision --mode prod
```

Provisioning takes 5-10 minutes. It will:
1. Detect your OS
2. Install packages from packages.conf
3. Set up Tailscale
4. Apply security hardening (SSH, firewall, fail2ban, sysctl)
5. Create /code/ directory
6. Run a security audit (before/after scores)

### Step 4: Set up your project (optional)

```sh
bash projects/<name>/setup.sh
```

### Step 5: Verify

```sh
mf status          # System status
mf ssh verify      # SSH health check
mf audit           # Security audit score
```

## Commands

### Provisioning

| Command | Description |
|---|---|
| `mf provision --mode dev` | Provision a dev VPS |
| `mf provision --mode prod` | Provision a prod VPS |
| `mf provision --dry-run` | Preview without executing |

### Security and SSH

| Command | Description |
|---|---|
| `mf harden` | Apply security hardening (auto-arms deadman timer first) |
| `mf audit` | Run security audit (before/after scoring) |
| `mf recover` | Emergency recovery - restores SSH if hardening locks you out |
| `mf ssh verify` | Health check (daemon, port, keys, permissions, firewall) |
| `mf ssh keygen --emergency` | Generate a break-glass SSH key pair |
| `mf ssh recover` | Emergency recovery with break-glass key support |
| `mf deadman arm 5` | Arm deadman timer (5 min) before risky changes |
| `mf deadman disarm` | Disarm (keep changes) |
| `mf deadman status` | Show timer status |

### Utilities

| Command | Description |
|---|---|
| `mf status` | Show system status |
| `mf doctor` | Diagnose common issues |
| `mf version` | Print version |
| `mf update` | Check for updates |

## Security

### What mainframe does

- **SSH**: Key-only auth, no root login (after hardening), fail2ban with auto-ban
- **Firewall**: Default deny incoming, allow SSH plus configured ports
- **Sysctl**: BBR congestion control, redirect/source-route hardening
- **Unattended upgrades**: Automatic security patches
- **Deadman switch**: Auto-rolls back SSH/firewall changes if not disarmed within 5 minutes
- **etckeeper**: Tracks all config changes in git

### What mainframe does NOT do

- Does NOT manage your application code or containers
- Does NOT replace Docker/k3s/Ansible for app deployment
- Does NOT store or manage secrets (use Bitwarden/Infisical separately)

### Deadman switch

Before any risky change, arm the deadman:
```sh
mainframe-deadman arm 5   # 5 minutes to test
# Make your change
# Test from a NEW SSH connection (not the current one!)
ssh -o ControlPath=none admin@host 'sudo mainframe-deadman disarm'
# If it fails, wait 5 min - config rolls back automatically
```

mainframe's `harden` command auto-arms the deadman for you.

### Recovery (if you get locked out)

```sh
# 1. VNC into the VPS via Contabo panel (or provider's console)
# 2. Log in as root with the root password
# 3. Run:
mf recover

# 4. SSH should now work with password
ssh -o PreferredAuthentications=password root@<IP>

# 5. Fix the underlying issue
# 6. Re-harden when ready:
mf harden
```

### Break-glass key (recommended, do this once)

```sh
mf ssh keygen --emergency
# SAVE THE PRIVATE KEY TO BITWARDEN IMMEDIATELY
# cat /root/.ssh/breakglass
```

This key is added to authorized_keys with a "# BREAK-GLASS - DO NOT REMOVE" comment and survives hardening.

## Configuration

All config via environment variables or .env file (from .env.template):

| Variable | Default | Description |
|---|---|---|
| TAILSCALE_AUTHKEY | (required) | Tailscale auth key |
| GH_TOKEN | (required) | GitHub PAT |
| VPS_NAME | mainframe | Hostname |
| BASE_DIR | /code | Code directory |
| BW_EMAIL | (empty) | Bitwarden email (optional) |
| TAILSCALE_ENABLED | true | Enable Tailscale |
| MAINFRAME_ENABLED | true | Enable mainframe hardening |
| FIREWALL_MODE | auto | tailscale-only, public, or custom |
| FIREWALL_EXTRA_PORTS | (empty) | Additional ports to open |
| PACKAGES_FILE | ./packages.conf | Path to packages config |
| PROJECT | (empty) | Project to set up after provisioning |

## Packages

Edit packages.conf to customize which packages are installed. One package per line,
with comments. Version pins use colon: go:1.26.6.

The OS module reads this file and installs only what's listed. If a package isn't
available on your OS, it's skipped with a warning.

## OS support

Multi-OS via lib/os/<family>.sh modules:
- Debian/Ubuntu (apt) - fully implemented
- Fedora/RHEL/CentOS (dnf) - partial
- Arch (pacman) - partial
- Alpine (apk) - partial

Adding a new OS is just adding a new file that implements the standard interface.

## Requirements

- Fresh Linux VPS (Ubuntu 24.04 recommended; Debian, Fedora, Arch, Alpine also supported)
- Root SSH access
- Tailscale account (free tier: 100 devices)
- GitHub PAT (repo + packages:write scopes)
- Bitwarden account (optional, for secrets)

## License

MIT
