package cmd

import (
	"fmt"
	"os"
)

// applyPackages installs system packages from packages.conf.
func applyPackages(dryRun bool) {
	fmt.Println("\n[2/14] Installing packages...")
	if _, err := os.Stat("packages.conf"); err != nil {
		fmt.Println("  No packages.conf found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would install packages from packages.conf")
		return
	}
	runShellStep("install packages", `
		if command -v brew &>/dev/null; then
			while IFS= read -r line; do
				line="${line%%#*}"
				line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
				[ -z "$line" ] && continue
				pkg="${line%%:*}"
				if command -v "$pkg" &>/dev/null 2>&1; then
					echo "  $pkg already available. Skipping."
				elif brew list "$pkg" &>/dev/null 2>&1; then
					echo "  $pkg already installed via brew. Skipping."
				else
					echo "  Installing $pkg..."
					brew install "$pkg" 2>&1 | tail -2 || echo "  Warning: failed to install $pkg"
				fi
			done < packages.conf
		elif command -v apt-get &>/dev/null; then
			apt-get update -qq
			while IFS= read -r line; do
				line="${line%%#*}"
				line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
				[ -z "$line" ] && continue
				pkg="${line%%:*}"
				apt-get install -y -qq "$pkg" 2>&1 | tail -1 || echo "  Warning: failed to install $pkg"
			done < packages.conf
		else
			echo "  No supported package manager found. Skipping."
		fi
	`)
}

// applyRuntimes installs language runtimes from runtimes.conf.
func applyRuntimes(dryRun bool) {
	fmt.Println("\n[3/14] Installing runtimes...")
	if _, err := os.Stat("runtimes.conf"); err == nil {
		if dryRun {
			fmt.Println("  [dry-run] Would install runtimes from runtimes.conf")
			return
		}
		runShellStep("install runtimes", `
			if command -v mise &>/dev/null; then
				while IFS= read -r line; do
					line="${line%%#*}"
					line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
					[ -z "$line" ] && continue
					if echo "$line" | grep -q ' ' && ! echo "$line" | grep -q '@'; then
						line=$(echo "$line" | sed 's/ /@/')
					fi
					echo "  Installing: $line"
					mise use -g "$line" 2>&1 | tail -1 || echo "  Warning: failed"
				done < runtimes.conf
			elif [ -f .tool-versions ]; then
				echo "  Using .tool-versions with asdf"
			else
				echo "  mise not found. Skipping runtimes."
			fi
		`)
	} else if _, err := os.Stat(".tool-versions"); err == nil {
		if dryRun {
			fmt.Println("  [dry-run] Would install runtimes from .tool-versions")
			return
		}
		runShellStep("install runtimes", `
			if command -v mise &>/dev/null; then
				mise install 2>&1 | tail -5
			fi
		`)
	} else {
		fmt.Println("  No runtimes.conf or .tool-versions found. Skipping.")
	}
}

// applySSHKeys fetches SSH keys from password manager.
func applySSHKeys(dryRun bool) {
	fmt.Println("\n[8b/14] Fetching SSH keys from password manager...")
	if _, err := os.Stat("mainframe.yml"); err != nil {
		fmt.Println("  No mainframe.yml found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would fetch SSH keys from password manager")
		return
	}
	runShellStep("fetch ssh keys", `
		if command -v bw &>/dev/null; then
			if bw status 2>/dev/null | grep -q '"status":"unauthenticated"'; then
				echo "  Bitwarden not logged in. Skipping SSH key fetch."
				echo "  Run 'bw login <email>' to enable SSH key management."
			elif bw status 2>/dev/null | grep -q '"status":"locked"'; then
				echo "  Vault is locked. Run 'bw unlock' to unlock."
			elif bw status 2>/dev/null | grep -q '"status":"unlocked"'; then
				echo "  Vault is unlocked. SSH key fetch ready."
				echo "  (Manual step: bw get item 'SSH Key' | jq -r '.notes')"
			fi
		else
			echo "  Bitwarden CLI not found. Skipping SSH key fetch."
		fi
	`)
}
