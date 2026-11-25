package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
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

		frameworkLib := tmpDir

		origDir, _ := os.Getwd()
		os.Chdir(configDir)
		defer os.Chdir(origDir)

		fmt.Printf("=== Applying mainframe config from %s ===\n", configDir)
		if applyDryRun {
			fmt.Println("[DRY RUN - no changes will be made]")
		}

		// Step 1: Detect OS
		fmt.Println("\n[1/13] Detecting OS...")
		osFamily := detectOSFamily(frameworkLib)
		fmt.Printf("  OS family: %s\n", osFamily)

		// Step 2: Install packages
		fmt.Println("\n[2/13] Installing packages...")
		if _, err := os.Stat("packages.conf"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would install packages from packages.conf")
			} else {
				runShellStep("install packages", fmt.Sprintf(`
					source %s/detect.sh 2>/dev/null
					source %s/os/%s.sh 2>/dev/null
					if type install_packages &>/dev/null; then install_packages packages.conf; else echo "  OS module not available"; fi
				`, frameworkLib, frameworkLib, osFamily))
			}
		} else {
			fmt.Println("  No packages.conf found. Skipping.")
		}

		// Step 3: Install runtimes
		fmt.Println("\n[3/13] Installing runtimes...")
		if _, err := os.Stat("runtimes.conf"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would install runtimes from runtimes.conf")
			} else {
				runShellStep("install runtimes", `
					if command -v mise &>/dev/null; then
						while IFS= read -r line; do
							line="${line%%#*}"
							line="$(echo "$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
							[ -z "$line" ] && continue
							echo "  Installing: $line"
							mise use -g "$line" 2>&1 | tail -1 || echo "  Warning: failed"
						done < runtimes.conf
					else
						echo "  mise not found. Skipping runtimes."
					fi
				`)
			}
		} else if _, err := os.Stat(".tool-versions"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would install runtimes from .tool-versions")
			} else {
				runShellStep("install runtimes", `
					if command -v mise &>/dev/null; then
						mise install 2>&1 | tail -5
					fi
				`)
			}
		} else {
			fmt.Println("  No runtimes.conf or .tool-versions found. Skipping.")
		}

		// Step 4: Sync shell config
		fmt.Println("\n[4/13] Syncing shell config...")
		if _, err := os.Stat("shell"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync shell/ → ~/.config/ and ~/")
			} else {
				runShellStep("sync shell", `
					mkdir -p "$HOME/.config"
					for dir in shell/*/; do
						[ -d "$dir" ] || continue
						name=$(basename "$dir")
						target="$HOME/.config/$name"
						mkdir -p "$target"
						cp -r "$dir"* "$target/" 2>/dev/null || true
						echo "  Synced $name → $target"
					done
					for f in shell/*; do
						[ -f "$f" ] || continue
						bn=$(basename "$f")
						[ "$bn" = "README.md" ] && continue
						target="$HOME/.$bn"
						if [ -L "$target" ]; then
							echo "  $target already linked"
						elif [ -e "$target" ]; then
							echo "  $target exists (skipping)"
						else
							ln -s "$(pwd)/$f" "$target"
							echo "  Linked $target"
						fi
					done
				`)
			}
		} else {
			fmt.Println("  No shell/ directory. Skipping.")
		}

		// Step 5: Sync git config
		fmt.Println("\n[5/13] Syncing git config...")
		if _, err := os.Stat("git"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync git/ → ~/")
			} else {
				runShellStep("sync git", `
					for f in git/*; do
						[ -f "$f" ] || continue
						bn=$(basename "$f")
						[ "$bn" = "README.md" ] && continue
						target_name="${bn%.symlink}"
						target="$HOME/.$target_name"
						if [ -L "$target" ]; then
							echo "  $target already linked"
						elif [ -e "$target" ]; then
							echo "  $target exists (skipping)"
						else
							ln -s "$(pwd)/$f" "$target"
							echo "  Linked $target"
						fi
					done
				`)
			}
		} else {
			fmt.Println("  No git/ directory. Skipping.")
		}

		// Step 6: Sync editor config
		fmt.Println("\n[6/13] Syncing editor config...")
		if _, err := os.Stat("editor"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync editor/ → ~/.config/")
			} else {
				runShellStep("sync editor", `
					for dir in editor/*/; do
						[ -d "$dir" ] || continue
						name=$(basename "$dir")
						target="$HOME/.config/$name"
						mkdir -p "$target"
						cp -r "$dir"* "$target/" 2>/dev/null || true
						echo "  Synced $name → $target"
					done
				`)
			}
		} else {
			fmt.Println("  No editor/ directory. Skipping.")
		}

		// Step 7: Sync terminal config
		fmt.Println("\n[7/13] Syncing terminal config...")
		if _, err := os.Stat("terminal"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync terminal/ → ~/.config/")
			} else {
				runShellStep("sync terminal", `
					for dir in terminal/*/; do
						[ -d "$dir" ] || continue
						name=$(basename "$dir")
						target="$HOME/.config/$name"
						mkdir -p "$target"
						cp -r "$dir"* "$target/" 2>/dev/null || true
						echo "  Synced $name → $target"
					done
				`)
			}
		} else {
			fmt.Println("  No terminal/ directory. Skipping.")
		}

		// Step 8: Sync SSH config
		fmt.Println("\n[8/13] Syncing SSH config...")
		if _, err := os.Stat("ssh"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync ssh/ → ~/.ssh/")
			} else {
				runShellStep("sync ssh", `
					mkdir -p "$HOME/.ssh"
					chmod 700 "$HOME/.ssh"
					for f in ssh/*; do
						[ -f "$f" ] || continue
						bn=$(basename "$f")
						target="$HOME/.ssh/$bn"
						if [ -L "$target" ]; then
							echo "  $target already linked"
						elif [ -e "$target" ]; then
							echo "  $target exists (skipping)"
						else
							ln -s "$(pwd)/$f" "$target"
							echo "  Linked $target"
						fi
					done
				`)
			}
		} else {
			fmt.Println("  No ssh/ directory. Skipping.")
		}

		// Step 9: Sync MCP config
		fmt.Println("\n[9/13] Syncing MCP config...")
		if _, err := os.Stat("mcp/config.json"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync mcp/config.json → ~/.zcode/cli/config.json")
			} else {
				runShellStep("sync mcp", `
					mkdir -p "$HOME/.zcode/cli"
					cp mcp/config.json "$HOME/.zcode/cli/config.json"
					echo "  MCP config synced"
				`)
			}
		} else {
			fmt.Println("  No mcp/config.json found. Skipping.")
		}

		// Step 10: Sync skills
		fmt.Println("\n[10/13] Syncing skills...")
		if _, err := os.Stat("skills"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync skills/ → ~/.agents/skills/")
			} else {
				runShellStep("sync skills", `
					mkdir -p "$HOME/.agents/skills"
					cp -r skills/* "$HOME/.agents/skills/" 2>/dev/null || true
					echo "  Skills synced"
				`)
			}
		} else {
			fmt.Println("  No skills/ directory. Skipping.")
		}

		// Step 11: Sync bin scripts
		fmt.Println("\n[11/13] Syncing bin scripts...")
		if _, err := os.Stat("bin"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would sync bin/ → ~/bin/")
			} else {
				runShellStep("sync bin", `
					mkdir -p "$HOME/bin"
					for f in bin/*; do
						[ -f "$f" ] || continue
						bn=$(basename "$f")
						target="$HOME/bin/$bn"
						if [ -L "$target" ]; then
							echo "  $target already linked"
						elif [ -e "$target" ]; then
							echo "  $target exists (skipping)"
						else
							chmod +x "$f"
							ln -s "$(pwd)/$f" "$target"
							echo "  Linked $target"
						fi
					done
				`)
			}
		} else {
			fmt.Println("  No bin/ directory. Skipping.")
		}

		// Step 12: OS-specific settings
		fmt.Println("\n[12/13] Applying OS-specific settings...")
		if _, err := os.Stat("os.sh"); err == nil {
			if applyDryRun {
				fmt.Println("  [dry-run] Would source os.sh")
			} else {
				runShellStep("os settings", `source os.sh`)
			}
		} else if _, err := os.Stat("os"); err == nil {
			osScript := fmt.Sprintf("os/%s.sh", osFamily)
			if _, err := os.Stat(osScript); err == nil {
				if applyDryRun {
					fmt.Printf("  [dry-run] Would source %s\n", osScript)
				} else {
					runShellStep("os settings", fmt.Sprintf("source %s", osScript))
				}
			} else {
				fmt.Println("  No OS-specific settings found. Skipping.")
			}
		} else {
			fmt.Println("  No os.sh or os/ directory. Skipping.")
		}

		// Step 13: macOS defaults (if applicable)
		if osFamily == "darwin" {
			fmt.Println("\n[13/13] Applying macOS defaults...")
			if _, err := os.Stat("macos/defaults.sh"); err == nil {
				if applyDryRun {
					fmt.Println("  [dry-run] Would apply macos/defaults.sh")
				} else {
					runShellStep("macos defaults", `source macos/defaults.sh`)
				}
			} else {
				fmt.Println("  No macos/defaults.sh found. Skipping.")
			}
		}

		fmt.Println("\n=== Apply complete ===")
		return nil
	},
}

func runShellStep(name, script string) {
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  Warning: %s failed: %v\n", name, err)
	}
}

func findFrameworkLib() string {
	// Check common locations
	for _, dir := range []string{
		"/usr/local/lib/mainframe/lib",
		"/opt/mainframe/lib",
		filepath.Join(os.Getenv("HOME"), ".mainframe", "lib"),
		"lib",
	} {
		if _, err := os.Stat(filepath.Join(dir, "detect.sh")); err == nil {
			return dir
		}
	}
	return "lib"
}

func detectOSFamily(frameworkLib string) string {
	out, err := exec.Command("bash", "-c", fmt.Sprintf("source %s/detect.sh 2>/dev/null && detect_os >/dev/null 2>&1 && echo $OS_FAMILY", frameworkLib)).Output()
	if err != nil {
		// Try local
		out, _ = exec.Command("bash", "-c", "source lib/detect.sh 2>/dev/null && detect_os >/dev/null 2>&1 && echo $OS_FAMILY").Output()
	}
	family := strings.TrimSpace(string(out))
	if family == "" {
		// Fallback: detect directly
		if _, err := os.Stat("/etc/os-release"); err != nil {
			family = "darwin"
		} else {
			family = "debian"
		}
	}
	return family
}

func init() {
	applyCmd.Flags().StringVar(&applyHost, "host", "", "Apply to remote host via SSH")
	applyCmd.Flags().BoolVar(&applyForce, "force", false, "Force overwrite existing files")
	applyCmd.Flags().BoolVar(&applyDryRun, "dry-run", false, "Preview without applying")
	rootCmd.AddCommand(applyCmd)
}
