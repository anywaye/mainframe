package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
	"strings"

	"github.com/spf13/cobra"
)

var hardenSections string
var hardenList bool
var hardenForce bool

var hardenCmd = &cobra.Command{
	Use:   "harden [sections...]",
	Short: "Apply security hardening (sane defaults)",
	Long: `Apply mainframe's security hardening with sane defaults.
Optional extras via env vars.

Run all sections:
  mf harden

Run specific sections:
  mf harden ssh firewall
  mf harden ssh sysctl fail2ban

List available sections:
  mf harden --list

Skip the audit baseline:
  mf harden --skip-audit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		hardenPath := findAsset("lib/harden.sh")
		if hardenPath == "" {
			return fmt.Errorf("harden.sh not found")
		}

		if hardenList {
			fmt.Println("Available hardening sections:")
			sections := []struct{ name, desc string }{
				{"audit", "Lynis security audit (read-only baseline)"},
				{"ssh", "SSH hardening (key-only, AllowGroups, crypto)"},
				{"docker", "Docker daemon hardening (icc, logging)"},
				{"firewall", "Firewall rules (ufw, default deny)"},
				{"sysctl", "Kernel parameters (BBR, redirects, rp_filter)"},
				{"journald", "Log size limits"},
				{"unattended", "Automatic security updates"},
				{"needrestart", "Service restart on library updates"},
				{"swap", "Create swap file if missing"},
				{"etckeeper", "Git tracking of /etc changes"},
				{"deadman", "Deadman switch for safe changes"},
				{"fail2ban", "Intrusion prevention (auto-ban)"},
				{"tailscale", "Tailscale watchdog timer"},
				{"crash-prevention", "OOM protection, resource monitoring"},
			}
			for _, s := range sections {
				fmt.Printf("  %-18s %s\n", s.name, s.desc)
			}
			return nil
		}

		// Build sections env var
		sections := ""
		if len(args) > 0 {
			sections = strings.Join(args, ",")
		}
		if hardenSections != "" {
			sections = hardenSections
		}

		c := exec.Command("bash", hardenPath)
		c.Env = append(os.Environ(), "HARDEN_SECTIONS="+sections)
		if hardenForce {
			c.Env = append(c.Env, "FORCE_SSH=true")
		}

		if jsonMode {
			stdout, _ := c.StdoutPipe()
			stderr, _ := c.StderrPipe()
			if err := c.Start(); err != nil {
				return err
			}
			go func() {
				scanner := bufio.NewScanner(stderr)
				for scanner.Scan() {
					fmt.Fprintf(os.Stderr, "%s\n", scanner.Text())
				}
			}()
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				line := scanner.Text()
				event := map[string]interface{}{
					"type": "log",
					"ts":   time.Now().UTC().Format(time.RFC3339),
					"msg":  line,
				}
				data, _ := json.Marshal(event)
				fmt.Println(string(data))
			}
			return c.Wait()
		}

		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	hardenCmd.Flags().BoolVar(&jsonMode, "json", false, "Output as NDJSON (machine-parseable)")
	hardenCmd.Flags().StringVar(&hardenSections, "sections", "", "Comma-separated sections to run (or pass as args)")
	hardenCmd.Flags().BoolVar(&hardenList, "list", false, "List available sections")
	hardenCmd.Flags().BoolVar(&hardenForce, "force", false, "Force SSH hardening even if existing config found")
	rootCmd.AddCommand(hardenCmd)
}
