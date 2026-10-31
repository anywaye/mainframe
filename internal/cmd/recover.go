package cmd

import (
	"fmt"
	"os"
	"strings"
	"os/exec"

	"github.com/spf13/cobra"
)

var recoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Emergency recovery: restore SSH and firewall to safe defaults",
	Long: `Restore SSH and firewall to safe defaults for lockout recovery.
This command:
  - Re-enables password authentication for root (console access)
  - Disables ufw (opens all ports)
  - Restarts SSH service
  - Does NOT modify Tailscale or other services

Use this when you are locked out and have console/VNC access.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Recovery ===")
		fmt.Println("Restoring SSH and firewall to safe defaults...")
		fmt.Println()

		// Re-enable password auth
		fmt.Println("[1/4] Re-enabling password authentication...")
		os.WriteFile("/etc/ssh/sshd_config.d/99-recovery.conf", []byte("PasswordAuthentication yes\nPermitRootLogin yes\n"), 0644)
		exec.Command("sshd", "-t").Run()
		exec.Command("systemctl", "reload", "ssh").Run()
		fmt.Println("  Done")

		// Disable firewall
		fmt.Println("[2/4] Disabling firewall...")
		exec.Command("ufw", "disable").Run()
		fmt.Println("  Done")

		// Restart SSH
		fmt.Println("[3/4] Restarting SSH...")
		exec.Command("systemctl", "restart", "ssh").Run()
		fmt.Println("  Done")

		// Show status
		fmt.Println("[4/4] Status:")
		out, _ := exec.Command("sshd", "-T").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "permitrootlogin") || strings.HasPrefix(line, "passwordauthentication") {
				fmt.Printf("  %s\n", line)
			}
		}
		ufwOut, _ := exec.Command("ufw", "status").Output()
		fmt.Printf("  firewall: %s\n", strings.TrimSpace(string(ufwOut)))

		fmt.Println()
		fmt.Println("Recovery complete. SSH is now accessible with password.")
		fmt.Println("Remember to run 'mf harden' again after fixing the issue.")
	},
}

func init() {
	rootCmd.AddCommand(recoverCmd)
}
