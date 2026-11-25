package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var captureOutput string
var captureForce bool

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Scan current machine and generate mainframe config",
	Long: `Scans the current machine's installed packages, runtimes, shell config, git config,
editor config, terminal config, MCP servers, skills, and SSH config, then generates
a mainframe config repository. Use --output to specify the output directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := captureOutput
		if outputDir == "" {
			outputDir = "."
		}
		os.MkdirAll(outputDir, 0755)

		home, _ := os.UserHomeDir()

		fmt.Println("=== Capturing machine configuration ===")

		// Step 1: Detect OS
		fmt.Println("\n[1/12] Detecting OS...")
		osFamily := "darwin"
		osID := "macos"
		if _, err := os.Stat("/etc/os-release"); err == nil {
			out, _ := exec.Command("bash", "-c", "source /etc/os-release && echo $ID").Output()
			osID = strings.TrimSpace(string(out))
			switch osID {
			case "ubuntu", "debian":
				osFamily = "debian"
			case "fedora", "rhel", "centos":
				osFamily = "fedora"
			case "arch", "manjaro":
				osFamily = "arch"
			case "alpine":
				osFamily = "alpine"
			}
		} else if _, err := exec.Command("sw_vers").Output(); err == nil {
			osFamily = "darwin"
			osID = "macos"
		}
		fmt.Printf("  OS: %s (family: %s)\n", osID, osFamily)

		// Step 2: Capture packages
		fmt.Println("\n[2/12] Capturing packages...")
		if osFamily == "darwin" {
			runCaptureStep("brew packages", `
				if command -v brew &>/dev/null; then
					brew list --formula 2>/dev/null | sort
				fi
			`, filepath.Join(outputDir, "packages.conf"), "# packages.conf - system packages\n")
		} else {
			runCaptureStep("apt packages", `
				if command -v dpkg &>/dev/null; then
					dpkg -l 2>/dev/null | grep '^ii' | awk '{print $2}' | sort
				fi
			`, filepath.Join(outputDir, "packages.conf"), "# packages.conf - system packages\n")
		}

		// Step 3: Capture runtimes
		fmt.Println("\n[3/12] Capturing runtimes...")
		runCaptureStep("mise runtimes", `
			if command -v mise &>/dev/null; then
				mise ls 2>/dev/null | awk '{print $1"@"$2}' | grep -v '^$'
			fi
		`, filepath.Join(outputDir, "runtimes.conf"), "# runtimes.conf - language runtimes (mise format)\n")

		// Step 4: Capture shell config
		fmt.Println("\n[4/12] Capturing shell config...")
		shellDir := filepath.Join(outputDir, "shell")
		os.MkdirAll(shellDir, 0755)
		os.MkdirAll(filepath.Join(shellDir, "zsh"), 0755)
		os.MkdirAll(filepath.Join(shellDir, "bash"), 0755)

		shellFiles := []string{".zshrc", ".zshenv", ".bashrc", ".bash_profile"}
		for _, f := range shellFiles {
			src := filepath.Join(home, f)
			if _, err := os.Stat(src); err == nil {
				dst := filepath.Join(shellDir, strings.TrimPrefix(f, "."))
				copyFile(src, dst)
				fmt.Printf("  Captured %s → %s\n", f, dst)
			}
		}
		// Copy zsh config dir if exists
		if _, err := os.Stat(filepath.Join(home, ".config", "zsh")); err == nil {
			copyDir(filepath.Join(home, ".config", "zsh"), filepath.Join(shellDir, "zsh", "config"))
			fmt.Println("  Captured ~/.config/zsh/")
		}

		// Step 5: Capture git config
		fmt.Println("\n[5/12] Capturing git config...")
		gitDir := filepath.Join(outputDir, "git")
		os.MkdirAll(gitDir, 0755)
		if _, err := os.Stat(filepath.Join(home, ".gitconfig")); err == nil {
			copyFile(filepath.Join(home, ".gitconfig"), filepath.Join(gitDir, "gitconfig"))
			fmt.Println("  Captured ~/.gitconfig")
		}
		if _, err := os.Stat(filepath.Join(home, ".gitignore_global")); err == nil {
			copyFile(filepath.Join(home, ".gitignore_global"), filepath.Join(gitDir, "gitignore_global"))
			fmt.Println("  Captured ~/.gitignore_global")
		}

		// Step 6: Capture editor config
		fmt.Println("\n[6/12] Capturing editor config...")
		editorDir := filepath.Join(outputDir, "editor")
		os.MkdirAll(editorDir, 0755)
		nvimDir := filepath.Join(home, ".config", "nvim")
		if _, err := os.Stat(nvimDir); err == nil {
			copyDir(nvimDir, filepath.Join(editorDir, "nvim"))
			fmt.Println("  Captured ~/.config/nvim/")
		}

		// Step 7: Capture terminal config
		fmt.Println("\n[7/12] Capturing terminal config...")
		terminalDir := filepath.Join(outputDir, "terminal")
		os.MkdirAll(terminalDir, 0755)
		for _, app := range []string{"tmux", "kitty", "alacritty", "lazygit", "ghostty"} {
			src := filepath.Join(home, ".config", app)
			if _, err := os.Stat(src); err == nil {
				copyDir(src, filepath.Join(terminalDir, app))
				fmt.Printf("  Captured ~/.config/%s/\n", app)
			}
		}
		// Ghostty macOS path
		ghosttyMac := filepath.Join(home, "Library", "Application Support", "com.mitchellh.ghostty")
		if _, err := os.Stat(ghosttyMac); err == nil {
			copyDir(ghosttyMac, filepath.Join(terminalDir, "ghostty-macos"))
			fmt.Println("  Captured ~/Library/Application Support/com.mitchellh.ghostty/")
		}

		// Step 8: Capture SSH config
		fmt.Println("\n[8/12] Capturing SSH config...")
		sshDir := filepath.Join(outputDir, "ssh")
		os.MkdirAll(sshDir, 0755)
		sshFiles := []string{"config", "known_hosts"}
		for _, f := range sshFiles {
			src := filepath.Join(home, ".ssh", f)
			if _, err := os.Stat(src); err == nil {
				copyFile(src, filepath.Join(sshDir, f))
				fmt.Printf("  Captured ~/.ssh/%s\n", f)
			}
		}

		// Step 9: Capture MCP config
		fmt.Println("\n[9/12] Capturing MCP config...")
		mcpDir := filepath.Join(outputDir, "mcp")
		os.MkdirAll(mcpDir, 0755)
		mcpConfig := filepath.Join(home, ".zcode", "cli", "config.json")
		if _, err := os.Stat(mcpConfig); err == nil {
			copyFile(mcpConfig, filepath.Join(mcpDir, "config.json"))
			fmt.Println("  Captured ~/.zcode/cli/config.json")
		}

		// Step 10: Capture skills
		fmt.Println("\n[10/12] Capturing skills...")
		skillsDir := filepath.Join(outputDir, "skills", "common")
		os.MkdirAll(skillsDir, 0755)
		srcSkills := filepath.Join(home, ".agents", "skills")
		if _, err := os.Stat(srcSkills); err == nil {
			copyDir(srcSkills, skillsDir)
			entries, _ := os.ReadDir(skillsDir)
			fmt.Printf("  Captured %d skills\n", len(entries))
		}

		// Step 11: Capture bin scripts
		fmt.Println("\n[11/12] Capturing bin scripts...")
		binDir := filepath.Join(outputDir, "bin")
		os.MkdirAll(binDir, 0755)
		srcBin := filepath.Join(home, "bin")
		if _, err := os.Stat(srcBin); err == nil {
			copyDir(srcBin, binDir)
			entries, _ := os.ReadDir(binDir)
			fmt.Printf("  Captured %d scripts\n", len(entries))
		}

		// Step 12: Generate mainframe.yml
		fmt.Println("\n[12/12] Generating mainframe.yml...")
		mainframeYML := `mode: dev

tailscale:
  enabled: true

firewall:
  mode: tailscale-only
  extra_ports: []

hardening:
  ssh_crypto: true
  bbr: true
  apparmor: true
  unattended: true
  needrestart: true
  swap: true
  journald: true
  etckeeper: true
  deadman: true
  fail2ban: true

secrets:
  backend: bitwarden

packages: packages.conf
runtimes: runtimes.conf

dev:
  shell: shell/
  git: git/
  editor: editor/
  terminal: terminal/
  ssh: ssh/
  bin: bin/
  skills: skills/
  mcp:
    registry: mcp/registry.json
    config: mcp/config.json
  os: os/darwin.sh

agents:
  registry: agents/registry.json
  enabled:
    - zcode
`
		os.WriteFile(filepath.Join(outputDir, "mainframe.yml"), []byte(mainframeYML), 0644)
		fmt.Println("  Generated mainframe.yml")

		fmt.Println("\n=== Capture complete ===")
		fmt.Printf("Config written to: %s\n", outputDir)
		fmt.Println("Review the generated files and commit to your mainframe repo.")
		return nil
	},
}

func runCaptureStep(name, script, outputFile, header string) {
	cmd := exec.Command("bash", "-c", script)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("  Warning: %s failed: %v\n", name, err)
		return
	}
	if len(out) > 0 {
		content := header + string(out)
		os.WriteFile(outputFile, []byte(content), 0644)
		fmt.Printf("  Written to %s\n", outputFile)
	} else {
		fmt.Printf("  No data for %s\n", name)
	}
}

func copyFile(src, dst string) {
	data, err := os.ReadFile(src)
	if err != nil {
		return
	}
	os.WriteFile(dst, data, 0644)
}

func copyDir(src, dst string) {
	os.MkdirAll(dst, 0755)
	filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(src, path)
		dstPath := filepath.Join(dst, rel)
		os.MkdirAll(filepath.Dir(dstPath), 0755)
		data, _ := os.ReadFile(path)
		os.WriteFile(dstPath, data, 0644)
		return nil
	})
}

func init() {
	captureCmd.Flags().StringVar(&captureOutput, "output", "", "Output directory (default: current)")
	captureCmd.Flags().BoolVar(&captureForce, "force", false, "Overwrite existing files")
	rootCmd.AddCommand(captureCmd)
}
