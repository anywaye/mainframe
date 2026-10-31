package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration before provisioning",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Validate ===")
		fmt.Println()
		issues := 0

		check := func(name string, ok bool, fix string) {
			if ok {
				fmt.Printf("  [OK]   %s\n", name)
			} else {
				fmt.Printf("  [FAIL] %s\n", name)
				if fix != "" {
					fmt.Printf("         Fix: %s\n", fix)
				}
				issues++
			}
		}

		// Check .env file
		_, err := os.Stat(".env")
		check(".env file exists", err == nil, "cp .env.template .env && edit .env")

		// Check TAILSCALE_AUTHKEY
		tsKey := os.Getenv("TAILSCALE_AUTHKEY")
		if tsKey == "" {
			// Try loading from .env
			loadEnv()
			tsKey = os.Getenv("TAILSCALE_AUTHKEY")
		}
		check("TAILSCALE_AUTHKEY set", tsKey != "", "export TAILSCALE_AUTHKEY=tskey-auth-...")
		if tsKey != "" && !strings.HasPrefix(tsKey, "tskey-auth-") {
			fmt.Println("  [WARN] TAILSCALE_AUTHKEY doesn't start with 'tskey-auth-' — may be invalid")
		}

		// Check GH_TOKEN
		ghToken := os.Getenv("GH_TOKEN")
		if ghToken == "" {
			loadEnv()
			ghToken = os.Getenv("GH_TOKEN")
		}
		check("GH_TOKEN set", ghToken != "", "export GH_TOKEN=ghp_... (or gho_...)")

		// Check packages.conf
		_, err = os.Stat("packages.conf")
		check("packages.conf exists", err == nil, "mf new <project>")

		// Check mainframe.yml
		_, err = os.Stat("mainframe.yml")
		check("mainframe.yml exists", err == nil, "mf new <project>")

		// Check OS
		data, err := os.ReadFile("/etc/os-release")
		osOK := err == nil && (strings.Contains(string(data), "ubuntu") || strings.Contains(string(data), "debian"))
		check("Supported OS (Ubuntu/Debian)", osOK, "mainframe supports Ubuntu/Debian (full), Fedora/Arch/Alpine (partial)")

		// Check root
		check("Running as root (for provision)", os.Getuid() == 0, "use sudo")

		// Summary
		fmt.Println()
		if issues == 0 {
			fmt.Println("All checks passed. Ready to provision.")
		} else {
			fmt.Printf("%d issue(s) found. Fix the items above before provisioning.\n", issues)
		}
	},
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
