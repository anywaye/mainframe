package cmd

import (
	"fmt"
	"os"
)

// applyPackages installs system packages from packages.conf.
func applyPackages(dryRun bool) {
	fmt.Println("\n[2/18] Installing packages...")
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
	fmt.Println("\n[3/18] Installing runtimes...")
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
	fmt.Println("\n[8b/18] Fetching SSH keys from password manager...")
	if _, err := os.Stat("mainframe.yml"); err != nil {
		fmt.Println("  No mainframe.yml found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would fetch SSH keys from password manager")
		return
	}
	runShellStep("fetch ssh keys", `
		# Source secrets backend
		SECRETS_BACKEND="${SECRETS_BACKEND:-bitwarden}"
		_backend=""
		for _p in "lib/secrets.sh" "$MAINFRAME_LIB/secrets.sh" "/usr/local/lib/mainframe/lib/secrets.sh"; do
			if [ -f "$_p" ]; then _backend="$_p"; break; fi
		done
		if [ -n "$_backend" ]; then source "$_backend"; fi

		if ! type secrets_available &>/dev/null || ! secrets_available 2>/dev/null; then
			echo "  Secrets backend '$SECRETS_BACKEND' not available. Skipping."
			return 0
		fi

		status=$(secrets_status 2>/dev/null || echo "unknown")
		echo "  Secrets backend: $SECRETS_BACKEND (status: $status)"

		if [ "$status" = "unauthenticated" ] || [ "$status" = "locked" ]; then
			echo "  Vault not accessible ($status). Skipping SSH key fetch."
			return 0
		fi

		# Read SSH keys from config (ssh/keys.conf or mainframe.yml)
		keys_conf="ssh/keys.conf"
		if [ ! -f "$keys_conf" ]; then
			echo "  No ssh/keys.conf found. Skipping SSH key fetch."
			echo "  Create ssh/keys.conf with: <bitwarden_item_name>:<path>"
			return 0
		fi

		echo "  Fetching SSH keys from $keys_conf..."
		while IFS= read -r line; do
			line="${line%%#*}"
			line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
			[ -z "$line" ] && continue
			item_name="${line%%:*}"
			key_path="${line#*:}"
			# Expand ~
			key_path="${key_path/#\~/$HOME}"
			if [ -f "$key_path" ]; then
				echo "  $key_path already exists. Skipping."
				continue
			fi
			echo "  Fetching: $item_name -> $key_path"
			mkdir -p "$(dirname "$key_path")"
			secrets_get_note "$item_name" 2>/dev/null | python3 -c "
import sys, re
notes = sys.stdin.read()
match = re.search(r'-----BEGIN OPENSSH PRIVATE KEY-----.*?-----END OPENSSH PRIVATE KEY-----', notes, re.DOTALL)
if match:
    with open('$key_path', 'w') as f:
        f.write(match.group(0) + chr(10))
    print('  Written to $key_path')
else:
    print('  No private key found in notes')
" 2>/dev/null
			if [ -f "$key_path" ]; then
				chmod 600 "$key_path"
				echo "  Permissions set: 600"
			fi
		done < "$keys_conf"
		echo "  SSH key fetch complete"
	`)
}

// applySecrets unlocks the secrets vault and stores the session.
func applySecrets(dryRun bool) {
	fmt.Println("\n[8a/18] Managing secrets vault...")
	if dryRun {
		fmt.Println("  [dry-run] Would check/unlock secrets vault")
		return
	}
	runShellStep("secrets status", `
		SECRETS_BACKEND="${SECRETS_BACKEND:-bitwarden}"
		if [ "$SECRETS_BACKEND" = "bitwarden" ]; then
			if ! command -v bw &>/dev/null; then
				echo "  Bitwarden CLI not found. Skipping."
				return 0
			fi
			status=$(timeout 5 bw status 2>/dev/null | python3 -c "import json,sys; print(json.load(sys.stdin).get('status','unknown'))" 2>/dev/null || echo "unknown")
			echo "  Bitwarden status: $status"
			if [ "$status" = "unauthenticated" ]; then
				echo "  Not logged in. Run 'bw login <email>' to enable."
			elif [ "$status" = "locked" ]; then
				echo "  Vault is locked. Run 'mf secrets unlock' to unlock."
			elif [ "$status" = "unlocked" ]; then
				echo "  Vault is unlocked."
				# Check if session file exists
				if [ -f "$HOME/.config/bitwarden/session" ]; then
					echo "  Session file: exists"
				else
					echo "  Session file: not found. Run 'mf secrets unlock' to create."
				fi
			fi
		else
			echo "  Backend: $SECRETS_BACKEND (not yet implemented)"
		fi
	`)
}
