package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Deprecated: use 'mf setup --dev' instead
var devSetupLegacyCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up a development machine (deprecated, use 'mf setup --dev')",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("NOTE: 'mf dev setup' is deprecated. Use 'mf setup --dev' instead.")
		setupDev = true
		return setupCmd.RunE(setupCmd, args)
	},
}

// Deprecated: use 'mf setup --prod' instead
var prodSetupLegacyCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up a production server (deprecated, use 'mf setup --prod')",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("NOTE: 'mf prod setup' is deprecated. Use 'mf setup --prod' instead.")
		setupProd = true
		return setupCmd.RunE(setupCmd, args)
	},
}

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Development machine commands (deprecated, use 'mf setup --dev')",
}

var prodCmd = &cobra.Command{
	Use:   "prod",
	Short: "Production server commands (deprecated, use 'mf setup --prod')",
}

func init() {
	devCmd.AddCommand(devSetupLegacyCmd)
	prodCmd.AddCommand(prodSetupLegacyCmd)
	rootCmd.AddCommand(devCmd)
	rootCmd.AddCommand(prodCmd)
}
