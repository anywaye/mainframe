package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var hardenCmd = &cobra.Command{
	Use:   "harden",
	Short: "Apply security hardening (sane defaults)",
	Long:  "Apply mainframe's security hardening with sane defaults. Optional extras via env vars.",
	RunE: func(cmd *cobra.Command, args []string) error {
		hardenPath := findAsset("lib/harden.sh")
		if hardenPath == "" {
			return fmt.Errorf("harden.sh not found")
		}

		c := exec.Command("bash", hardenPath)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Env = os.Environ()
		return c.Run()
	},
}

func init() {
}