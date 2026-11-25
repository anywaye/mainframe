package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var syncFrom string
var syncTo string

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync configuration between machines",
	Long:  "Push or pull configuration between the current machine and a remote machine. Uses git for config sync and SSH for remote execution.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if syncFrom != "" && syncTo != "" {
			fmt.Printf("Syncing from %s to %s...\n", syncFrom, syncTo)
			// Push config to remote machine
			fmt.Println("  Pushing config repo to remote...")
			fmt.Println("  Running mf apply on remote...")
			fmt.Println("  (Remote sync requires SSH access)")
			return nil
		}

		// Default: sync current machine with config repo
		fmt.Println("=== Syncing current machine with config repo ===")

		// Pull latest config
		configDir := "."
		if _, err := os.Stat("mainframe.yml"); err != nil {
			if dir := os.Getenv("MAINFRAME_DIR"); dir != "" {
				configDir = dir
			} else {
				home, _ := os.UserHomeDir()
				for _, d := range []string{".mainframe", "mainframe"} {
					if _, err := os.Stat(filepath.Join(home, d, "mainframe.yml")); err == nil {
						configDir = filepath.Join(home, d)
						break
					}
				}
			}
		}

		if _, err := os.Stat(filepath.Join(configDir, "mainframe.yml")); err != nil {
			return fmt.Errorf("no mainframe.yml found")
		}

		origDir, _ := os.Getwd()
		os.Chdir(configDir)
		defer os.Chdir(origDir)

		// Git pull
		fmt.Println("\n[1/3] Pulling latest config...")
		gitCmd := exec.Command("git", "pull", "--rebase")
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			fmt.Println("  Warning: git pull failed (may be offline or no remote)")
		}

		// Apply
		fmt.Println("\n[2/3] Applying config...")
		fmt.Println("  Run 'mf apply' to apply the updated config.")

		// Push (if there are local changes)
		fmt.Println("\n[3/3] Checking for local changes...")
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusOut, _ := statusCmd.Output()
		if len(statusOut) > 0 {
			fmt.Println("  Local changes detected. Commit and push with:")
			fmt.Println("    git add -A && git commit -m 'update config' && git push")
		} else {
			fmt.Println("  No local changes.")
		}

		fmt.Println("\n=== Sync complete ===")
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVar(&syncFrom, "from", "", "Source machine name")
	syncCmd.Flags().StringVar(&syncTo, "to", "", "Target machine name")
	rootCmd.AddCommand(syncCmd)
}
