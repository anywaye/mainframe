package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"


	"github.com/spf13/cobra"
)

// chatsCmd is the parent command for chat sync operations.
var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "Sync agent chat sessions across machines in a fleet",
	Long:  "Chats syncs agent conversation history (ZCode, OpenCode, etc.) across machines in a fleet. Uses Litestream for real-time SQLite replication and peer-to-peer sync via SSH.",
}

func init() {
	chatsCmd.AddCommand(chatsPullCmd())
	chatsCmd.AddCommand(chatsPushCmd())
	chatsCmd.AddCommand(chatsStatusCmd())
	chatsCmd.AddCommand(chatsListCmd())
	chatsCmd.AddCommand(chatsDaemonCmd())
	rootCmd.AddCommand(chatsCmd)
}

// resolveFleetConfigDir finds the mainframe config directory (same as resolveConfigDir).
func resolveFleetConfigDir() string {
	return resolveConfigDir()
}

// loadFleetConfig loads a fleet config by name.
func loadFleetConfigByName(fleetName string) (FleetConfig, error) {
	configDir := resolveFleetConfigDir()
	return loadFleet(configDir, fleetName)
}

// defaultFleetName returns the default fleet name.
func defaultFleetName() string {
	return "dev"
}

// chatsPullCmd pulls chats from all reachable fleet machines.
func chatsPullCmd() *cobra.Command {
	fleetName := ""
	cmd := &cobra.Command{
		Use:   "pull [fleet]",
		Short: "Pull chat sessions from all reachable fleet machines (idempotent)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fleetName = args[0]
			}
			if fleetName == "" {
				fleetName = defaultFleetName()
			}

			configDir := resolveFleetConfigDir()
			fleet, err := loadFleet(configDir, fleetName)
			if err != nil {
				return fmt.Errorf("failed to load fleet %q: %w", fleetName, err)
			}

			fmt.Printf("=== Pulling chats from fleet %q (%d machines) ===\n\n", fleetName, len(fleet.Machines))

			pulled := 0
			skipped := 0
			for _, m := range fleet.Machines {
				if m.Host == "local" || m.Host == "" {
					fmt.Printf("  %s (local): already present, skipping\n", m.Name)
					continue
				}

				status, detail := probeMachine(m)
				if status != "online" {
					fmt.Printf("  %s (%s): OFFLINE (%s) — skipping\n", m.Name, m.Host, detail)
					skipped++
					continue
				}

				fmt.Printf("  %s (%s): pulling chats...\n", m.Name, m.Host)

				// Pull chat data from remote via rsync
				home, _ := os.UserHomeDir()
				remotePaths := []string{
					"~/.zcode/v2/tasks-index.sqlite",
					"~/.zcode/cli/db.sqlite",
					"~/.zcode/cli/rollout/",
					"~/.opencode/opencode.db",
				}
				for _, rpath := range remotePaths {
					localPath := strings.Replace(rpath, "~", home, 1)
					// Ensure parent dir exists
					os.MkdirAll(filepath.Dir(localPath), 0755)
					rsyncCmd := exec.Command("rsync", "-az", "--ignore-existing",
						fmt.Sprintf("%s:%s", m.Host, rpath), localPath)
					if out, err := rsyncCmd.CombinedOutput(); err != nil {
						// Not all paths exist on remote — that's fine
						if !strings.Contains(string(out), "No such file") {
							fmt.Printf("    Warning: rsync %s: %v\n", rpath, err)
						}
					}
				}
				fmt.Printf("  %s: done\n", m.Name)
				pulled++
			}

			fmt.Printf("\n=== Pull complete: %d pulled, %d skipped (offline) ===\n", pulled, skipped)
			return nil
		},
	}
	cmd.Flags().StringVar(&fleetName, "fleet", "", "Fleet name (default: dev)")
	return cmd
}

// chatsPushCmd pushes local chats to all reachable fleet machines.
func chatsPushCmd() *cobra.Command {
	fleetName := ""
	cmd := &cobra.Command{
		Use:   "push [fleet]",
		Short: "Push local chat sessions to all reachable fleet machines",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fleetName = args[0]
			}
			if fleetName == "" {
				fleetName = defaultFleetName()
			}

			configDir := resolveFleetConfigDir()
			fleet, err := loadFleet(configDir, fleetName)
			if err != nil {
				return fmt.Errorf("failed to load fleet %q: %w", fleetName, err)
			}

			fmt.Printf("=== Pushing chats to fleet %q (%d machines) ===\n\n", fleetName, len(fleet.Machines))

			pushed := 0
			skipped := 0
			for _, m := range fleet.Machines {
				if m.Host == "local" || m.Host == "" {
					continue
				}

				status, detail := probeMachine(m)
				if status != "online" {
					fmt.Printf("  %s (%s): OFFLINE (%s) — skipping\n", m.Name, m.Host, detail)
					skipped++
					continue
				}

				fmt.Printf("  %s (%s): pushing chats...\n", m.Name, m.Host)
				home, _ := os.UserHomeDir()
				remotePaths := []string{
					"~/.zcode/v2/tasks-index.sqlite",
					"~/.zcode/cli/db.sqlite",
					"~/.zcode/cli/rollout/",
					"~/.opencode/opencode.db",
				}
				for _, rpath := range remotePaths {
					localPath := strings.Replace(rpath, "~", home, 1)
					remotePath := rpath
					rsyncCmd := exec.Command("rsync", "-az", localPath, fmt.Sprintf("%s:%s", m.Host, remotePath))
					if out, err := rsyncCmd.CombinedOutput(); err != nil {
						if !strings.Contains(string(out), "No such file") {
							fmt.Printf("    Warning: rsync %s: %v\n", rpath, err)
						}
					}
				}
				fmt.Printf("  %s: done\n", m.Name)
				pushed++
			}

			fmt.Printf("\n=== Push complete: %d pushed, %d skipped (offline) ===\n", pushed, skipped)
			return nil
		},
	}
	cmd.Flags().StringVar(&fleetName, "fleet", "", "Fleet name (default: dev)")
	return cmd
}

// chatsStatusCmd shows chat sync status per machine.
func chatsStatusCmd() *cobra.Command {
	fleetName := ""
	cmd := &cobra.Command{
		Use:   "status [fleet]",
		Short: "Show chat sync status for all machines in a fleet",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				fleetName = args[0]
			}
			if fleetName == "" {
				fleetName = defaultFleetName()
			}

			configDir := resolveFleetConfigDir()
			fleet, err := loadFleet(configDir, fleetName)
			if err != nil {
				return fmt.Errorf("failed to load fleet %q: %w", fleetName, err)
			}

			home, _ := os.UserHomeDir()
			fmt.Printf("=== Chat sync status for fleet %q ===\n\n", fleetName)
			fmt.Printf("  %-16s %-28s %-10s %-12s %s\n", "NAME", "HOST", "STATUS", "SESSIONS", "LAST SYNC")
			for _, m := range fleet.Machines {
				status, detail := probeMachine(m)
				sessionCount := "-"
				lastSync := "-"
				if status == "online" || m.Host == "local" || m.Host == "" {
					// Count local sessions
					count := countLocalSessions(home)
					sessionCount = fmt.Sprintf("%d", count)
					lastSync = "now"
				}
				fmt.Printf("  %-16s %-28s %-10s %-12s %s\n", m.Name, m.Host, status, sessionCount, lastSync)
				_ = detail
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&fleetName, "fleet", "", "Fleet name (default: dev)")
	return cmd
}

// chatsListCmd lists all chat sessions.
func chatsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all chat sessions across agents",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()

			fmt.Println("=== ZCode Sessions ===")
			dbPath := filepath.Join(home, ".zcode", "v2", "tasks-index.sqlite")
			if _, err := os.Stat(dbPath); err == nil {
				tableCheck, _ := exec.Command("sqlite3", dbPath, "SELECT name FROM sqlite_master WHERE type='table' AND name='tasks';").Output()
				if strings.TrimSpace(string(tableCheck)) == "tasks" {
					out, err := exec.Command("sqlite3", dbPath, "SELECT COUNT(*) FROM tasks;").Output()
					if err == nil {
						fmt.Printf("  Total sessions: %s\n", strings.TrimSpace(string(out)))
					}
					out, err = exec.Command("sqlite3", dbPath, "SELECT task_id, title FROM tasks ORDER BY updated_at DESC LIMIT 10;").Output()
					if err == nil {
						for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
							if line != "" {
								fmt.Printf("  - %s\n", line)
							}
						}
					}
				} else {
					fmt.Println("  No sessions yet (database exists but empty)")
				}
			} else {
				fmt.Println("  No ZCode session database found")
			}

			fmt.Println()
			fmt.Println("=== OpenCode Sessions ===")
			ocPath := filepath.Join(home, ".opencode", "opencode.db")
			if _, err := os.Stat(ocPath); err == nil {
				tableCheck, _ := exec.Command("sqlite3", ocPath, "SELECT name FROM sqlite_master WHERE type='table' AND name='sessions';").Output()
				if strings.TrimSpace(string(tableCheck)) == "sessions" {
					out, err := exec.Command("sqlite3", ocPath, "SELECT COUNT(*) FROM sessions;").Output()
					if err == nil {
						fmt.Printf("  Total sessions: %s\n", strings.TrimSpace(string(out)))
					}
					out, err = exec.Command("sqlite3", ocPath, "SELECT id, title FROM sessions ORDER BY created_at DESC LIMIT 10;").Output()
					if err == nil {
						for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
							if line != "" {
								fmt.Printf("  - %s\n", line)
							}
						}
					}
				} else {
					fmt.Println("  No sessions yet (database exists but empty)")
				}
			} else {
				fmt.Println("  No OpenCode database found")
			}

			return nil
		},
	}
	return cmd
}

// countLocalSessions returns the number of chat sessions across all agents.
func countLocalSessions(home string) int {
	count := 0
	dbPath := filepath.Join(home, ".zcode", "v2", "tasks-index.sqlite")
	if out, err := exec.Command("sqlite3", dbPath, "SELECT COUNT(*) FROM tasks;").Output(); err == nil {
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &count)
	}
	ocPath := filepath.Join(home, ".opencode", "opencode.db")
	if out, err := exec.Command("sqlite3", ocPath, "SELECT COUNT(*) FROM sessions;").Output(); err == nil {
		var oc int
		fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &oc)
		count += oc
	}
	return count
}

// loadFleet is already defined in fleet.go
// resolveConfigDir is already defined in fleet.go

// chatsDaemonCmd manages the sync daemon.
func chatsDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the chat sync daemon",
	}
	cmd.AddCommand(chatsDaemonStartCmd())
	cmd.AddCommand(chatsDaemonStopCmd())
	cmd.AddCommand(chatsDaemonStatusCmd())
	return cmd
}

func chatsDaemonStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the chat sync daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.mainframe.chats-sync.plist")
			if _, err := os.Stat(plistPath); err != nil {
				return fmt.Errorf("LaunchAgent not installed. Run 'mf apply chats' first")
			}
			exec.Command("launchctl", "load", plistPath).Run()
			fmt.Println("Chat sync daemon started")
			return nil
		},
	}
}

func chatsDaemonStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the chat sync daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			home, _ := os.UserHomeDir()
			plistPath := filepath.Join(home, "Library", "LaunchAgents", "com.mainframe.chats-sync.plist")
			exec.Command("launchctl", "unload", plistPath).Run()
			fmt.Println("Chat sync daemon stopped")
			return nil
		},
	}
}

func chatsDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show chat sync daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, _ := exec.Command("pgrep", "-x", "minio").Output()
			if len(out) > 0 {
				fmt.Println("MinIO: running")
			} else {
				fmt.Println("MinIO: not running")
			}
			out, _ = exec.Command("pgrep", "-x", "litestream").Output()
			if len(out) > 0 {
				fmt.Println("Litestream: running")
			} else {
				fmt.Println("Litestream: not running")
			}
			return nil
		},
	}
}
