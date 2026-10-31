package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose issues and suggest fixes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Doctor ===")
		fmt.Println()
		issues := 0

		check := func(name string, ok bool, fix string) {
			if ok {
				fmt.Printf("  [OK]   %s\n", name)
			} else {
				fmt.Printf("  [FAIL] %s\n", name)
				if fix != "" {
					fmt.Printf("         Fix: %s\n", fix)
				}
				issues++
			}
		}

		// SSH keys
		_, err := os.Stat("/root/.ssh/authorized_keys")
		hasRootKey := err == nil
		_, err = os.Stat("/home/ubuntu/.ssh/authorized_keys")
		hasUbuntuKey := err == nil
		// Root key is optional if ubuntu has a key and sudo works
		if !hasRootKey && hasUbuntuKey {
			fmt.Println("  [OK]   SSH key for root (optional — ubuntu user has key)")
		} else {
			check("SSH key for root", hasRootKey, "ssh-copy-id root@<ip>")
		}
		check("SSH key for ubuntu", hasUbuntuKey, "ssh-copy-id ubuntu@<ip>")

		// Tailscale
		out, _ := exec.Command("tailscale", "status").Output()
		tsConnected := !strings.Contains(string(out), "not running") && len(strings.TrimSpace(string(out))) > 0
		check("Tailscale connected", tsConnected, "tailscale up --authkey=...")

		// Firewall
		ufwOut, _ := exec.Command("sudo", "ufw", "status").Output()
		ufwActive := strings.Contains(string(ufwOut), "active")
		check("Firewall active", ufwActive, "mf provision --mode dev")

		// Tools
		for _, t := range []string{"docker", "go", "node", "gh", "helm", "psql"} {
			_, err := exec.LookPath(t)
			check(t+" installed", err == nil, "mf provision --mode dev")
		}

		// Directories
		for _, dir := range []string{"/code", "/data/workspace"} {
			_, err := os.Stat(dir)
			check(dir+" exists", err == nil, "mf provision --mode dev")
		}

		// Root password
		rootpw, _ := exec.Command("sudo", "bash", "-c", "awk -F: '$1==\"root\"{print substr($2,1,1)}' /etc/shadow").Output()
		hasRootPw := strings.TrimSpace(string(rootpw)) == "$"
		check("Root password set (console access)", hasRootPw, "passwd root")

		// Go version
		goOut, _ := exec.Command("go", "version").Output()
		goOK := strings.Contains(string(goOut), "go1.2")
		check("Go 1.26+ installed", goOK, "install Go 1.26.6")

		// Summary
		fmt.Println()
		if issues == 0 {
			fmt.Println("All checks passed. VPS is healthy.")
		} else {
			fmt.Printf("%d issue(s) found. Fix the items above and re-run 'mf doctor'.\n", issues)
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
