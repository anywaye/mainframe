package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between machine and config",
	Long:  "Compares the current machine state with the mainframe config and shows what's different.",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return fmt.Errorf("no mainframe.yml found in %s", configDir)
		}

		fmt.Printf("=== Diffing machine vs config (%s) ===\n", configDir)

		// Check packages
		fmt.Println("\n[packages]")
		if _, err := os.Stat(filepath.Join(configDir, "packages.conf")); err == nil {
			runDiffStep("packages", `
				if command -v brew &>/dev/null; then
					missing=""
					while IFS= read -r line; do
						line="${line%%#*}"
						line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
						[ -z "$line" ] && continue
						pkg="${line%%:*}"
						if ! command -v "$pkg" &>/dev/null && ! brew list "$pkg" &>/dev/null 2>&1; then
							missing="$missing $pkg"
						fi
					done < packages.conf
					if [ -n "$missing" ]; then
						echo "  Missing:$missing"
					else
						echo "  All packages installed"
					fi
				fi
			`, configDir)
		}

		// Check runtimes
		fmt.Println("\n[runtimes]")
		if _, err := os.Stat(filepath.Join(configDir, "runtimes.conf")); err == nil {
			runDiffStep("runtimes", `
				if command -v mise &>/dev/null; then
					while IFS= read -r line; do
						line="${line%%#*}"
						line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
						[ -z "$line" ] && continue
						if echo "$line" | grep -q ' ' && ! echo "$line" | grep -q '@'; then
							line=$(echo "$line" | sed 's/ /@/')
						fi
						tool="${line%%@*}"
						if ! mise ls 2>/dev/null | grep -q "^$tool"; then
							echo "  Missing: $line"
						fi
					done < runtimes.conf
				fi
			`, configDir)
		}

		// Check shell config
		fmt.Println("\n[shell]")
		if _, err := os.Stat(filepath.Join(configDir, "shell")); err == nil {
			// Check if ~/.zshrc is symlinked
			home, _ := os.UserHomeDir()
			zshrc := filepath.Join(home, ".zshrc")
			if info, err := os.Lstat(zshrc); err == nil {
				if info.Mode()&os.ModeSymlink != 0 {
					fmt.Println("  ~/.zshrc: symlinked ✓")
				} else {
					fmt.Println("  ~/.zshrc: exists but not symlinked (will be replaced on apply)")
				}
			} else {
				fmt.Println("  ~/.zshrc: missing")
			}
		}

		// Check git config
		fmt.Println("\n[git]")
		home, _ := os.UserHomeDir()
		gitconfig := filepath.Join(home, ".gitconfig")
		if info, err := os.Lstat(gitconfig); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				fmt.Println("  ~/.gitconfig: symlinked ✓")
			} else {
				fmt.Println("  ~/.gitconfig: exists but not symlinked")
			}
		} else {
			fmt.Println("  ~/.gitconfig: missing")
		}

		// Check MCP config
		fmt.Println("\n[mcp]")
		mcpPath := filepath.Join(home, ".zcode", "cli", "config.json")
		if _, err := os.Stat(mcpPath); err == nil {
			fmt.Println("  ~/.zcode/cli/config.json: exists ✓")
		} else {
			fmt.Println("  ~/.zcode/cli/config.json: missing")
		}

		// Check skills
		fmt.Println("\n[skills]")
		skillsPath := filepath.Join(home, ".agents", "skills")
		if entries, err := os.ReadDir(skillsPath); err == nil {
			fmt.Printf("  ~/.agents/skills/: %d skills installed\n", len(entries))
		} else {
			fmt.Println("  ~/.agents/skills/: missing")
		}

		fmt.Println("\n=== Diff complete ===")
		return nil
	},
}

func runDiffStep(name, script, configDir string) {
	origDir, _ := os.Getwd()
	os.Chdir(configDir)
	defer os.Chdir(origDir)

	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
