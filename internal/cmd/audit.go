package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "Run the security audit (read-only)",
	Long:  "Run the mainframe security audit. Shows a score (0-100) and lists PASS/WARN/FAIL/INFO/SKIP for each check.",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")
		quietFlag, _ := cmd.Flags().GetBool("quiet")

		auditPath := findAsset("audit/audit.sh")
		if auditPath == "" {
			return fmt.Errorf("audit.sh not found")
		}

		cmdArgs := []string{auditPath}
		if jsonFlag {
			cmdArgs = append(cmdArgs, "--json")
		}
		if quietFlag {
			cmdArgs = append(cmdArgs, "--quiet")
		}

		c := exec.Command("bash", cmdArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	auditCmd.Flags().Bool("json", false, "Machine-readable JSON output")
	auditCmd.Flags().Bool("quiet", false, "Hide PASS/INFO lines")
}