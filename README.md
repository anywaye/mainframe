# Mainframe

**Secure, reproducible VPS bootstrap and machine configuration for developers.**

Take a fresh machine to a fully configured dev box in one command.
Supports both **dev** and **prod** modes.

```sh
# Install the mf CLI
curl -fsSL https://raw.githubusercontent.com/anywaye/mainframe/master/install.sh | bash

# Create a new config repo
mf new my-mainframe
cd my-mainframe

# Apply config to this machine
mf apply

# Or clone an existing config repo
mf init https://github.com/your-org/mainframe.git
mf apply
```

## Philosophy

**Mainframe is a framework, not a configuration.**

Like Rails provides the structure for web apps, Mainframe provides the structure for machine configurations. You don't write the framework — you write your config. The framework handles the how; you provide the what.

- `mf new my-mainframe` scaffolds the structure
- You fill in `packages.conf`, `shell/`, `editor/`, `mcp/`, etc.
- `mf apply` reads your config and sets up any machine
- `mf capture` scans a machine and generates config

**The framework provides the structure and logic. The user provides the content.**

Not everyone uses zsh. Not everyone uses neovim. Not everyone uses tmux. Mainframe doesn't assume any of these. If a directory is empty, it's skipped. If a section is missing, it's ignored. You use what you need, skip what you don't.

The framework grows with you. As your tools change, your config changes. As the framework adds features, your existing config keeps working. `mf apply` is idempotent — run it 100 times, it's safe every time.

## Two repos, one system

| | Framework (`anywaye/mainframe`) | Config (`your-org/mainframe`) |
|---|---|---|
| **What** | CLI tool + OS provisioning engine | Your personal configuration |
| **Installed via** | `curl -fsSL https://raw.githubusercontent.com/anywaye/mainframe/master/install.sh | bash` | `mf init <url>` |
| **Edited by** | Framework maintainers | You |
| **Contains** | Go code, shell scripts, templates | YAML, JSON, shell configs, dotfiles |
| **OS-specific** | `lib/os/<family>.sh` (provisioning logic) | `os/<family>.sh` (your preferences, optional) |

The framework is installed once. The config repo is cloned per-machine. The framework reads the config repo and applies it.

## Base + environment-specific pattern

Mainframe follows the **base + environment-specific** pattern used by Terraform, Helm, Docker Compose, and Ansible:

- `packages.conf` — base packages (all machines)
- `packages.dev.conf` — dev-only packages (optional, only applied in dev mode)
- `packages.prod.conf` — prod-only packages (optional, only applied in prod mode)
- `mcp/`, `skills/`, `shell/`, etc. — dev config (only applied via `mf apply`)
- `mainframe.yml` has `dev:` and `prod:` sections for environment-specific config

The framework provides the merge logic; the user provides the content.

## Config repo structure

```
my-mainframe/
├── mainframe.yml          # Machine config (mode, secrets, references)
├── packages.conf          # System packages (brew/apt/dnf)
├── runtimes.conf          # Language versions (mise format)
├── mcp/                   # MCP servers (registry + configs)
├── skills/                # AI agent skills
├── shell/                 # Shell config (optional)
├── git/                   # Git config (optional)
├── editor/                # Editor config (optional)
├── terminal/              # Terminal config (optional)
├── ssh/                   # SSH config (optional, no keys)
├── bin/                   # Custom scripts (optional)
├── os/                    # OS-specific settings (optional)
├── os.sh                  # OS-conditional settings (optional)
├── fleet/                 # Machine fleet definitions (optional)
│   ├── dev.yml            # Development machines
│   └── prod.yml           # Production servers
├── .env.template          # Environment variables (no values)
├── sandbox.yml            # Sandbox spec
├── Dockerfile             # Container image
├── sync.sh                # Sync script
└── README.md
```

Every directory is optional. The framework applies what's there, skips what's not.

## Commands

### Setup (recommended)
| Command | Description |
|---|---|
| `mf dev setup` | Full dev machine setup (provision + apply) |
| `mf prod setup` | Production server setup (provision only) |

### Provisioning (advanced)
| Command | Description |
|---|---|
| `mf provision` | Provision a VPS (dev mode, default) |
| `mf provision --mode prod` | Provision a production server |
| `mf provision --dry-run` | Preview without executing |

### Configuration (advanced)
| Command | Description |
|---|---|
| `mf apply` | Apply dev config to current machine |
| `mf apply --dry-run` | Preview without applying |
| `mf apply --host user@host` | Apply to remote machine |
| `mf capture` | Scan current machine → generate config |
| `mf diff` | Show differences between machine and config |
| `mf sync` | Sync config between machines |

### Project management
| Command | Description |
|---|---|
| `mf new [name]` | Create a new config repo |
| `mf init <url>` | Clone an existing config repo |

### Fleet (machine groups)
| Command | Description |
|---|---|
| `mf fleet create <name>` | Create a new fleet |
| `mf fleet create <name> --boax` | Create a fleet with boax pool registration |
| `mf fleet list` | List all fleets |
| `mf fleet show <name>` | Show fleet details and machines |
| `mf fleet add <fleet> <name>` | Add a machine to a fleet |
| `mf fleet remove <fleet> <name>` | Remove a machine from a fleet |
| `mf fleet apply <fleet>` | Apply config to all machines in a fleet |
| `mf fleet sync <fleet>` | Sync config across all machines in a fleet |
| `mf fleet status <fleet>` | Show health status of fleet machines |
| `mf fleet diff <fleet>` | Compare all machines in a fleet vs config |
| `mf fleet diff <fleet>` | Compare all machines in a fleet vs config |

### Security & SSH
| Command | Description |
|---|---|
| `mf harden` | Apply security hardening |
| `mf audit` | Run security audit |
| `mf recover` | Emergency recovery |
| `mf ssh verify` | SSH health check |
| `mf deadman arm 5` | Arm deadman timer |

### MCP
| Command | Description |
|---|---|
| `mf mcp list` | List MCP servers |
| `mf mcp add SERVER` | Add MCP server |
| `mf mcp apply` | Apply MCP config to agents |
| `mf mcp test SERVER` | Test MCP server |

### Secrets
| Command | Description |
|---|---|
| `mf secrets status` | Show secrets backend status |
| `mf secrets unlock` | Unlock vault, store session |
| `mf secrets unlock --passwordenv BW_PASSWORD` | Unlock non-interactively |
| `mf secrets lock` | Lock vault, remove session |

### Utilities
| Command | Description |
|---|---|
| `mf doctor` | Diagnose issues |
| `mf status` | Show system status |
| `mf version` | Print version |
| `mf update` | Check for updates |

## OS support

Multi-OS via `lib/os/<family>.sh` modules:
- macOS (Darwin) — Homebrew, mise, MAS apps
- Debian/Ubuntu (apt) — fully implemented
- Fedora/RHEL/CentOS (dnf) — partial
- Arch (pacman) — partial
- Alpine (apk) — partial

Adding a new OS is just adding a new file that implements the standard interface.

## Modes

- **dev** (default): Interactive, relaxed firewall, dev tools installed
- **prod**: Hardened, tailscale-only, no public ports, k3s-ready

## Secrets

Secrets are never stored in the config repo. They're referenced by name and fetched from a password manager at apply time.

Supported backends: Bitwarden, 1Password, Infisical, environment variables.

## License

MIT
