package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var uptimeCmd = &cobra.Command{
	Use:   "uptime",
	Short: "Set up uptime monitoring with external health check",
	Long:  "Configure a heartbeat cron job that pings an external monitoring service (e.g., healthchecks.io). If the VPS goes down, the ping stops and you get alerted.",
	RunE: func(cmd *cobra.Command, args []string) error {
		url, _ := cmd.Flags().GetString("url")
		status, _ := cmd.Flags().GetBool("status")
		remove, _ := cmd.Flags().GetBool("remove")

		if remove {
			fmt.Println("Removing uptime monitor...")
			c := exec.Command("bash", "-c", "crontab -l 2>/dev/null | grep -v 'mainframe-uptime' | crontab - 2>/dev/null || true")
			c.Run()
			os.Remove("/etc/cron.d/mainframe-uptime")
			fmt.Println("Uptime monitor removed.")
			return nil
		}

		if status {
			fmt.Println("=== Uptime Monitor Status ===")
			out, _ := exec.Command("bash", "-c", "crontab -l 2>/dev/null | grep 'mainframe-uptime' || echo 'not configured'").Output()
			fmt.Printf("Cron: %s\n", strings.TrimSpace(string(out)))
			// Test the URL if configured
			if url != "" {
				c := exec.Command("curl", "-fsS", "-o", "/dev/null", "-w", "%{http_code}", url)
				out, _ := c.Output()
				fmt.Printf("URL test: %s\n", strings.TrimSpace(string(out)))
			}
			return nil
		}

		// Setup
		if url == "" {
			url = os.Getenv("HEALTHCHECK_URL")
		}
		if url == "" {
			fmt.Println("No health check URL provided.")
			fmt.Println()
			fmt.Println("To set up uptime monitoring:")
			fmt.Println("  1. Create a check at https://healthchecks.io (free, 20 checks)")
			fmt.Println("  2. Set period to 5 minutes, grace to 5 minutes")
			fmt.Println("  3. Copy the ping URL (e.g., https://hc-ping.com/your-uuid)")
			fmt.Println("  4. Run: mf uptime --url https://hc-ping.com/your-uuid")
			fmt.Println()
			fmt.Println("Or set HEALTHCHECK_URL in your .env file.")
			return nil
		}

		fmt.Printf("Setting up uptime monitor with URL: %s\n", url)

		// Add cron job (every minute)
		cronLine := fmt.Sprintf("* * * * * curl -fsS -o /dev/null %s # mainframe-uptime", url)
		// Remove existing entries first
		exec.Command("bash", "-c", "crontab -l 2>/dev/null | grep -v 'mainframe-uptime' | crontab - 2>/dev/null || true").Run()
		// Add new entry
		c := exec.Command("bash", "-c", fmt.Sprintf("(crontab -l 2>/dev/null || true; echo '%s') | crontab -", cronLine))
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to set up cron: %w", err)
		}
		fmt.Println("Cron job installed (pings every minute).")

		// Test the URL
		fmt.Print("Testing URL... ")
		testCmd := exec.Command("curl", "-fsS", "-o", "/dev/null", "-w", "%{http_code}", url)
		out, err := testCmd.Output()
		if err != nil {
			fmt.Printf("FAILED (%v)\n", err)
			fmt.Println("The URL may be incorrect. Check and re-run.")
		} else {
			fmt.Printf("%s (OK)\n", strings.TrimSpace(string(out)))
		}

		fmt.Println()
		fmt.Println("Uptime monitor configured. If the VPS goes down, the ping stops")
		fmt.Println("and your monitoring service will alert you (Slack, email, etc.).")
		fmt.Println()
		fmt.Println("Recommended services:")
		fmt.Println("  - healthchecks.io (free, 20 checks, Slack alerts)")
		fmt.Println("  - Better Stack (free, 10 monitors, 3-min checks)")
		fmt.Println("  - UptimeRobot (free, 50 monitors, personal use only)")

		// Save URL to .env for reproducibility
		envFile := ".env"
		if _, err := os.Stat(envFile); err != nil {
			envFile = os.ExpandEnv("$HOME/.env")
		}
		if data, err := os.ReadFile(envFile); err == nil {
			content := string(data)
			if !strings.Contains(content, "HEALTHCHECK_URL") {
				content += fmt.Sprintf("\n# Uptime monitoring\nexport HEALTHCHECK_URL=\"%s\"\n", url)
				os.WriteFile(envFile, []byte(content), 0644)
				fmt.Printf("Saved HEALTHCHECK_URL to %s\n", envFile)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(uptimeCmd)
	uptimeCmd.Flags().String("url", "", "Health check URL (e.g., https://hc-ping.com/your-uuid)")
	uptimeCmd.Flags().Bool("status", false, "Show uptime monitor status")
	uptimeCmd.Flags().Bool("remove", false, "Remove uptime monitor")
}
