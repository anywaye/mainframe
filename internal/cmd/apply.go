package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
	"strings"

	"github.com/anywaye/mainframe/internal/assets"
	"github.com/spf13/cobra"
)

var applyHost string
var applyForce bool
var applyDryRun bool
var applyOnly string
var applySkip string
var applyList bool
var applyMode string
var applyEnv string
var applyResume bool
var applyVerbose int

// stepFunc defines a named apply step.
type stepFunc struct {
	name string
	desc string
	fn   func(bool)
}

// allSteps returns the ordered list of apply steps.
func allSteps() []stepFunc {
	return []stepFunc{
		{"os-detect", "Detect operating system", func(dry bool) {
			fmt.Println("\n[1/18] Detecting OS...")
			fam := detectOSFamily("")
			fmt.Printf("  OS family: %s\n", fam)
		}},
		{"tailscale", "Check Tailscale connectivity", applyTailscale},
		{"runtimes", "Install language runtimes", applyRuntimes},
		{"custom-installs", "Run custom install scripts", applyCustomInstalls},
		{"shell", "Sync shell configuration", applyShellConfig},
		{"git", "Sync git configuration", applyGitConfig},
		{"editor", "Sync editor configuration", applyEditorConfig},
		{"terminal", "Sync terminal configuration", applyTerminalConfig},
		{"ssh", "Sync SSH configuration", applySSHConfig},
		{"secrets", "Unlock secrets vault and store session", applySecrets},
		{"ssh-keys", "Fetch SSH keys from password manager", applySSHKeys},
		{"gpg-keys", "Fetch GPG keys from password manager", applyGPGKeys},
		{"mcp", "Sync MCP configuration", applyMCPConfig},
		{"skills", "Sync AI agent skills", applySkills},
		{"bin", "Sync bin scripts", applyBinScripts},
		{"os-settings", "Apply OS-specific settings", func(dry bool) {
			fam := detectOSFamily("")
			applyOSSettings(dry, fam)
		}},
		{"macos-defaults", "Apply macOS defaults", func(dry bool) {
			fam := detectOSFamily("")
			applyMacOSDefaults(dry, fam)
		}},
		{"agents", "Sync agent configurations", applyAgentConfigs},
		{"chats", "Sync agent chat sessions", applyChatsConfig},
	}
}

var applyCmd = &cobra.Command{
	Use:   "apply [steps...]",
	Short: "Apply dev configuration to the current machine (advanced)",
	Long: `Reads mainframe.yml and applies dev configuration (shell, git, editor, terminal, MCP, skills, secrets) to the current machine. Idempotent - safe to run multiple times.

NOTE: For most users, use 'mf setup --dev' instead.
WARNING: Do not run on production servers. Use 'mf setup --prod' for production.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// List steps (doesn't need config dir)
		if applyList {
			fmt.Println("Available apply steps:")
			for _, s := range allSteps() {
				fmt.Printf("  %-16s %s\n", s.name, s.desc)
			}
			return nil
		}

		// Determine config directory
		configDir := "."
		if _, err := os.Stat("mainframe.yml"); err != nil {
			if dir := os.Getenv("MAINFRAME_DIR"); dir != "" {
				configDir = dir
			} else {
				home, _ := os.UserHomeDir()
				for _, d := range []string{".mainframe", "mainframe"} {
					if _, err := os.Stat(filepath.Join(home, d, "mainframe.yml")); err == nil {
						configDir = filepath.Join(home, d)
						break
					}
				}
			}
		}

		if _, err := os.Stat(filepath.Join(configDir, "mainframe.yml")); err != nil {
			return fmt.Errorf("no mainframe.yml found in %s. Run 'mf init <url>' or 'cd' to your config repo", configDir)
		}

		steps := allSteps()

		// Filter steps
		var selected []stepFunc
		if len(args) > 0 {
			// Specific steps requested
			stepMap := map[string]stepFunc{}
			for _, s := range steps {
				stepMap[s.name] = s
			}
			for _, arg := range args {
				if s, ok := stepMap[arg]; ok {
					selected = append(selected, s)
				} else {
					return fmt.Errorf("unknown step: %s (use --list to see available steps)", arg)
				}
			}
		} else {
			selected = steps
		}

		// In prod mode, skip dev-only steps
		if applyMode == "prod" {
			devOnly := map[string]bool{
				"shell": true, "git": true, "editor": true, "terminal": true,
				"mcp": true, "skills": true, "bin": true, "agents": true,
				"macos-defaults": true,
			}
			var filtered []stepFunc
			for _, step := range selected {
				if !devOnly[step.name] {
					filtered = append(filtered, step)
				} else {
					fmt.Printf("  Skipping %s (dev-only, prod mode)\n", step.name)
				}
			}
			selected = filtered
		}

		// Apply skip filter
		if applySkip != "" {
			skipSet := map[string]bool{}
			for _, s := range strings.Split(applySkip, ",") {
				skipSet[strings.TrimSpace(s)] = true
			}
			var filtered []stepFunc
			for _, s := range selected {
				if !skipSet[s.name] {
					filtered = append(filtered, s)
				}
			}
			selected = filtered
		}

		// Apply only filter
		if applyOnly != "" {
			onlySet := map[string]bool{}
			for _, s := range strings.Split(applyOnly, ",") {
				onlySet[strings.TrimSpace(s)] = true
			}
			var filtered []stepFunc
			for _, s := range selected {
				if onlySet[s.name] {
					filtered = append(filtered, s)
				}
			}
			selected = filtered
		}

		// Extract embedded framework lib to temp dir
		tmpDir, err := os.MkdirTemp("", "mainframe-lib-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		assetsWalk(tmpDir)
		_ = tmpDir

		origDir, _ := os.Getwd()
		os.Chdir(configDir)
		defer os.Chdir(origDir)

		os.Setenv("MAINFRAME_LIB", tmpDir)

		// Determine environment (--env takes priority, then --mode, then auto-detect)
		if applyEnv != "" {
			applyMode = applyEnv
		} else if applyMode != "" {
			// backward compat: --mode=prod/dev maps to env
			applyMode = applyMode
		} else {
			// Auto-detect from mainframe.yml or env var
			if envVar := os.Getenv("INFISICAL_ENV"); envVar != "" {
				applyMode = envVar
			} else {
				modeData, _ := os.ReadFile(filepath.Join(configDir, "mainframe.yml"))
				if strings.Contains(string(modeData), "mode: prod") {
					applyMode = "prod"
				} else {
					applyMode = "dev"
				}
			}
		}
		fmt.Printf("=== Applying mainframe config from %s (env: %s) ===\n", configDir, applyMode)
		if applyDryRun {
			fmt.Println("[DRY RUN - no changes will be made]")
		}
		if len(selected) < len(steps) {
			fmt.Printf("Running %d of %d steps\n", len(selected), len(steps))
		}

		// Initialize logger
		logger := initApplyLogger(applyVerbose)
		defer logger.close()

		// Load previous state for resume
		state := loadApplyState()
		if applyResume && state != nil {
			fmt.Printf("Resuming from previous run (%s)\n", state.LastRun.Format("2006-01-02 15:04:05"))
			completed := map[string]bool{}
			for _, s := range state.CompletedSteps {
				completed[s] = true
			}
			var remaining []stepFunc
			for _, step := range selected {
				if !completed[step.name] {
					remaining = append(remaining, step)
				} else {
					fmt.Printf("  Skipping %s (already completed)\n", step.name)
				}
			}
			selected = remaining
		}

		if state == nil {
			state = &applyState{ConfigDir: configDir}
		}

		for _, step := range selected {
			start := logger.startStep(step.name)
			step.fn(applyDryRun)
			logger.endStep(step.name, start, "done")

			// Update state
			state.CompletedSteps = append(state.CompletedSteps, step.name)
			state.LastRun = time.Now()
			state.ConfigDir = configDir
			saveApplyState(state)
		}

		// Clear state on successful completion
		if !applyDryRun {
			clearApplyState()
		}

		logger.printSummary()
		fmt.Println("\n=== Apply complete ===")
		return nil
	},
}

func assetsWalk(tmpDir string) {
	assets.Walk("lib", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		data, err := assets.ReadFile(path)
		if err != nil {
			return nil
		}
		relPath := strings.TrimPrefix(path, "lib/")
		destPath := filepath.Join(tmpDir, relPath)
		os.MkdirAll(filepath.Dir(destPath), 0755)
		os.WriteFile(destPath, data, 0755)
		return nil
	})
}

func init() {
	applyCmd.Flags().StringVar(&applyHost, "host", "", "Apply to remote host via SSH")
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "Force overwrite existing files")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Preview without applying")
	applyCmd.Flags().StringVar(&applyOnly, "only", "", "Run only these steps (comma-separated)")
	applyCmd.Flags().StringVar(&applySkip, "skip", "", "Skip these steps (comma-separated)")
	applyCmd.Flags().BoolVar(&applyList, "list", false, "List available steps")
	applyCmd.Flags().StringVar(&applyEnv, "env", "", "Environment: dev or prod (default: auto-detect)")
	applyCmd.Flags().StringVar(&applyMode, "mode", "", "Deprecated: use --env instead")
	applyCmd.Flags().BoolVar(&applyResume, "resume", false, "Resume from previous interrupted run")
	applyCmd.Flags().IntVarP(&applyVerbose, "verbose", "v", 0, "Verbose level (0=normal, 1=timing, 2=debug)")
	rootCmd.AddCommand(applyCmd)
}
