package cmd

import (
	"fmt"
	"os"
	"strings"
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

// applySSHKeys fetches SSH keys from local secrets store.
func applySSHKeys(dryRun bool) {
	fmt.Println("\n[8b/18] Fetching SSH keys from local secrets store...")
	if _, err := os.Stat("mainframe.yml"); err != nil {
		fmt.Println("  No mainframe.yml found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would fetch SSH keys from local secrets store")
		return
	}
	runShellStep("fetch ssh keys", `
		# Read SSH keys from config
		keys_conf="ssh/keys.conf"
		if [ ! -f "$keys_conf" ]; then
			echo "  No ssh/keys.conf found. Skipping SSH key fetch."
			exit 0
		fi

		echo "  Fetching SSH keys from $keys_conf..."
		while IFS= read -r line; do
			line="${line%%#*}"
			line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
			[ -z "$line" ] && continue
			item_name="${line%%:*}"
			key_path="${line#*:}"
			key_path="${key_path/#\~/$HOME}"
			if [ -f "$key_path" ]; then
				echo "  $key_path already exists. Skipping."
				continue
			fi
			echo "  Fetching: $item_name -> $key_path"
			mkdir -p "$(dirname "$key_path")"
			key_value=$(mf secrets get "$item_name" 2>/dev/null)
			if [ -n "$key_value" ]; then
				echo "$key_value" > "$key_path"
				echo "  Written to $key_path"
			else
				echo "  No secret value found for $item_name"
			fi
			if [ -f "$key_path" ]; then
				chmod 600 "$key_path"
				echo "  Permissions set: 600"
			fi
		done < "$keys_conf"
		echo "  SSH key fetch complete"
	`)
}

// applySecrets checks local secrets store.
func applySecrets(dryRun bool) {
	fmt.Println("\n[8a/18] Managing local secrets store...")
	if dryRun {
		fmt.Println("  [dry-run] Would check local secrets store")
		return
	}
	// Check age keypair
	if _, err := os.Stat(ageKeyPath()); err != nil {
		fmt.Println("  No age keypair found. Run 'mf secrets init' first.")
		return
	}
	pubData, _ := os.ReadFile(agePubPath())
	fmt.Printf("  Age keypair: exists (%s)\n", strings.TrimSpace(string(pubData))[:20]+"...")

	secrets, err := decryptLocalSecrets()
	if err != nil {
		fmt.Printf("  Error decrypting secrets: %v\n", err)
		return
	}
	fmt.Printf("  Local secrets: %d stored\n", len(secrets))

	cfg := loadPeers()
	fmt.Printf("  Peers: %d configured\n", len(cfg.Peers))
}

// applyGPGKeys fetches GPG keys from password manager and configures git signing.
func applyGPGKeys(dryRun bool) {
	fmt.Println("\n[8c/18] Fetching GPG keys from password manager...")
	if _, err := os.Stat("mainframe.yml"); err != nil {
		fmt.Println("  No mainframe.yml found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would fetch GPG keys from password manager")
		return
	}
	runShellStep("fetch gpg keys", `
		# Source secrets backend
		SECRETS_BACKEND="infisical"
		_backend=""
		for _p in "lib/secrets.sh" "$MAINFRAME_LIB/secrets.sh" "/usr/local/lib/mainframe/lib/secrets.sh"; do
			if [ -f "$_p" ]; then _backend="$_p"; break; fi
		done
		if [ -n "$_backend" ]; then source "$_backend"; fi

		if ! type secrets_available &>/dev/null || ! secrets_available 2>/dev/null; then
			echo "  Secrets backend '$SECRETS_BACKEND' not available. Skipping."
			exit 0
		fi

		status=$(secrets_status 2>/dev/null || echo "unknown")
		echo "  Secrets backend: $SECRETS_BACKEND (status: $status)"

		if [ "$status" = "unauthenticated" ] || [ "$status" = "locked" ]; then
			echo "  Vault not accessible ($status). Skipping GPG key fetch."
			exit 0
		fi

		# Read GPG keys from config
		gpg_conf="gpg/keys.conf"
		if [ ! -f "$gpg_conf" ]; then
			echo "  No gpg/keys.conf found. Skipping GPG key fetch."
			exit 0
		fi

		echo "  Fetching GPG keys from $gpg_conf..."
		while IFS= read -r line; do
			line="${line%%#*}"
			line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
			[ -z "$line" ] && continue
			item_name="$line"
			echo "  Fetching: $item_name"
			# Get the GPG private key from Infisical and import
			key_data=$(secrets_get_value "$item_name" 2>/dev/null)
			if [ -n "$key_data" ]; then
				echo "$key_data" | gpg --batch --import 2>&1 | tail -2
				# Extract the key ID for git config
				key_id=$(echo "$key_data" | gpg --list-packets 2>/dev/null | grep -i "key id" | head -1 | sed 's/.*key id: //')
				if [ -n "$key_id" ]; then
					echo "  Imported GPG key: $key_id"
				fi
			else
				echo "  No key data found for $item_name"
			fi
		done < "$gpg_conf"

		# Configure git to use GPG signing if a key was imported
		if gpg --list-secret-keys 2>/dev/null | grep -q "sec"; then
			signing_key=$(gpg --list-secret-keys --with-colons 2>/dev/null | grep "^sec" | head -1 | cut -d: -f5)
			if [ -n "$signing_key" ]; then
				git config --global user.signingkey "$signing_key"
				git config --global commit.gpgsign true
				echo "  Git configured for GPG signing (key: $signing_key)"
			fi
		fi
		echo "  GPG key fetch complete"
	`)
}
