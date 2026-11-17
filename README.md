# Mainframe

A secure, reproducible VPS bootstrap for developers.

Take a fresh Ubuntu/Debian VPS to a secure, configured dev box in one command.

## What it does

1. **Base provisioning** (`mf provision --mode dev|prod`):
   - Detects OS (Debian/Ubuntu, Fedora, Arch, Alpine)
   - Installs packages from `packages.conf` (editable, with comments)
   - Sets up Tailscale (private network, strongly recommended)
   - Runs mainframe hardening (audit → SSH hardening, firewall, fail2ban, sysctl, unattended upgrades)
   - Sets up firewall (mode-specific: tailscale-only or public)
   - Sets up Bitwarden CLI for secrets (optional)
   - Creates `/code/` directory structure
   - No project-specific code or infra

2. **Project setup** (`projects/<name>/setup.sh`):
   - Clones project repos to `/code/<project>/<repo>`
   - Sets up project-specific infra
   - Starts dev services

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

## Security model

- **Mandatory:** SSH key-only auth, firewall (default deny), OS updates, etckeeper,
  deadman switch, no secrets in git
- **Configurable:** Tailscale (strongly recommended), mainframe hardening level,
  Bitwarden CLI, firewall mode, extra ports
- **Audit scoring:** mainframe audit runs before and after hardening, showing
  the security score improvement
- **Recovery:** `mf recover` restores SSH access if hardening locks you out.
  `mf ssh verify` checks SSH health. `mf ssh keygen --emergency` generates a
  break-glass key.

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

## Quick start (dev VPS)

```sh
# 1. Provision the VPS (on the VPS, as root)
export TAILSCALE_AUTHKEY="tskey-auth-..."
export GH_TOKEN="ghp_..."
export BW_EMAIL="your@email.com"  # optional
mf provision --mode dev

# 2. Set up your project
bash projects/<name>/setup.sh
```

## Quick start (prod VPS)

```sh
export TAILSCALE_AUTHKEY="tskey-auth-..."
export GH_TOKEN="ghp_..."
mf provision --mode prod
# Then follow the printed instructions
```

## Adding a new project

1. Create `projects/<name>/` directory
2. Copy `projects/example/setup.sh` and customize
3. Run `bash projects/<name>/setup.sh`

## Security

- **Tailscale-only access** — no public ports by default, no SSH from internet
- **mainframe hardened** — SSH keys only, fail2ban, unattended upgrades, firewall
- **Secrets in Bitwarden** — never in git, never in plaintext
- **Deadman switch** — auto-rolls back SSH/firewall changes if not disarmed
- **Recovery** — `mf recover` restores SSH access if hardening locks you out
- **Break-glass key** — `mf ssh keygen --emergency` generates an emergency key

## Requirements

- Fresh Ubuntu 24.04 VPS (Contabo, Hetzner, DigitalOcean, etc.)
- Root SSH access
- Tailscale account (free tier: 100 devices)
- GitHub PAT (repo + packages:write scopes)
- Bitwarden account (optional, for secrets)

## License

MIT
