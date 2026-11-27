package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// applyShellConfig syncs shell configuration files.
func applyShellConfig(dryRun bool) {
	fmt.Println("\n[4/18] Syncing shell config...")
	if _, err := os.Stat("shell"); err != nil {
		fmt.Println("  No shell/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync shell/ → ~/.config/ and ~/")
		return
	}
	runShellStep("sync shell", `
		mkdir -p "$HOME/.config"
		for dir in shell/*/; do
			[ -d "$dir" ] || continue
			name=$(basename "$dir")
			target="$HOME/.config/$name"
			mkdir -p "$target"
			cp -r "$dir"* "$target/" 2>/dev/null || true
			echo "  Synced $name → $target"
		done
		for f in shell/*; do
			[ -f "$f" ] || continue
			bn=$(basename "$f")
			[ "$bn" = "README.md" ] && continue
			target="$HOME/.$bn"
			if [ -L "$target" ]; then
				echo "  $target already linked"
			elif [ -e "$target" ]; then
				echo "  $target exists (skipping)"
			else
				ln -s "$(pwd)/$f" "$target"
				echo "  Linked $target"
			fi
		done
		if [ -f "shell/zsh/zshrc" ]; then
			target="$HOME/.zshrc"
			if [ -L "$target" ]; then echo "  $target already linked"
			elif [ -e "$target" ]; then
				cp "$target" "$target.bak.$(date +%s)" 2>/dev/null
				rm -f "$target"
				ln -s "$(pwd)/shell/zsh/zshrc" "$target"
				echo "  Replaced $target (backup saved)"
			else
				ln -s "$(pwd)/shell/zsh/zshrc" "$target"
				echo "  Linked $target"
			fi
		fi
		if [ -f "shell/zsh/zshenv" ]; then
			target="$HOME/.zshenv"
			if [ -L "$target" ]; then echo "  $target already linked"
			elif [ -e "$target" ]; then
				cp "$target" "$target.bak.$(date +%s)" 2>/dev/null
				rm -f "$target"
				ln -s "$(pwd)/shell/zsh/zshenv" "$target"
				echo "  Replaced $target (backup saved)"
			else
				ln -s "$(pwd)/shell/zsh/zshenv" "$target"
				echo "  Linked $target"
			fi
		fi
	`)
}

// applyGitConfig syncs git configuration files.
func applyGitConfig(dryRun bool) {
	fmt.Println("\n[5/18] Syncing git config...")
	if _, err := os.Stat("git"); err != nil {
		fmt.Println("  No git/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync git/ → ~/")
		return
	}
	runShellStep("sync git", `
		for f in git/*; do
			[ -f "$f" ] || continue
			bn=$(basename "$f")
			[ "$bn" = "README.md" ] && continue
			target_name="${bn%.symlink}"
			target="$HOME/.$target_name"
			if [ -L "$target" ]; then
				echo "  $target already linked"
			elif [ -e "$target" ]; then
				cp "$target" "$target.bak.$(date +%s)" 2>/dev/null
				rm -f "$target"
				ln -s "$(pwd)/$f" "$target"
				echo "  Replaced $target (backup saved)"
			else
				ln -s "$(pwd)/$f" "$target"
				echo "  Linked $target"
			fi
		done
	`)
}

// applyEditorConfig syncs editor configuration.
func applyEditorConfig(dryRun bool) {
	fmt.Println("\n[6/18] Syncing editor config...")
	if _, err := os.Stat("editor"); err != nil {
		fmt.Println("  No editor/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync editor/ → ~/.config/")
		return
	}
	runShellStep("sync editor", `
		for dir in editor/*/; do
			[ -d "$dir" ] || continue
			name=$(basename "$dir")
			target="$HOME/.config/$name"
			mkdir -p "$target"
			cp -r "$dir"* "$target/" 2>/dev/null || true
			echo "  Synced $name → $target"
		done
	`)
}

// applyTerminalConfig syncs terminal configuration.
func applyTerminalConfig(dryRun bool) {
	fmt.Println("\n[7/18] Syncing terminal config...")
	if _, err := os.Stat("terminal"); err != nil {
		fmt.Println("  No terminal/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync terminal/ → ~/.config/")
		return
	}
	runShellStep("sync terminal", `
		for dir in terminal/*/; do
			[ -d "$dir" ] || continue
			name=$(basename "$dir")
			target="$HOME/.config/$name"
			mkdir -p "$target"
			cp -r "$dir"* "$target/" 2>/dev/null || true
			echo "  Synced $name → $target"
		done
		if [ -d "terminal/ghostty" ] && [ "$(uname)" = "Darwin" ]; then
			ghostty_dir="$HOME/Library/Application Support/com.mitchellh.ghostty"
			mkdir -p "$ghostty_dir"
			cp terminal/ghostty/* "$ghostty_dir/" 2>/dev/null || true
			echo "  Synced ghostty → $ghostty_dir"
		fi
	`)
}

// applySSHConfig syncs SSH configuration.
func applySSHConfig(dryRun bool) {
	fmt.Println("\n[8/18] Syncing SSH config...")
	if _, err := os.Stat("ssh"); err != nil {
		fmt.Println("  No ssh/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync ssh/ → ~/.ssh/")
		return
	}
	runShellStep("sync ssh", `
		mkdir -p "$HOME/.ssh"
		chmod 700 "$HOME/.ssh"
		for f in ssh/*; do
			[ -f "$f" ] || continue
			bn=$(basename "$f")
			target="$HOME/.ssh/$bn"
			if [ -L "$target" ]; then
				echo "  $target already linked"
			elif [ -e "$target" ]; then
				cp "$target" "$target.bak.$(date +%s)" 2>/dev/null
				rm -f "$target"
				ln -s "$(pwd)/$f" "$target"
				echo "  Replaced $target (backup saved)"
			else
				ln -s "$(pwd)/$f" "$target"
				echo "  Linked $target"
			fi
		done
	`)
}

// applyMCPConfig syncs MCP configuration.
func applyMCPConfig(dryRun bool) {
	fmt.Println("\n[9/18] Syncing MCP config...")
	if _, err := os.Stat("mcp/config.json"); err != nil {
		fmt.Println("  No mcp/config.json found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync mcp/config.json → ~/.zcode/cli/config.json")
		return
	}
	runShellStep("sync mcp", `
		mkdir -p "$HOME/.zcode/cli"
		# Preserve existing BW_SESSION if present in local config
		local_bw_session=""
		if [ -f "$HOME/.zcode/cli/config.json" ]; then
			local_bw_session=$(python3 -c "import json,sys; d=json.load(open('$HOME/.zcode/cli/config.json')); print(d.get('mcp',{}).get('servers',{}).get('bitwarden',{}).get('env',{}).get('BW_SESSION',''))" 2>/dev/null)
		fi
		cp mcp/config.json "$HOME/.zcode/cli/config.json"
		# Restore BW_SESSION if it was set and not a placeholder
		if [ -n "$local_bw_session" ] && [ "$local_bw_session" != "{{secret:bw-session}}" ]; then
			python3 -c "
import json
p='$HOME/.zcode/cli/config.json'
d=json.load(open(p))
if 'bitwarden' in d.get('mcp',{}).get('servers',{}):
    d['mcp']['servers']['bitwarden']['env']['BW_SESSION']='$local_bw_session'
json.dump(d, open(p,'w'), indent=2)
" 2>/dev/null
			echo "  Preserved existing BW_SESSION"
		fi
		echo "  MCP config synced"
	`)
}

// applySkills syncs AI agent skills.
func applySkills(dryRun bool) {
	fmt.Println("\n[10/18] Syncing skills...")
	if _, err := os.Stat("skills"); err != nil {
		fmt.Println("  No skills/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync skills/ → ~/.agents/skills/")
		return
	}
	runShellStep("sync skills", `
		mkdir -p "$HOME/.agents/skills"
		cp -r skills/* "$HOME/.agents/skills/" 2>/dev/null || true
		echo "  Skills synced"
	`)
}

// applyBinScripts syncs custom bin scripts.
func applyBinScripts(dryRun bool) {
	fmt.Println("\n[11/18] Syncing bin scripts...")
	if _, err := os.Stat("bin"); err != nil {
		fmt.Println("  No bin/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync bin/ → ~/bin/")
		return
	}
	runShellStep("sync bin", `
		mkdir -p "$HOME/bin"
		for f in bin/*; do
			[ -f "$f" ] || continue
			bn=$(basename "$f")
			target="$HOME/bin/$bn"
			if [ -L "$target" ]; then
				echo "  $target already linked"
			elif [ -e "$target" ]; then
				echo "  $target exists (skipping)"
			else
				chmod +x "$f"
				ln -s "$(pwd)/$f" "$target"
				echo "  Linked $target"
			fi
		done
	`)
}


// applyCustomInstalls runs custom install scripts from install/ directory.
func applyCustomInstalls(dryRun bool) {
	fmt.Println("\n[12b/18] Running custom install scripts...")
	if _, err := os.Stat("install"); err != nil {
		fmt.Println("  No install/ directory. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would run install/*.sh scripts")
		return
	}
	runShellStep("custom installs", `
		for script in install/*.sh; do
			[ -f "$script" ] || continue
			echo "  Running: $script"
			bash "$script" 2>&1 | tail -3 || echo "  Warning: $script failed"
		done
		echo "  Custom installs complete"
	`)
}

// applyChatsConfig installs chat sync infrastructure and configures agents.
func applyChatsConfig(dryRun bool) {
	fmt.Println("\n[19/19] Syncing agent chats...")
	// Check if any agent has chats configured
	hasChats := false
	for _, agentDir := range []string{"agents/zcode", "agents/opencode"} {
		if _, err := os.Stat(filepath.Join(agentDir, "chats")); err == nil {
			hasChats = true
			break
		}
	}
	if !hasChats {
		fmt.Println("  No agent chats configured. Skipping.")
		fmt.Println("  (Add agents/<name>/chats/ to your config repo to enable)")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would install MinIO, Litestream, and configure chat sync")
		return
	}

	// Step 0: Install sqlite3 if not available (needed for chat DB operations)
	runShellStep("install sqlite3", `
		if command -v sqlite3 &>/dev/null; then
			echo "  sqlite3: already installed ($(sqlite3 --version | head -1))"
		elif command -v apt-get &>/dev/null; then
			echo "  Installing sqlite3 via apt..."
			apt-get install -y -qq sqlite3 2>&1 | tail -1
		elif command -v brew &>/dev/null; then
			echo "  Installing sqlite3 via Homebrew..."
			brew install sqlite3 2>&1 | tail -2
		else
			echo "  Warning: sqlite3 not found and no package manager available"
		fi
	`)

	// Step 1: Install MinIO if not available
	runShellStep("install minio", `
		if command -v minio &>/dev/null; then
			echo "  MinIO: already installed ($(minio --version 2>&1 | head -1))"
		elif command -v apt-get &>/dev/null; then
			echo "  Installing MinIO via apt..."
			# MinIO provides .deb packages
			arch=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
			curl -fsSL "https://dl.min.io/server/minio/release/linux-${arch}/minio" -o /usr/local/bin/minio 2>/dev/null || \
			wget -q "https://dl.min.io/server/minio/release/linux-${arch}/minio" -O /usr/local/bin/minio 2>/dev/null || \
			echo "  Warning: MinIO binary download failed. Install manually from https://min.io/download"
			chmod +x /usr/local/bin/minio 2>/dev/null || true
			echo "  MinIO installed (or download manually)"
		elif command -v brew &>/dev/null; then
			echo "  Installing MinIO via Homebrew..."
			brew install minio 2>&1 | tail -2
		else
			echo "  Installing MinIO binary..."
			arch=$(uname -m)
			case "$arch" in
				arm64|aarch64) arch="arm64" ;;
				x86_64) arch="amd64" ;;
			esac
			os_name=$(uname -s | tr '[:upper:]' '[:lower:]')
			curl -fsSL "https://dl.min.io/server/minio/release/${os_name}-${arch}/minio" -o /usr/local/bin/minio
			chmod +x /usr/local/bin/minio
			echo "  MinIO installed"
		fi
	`)

	// Step 2: Install Litestream if not available
	runShellStep("install litestream", `
		if command -v litestream &>/dev/null; then
			echo "  Litestream: already installed ($(litestream version 2>&1 | head -1))"
		elif command -v apt-get &>/dev/null; then
			echo "  Installing Litestream via binary download..."
			arch=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
			case "$arch" in
				arm64) arch="arm64" ;;
				amd64) arch="amd64" ;;
			esac
			curl -fsSL "https://github.com/benbjohnson/litestream/releases/latest/download/litestream-linux-${arch}.tar.gz" -o /tmp/litestream.tar.gz && tar -xzf /tmp/litestream.tar.gz -C /usr/local/bin litestream
			chmod +x /usr/local/bin/litestream
			echo "  Litestream installed"
		elif command -v brew &>/dev/null; then
			echo "  Installing Litestream via Homebrew..."
			brew install litestream 2>&1 | tail -2
		else
			echo "  Installing Litestream binary..."
			arch=$(uname -m)
			case "$arch" in
				arm64|aarch64) arch="arm64" ;;
				x86_64) arch="amd64" ;;
			esac
			os_name=$(uname -s | tr '[:upper:]' '[:lower:]')
			curl -fsSL "https://github.com/benbjohnson/litestream/releases/latest/download/litestream-${os_name}-${arch}.tar.gz" -o /tmp/litestream.tar.gz && tar -xzf /tmp/litestream.tar.gz -C /usr/local/bin litestream
			chmod +x /usr/local/bin/litestream
			echo "  Litestream installed"
		fi
	`)

	// Step 3: Install mc (MinIO client) if not available
	runShellStep("install mc", `
		if command -v mc &>/dev/null; then
			echo "  mc: already installed"
		elif command -v apt-get &>/dev/null; then
			echo "  Installing mc via binary download..."
			arch=$(dpkg --print-architecture 2>/dev/null || echo "amd64")
			case "$arch" in
				arm64) arch="arm64" ;;
				amd64) arch="amd64" ;;
			esac
			curl -fsSL "https://dl.min.io/client/mc/release/linux-${arch}/mc" -o /usr/local/bin/mc 2>/dev/null || \
			wget -q "https://dl.min.io/client/mc/release/linux-${arch}/mc" -O /usr/local/bin/mc 2>/dev/null || \
			echo "  Warning: mc binary download failed. Install manually from https://min.io/download"
			chmod +x /usr/local/bin/mc 2>/dev/null || true
			echo "  mc installed (or download manually)"
		elif command -v brew &>/dev/null; then
			echo "  Installing mc via Homebrew..."
			brew install minio/stable/mc 2>&1 | tail -2
		else
			echo "  Installing mc binary..."
			arch=$(uname -m)
			case "$arch" in
				arm64|aarch64) arch="arm64" ;;
				x86_64) arch="amd64" ;;
			esac
			os_name=$(uname -s | tr '[:upper:]' '[:lower:]')
			curl -fsSL "https://dl.min.io/client/mc/release/${os_name}-${arch}/mc" -o /usr/local/bin/mc
			chmod +x /usr/local/bin/mc
			echo "  mc installed"
		fi
	`)

	// Step 4: Set up chat data directories
	runShellStep("setup chat dirs", `
		mkdir -p "$HOME/.zcode/v2" "$HOME/.zcode/cli/rollout" "$HOME/.opencode"
		mkdir -p "$HOME/.local/share/minio/data"
		echo "  Chat data directories ready"
	`)

	// Step 5: Start MinIO if not running
	runShellStep("start minio", `
		if pgrep -x minio &>/dev/null; then
			echo "  MinIO: already running"
		else
			echo "  Starting MinIO..."
			nohup minio server "$HOME/.local/share/minio/data" --console-address ":9001" >/tmp/minio.log 2>&1 &
			sleep 2
			if pgrep -x minio &>/dev/null; then
				echo "  MinIO: started (API: http://localhost:9000, Console: http://localhost:9001)"
			else
				echo "  Warning: MinIO failed to start. Check /tmp/minio.log"
			fi
		fi
		# Create the chats bucket
		mc alias set local http://localhost:9000 mainframe mainframe-chats-secret --api S3v4 >/dev/null 2>&1 || true
		mc mb local/chats 2>/dev/null || true
	`)

	// Step 6: Configure agent chat adapters
	for _, agentDir := range []string{"agents/zcode", "agents/opencode"} {
		chatsDir := filepath.Join(agentDir, "chats")
		if _, err := os.Stat(chatsDir); err != nil {
			continue
		}
		adapterPath := filepath.Join(chatsDir, "adapter.sh")
		if _, err := os.Stat(adapterPath); err != nil {
			continue
		}
		fmt.Printf("  Configured: %s\n", agentDir)
	}

	// Step 7: Install sync daemon (macOS LaunchAgent)
	runShellStep("install sync daemon", `
		if [ "$(uname)" = "Darwin" ]; then
			PLIST="$HOME/Library/LaunchAgents/com.mainframe.chats-sync.plist"
			if [ -f "storage/launchd/com.mainframe.chats-sync.plist" ]; then
				mkdir -p "$HOME/Library/LaunchAgents"
				sed "s|\$HOME|$HOME|g" "storage/launchd/com.mainframe.chats-sync.plist" > "$PLIST"
				echo "  LaunchAgent installed: $PLIST"
				echo "  Start with: mf chats daemon start"
			else
				echo "  No LaunchAgent plist found. Skipping."
			fi
		elif [ -f /etc/systemd/system/mainframe-chats-sync.service ] || [ -f "storage/systemd/mainframe-chats-sync.service" ]; then
			if [ -f "storage/systemd/mainframe-chats-sync.service" ]; then
				cp "storage/systemd/mainframe-chats-sync.service" /etc/systemd/system/mainframe-chats-sync.service
				systemctl daemon-reload
				systemctl enable mainframe-chats-sync
				echo "  systemd service installed and enabled"
				echo "  Start with: systemctl start mainframe-chats-sync"
			fi
		else
			echo "  No sync daemon config found for this OS. Skipping."
		fi
	`)

	fmt.Println("  Chat sync configured. Run 'mf chats pull' to sync from peers.")
	fmt.Println("  Start sync daemon with: mf chats daemon start")
}
