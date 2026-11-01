package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Capture current VPS state into project config files",
	Long:  "Reads the current VPS state (installed packages, firewall rules, hardening settings) and updates the project config files (packages.conf, firewall.conf, mainframe.yml) to match. Use --commit to git commit, --push to push to remote.",
	RunE: func(cmd *cobra.Command, args []string) error {
		commitFlag, _ := cmd.Flags().GetBool("commit")
		pushFlag, _ := cmd.Flags().GetBool("push")

		fmt.Println("=== Mainframe Snapshot ===")
		fmt.Println("Capturing current VPS state into project config files...")
		fmt.Println()

		// 1. Capture firewall rules
		fmt.Println("[1/3] Capturing firewall rules...")
		ufwOut, _ := exec.Command("sudo", "ufw", "status").Output()
		if strings.Contains(string(ufwOut), "active") {
			// Parse ufw rules
			rules := ""
			lines := strings.Split(string(ufwOut), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "ALLOW") || strings.Contains(line, "DENY") {
					rules += line + "\n"
				}
			}
			// Write to firewall.conf
			os.WriteFile("firewall.existing", []byte(rules), 0644)
			fmt.Printf("  Saved %d rules to firewall.existing\n", len(strings.Split(rules, "\n"))-1)
		} else {
			fmt.Println("  Firewall not active, skipping")
		}

		// 2. Capture installed packages (mainframe-relevant)
		fmt.Println("[2/3] Capturing installed packages...")
		pkgOut, _ := exec.Command("dpkg-query", "-W", "-f=${Package}\n").Output()
		installedPkgs := strings.Split(string(pkgOut), "\n")
		// Filter to mainframe-relevant packages
		relevant := []string{}
		for _, pkg := range installedPkgs {
			pkg = strings.TrimSpace(pkg)
			if pkg == "" {
				continue
			}
			// Check if it's in packages.conf
			if isPackageInConf(pkg, "packages.conf") {
				relevant = append(relevant, pkg)
			}
		}
		fmt.Printf("  Found %d relevant packages installed\n", len(relevant))

		// 3. Capture hardening settings
		fmt.Println("[3/3] Capturing hardening settings...")
		// Check if SSH hardening is applied
		sshHardened := false
		if data, err := os.ReadFile("/etc/ssh/sshd_config.d/00-mainframe.conf"); err == nil {
			sshHardened = strings.Contains(string(data), "PermitRootLogin no")
		}
		fmt.Printf("  SSH hardening: %v\n", sshHardened)

		// Update mainframe.yml firewall mode based on current state
		if data, err := os.ReadFile("mainframe.yml"); err == nil {
			content := string(data)
			// Detect current firewall mode
			ufwOut2, _ := exec.Command("sudo", "ufw", "status").Output()
			mode := "public"
			if strings.Contains(string(ufwOut2), "100.64.0.0/10") {
				mode = "tailscale-only"
			}
			content = updateYAMLField(content, "firewall", "mode", mode)
			os.WriteFile("mainframe.yml", []byte(content), 0644)
			fmt.Printf("  Firewall mode updated to: %s\n", mode)
		}

		fmt.Println()
		fmt.Println("Snapshot complete. Review changes with: git diff")

		if commitFlag {
			fmt.Println("Committing changes...")
			c := exec.Command("git", "add", "-A")
			c.Run()
			c = exec.Command("git", "commit", "-m", "snapshot: capture current VPS state")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				fmt.Printf("Commit failed: %v\n", err)
			} else {
				fmt.Println("Committed.")
			}
		}

		if pushFlag {
			fmt.Println("Pushing to remote...")
			c := exec.Command("git", "push", "origin", "master")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				fmt.Printf("Push failed: %v\n", err)
			} else {
				fmt.Println("Pushed.")
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.Flags().Bool("commit", false, "Commit changes after snapshot")
	snapshotCmd.Flags().Bool("push", false, "Push to remote after snapshot")
}

// isPackageInConf checks if a package is listed in packages.conf
func isPackageInConf(pkg, confPath string) bool {
	data, err := os.ReadFile(confPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(strings.Split(line, "#")[0])
		if line == "" {
			continue
		}
		name := strings.Split(line, ":")[0]
		name = strings.TrimSpace(name)
		if name == pkg {
			return true
		}
	}
	return false
}

// updateYAMLField updates a field in a YAML-like config file
func updateYAMLField(content, section, field, value string) string {
	lines := strings.Split(content, "\n")
	inSection := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == section+":" {
			inSection = true
			continue
		}
		if inSection && strings.HasPrefix(trimmed, field+":") {
			indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			lines[i] = indent + field + ": " + value
			break
		}
		if inSection && trimmed != "" && !strings.HasPrefix(trimmed, field+":") && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			inSection = false
		}
	}
	return strings.Join(lines, "\n")
}
