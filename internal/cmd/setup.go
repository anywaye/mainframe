package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var setupDev bool
var setupProd bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up this machine (dev or prod)",
	Long: `Set up this machine as a development or production environment.

Dev setup (full dev environment):
  mf setup --dev

Production setup (minimal, hardened):
  mf setup --prod

Dev setup provisions the OS and applies your dev config (shell, git, editor,
terminal, MCP, skills, etc.). Prod setup provisions the OS only with full
hardening — no dev tools, no editor config, no MCP servers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !setupDev && !setupProd {
			return fmt.Errorf("specify --dev or --prod")
		}
		if setupDev && setupProd {
			return fmt.Errorf("cannot use both --dev and --prod")
		}

		if setupDev {
			fmt.Println("=== Dev Setup ===")
			fmt.Println("Step 1/2: Provisioning (dev mode)...")
			provisionCmd.SetArgs([]string{"--mode", "dev"})
			if err := provisionCmd.RunE(provisionCmd, []string{}); err != nil {
				return fmt.Errorf("provision failed: %w", err)
			}

			fmt.Println("\nStep 2/2: Applying dev config...")
			applyCmd.SetArgs([]string{})
			if err := applyCmd.RunE(applyCmd, []string{}); err != nil {
				return fmt.Errorf("apply failed: %w", err)
			}

			fmt.Println("\n=== Dev setup complete ===")
		} else {
			fmt.Println("=== Prod Setup ===")
			fmt.Println("Provisioning (prod mode)...")
			provisionCmd.SetArgs([]string{"--mode", "prod"})
			if err := provisionCmd.RunE(provisionCmd, []string{}); err != nil {
				return fmt.Errorf("provision failed: %w", err)
			}

			fmt.Println("\n=== Prod setup complete ===")
		}

		return nil
	},
}

func init() {
	setupCmd.Flags().BoolVar(&setupDev, "dev", false, "Set up as development machine")
	setupCmd.Flags().BoolVar(&setupProd, "prod", false, "Set up as production server")
	rootCmd.AddCommand(setupCmd)
}
