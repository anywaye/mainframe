## [0.20.0] - 2026-11-27

### Added
- `mf chats` command group for peer-to-peer agent chat sync
  - `mf chats pull [fleet]` — idempotent pull from all reachable fleet machines
  - `mf chats push [fleet]` — push local chats to all reachable fleet machines
  - `mf chats status [fleet]` — per-machine sync status
  - `mf chats list` — list all chat sessions across agents (ZCode, OpenCode)
  - `mf chats daemon start/stop/status` — manage sync daemon
- Chat sync storage backend (MinIO + Litestream)
  - `storage/backend.conf` — MinIO endpoint, bucket, credentials
  - `storage/s3-adapter.sh` — generic S3/MinIO operations
  - `storage/litestream-adapter.sh` — SQLite replication via Litestream
  - `storage/sync-daemon.sh` — continuous sync daemon (MinIO + Litestream + file watcher)
  - `storage/launchd/` and `storage/systemd/` — daemon configs for macOS/Linux
- Per-agent chat adapters (ZCode, OpenCode) with capture/restore/watch
- GPG key sync from password manager (fetch, import, configure git signing)
- OpenCode agent support (install, config sync, chat sync)
- `mf apply` step 19: chat sync infrastructure (MinIO, Litestream, mc, sqlite3)
- `mf fleet sync` and `mf fleet apply` now trigger `mf chats pull` on each machine
- SQLite database merge on pull (INSERT OR IGNORE) instead of skipping existing files
- `mf chats list` handles missing sessions/tasks tables gracefully
- Embedded secrets backends in binary (fixes SSH/GPG key fetch)
- Chat sync under `agents/<name>/chats/` namespace for extensibility

### Fixed
- Shell scripts: `return 0` → `exit 0` (scripts run via `bash -c`)
- `.gitignore`: `secrets/` → `/secrets/` (was ignoring `lib/secrets/`)
- OpenCode install command (was using wrong npm package name)
- OpenCode data directory config (now uses global `~/.opencode/`)
- Fleet sync SSH command (removed `mf chats pull` from remote machines)
- known_hosts: added dev-mf host key for fleet connectivity
- Dotfiles migration: all `~/.dotfiles` references → mainframe paths
- Default git branch changed to `master`

### Changed
- `mf apply` step count: 18 → 19 (added chats step)
- `mf apply ssh-keys` and `gpg-keys` now work gracefully when vault is locked

# Changelog

## [0.16.0] - 2026-11-26

### Added
- macOS support: `lib/os/darwin.sh` with Homebrew, mise, MAS apps, Xcode CLI
- `mf apply` command — 16-step idempotent config application
  - Single/multi-step execution: `mf apply packages`, `mf apply shell git`
  - `--skip`, `--only`, `--list`, `--dry-run`, `--resume`, `--verbose` flags
- `mf init <url>` — clone config repos
- `mf capture` — scan machine and generate config files
- `mf diff` — compare machine state vs config
- `mf sync` — sync config between machines
- `mf secrets` command group (unlock, status, lock)
- Generic secrets backend interface (Bitwarden, env, extensible)
- SSH key management via password manager
- Agent config sync (ZCode, OpenCode, Claude)
- Custom install scripts (`install/*.sh`)
- Logging with timing, summary report, resume capability
- Embedded lib extraction (self-contained binary)
- Ghostty macOS config path support
- Package availability checks before brew install
- Git config sync with backup-and-replace
- `mf new` template updated with new structure

### Changed
- Refactored `apply.go` (506 lines) into 5 focused files
- `mf provision` defaults to dev mode
- OS detection now recognizes macOS (Darwin)
- `mf mcp install/test` support `--all` flag and multiple servers
- `mf mcp apply` preserves existing BW_SESSION

### Fixed
- Git identity now properly configured from mainframe config
- Zshrc autoload ordering (prepend_path before use)
- Ghostty theme name (Catppuccin Mocha)
- Terminfo references guarded
- Removed legacy references (reattach-to-user-namespace, oh-my-zsh, hub)

## [0.15.0] - 2026-11-18

### Added
- macOS support and `mf apply` command
- Embedded lib extraction

## [0.14.22] - 2026-11-17

### Fixed
- Doctor: drop psql from required tools
- Doctor: skip dev-tool checks on prod/k3s nodes
