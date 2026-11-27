package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var devSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up a development machine (provision + apply)",
	Long: `Full development machine setup: provisions the OS and applies your dev config.
Equivalent to: mf provision --mode dev && mf apply`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
		return nil
	},
}

var prodSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up a production server (provision only)",
	Long: `Production server setup: provisions the OS with full hardening.
No dev tools, no editor config, no MCP servers. Equivalent to: mf provision --mode prod`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Prod Setup ===")
		fmt.Println("Provisioning (prod mode)...")
		provisionCmd.SetArgs([]string{"--mode", "prod"})
		if err := provisionCmd.RunE(provisionCmd, []string{}); err != nil {
			return fmt.Errorf("provision failed: %w", err)
		}

		fmt.Println("\n=== Prod setup complete ===")
		return nil
	},
}

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Development machine commands",
}

var prodCmd = &cobra.Command{
	Use:   "prod",
	Short: "Production server commands",
}

func init() {
	devCmd.AddCommand(devSetupCmd)
	prodCmd.AddCommand(prodSetupCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(prodCmd)
}
