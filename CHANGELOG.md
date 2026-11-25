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
