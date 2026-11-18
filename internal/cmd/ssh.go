package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var sshCmd = &cobra.Command{
	Use:   "ssh",
	Short: "SSH access management and recovery",
	Long:  "Manage SSH keys, verify SSH access, and recover from lockouts.",
}

var sshKeygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Generate an SSH key pair",
	Run: func(cmd *cobra.Command, args []string) {
		emergency, _ := cmd.Flags().GetBool("emergency")
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			name = "id_ed25519"
		}
		if emergency {
			name = "breakglass"
		}

		keyPath := filepath.Join("/root/.ssh", name)
		fmt.Printf("=== Generating SSH key: %s ===\n", name)

		// Check if key already exists
		if _, err := os.Stat(keyPath); err == nil {
			force, _ := cmd.Flags().GetBool("force")
			if !force {
				fmt.Printf("Key already exists at %s\n", keyPath)
				fmt.Println("Use --force to overwrite.")
				return
			}
			os.Remove(keyPath)
			os.Remove(keyPath + ".pub")
		}

		// Generate key
		c := exec.Command("ssh-keygen", "-t", "ed25519", "-f", keyPath, "-N", "", "-C", name+"@mainframe")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			fmt.Printf("Failed to generate key: %v\n", err)
			return
		}

		// Add to authorized_keys if emergency
		if emergency {
			pubKey, _ := os.ReadFile(keyPath + ".pub")
			akPath := "/root/.ssh/authorized_keys"
			// Append with break-glass comment
			f, err := os.OpenFile(akPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
			if err != nil {
				fmt.Printf("Failed to open authorized_keys: %v\n", err)
				return
			}
			defer f.Close()
			fmt.Fprintf(f, "%s # BREAK-GLASS - DO NOT REMOVE\n", strings.TrimSpace(string(pubKey)))
			fmt.Println()
			fmt.Println("=== BREAK-GLASS KEY GENERATED ===")
			fmt.Printf("Private key: %s\n", keyPath)
			fmt.Printf("Public key:  %s.pub\n", keyPath)
			fmt.Println()
			fmt.Println("SAVE THE PRIVATE KEY TO BITWARDEN NOW:")
			fmt.Printf("  cat %s\n", keyPath)
			fmt.Println()
			fmt.Println("This key is in authorized_keys with a BREAK-GLASS comment.")
			fmt.Println("Do NOT remove it during hardening or cleanup.")
		} else {
			fmt.Printf("Key generated at %s\n", keyPath)
		}
	},
}

var sshVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify SSH access is working",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== SSH Verification ===")
		errors := 0

		// Check SSH daemon
		fmt.Print("[1/6] SSH daemon... ")
		out, err := exec.Command("systemctl", "is-active", "ssh").Output()
		if err != nil || strings.TrimSpace(string(out)) != "active" {
			fmt.Println("NOT RUNNING")
			errors++
		} else {
			fmt.Println("running")
		}

		// Check port 22
		fmt.Print("[2/6] Port 22 listening... ")
		out, _ = exec.Command("bash", "-c", "ss -tlnp | grep ':22 '").Output()
		if len(out) == 0 {
			fmt.Println("NOT LISTENING")
			errors++
		} else {
			fmt.Println("listening")
		}

		// Check authorized_keys
		fmt.Print("[3/6] authorized_keys... ")
		akPath := "/root/.ssh/authorized_keys"
		data, err := os.ReadFile(akPath)
		if err != nil || len(strings.TrimSpace(string(data))) == 0 {
			fmt.Println("EMPTY OR MISSING")
			errors++
		} else {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			fmt.Printf("%d key(s)\n", len(lines))
		}

		// Check permissions
		fmt.Print("[4/6] Permissions... ")
		info, err := os.Stat(akPath)
		if err != nil {
			fmt.Println("FILE NOT FOUND")
			errors++
		} else if info.Mode().Perm() != 0600 {
			fmt.Printf("WRONG (%o, expected 600)\n", info.Mode().Perm())
			errors++
		} else {
			fmt.Println("OK (600)")
		}

		// Check sshusers group
		fmt.Print("[5/6] sshusers group... ")
		out, _ = exec.Command("grep", "sshusers", "/etc/group").Output()
		if len(out) == 0 {
			fmt.Println("NOT FOUND")
			errors++
		} else {
			fmt.Printf("%s\n", strings.TrimSpace(string(out)))
		}

		// Check iptables
		fmt.Print("[6/6] Port 22 in firewall... ")
		out, _ = exec.Command("bash", "-c", "(iptables -L INPUT -n 2>/dev/null | grep -q 'dpt:22\\|dport 22') || (ufw status 2>/dev/null | grep -q '22/tcp.*ALLOW') && echo allowed || echo blocked").Output()
		result := strings.TrimSpace(string(out))
		if result == "allowed" {
			fmt.Println("allowed")
		} else {
			fmt.Println("BLOCKED or unknown")
			errors++
		}

		fmt.Println()
		if errors == 0 {
			fmt.Println("All checks passed. SSH should be accessible.")
		} else {
			fmt.Printf("%d issue(s) found. Fix them before relying on SSH.\n", errors)
		}
	},
}

var sshRecoverCmd = &cobra.Command{
	Use:   "recover",
	Short: "Emergency recovery: restore SSH and firewall to safe defaults",
	Run: func(cmd *cobra.Command, args []string) {
		keyName, _ := cmd.Flags().GetString("key")
		fmt.Println("=== SSH Recovery ===")
		fmt.Println()

		// Remove mainframe hardening config
		fmt.Println("[1/5] Removing mainframe SSH hardening...")
		os.Remove("/etc/ssh/sshd_config.d/00-mainframe.conf")
		os.WriteFile("/etc/ssh/sshd_config.d/00-recovery.conf", []byte("PasswordAuthentication yes\nPermitRootLogin yes\nAuthenticationMethods any\n"), 0644)
		exec.Command("sshd", "-t").Run()
		exec.Command("systemctl", "reload", "ssh").Run()
		fmt.Println("  Done")

		// Ensure break-glass key is in authorized_keys
		if keyName != "" {
			fmt.Printf("[2/5] Ensuring break-glass key '%s' is in authorized_keys...\n", keyName)
			keyPath := filepath.Join("/root/.ssh", keyName+".pub")
			if pubKey, err := os.ReadFile(keyPath); err == nil {
				akPath := "/root/.ssh/authorized_keys"
				content, _ := os.ReadFile(akPath)
				if !strings.Contains(string(content), strings.TrimSpace(string(pubKey))) {
					f, _ := os.OpenFile(akPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
					fmt.Fprintf(f, "%s # BREAK-GLASS - DO NOT REMOVE\n", strings.TrimSpace(string(pubKey)))
					f.Close()
					fmt.Println("  Key added")
				} else {
					fmt.Println("  Key already present")
				}
			} else {
				fmt.Printf("  WARNING: Key file %s not found\n", keyPath)
			}
		} else {
			fmt.Println("[2/5] No break-glass key specified (use --key <name>)")
		}

		// Disable firewall
		fmt.Println("[3/5] Disabling firewall...")
		exec.Command("ufw", "disable").Run()
		// Also add iptables rule as backup
		exec.Command("iptables", "-A", "INPUT", "-p", "tcp", "--dport", "22", "-j", "ACCEPT").Run()
		fmt.Println("  Done")

		// Restart SSH
		fmt.Println("[4/5] Restarting SSH...")
		exec.Command("systemctl", "restart", "ssh").Run()
		fmt.Println("  Done")

		// Show status
		fmt.Println("[5/5] Status:")
		out, _ := exec.Command("sshd", "-T").Output()
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "permitrootlogin") || strings.HasPrefix(line, "passwordauthentication") || strings.HasPrefix(line, "authenticationmethods") {
				fmt.Printf("  %s\n", line)
			}
		}

		fmt.Println()
		fmt.Println("Recovery complete. SSH should be accessible with password.")
		fmt.Println("Test: ssh -o PreferredAuthentications=password root@<IP>")
	},
}

func init() {
	sshKeygenCmd.Flags().Bool("emergency", false, "Generate a break-glass emergency key")
	sshKeygenCmd.Flags().String("name", "", "Key name (default: id_ed25519)")
	sshKeygenCmd.Flags().Bool("force", false, "Overwrite existing key")
	sshRecoverCmd.Flags().String("key", "", "Break-glass key name to ensure in authorized_keys")

	sshCmd.AddCommand(sshKeygenCmd)
	sshCmd.AddCommand(sshVerifyCmd)
	sshCmd.AddCommand(sshRecoverCmd)
	rootCmd.AddCommand(sshCmd)
}
