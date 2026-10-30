package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update mainframe to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		url := "https://raw.githubusercontent.com/anywaye/mainframe/master/mf"
		tmp := "/tmp/mf-update"
		fmt.Println("Checking for updates...")

		c := exec.Command("curl", "-sSL", url, "-o", tmp)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}

		out, _ := exec.Command(tmp, "version").Output()
		fmt.Printf("Latest: %s", string(out))

		exe, _ := os.Executable()
		if err := os.Rename(tmp, exe); err != nil {
			c2 := exec.Command("sudo", "mv", tmp, exe)
			c2.Stdout = os.Stdout
			c2.Stderr = os.Stderr
			if err := c2.Run(); err != nil {
				return fmt.Errorf("failed to install update: %w", err)
			}
		}

		fmt.Println("Updated successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}