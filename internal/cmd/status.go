package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show provisioning status and diagnose issues",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Status ===")
		fmt.Println()

		// OS
		fmt.Println("OS:")
		data, _ := os.ReadFile("/etc/os-release")
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") || strings.HasPrefix(line, "ID=") {
				fmt.Printf("  %s\n", line)
			}
		}
		fmt.Println()

		// Tools
		fmt.Println("Tools:")
		tools := []string{"mf", "tailscale", "docker", "go", "node", "gh", "helm", "psql", "ufw", "fail2ban", "lynis"}
		for _, t := range tools {
			path, err := exec.LookPath(t)
			if err != nil {
				fmt.Printf("  %-12s NOT INSTALLED\n", t)
			} else {
				fmt.Printf("  %-12s %s\n", t, path)
			}
		}
		fmt.Println()

		// Tailscale
		fmt.Println("Tailscale:")
		out, _ := exec.Command("tailscale", "status").Output()
		if strings.Contains(string(out), "not running") || len(out) == 0 {
			fmt.Println("  Status: NOT CONNECTED")
		} else {
			lines := strings.Split(string(out), "\n")
			if len(lines) > 0 {
				fmt.Printf("  %s\n", lines[0])
			}
		}
		ipOut, _ := exec.Command("tailscale", "ip", "-4").Output()
		fmt.Printf("  IP: %s\n", strings.TrimSpace(string(ipOut)))
		fmt.Println()

		// Firewall
		fmt.Println("Firewall:")
		ufwOut, _ := exec.Command("ufw", "status").Output()
		fmt.Printf("  %s\n", strings.TrimSpace(string(ufwOut)))
		fmt.Println()

		// Directories
		fmt.Println("Directories:")
		for _, dir := range []string{"/code", "/data/workspace"} {
			if info, err := os.Stat(dir); err == nil {
				fmt.Printf("  %-20s exists (%s)\n", dir, info.Mode())
			} else {
				fmt.Printf("  %-20s MISSING\n", dir)
			}
		}
		fmt.Println()

		// SSH
		fmt.Println("SSH:")
		sshdOut, _ := exec.Command("sshd", "-T").Output()
		for _, line := range strings.Split(string(sshdOut), "\n") {
			if strings.HasPrefix(line, "permitrootlogin") || strings.HasPrefix(line, "passwordauthentication") || strings.HasPrefix(line, "allowgroups") {
				fmt.Printf("  %s\n", line)
			}
		}
		fmt.Println()

		// Audit score
		fmt.Println("Security audit:")
		auditPath := findAsset("audit/audit.sh")
		if auditPath != "" {
			out, _ := exec.Command("bash", auditPath, "--quiet").CombinedOutput()
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "Score:") {
					fmt.Printf("  %s\n", line)
				}
			}
		} else {
			fmt.Println("  audit.sh not found")
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
