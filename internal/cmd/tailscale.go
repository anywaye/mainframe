package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var tailscaleCmd = &cobra.Command{
	Use:   "tailscale",
	Short: "Manage Tailscale mesh VPN (machine-to-machine connectivity)",
}

var tailscaleStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Tailscale status and connected machines",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tailscale"); err != nil {
			fmt.Println("Tailscale not installed.")
			fmt.Println("  Install: brew install tailscale (macOS) or apt install tailscale (Linux)")
			return nil
		}
		out, err := exec.Command("tailscale", "status").Output()
		if err != nil {
			fmt.Printf("Tailscale: not connected or error: %v\n", err)
			return nil
		}
		fmt.Println("Tailscale Status:")
		fmt.Println(string(out))
		ip, _ := exec.Command("tailscale", "ip", "-4").Output()
		fmt.Printf("This machine's IP: %s\n", strings.TrimSpace(string(ip)))
		return nil
	},
}

var tailscaleUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Connect to Tailscale tailnet",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tailscale"); err != nil {
			return fmt.Errorf("tailscale not installed")
		}
		authkey := os.Getenv("TAILSCALE_AUTHKEY")
		if authkey == "" {
			// Try reading from file
			data, err := os.ReadFile("/var/lib/mainframe/tailscale-authkey")
			if err == nil {
				authkey = strings.TrimSpace(string(data))
			}
		}
		if authkey != "" {
			fmt.Println("Connecting to Tailscale with authkey...")
			upCmd := exec.Command("tailscale", "up", "--authkey="+authkey, "--ssh=false")
			upCmd.Stdout = os.Stdout
			upCmd.Stderr = os.Stderr
			if err := upCmd.Run(); err != nil {
				return fmt.Errorf("tailscale up failed: %w", err)
			}
		} else {
			fmt.Println("Connecting to Tailscale (interactive)...")
			upCmd := exec.Command("tailscale", "up", "--ssh=false")
			upCmd.Stdin = os.Stdin
			upCmd.Stdout = os.Stdout
			upCmd.Stderr = os.Stderr
			if err := upCmd.Run(); err != nil {
				return fmt.Errorf("tailscale up failed: %w", err)
			}
		}
		ip, _ := exec.Command("tailscale", "ip", "-4").Output()
		fmt.Printf("Connected. IP: %s\n", strings.TrimSpace(string(ip)))
		return nil
	},
}

var tailscaleDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Disconnect from Tailscale tailnet",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tailscale"); err != nil {
			return fmt.Errorf("tailscale not installed")
		}
		downCmd := exec.Command("tailscale", "down")
		downCmd.Stdout = os.Stdout
		downCmd.Stderr = os.Stderr
		return downCmd.Run()
	},
}

var tailscalePeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List machines on the tailnet",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tailscale"); err != nil {
			fmt.Println("Tailscale not installed.")
			return nil
		}
		out, err := exec.Command("tailscale", "status", "--json").Output()
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			return nil
		}
		// Parse and display peer info
		fmt.Println("Tailnet Peers:")
		fmt.Println(string(out))
		return nil
	},
}

var tailscaleIPCmd = &cobra.Command{
	Use:   "ip",
	Short: "Show this machine's Tailscale IP",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("tailscale"); err != nil {
			return fmt.Errorf("tailscale not installed")
		}
		out, err := exec.Command("tailscale", "ip", "-4").Output()
		if err != nil {
			return fmt.Errorf("not connected to Tailscale")
		}
		fmt.Printf("Tailscale IP: %s\n", strings.TrimSpace(string(out)))
		return nil
	},
}

func init() {
	tailscaleCmd.AddCommand(tailscaleStatusCmd)
	tailscaleCmd.AddCommand(tailscaleUpCmd)
	tailscaleCmd.AddCommand(tailscaleDownCmd)
	tailscaleCmd.AddCommand(tailscalePeersCmd)
	tailscaleCmd.AddCommand(tailscaleIPCmd)
	rootCmd.AddCommand(tailscaleCmd)
}
