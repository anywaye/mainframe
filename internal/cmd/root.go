package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var version = "0.10.6"

var rootCmd = &cobra.Command{
	Use:   "mf",
	Short: "Mainframe - reproducible, secure VPS provisioning",
	Long:  "Mainframe takes a fresh Ubuntu/Debian VPS to a secure, configured dev box.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Version = version
	rootCmd.AddCommand(provisionCmd)
	rootCmd.AddCommand(auditCmd)
	rootCmd.AddCommand(hardenCmd)
	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(lockCmd)
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("mainframe v" + version)
	},
}