package cmd

import (
	"fmt"
	"os"
)

// applyShellConfig syncs shell configuration files.
func applyShellConfig(dryRun bool) {
	fmt.Println("\n[4/14] Syncing shell config...")
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
	fmt.Println("\n[5/14] Syncing git config...")
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
	fmt.Println("\n[6/14] Syncing editor config...")
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
	fmt.Println("\n[7/14] Syncing terminal config...")
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
	fmt.Println("\n[8/14] Syncing SSH config...")
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
				echo "  $target exists (skipping)"
			else
				ln -s "$(pwd)/$f" "$target"
				echo "  Linked $target"
			fi
		done
	`)
}

// applyMCPConfig syncs MCP configuration.
func applyMCPConfig(dryRun bool) {
	fmt.Println("\n[9/14] Syncing MCP config...")
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
		cp mcp/config.json "$HOME/.zcode/cli/config.json"
		echo "  MCP config synced"
	`)
}

// applySkills syncs AI agent skills.
func applySkills(dryRun bool) {
	fmt.Println("\n[10/14] Syncing skills...")
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
	fmt.Println("\n[11/14] Syncing bin scripts...")
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
