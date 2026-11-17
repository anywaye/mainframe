package cmd

import (
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var deadmanCmd = &cobra.Command{
	Use:   "deadman",
	Short: "Arm/disarm the deadman switch for safe config changes",
	Long: `Arm a timer that automatically rolls back SSH/firewall/sudo/PAM config
unless disarmed. Use this before making risky changes.

Examples:
  mf deadman arm 5       # arm for 5 minutes
  mf deadman disarm      # disarm (keep changes)
  mf deadman status      # show timer status
  mf deadman extend 10   # extend by 10 minutes
  mf deadman restore     # roll back now`,
}

var deadmanArmCmd = &cobra.Command{
	Use:   "arm [minutes]",
	Short: "Arm the deadman timer (default: 5 minutes)",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		mins := "5"
		if len(args) > 0 {
			mins = args[0]
		}
		ip, _ := cmd.Flags().GetString("ip")
		cmdArgs := []string{"arm", mins}
		if ip != "" {
			cmdArgs = append(cmdArgs, "--ip", ip)
		}
		c := exec.Command("/usr/local/sbin/mainframe-deadman", cmdArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
	},
}

var deadmanDisarmCmd = &cobra.Command{
	Use:   "disarm",
	Short: "Disarm the deadman timer (keep changes)",
	Run: func(cmd *cobra.Command, args []string) {
		c := exec.Command("/usr/local/sbin/mainframe-deadman", "disarm")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
	},
}

var deadmanStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deadman timer status",
	Run: func(cmd *cobra.Command, args []string) {
		c := exec.Command("/usr/local/sbin/mainframe-deadman", "status")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
	},
}

var deadmanExtendCmd = &cobra.Command{
	Use:   "extend [minutes]",
	Short: "Extend the deadman timer",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		c := exec.Command("/usr/local/sbin/mainframe-deadman", "extend", args[0])
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
	},
}

var deadmanRestoreCmd = &cobra.Command{
	Use:   "restore [dir]",
	Short: "Roll back config now",
	Run: func(cmd *cobra.Command, args []string) {
		cmdArgs := []string{"restore"}
		cmdArgs = append(cmdArgs, args...)
		c := exec.Command("/usr/local/sbin/mainframe-deadman", cmdArgs...)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
	},
}

var deadmanSnapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Take a config snapshot without arming",
	Run: func(cmd *cobra.Command, args []string) {
		c := exec.Command("/usr/local/sbin/mainframe-deadman", "snapshot")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Run()
	},
}

func init() {
	deadmanArmCmd.Flags().String("ip", "", "Admin IP for fail2ban unbanning")
	deadmanCmd.AddCommand(deadmanArmCmd)
	deadmanCmd.AddCommand(deadmanDisarmCmd)
	deadmanCmd.AddCommand(deadmanStatusCmd)
	deadmanCmd.AddCommand(deadmanExtendCmd)
	deadmanCmd.AddCommand(deadmanRestoreCmd)
	deadmanCmd.AddCommand(deadmanSnapshotCmd)
	rootCmd.AddCommand(deadmanCmd)
	_ = strings.TrimSpace
}
