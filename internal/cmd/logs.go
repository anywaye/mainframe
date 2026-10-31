package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Show the last provision run log",
	Run: func(cmd *cobra.Command, args []string) {
		data, err := os.ReadFile("/tmp/mainframe-provision.log")
		if err != nil {
			fmt.Println("No provision log found. Run 'mf provision' first.")
			return
		}
		fmt.Print(string(data))
	},
}

func init() {
	rootCmd.AddCommand(logsCmd)
}
