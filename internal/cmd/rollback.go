package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "Restore /code from the last backup",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Rollback ===")
		backupDir := "/tmp/mainframe-backups"
		files, err := filepath.Glob(backupDir + "/mainframe-backup-*.tar.gz")
		if err != nil || len(files) == 0 {
			fmt.Println("No backups found in", backupDir)
			return
		}
		sort.Strings(files)
		lastBackup := files[len(files)-1]
		fmt.Printf("Restoring from: %s\n", lastBackup)
		fmt.Println("WARNING: This will overwrite /code. Continue? (y/N)")
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "y" && confirm != "Y" {
			fmt.Println("Aborted.")
			return
		}
		c := exec.Command("tar", "-xzf", lastBackup, "-C", "/code")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Printf("Restore failed: %v\n", err)
			return
		}
		fmt.Println("Restore complete.")
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}
