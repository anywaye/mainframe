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
			fmt.Println("\n[1/14] Detecting OS...")
			fam := detectOSFamily("")
			fmt.Printf("  OS family: %s\n", fam)
		}},
		{"packages", "Install system packages", applyPackages},
		{"runtimes", "Install language runtimes", applyRuntimes},
		{"custom-installs", "Run custom install scripts", applyCustomInstalls},
		{"shell", "Sync shell configuration", applyShellConfig},
		{"git", "Sync git configuration", applyGitConfig},
		{"editor", "Sync editor configuration", applyEditorConfig},
		{"terminal", "Sync terminal configuration", applyTerminalConfig},
		{"ssh", "Sync SSH configuration", applySSHConfig},
		{"secrets", "Unlock secrets vault and store session", applySecrets},
		{"ssh-keys", "Fetch SSH keys from password manager", applySSHKeys},
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
	}
}

var applyCmd = &cobra.Command{
	Use:   "apply [steps...]",
	Short: "Apply machine configuration to the current machine",
	Long: `Reads mainframe.yml and applies configuration to the current machine.
Idempotent - safe to run multiple times.

Run all steps:
  mf apply

Run specific steps:
  mf apply packages runtimes
  mf apply shell git
  mf apply ssh

Skip steps:
  mf apply --skip packages,runtimes

List available steps:
  mf apply --list`,
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

		fmt.Printf("=== Applying mainframe config from %s ===\n", configDir)
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
	applyCmd.Flags().BoolVar(&applyResume, "resume", false, "Resume from previous interrupted run")
	applyCmd.Flags().IntVarP(&applyVerbose, "verbose", "v", 0, "Verbose level (0=normal, 1=timing, 2=debug)")
	rootCmd.AddCommand(applyCmd)
}
