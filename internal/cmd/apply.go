package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/anywaye/mainframe/internal/assets"
	"github.com/spf13/cobra"
)

var applyHost string
var applyForce bool
var applyDryRun bool

var applyCmd = &cobra.Command{
	Use:   "apply",
	Short: "Apply machine configuration to the current machine",
	Long:  "Reads mainframe.yml and applies all configuration (packages, runtimes, shell, git, editor, terminal, MCP, skills, secrets) to the current machine. Idempotent - safe to run multiple times.",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		// Extract embedded framework lib to temp dir
		tmpDir, err := os.MkdirTemp("", "mainframe-lib-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		// Extract all embedded lib files
		assetsWalk(tmpDir)

		frameworkLib := tmpDir

		origDir, _ := os.Getwd()
		os.Chdir(configDir)
		defer os.Chdir(origDir)

		fmt.Printf("=== Applying mainframe config from %s ===\n", configDir)
		if applyDryRun {
			fmt.Println("[DRY RUN - no changes will be made]")
		}

		// Step 1: Detect OS
		fmt.Println("\n[1/14] Detecting OS...")
		osFamily := detectOSFamily(frameworkLib)
		fmt.Printf("  OS family: %s\n", osFamily)

		// Step 2-3: Install packages and runtimes
		applyPackages(applyDryRun)
		applyRuntimes(applyDryRun)

		// Step 4-11: Sync configs
		applyShellConfig(applyDryRun)
		applyGitConfig(applyDryRun)
		applyEditorConfig(applyDryRun)
		applyTerminalConfig(applyDryRun)
		applySSHConfig(applyDryRun)
		applySSHKeys(applyDryRun)
		applyMCPConfig(applyDryRun)
		applySkills(applyDryRun)
		applyBinScripts(applyDryRun)

		// Step 12-14: OS settings, macOS defaults, agent configs
		applyOSSettings(applyDryRun, osFamily)
		applyMacOSDefaults(applyDryRun, osFamily)
		applyAgentConfigs(applyDryRun)

		fmt.Println("\n=== Apply complete ===")
		return nil
	},
}

func assetsWalk(tmpDir string) {
	// Walk embedded assets and extract lib/ files
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
	rootCmd.AddCommand(applyCmd)
}
