package cmd

import (
	"fmt"
	"os"
	"path/filepath"
)

// applyOSSettings applies OS-specific settings from os.sh or os/ directory.
func applyOSSettings(dryRun bool, osFamily string) {
	fmt.Println("\n[12/14] Applying OS-specific settings...")
	osScript := ""
	if _, err := os.Stat("os.sh"); err == nil {
		osScript = "os.sh"
	} else if _, err := os.Stat("os"); err == nil {
		for _, name := range []string{osFamily + ".sh", "darwin.sh", "linux.sh"} {
			if _, err := os.Stat(filepath.Join("os", name)); err == nil {
				osScript = filepath.Join("os", name)
				break
			}
		}
	}
	if osScript != "" {
		if dryRun {
			fmt.Printf("  [dry-run] Would source %s\n", osScript)
		} else {
			runShellStep("os settings", fmt.Sprintf("source %s", osScript))
		}
	} else {
		fmt.Println("  No OS-specific settings found. Skipping.")
	}
}

// applyMacOSDefaults applies macOS-specific defaults.
func applyMacOSDefaults(dryRun bool, osFamily string) {
	if osFamily != "darwin" {
		return
	}
	fmt.Println("\n[13/14] Applying macOS defaults...")
	if _, err := os.Stat("macos/defaults.sh"); err == nil {
		if dryRun {
			fmt.Println("  [dry-run] Would apply macos/defaults.sh")
		} else {
			runShellStep("macos defaults", `source macos/defaults.sh`)
		}
	} else {
		fmt.Println("  No macos/defaults.sh found. Skipping.")
	}
}

// applyAgentConfigs syncs agent configurations.
func applyAgentConfigs(dryRun bool) {
	fmt.Println("\n[14/14] Syncing agent configs...")
	if _, err := os.Stat("agents/registry.json"); err != nil {
		fmt.Println("  No agents/registry.json found. Skipping.")
		return
	}
	if dryRun {
		fmt.Println("  [dry-run] Would sync agent configs")
		return
	}
	runShellStep("sync agents", `
		if command -v jq &>/dev/null; then
			enabled=$(jq -r '.agents | to_entries[] | select(.value.enabled == true) | .key' agents/registry.json 2>/dev/null)
			for agent in $enabled; do
				echo "  Syncing agent: $agent"
				config=$(jq -r ".agents.$agent.config" agents/registry.json 2>/dev/null)
				if [ -f "$config" ]; then
					echo "    Config: $config"
				fi
			done
		else
			echo "  jq not found. Skipping agent sync."
		fi
		echo "  Agent sync complete"
	`)
}
