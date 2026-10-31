package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a backup of /code directory",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Backup ===")
		backupDir := "/tmp/mainframe-backups"
		os.MkdirAll(backupDir, 0755)
		ts := time.Now().Format("20060102-150405")
		backupFile := fmt.Sprintf("%s/mainframe-backup-%s.tar.gz", backupDir, ts)
		fmt.Printf("Creating backup: %s\n", backupFile)
		cmd2 := exec.Command("tar", "-czf", backupFile, "-C", "/code", ".")
		cmd2.Stdout = os.Stdout
		cmd2.Stderr = os.Stderr
		if err := cmd2.Run(); err != nil {
			fmt.Printf("Backup failed: %v\n", err)
			return
		}
		info, _ := os.Stat(backupFile)
		fmt.Printf("Backup complete: %s (%d bytes)\n", backupFile, info.Size())
		fmt.Println("To restore: tar -xzf " + backupFile + " -C /code")
	},
}

func init() {
	rootCmd.AddCommand(backupCmd)
}
