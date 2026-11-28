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

// defaultFleetName returns the default fleet name.
func defaultFleetName() string {
	return "dev"
}

// chatDB describes one agent chat database/dir to sync between machines.
type chatDB struct {
	label       string   // human label for progress output
	remote      []string // candidate paths on the peer (first existing wins)
	local       string   // local path (tilde form)
	mergeTables []string // tables to INSERT OR IGNORE (empty for dirs)
	isDir       bool
}

// builtinChatDBs returns the built-in chat paths for known agents.
// NOTE: the ZCode CLI database lives at ~/.zcode/cli/db/db.sqlite since the
// CLI restructure; the legacy ~/.zcode/cli/db.sqlite path is still probed so
// older installs keep syncing.
func builtinChatDBs() []chatDB {
	return []chatDB{
		{
			label:       "zcode/tasks-index",
			remote:      []string{"~/.zcode/v2/tasks-index.sqlite"},
			local:       "~/.zcode/v2/tasks-index.sqlite",
			mergeTables: []string{"tasks", "projects", "project_roots"},
		},
		{
			label:       "zcode/cli-db",
			remote:      []string{"~/.zcode/cli/db/db.sqlite", "~/.zcode/cli/db.sqlite"},
			local:       "~/.zcode/cli/db/db.sqlite",
			mergeTables: []string{"session", "message", "part"},
		},
		{
			label: "zcode/rollout",
			remote: []string{"~/.zcode/cli/rollout/"},
			local:  "~/.zcode/cli/rollout/",
			isDir:  true,
		},
		{
			label:       "opencode/db",
			remote:      []string{"~/.opencode/opencode.db"},
			local:       "~/.opencode/opencode.db",
			mergeTables: []string{"sessions"},
		},
	}
}

// agentChatDBs returns the chat DBs to sync: built-ins for known agents plus
// any additional agents that ship agents/<name>/chats/paths.conf in the config
// repo (so future agents work without code changes).
func agentChatDBs(configDir string) []chatDB {
	dbs := builtinChatDBs()
	if configDir == "" || configDir == "." {
		return dbs
	}
	entries, err := os.ReadDir(filepath.Join(configDir, "agents"))
	if err != nil {
		return dbs
	}
	known := map[string]bool{"zcode": true, "opencode": true}
	for _, e := range entries {
		if !e.IsDir() || known[e.Name()] {
			continue
		}
		conf := filepath.Join(configDir, "agents", e.Name(), "chats", "paths.conf")
		data, err := os.ReadFile(conf)
		if err != nil {
			continue
		}
		inPaths := false
		for _, line := range strings.Split(string(data), "\n") {
			s := strings.TrimSpace(line)
			if strings.HasPrefix(s, "PATHS=(") {
				inPaths = true
				continue
			}
			if inPaths && strings.HasPrefix(s, ")") {
				inPaths = false
				continue
			}
			if !inPaths {
				continue
			}
			s = strings.Trim(s, "()\" ")
			if s == "" || strings.HasPrefix(s, "#") {
				continue
			}
			if strings.HasSuffix(s, "/") {
				dbs = append(dbs, chatDB{label: e.Name() + "/" + filepath.Base(strings.TrimSuffix(s, "/")), remote: []string{s}, local: s, isDir: true})
			} else {
				dbs = append(dbs, chatDB{label: e.Name() + "/" + filepath.Base(s), remote: []string{s}, local: s, mergeTables: []string{"*"}})
			}
		}
	}
	return dbs
}

func expandHome(p string) string {
	home, _ := os.UserHomeDir()
	return strings.Replace(p, "~", home, 1)
}

// sqliteTables lists table names in a SQLite database.
func sqliteTables(path string) (map[string]bool, error) {
	out, err := exec.Command("sqlite3", path, "SELECT name FROM sqlite_master WHERE type='table';").Output()
	if err != nil {
		return nil, err
	}
	tables := map[string]bool{}
	for _, name := range strings.Fields(string(out)) {
		tables[name] = true
	}
	return tables, nil
}

// mergeSQLiteInto merges the given tables from tmpPath into localPath using
// INSERT OR IGNORE, skipping tables missing on either side. Never deletes.
func mergeSQLiteInto(localPath, tmpPath string, tables []string) error {
	remoteTables, err := sqliteTables(tmpPath)
	if err != nil {
		return fmt.Errorf("remote snapshot unreadable: %w", err)
	}
	localTables, err := sqliteTables(localPath)
	if err != nil {
		return fmt.Errorf("local db unreadable: %w", err)
	}
	sql := "PRAGMA busy_timeout=60000;ATTACH DATABASE '" + tmpPath + "' AS r;"
	used := 0
	for _, t := range tables {
		if t == "*" {
			for name := range remoteTables {
				if strings.HasPrefix(name, "_") || localTables[name] {
					sql += fmt.Sprintf("INSERT OR IGNORE INTO main.%s SELECT * FROM r.%s;", name, name)
					used++
				}
			}
			continue
		}
		if remoteTables[t] && localTables[t] {
			sql += fmt.Sprintf("INSERT OR IGNORE INTO main.%s SELECT * FROM r.%s;", t, t)
			used++
		}
	}
	if used == 0 {
		return fmt.Errorf("no mergeable tables")
	}
	if out, err := exec.Command("sqlite3", localPath, sql).CombinedOutput(); err != nil {
		return fmt.Errorf("merge failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
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

				for _, c := range agentChatDBs(configDir) {
					if c.isDir {
						localPath := expandHome(c.local)
						os.MkdirAll(localPath, 0o755)
						rsyncCmd := exec.Command("rsync", "-az", "--ignore-existing",
							fmt.Sprintf("%s:%s", m.Host, c.remote[0]), localPath)
						if out, err := rsyncCmd.CombinedOutput(); err != nil {
							if !strings.Contains(string(out), "No such file") {
								fmt.Printf("    Warning: rsync %s: %v\n", c.remote[0], err)
							}
						}
						continue
					}

					localPath := expandHome(c.local)
					// Prefer an existing local DB at the legacy location when the
					// new one doesn't exist yet.
					if _, err := os.Stat(localPath); err != nil {
						for _, cand := range c.remote {
							lp := expandHome(cand)
							if _, err := os.Stat(lp); err == nil {
								localPath = lp
								break
							}
						}
					}
					if dir := filepath.Dir(localPath); dir != "" {
						os.MkdirAll(dir, 0o755)
					}

					// Pull the first remote candidate that exists.
					pulled_any := false
					for _, cand := range c.remote {
						tmpPath := expandHome(c.local) + ".remote.tmp"
						rsyncCmd := exec.Command("rsync", "-az",
							fmt.Sprintf("%s:%s", m.Host, cand), tmpPath)
						if out, err := rsyncCmd.CombinedOutput(); err != nil {
							if !strings.Contains(string(out), "No such file") {
								fmt.Printf("    Warning: rsync %s: %v\n", cand, err)
							}
							continue
						}
						pulled_any = true
						// bring WAL siblings along so the snapshot is complete
						for _, suffix := range []string{"-wal", "-shm"} {
							walTmp := tmpPath + suffix
							exec.Command("rsync", "-az", fmt.Sprintf("%s:%s%s", m.Host, cand, suffix), walTmp).Run()
						}
						if _, err := os.Stat(localPath); err == nil {
							if err := mergeSQLiteInto(localPath, tmpPath, c.mergeTables); err != nil {
								fmt.Printf("    Warning: merge %s: %v\n", c.label, err)
							}
						} else {
							os.Rename(tmpPath, localPath)
							for _, suffix := range []string{"-wal", "-shm"} {
								if _, err := os.Stat(tmpPath + suffix); err == nil {
									os.Rename(tmpPath+suffix, localPath+suffix)
								}
							}
						}
						os.Remove(tmpPath)
						for _, suffix := range []string{"-wal", "-shm"} {
							os.Remove(tmpPath + suffix)
						}
						break
					}
					if !pulled_any {
						fmt.Printf("    (no %s on peer — skipping)\n", c.label)
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
// Push MERGES into the peer's databases — it never overwrites them, so each
// machine keeps its own sessions alongside the pushed ones.
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

				for _, c := range agentChatDBs(configDir) {
					localPath := expandHome(c.local)
					if c.isDir {
						rsyncCmd := exec.Command("rsync", "-az", "--ignore-existing", localPath,
							fmt.Sprintf("%s:%s", m.Host, c.remote[0]))
						if out, err := rsyncCmd.CombinedOutput(); err != nil {
							if !strings.Contains(string(out), "No such file") {
								fmt.Printf("    Warning: rsync %s: %v\n", c.local, err)
							}
						}
						continue
					}
					if _, err := os.Stat(localPath); err != nil {
						continue
					}

					// WAL-safe local snapshot
					snapshot := localPath + ".push.snap"
					if out, err := exec.Command("sqlite3", localPath, ".backup '"+snapshot+"'").CombinedOutput(); err != nil {
						fmt.Printf("    Warning: snapshot %s: %v: %s\n", c.local, err, strings.TrimSpace(string(out)))
						continue
					}
					defer os.Remove(snapshot)

					// First push to a NEW peer DB: send the snapshot as-is.
					// Otherwise: send to a temp path and MERGE on the peer.
					peerTarget := ""
					for _, cand := range c.remote {
						if _, err := exec.Command("ssh", m.Host, "test", "-e", cand).CombinedOutput(); err == nil {
							peerTarget = cand
							break
						}
					}
					if peerTarget == "" {
						peerTarget = c.remote[0]
						rsyncCmd := exec.Command("rsync", "-az", snapshot,
							fmt.Sprintf("%s:%s", m.Host, peerTarget))
						if _, err := rsyncCmd.CombinedOutput(); err != nil {
							fmt.Printf("    Warning: rsync %s: %v\n", peerTarget, err)
						}
						continue
					}

					peerTmp := peerTarget + ".incoming"
					rsyncCmd := exec.Command("rsync", "-az", snapshot, fmt.Sprintf("%s:%s", m.Host, peerTmp))
					if _, err := rsyncCmd.CombinedOutput(); err != nil {
						fmt.Printf("    Warning: rsync %s: %v\n", peerTmp, err)
						continue
					}
					// Merge on the peer via stdin SQL (missing tables there are
					// reported but don't abort the remaining statements).
					var sql strings.Builder
					sql.WriteString("PRAGMA busy_timeout=60000;ATTACH DATABASE '" + peerTmp + "' AS r;")
					for _, t := range c.mergeTables {
						if t == "*" {
							continue
						}
						sql.WriteString(fmt.Sprintf("INSERT OR IGNORE INTO main.%s SELECT * FROM r.%s;", t, t))
					}
					mergeCmd := exec.Command("ssh", m.Host, "sqlite3", peerTarget)
					mergeCmd.Stdin = strings.NewReader(sql.String())
					if out, err := mergeCmd.CombinedOutput(); err != nil {
						fmt.Printf("    Warning: peer merge %s: %v: %s\n", c.label, err, strings.TrimSpace(string(out)))
					}
					exec.Command("ssh", m.Host, "rm", "-f", peerTmp, peerTmp+"-wal", peerTmp+"-shm").Run()
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
