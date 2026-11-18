package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var doctorFix bool
var doctorDryRun bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose issues and suggest fixes",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("=== Mainframe Doctor ===")
		fmt.Println()
		issues := 0
		fixed := 0

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

		fixable := func(name string, ok bool, fixDesc string, fixFn func() error) {
			if ok {
				fmt.Printf("  [OK]   %s\n", name)
				return
			}
			fmt.Printf("  [FAIL] %s\n", name)
			if !doctorFix {
				fmt.Printf("         Fix: %s (run 'mf doctor --fix' to apply)\n", fixDesc)
				issues++
				return
			}
			if doctorDryRun {
				fmt.Printf("         [dry-run] would apply: %s\n", fixDesc)
				issues++
				return
			}
			if err := fixFn(); err != nil {
				fmt.Printf("         [FAIL] fix failed: %v\n", err)
				issues++
			} else {
				fmt.Printf("         [FIXED] %s\n", fixDesc)
				fixed++
			}
		}

		// --- SSH keys ---
		_, err := os.Stat("/root/.ssh/authorized_keys")
		hasRootKey := err == nil
		_, err = os.Stat("/home/ubuntu/.ssh/authorized_keys")
		hasUbuntuKey := err == nil
		if !hasRootKey && hasUbuntuKey {
			fmt.Println("  [OK]   SSH key for root (optional — ubuntu user has key)")
		} else {
			check("SSH key for root", hasRootKey, "ssh-copy-id root@<ip>")
		}
		check("SSH key for ubuntu", hasUbuntuKey, "ssh-copy-id ubuntu@<ip>")

		// --- Tailscale ---
		out, _ := exec.Command("tailscale", "status").Output()
		tsConnected := !strings.Contains(string(out), "not running") && len(strings.TrimSpace(string(out))) > 0
		check("Tailscale connected", tsConnected, "tailscale up --authkey=...")

		// --- Firewall ---
		ufwOut, _ := exec.Command("sudo", "ufw", "status").Output()
		ufwActive := strings.Contains(string(ufwOut), "active")
		check("Firewall active", ufwActive, "mf provision --mode dev")

		// --- Tools & dirs (dev boxes only; prod/k3s nodes don't need these) ---
		if _, err := os.Stat("/code"); err == nil {
			for _, t := range []string{"docker", "go", "node", "gh", "helm", "psql"} {
				_, err := exec.LookPath(t)
				check(t+" installed", err == nil, "mf provision --mode dev")
			}
			for _, dir := range []string{"/code", "/data/workspace"} {
				_, err := os.Stat(dir)
				check(dir+" exists", err == nil, "mf provision --mode dev")
			}
		} else {
			fmt.Println("  [SKIP] dev tool/dir checks (prod node)")
		}

		// --- Root password ---
		rootpw, _ := exec.Command("sudo", "bash", "-c", "awk -F: '$1==\"root\"{print substr($2,1,1)}' /etc/shadow").Output()
		hasRootPw := strings.TrimSpace(string(rootpw)) == "$"
		check("Root password set (console access)", hasRootPw, "passwd root")

		// --- Go version ---
		goOut, _ := exec.Command("go", "version").Output()
		goOK := strings.Contains(string(goOut), "go1.2")
		check("Go 1.26+ installed", goOK, "install Go 1.26.6")

		// --- Network / flannel (k3s multi-node) ---
		// rp_filter: strict (1) drops decapsulated VXLAN packets on multi-homed hosts.
		rpOut, _ := exec.Command("sysctl", "-n", "net.ipv4.conf.all.rp_filter").Output()
		rpVal := strings.TrimSpace(string(rpOut))
		rpOK := rpVal == "0" || rpVal == "2"
		fixable("rp_filter (loose for k3s/flannel)", rpOK,
			"set net.ipv4.conf.all.rp_filter=2",
			func() error {
				exec.Command("sudo", "sysctl", "-w", "net.ipv4.conf.all.rp_filter=2").Run()
				// persist
				conf := "/etc/sysctl.d/99-mainframe.conf"
				if data, err := os.ReadFile(conf); err == nil {
					s := strings.ReplaceAll(string(data), "net.ipv4.conf.all.rp_filter = 1", "net.ipv4.conf.all.rp_filter = 2")
					s = strings.ReplaceAll(s, "net.ipv4.conf.default.rp_filter = 1", "net.ipv4.conf.default.rp_filter = 2")
					os.WriteFile(conf, []byte(s), 0644)
				}
				return nil
			})

		// FORWARD policy: ufw defaults to DROP, breaks pod-to-pod traffic.
		fwdOut, _ := exec.Command("sudo", "iptables", "-L", "FORWARD").Output()
		fwdOK := strings.Contains(string(fwdOut), "policy ACCEPT")
		fixable("FORWARD policy ACCEPT", fwdOK,
			"set DEFAULT_FORWARD_POLICY=ACCEPT in /etc/default/ufw",
			func() error {
				exec.Command("sudo", "iptables", "-P", "FORWARD", "ACCEPT").Run()
				// persist
				conf := "/etc/default/ufw"
				if data, err := os.ReadFile(conf); err == nil {
					s := string(data)
					s = strings.ReplaceAll(s, `DEFAULT_FORWARD_POLICY="DROP"`, `DEFAULT_FORWARD_POLICY="ACCEPT"`)
					os.WriteFile(conf, []byte(s), 0644)
				}
				return nil
			})

		// Flannel VXLAN port (8472/udp) allowed in ufw.
		// Only relevant on multi-node k3s hosts (flannel.1 present) with ufw active.
		flannelIface, _ := exec.Command("ip", "link", "show", "flannel.1").Output()
		hasFlannel := strings.Contains(string(flannelIface), "flannel.1")
		ufwFull, _ := exec.Command("sudo", "ufw", "status").Output()
		flannelOK := true
		if hasFlannel && ufwActive {
			flannelOK = strings.Contains(string(ufwFull), "8472/udp")
		}
		fixable("Flannel 8472/udp allowed in ufw", flannelOK,
			"ufw allow 8472/udp (flannel VXLAN)",
			func() error {
				exec.Command("sudo", "ufw", "allow", "8472/udp", "comment", "Flannel VXLAN (k3s)").Run()
				exec.Command("sudo", "ufw", "reload").Run()
				return nil
			})

		// --- Summary ---
		fmt.Println()
		if issues == 0 && fixed == 0 {
			fmt.Println("All checks passed. VPS is healthy.")
		} else if doctorFix {
			fmt.Printf("%d issue(s) found, %d fixed. Re-run 'mf doctor' to verify.\n", issues, fixed)
		} else {
			fmt.Printf("%d issue(s) found. Run 'mf doctor --fix' to repair.\n", issues)
		}
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorFix, "fix", false, "Apply fixes for detected issues")
	doctorCmd.Flags().BoolVar(&doctorDryRun, "dry-run", false, "Show what --fix would do without applying")
	rootCmd.AddCommand(doctorCmd)
}
