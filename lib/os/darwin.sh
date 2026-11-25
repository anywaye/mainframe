#!/usr/bin/env bash
# macOS provisioning module.
# Provides: install_packages, apply_defaults, harden (no-op on macOS)

set -euo pipefail

OS_FAMILY="darwin"

detect_os_darwin() {
  OS_ID="macos"
  OS_VERSION="$(sw_vers -productVersion)"
  OS_FAMILY="darwin"
  echo "[detect] OS: macOS $OS_VERSION (family: darwin)"
}

# Install Homebrew if not present
ensure_brew() {
  if ! command -v brew &>/dev/null; then
    echo "[darwin] Installing Homebrew..."
    /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
    if [ -f /opt/homebrew/bin/brew ]; then
      eval "$(/opt/homebrew/bin/brew shellenv)"
    elif [ -f /usr/local/bin/brew ]; then
      eval "$(/usr/local/bin/brew shellenv)"
    fi
  fi
  echo "[darwin] Homebrew: $(brew --version | head -1)"
}

# Install packages from packages.conf (brew formulas + casks)
install_packages() {
  local conf="${1:-packages.conf}"
  if [ ! -f "$conf" ]; then
    echo "[darwin] No packages.conf found. Skipping."
    return 0
  fi

  ensure_brew

  echo "[darwin] Installing packages from $conf..."

  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [ -z "$line" ] && continue

    local pkg="${line%%:*}"
    local version="${line#*:}"
    [ "$version" = "$line" ] && version=""

    # Check if already available as a command
    if command -v "$pkg" &>/dev/null 2>&1; then
      echo "[darwin] $pkg already available. Skipping."
      return 0
    fi
    # Check if installed via brew
    if brew list "$pkg" &>/dev/null 2>&1; then
      echo "[darwin] $pkg already installed via brew. Skipping."
      return 0
    fi
    echo "[darwin] Installing $pkg..."
    brew install "$pkg" 2>&1 | tail -2 || echo "[darwin] Warning: failed to install $pkg (may need manual install)"
  done < "$conf"

  echo "[darwin] Package installation complete."
}

# Apply macOS defaults (sourced from user's os/darwin.sh or os.sh)
apply_defaults() {
  if [ -f "os/darwin.sh" ]; then
    echo "[darwin] Applying macOS defaults from os/darwin.sh..."
    source "os/darwin.sh"
  elif [ -f "os.sh" ]; then
    # Check if os.sh has darwin-specific section
    if grep -q 'uname.*Darwin' os.sh 2>/dev/null; then
      echo "[darwin] Applying macOS defaults from os.sh..."
      source "os.sh"
    fi
  fi
}

# No-op hardening on macOS
harden() {
  echo "[darwin] Skipping VPS hardening on macOS (not applicable)."
}

# Install Xcode CLI tools
ensure_xcode_cli() {
  if ! xcode-select -p &>/dev/null; then
    echo "[darwin] Installing Xcode CLI tools..."
    xcode-select --install
    echo "[darwin] Waiting for Xcode CLI installation to complete..."
    until xcode-select -p &>/dev/null; do
      sleep 5
    done
  fi
  echo "[darwin] Xcode CLI: $(xcode-select -p)"
}

# Install mise (runtime manager)
ensure_mise() {
  if ! command -v mise &>/dev/null; then
    echo "[darwin] Installing mise..."
    brew install mise
  fi
  echo "[darwin] mise: $(mise --version)"
}

# Install runtimes from runtimes.conf
install_runtimes() {
  local conf="${1:-runtimes.conf}"
  if [ ! -f "$conf" ]; then
    echo "[darwin] No runtimes.conf found. Skipping."
    return 0
  fi

  ensure_mise
  echo "[darwin] Installing runtimes from $conf..."

  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [ -z "$line" ] && continue
    echo "[darwin] Installing runtime: $line"
    mise use -g "$line" 2>&1 | tail -1 || echo "[darwin] Warning: failed to install $line"
  done < "$conf"

  echo "[darwin] Runtime installation complete."
}

# Install Mac App Store apps
install_mas_apps() {
  local conf="${1:-macos/apps.conf}"
  if [ ! -f "$conf" ]; then
    echo "[darwin] No macos/apps.conf found. Skipping MAS apps."
    return 0
  fi

  if ! command -v mas &>/dev/null; then
    brew install mas
  fi

  echo "[darwin] Installing Mac App Store apps..."
  while IFS= read -r line; do
    line="${line%%#*}"
    line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
    [ -z "$line" ] && continue
    local app_id="${line##*:}"
    local app_name="${line%%:*}"
    echo "[darwin] Installing $app_name (id: $app_id)..."
    mas install "$app_id" 2>&1 | tail -1 || echo "[darwin] Warning: failed to install $app_name"
  done < "$conf"

  echo "[darwin] MAS apps complete."
}

# Sync a directory (symlink files)
sync_dir() {
  local src="$1"
  local dst="$2"
  if [ ! -d "$src" ]; then
    return 0
  fi

  mkdir -p "$dst"
  for file in "$src"/* "$src"/.[!.]*; do
    [ -e "$file" ] || continue
    local basename=$(basename "$file")
    local target="$dst/$basename"
    if [ -L "$target" ] && [ "$(readlink "$target")" = "$file" ]; then
      continue  # already symlinked correctly
    elif [ -e "$target" ]; then
      echo "[darwin] $target exists, skipping (use --force to overwrite)"
    else
      ln -s "$file" "$target"
      echo "[darwin] Linked $target → $file"
    fi
  done
}

# Main apply function
apply_config() {
  echo "[darwin] Applying configuration..."

  ensure_xcode_cli
  ensure_brew
  ensure_mise

  # Packages
  if [ -f "packages.conf" ]; then
    install_packages "packages.conf"
  fi

  # Runtimes
  if [ -f "runtimes.conf" ] || [ -f ".tool-versions" ]; then
    install_runtimes "runtimes.conf"
  fi

  # Shell config
  if [ -d "shell" ]; then
    echo "[darwin] Syncing shell config..."
    sync_dir "shell/zsh" "$HOME/.config/zsh"
    sync_dir "shell/bash" "$HOME/.config/bash"
    # Symlink main shell files
    for f in shell/*; do
      [ -f "$f" ] && [ "$(basename "$f")" != "README.md" ] && {
        target="$HOME/.$(basename "$f")"
        [ -e "$target" ] || ln -s "$(pwd)/$f" "$target"
      }
    done
  fi

  # Git config
  if [ -d "git" ]; then
    echo "[darwin] Syncing git config..."
    for f in git/*; do
      [ -f "$f" ] && [ "$(basename "$f")" != "README.md" ] && {
        target="$HOME/.$(basename "$f" | sed 's/\.symlink$//')"
        [ -e "$target" ] || ln -s "$(pwd)/$f" "$target"
      }
    done
  fi

  # Editor config
  if [ -d "editor" ]; then
    echo "[darwin] Syncing editor config..."
    for dir in editor/*/; do
      [ -d "$dir" ] && {
        name=$(basename "$dir")
        target="$HOME/.config/$name"
        mkdir -p "$target"
        sync_dir "$dir" "$target"
      }
    done
  fi

  # Terminal config
  if [ -d "terminal" ]; then
    echo "[darwin] Syncing terminal config..."
    for dir in terminal/*/; do
      [ -d "$dir" ] && {
        name=$(basename "$dir")
        target="$HOME/.config/$name"
        mkdir -p "$target"
        sync_dir "$dir" "$target"
      }
    done
  fi

  # SSH config
  if [ -d "ssh" ]; then
    echo "[darwin] Syncing SSH config..."
    mkdir -p "$HOME/.ssh"
    chmod 700 "$HOME/.ssh"
    for f in ssh/*; do
      [ -f "$f" ] && {
        target="$HOME/.ssh/$(basename "$f")"
        [ -e "$target" ] || ln -s "$(pwd)/$f" "$target"
      }
    done
  fi

  # MCP config
  if [ -f "mcp/config.json" ]; then
    echo "[darwin] Syncing MCP config..."
    mkdir -p "$HOME/.zcode/cli"
    cp "mcp/config.json" "$HOME/.zcode/cli/config.json"
  fi

  # Skills
  if [ -d "skills" ]; then
    echo "[darwin] Syncing skills..."
    mkdir -p "$HOME/.agents/skills"
    cp -r skills/* "$HOME/.agents/skills/" 2>/dev/null || true
  fi

  # Bin scripts
  if [ -d "bin" ]; then
    echo "[darwin] Syncing bin scripts..."
    mkdir -p "$HOME/bin"
    for f in bin/*; do
      [ -f "$f" ] && [ -x "$f" ] && {
        target="$HOME/bin/$(basename "$f")"
        [ -e "$target" ] || ln -s "$(pwd)/$f" "$target"
      }
    done
  fi

  # macOS defaults
  apply_defaults

  # MAS apps
  if [ -f "macos/apps.conf" ]; then
    install_mas_apps "macos/apps.conf"
  fi

  echo "[darwin] Configuration applied successfully."
}
