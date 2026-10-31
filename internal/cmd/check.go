package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Run all diagnostics (status + doctor + audit)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("=== Mainframe Full Check ===")
		fmt.Println()

		fmt.Println("--- Status ---")
		statusCmd.Run(cmd, args)
		fmt.Println()

		fmt.Println("--- Doctor ---")
		doctorCmd.Run(cmd, args)
		fmt.Println()

		fmt.Println("--- Audit ---")
		if err := auditCmd.RunE(cmd, args); err != nil {
			// audit returns non-zero for warnings, that's OK
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
