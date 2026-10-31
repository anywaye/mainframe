package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run all diagnostics (status + doctor + audit)",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Full Check ===")
		fmt.Println()
		// Run status
		statusCmd.Run(cmd, args)
		fmt.Println()
		// Run doctor
		doctorCmd.Run(cmd, args)
		fmt.Println()
		// Run audit
		auditCmd.Run(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
