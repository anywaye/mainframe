package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

		home, _ := os.UserHomeDir()

		fmt.Printf("=== Diffing machine vs config (%s) ===\n", configDir)

		// Check packages
		fmt.Println("\n[packages]")
		if _, err := os.Stat(filepath.Join(configDir, "packages.conf")); err == nil {
			runDiffStep("packages", `
				missing=""
				while IFS= read -r line; do
					line="${line%%#*}"
					line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
					[ -z "$line" ] && continue
					pkg="${line%%:*}"
					# Check command aliases
					case "$pkg" in
						ripgrep) cmd="rg" ;;
						neovim) cmd="nvim" ;;
						postgresql) cmd="psql" ;;
						*) cmd="$pkg" ;;
					esac
					if ! command -v "$cmd" &>/dev/null && ! brew list "$pkg" &>/dev/null 2>&1; then
						missing="$missing $pkg"
					fi
				done < packages.conf
				if [ -n "$missing" ]; then
					echo "  Missing:$missing"
				else
					echo "  All packages installed ✓"
				fi
			`, configDir)
		} else {
			fmt.Println("  No packages.conf found. Skipping.")
		}

		// Check runtimes
		fmt.Println("\n[runtimes]")
		if _, err := os.Stat(filepath.Join(configDir, "runtimes.conf")); err == nil {
			runDiffStep("runtimes", `
				if command -v mise &>/dev/null; then
					missing=""
					while IFS= read -r line; do
						line="${line%%#*}"
						line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
						[ -z "$line" ] && continue
						if echo "$line" | grep -q ' ' && ! echo "$line" | grep -q '@'; then
							line=$(echo "$line" | sed 's/ /@/')
						fi
						tool="${line%%@*}"
						if ! mise ls 2>/dev/null | grep -q "^$tool"; then
							missing="$missing $line"
						fi
					done < runtimes.conf
					if [ -n "$missing" ]; then
						echo "  Missing:$missing"
					else
						echo "  All runtimes installed ✓"
					fi
				else
					echo "  mise not found. Skipping."
				fi
			`, configDir)
		} else {
			fmt.Println("  No runtimes.conf found. Skipping.")
		}

		// Check shell config
		fmt.Println("\n[shell]")
		checkSymlink(filepath.Join(home, ".zshrc"), "shell/zsh/zshrc", configDir)
		checkSymlink(filepath.Join(home, ".zshenv"), "shell/zsh/zshenv", configDir)

		// Check git config
		fmt.Println("\n[git]")
		checkSymlink(filepath.Join(home, ".gitconfig"), "git/gitconfig", configDir)
		checkSymlink(filepath.Join(home, ".gitignore_global"), "git/gitignore_global", configDir)

		// Check editor config
		fmt.Println("\n[editor]")
		editorDir := filepath.Join(configDir, "editor")
		if entries, err := os.ReadDir(editorDir); err == nil && len(entries) > 0 {
			for _, e := range entries {
				if e.IsDir() {
					target := filepath.Join(home, ".config", e.Name())
					if _, err := os.Stat(target); err == nil {
						fmt.Printf("  %s: installed ✓\n", e.Name())
					} else {
						fmt.Printf("  %s: NOT installed\n", e.Name())
					}
				}
			}
		} else {
			fmt.Println("  No editor/ directory. Skipping.")
		}

		// Check terminal config
		fmt.Println("\n[terminal]")
		terminalDir := filepath.Join(configDir, "terminal")
		if entries, err := os.ReadDir(terminalDir); err == nil && len(entries) > 0 {
			for _, e := range entries {
				if e.IsDir() {
					target := filepath.Join(home, ".config", e.Name())
					if _, err := os.Stat(target); err == nil {
						fmt.Printf("  %s: installed ✓\n", e.Name())
					} else {
						fmt.Printf("  %s: NOT installed\n", e.Name())
					}
				}
			}
		} else {
			fmt.Println("  No terminal/ directory. Skipping.")
		}

		// Check SSH config
		fmt.Println("\n[ssh]")
		checkSymlink(filepath.Join(home, ".ssh", "config"), "ssh/config", configDir)
		checkSymlink(filepath.Join(home, ".ssh", "known_hosts"), "ssh/known_hosts", configDir)

		// Check MCP config
		fmt.Println("\n[mcp]")
		mcpPath := filepath.Join(home, ".zcode", "cli", "config.json")
		if _, err := os.Stat(mcpPath); err == nil {
			fmt.Println("  ~/.zcode/cli/config.json: exists ✓")
			// Check if all configured MCP servers are present
			if _, err := os.Stat(filepath.Join(configDir, "mcp", "registry.json")); err == nil {
				runDiffStep("mcp servers", `
					if command -v jq &>/dev/null; then
						enabled=$(jq -r '.servers | to_entries[] | select(.value.enabled == true) | .key' mcp/registry.json 2>/dev/null)
						for server in $enabled; do
							if grep -q "\"$server\"" "$HOME/.zcode/cli/config.json" 2>/dev/null; then
								echo "  $server: configured ✓"
							else
								echo "  $server: MISSING from local config"
							fi
						done
					fi
				`, configDir)
			}
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
		// Compare with config repo
		configSkills := filepath.Join(configDir, "skills")
		if entries, err := os.ReadDir(configSkills); err == nil {
			fmt.Printf("  Config repo skills/: %d skill directories\n", len(entries))
		}

		// Check bin scripts
		fmt.Println("\n[bin]")
		binDir := filepath.Join(configDir, "bin")
		if entries, err := os.ReadDir(binDir); err == nil && len(entries) > 0 {
			missing := 0
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				target := filepath.Join(home, "bin", e.Name())
				if _, err := os.Stat(target); err != nil {
					missing++
					fmt.Printf("  %s: NOT linked\n", e.Name())
				}
			}
			if missing == 0 {
				fmt.Println("  All bin scripts linked ✓")
			}
		} else {
			fmt.Println("  No bin/ directory. Skipping.")
		}

		// Check agents
		fmt.Println("\n[agents]")
		if _, err := os.Stat(filepath.Join(configDir, "agents", "registry.json")); err == nil {
			runDiffStep("agents", `
				if command -v jq &>/dev/null; then
					enabled=$(jq -r '.agents | to_entries[] | select(.value.enabled == true) | .key' agents/registry.json 2>/dev/null)
					for agent in $enabled; do
						echo "  $agent: enabled in config"
					done
				fi
			`, configDir)
		} else {
			fmt.Println("  No agents/registry.json found. Skipping.")
		}

		fmt.Println("\n=== Diff complete ===")
		return nil
	},
}

func checkSymlink(target, source, configDir string) {
	home, _ := os.UserHomeDir()
	_ = home
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			link, _ := os.Readlink(target)
			// Check if link points to a mainframe config repo
			if strings.Contains(link, "mainframe") {
				fmt.Printf("  %s: symlinked ✓\n", target)
			} else {
				fmt.Printf("  %s: symlinked to %s (different source)\n", target, link)
			}
		} else {
			fmt.Printf("  %s: exists but not symlinked\n", target)
		}
	} else {
		fmt.Printf("  %s: missing\n", target)
	}
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
