# Mainframe — reproducible dev VPS provisioning

A project-agnostic, user-agnostic VPS provisioning system. Take a fresh
Ubuntu 24.04 VPS to a secure, configured dev box in one command.

## What it does

1. **Base provisioning** (`provision.sh --mode dev|prod`):
   - Detects OS (Debian/Ubuntu, Fedora, Arch, Alpine)
   - Installs packages from `packages.conf` (editable, with comments)
   - Sets up Tailscale (private network, mandatory but configurable)
   - Runs mainframe hardening (audit → hardened, vpn profile, dissolved into mainframe)
   - Sets up firewall (mode-specific: tailscale-only or public)
   - Sets up Bitwarden CLI for secrets (optional)
   - Creates `/code/` directory structure
   - No project-specific code or infra

2. **Project setup** (`projects/<name>/setup.sh`):
   - Clones project repos to `/code/<project>/<repo>`
   - Sets up project-specific infra
   - Starts dev services

3. **Prod setup** (`provision.sh --mode prod`):
   - Base hardening + Tailscale
   - Runs anywaye substrate install.sh (k3s + Argo + services + observability)
   - Sets up GlitchTip, observability, etc.

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
See `docs/adding-os.md`.

## Security model

- **Mandatory:** SSH key-only auth, firewall (default deny), OS updates, etckeeper,
  deadman switch, no secrets in git
- **Configurable:** Tailscale (strongly recommended), mainframe hardening level,
  Bitwarden CLI, firewall mode, extra ports
- **Audit scoring:** mainframe audit runs before and after hardening, showing
  the security score improvement

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
bash provision.sh --mode dev

# 2. Set up your project
bash projects/anywaye/setup.sh

# 3. Start shared infra
cd /code/anywaye/identity
export KIT_STACK_DIR=/code/anywaye/kit
export KIT_STACK_REF=e20d20664e6f1dc0bfb1023874367d2be43370d5
export KIT_DEV_PROFILES=''
export KIT_POSTGRES_DATABASES="identity,ship,ella,cafe,core,courier,billing,gateway,keyholder,kit,loom,notes,oauth-stub,platform,press,website,docs,anywaye-py,anywaye-rb,anywaye-ts,box,darkroom"
export COURIER_TOKEN=dummy
./bin/dev
```

## Quick start (prod VPS)

```sh
export TAILSCALE_AUTHKEY="tskey-auth-..."
export GH_TOKEN="ghp_..."
bash provision.sh --mode prod
# Then follow the printed instructions for substrate install
```

## Adding a new project

1. Create `projects/<name>/` directory
2. Copy `projects/example/setup.sh` and customize
3. Run `bash projects/<name>/setup.sh`

## Security

- **Tailscale-only access** — no public ports, no SSH from internet
- **mainframe hardened** — SSH keys only, fail2ban, unattended upgrades, firewall
- **Secrets in Bitwarden** — never in git, never in plaintext
- **No extra charges** — minimal VPS, no add-ons

## Requirements

- Fresh Ubuntu 24.04 VPS (Contabo, Hetzner, DigitalOcean, etc.)
- Root SSH access
- Tailscale account (free tier: 100 devices)
- GitHub PAT (repo + packages:write scopes)
- Bitwarden account (optional, for secrets)

## License

MIT