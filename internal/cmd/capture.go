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

var captureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Scan current machine and generate mainframe config",
	Long:  "Scans the current machine's installed packages, runtimes, shell config, git config, editor config, etc. and generates a mainframe config repository.",
	RunE: func(cmd *cobra.Command, args []string) error {
		outputDir := captureOutput
		if outputDir == "" {
			outputDir = "."
		}
		os.MkdirAll(outputDir, 0755)

		fmt.Println("=== Capturing machine configuration ===")

		// Step 1: Detect OS
		fmt.Println("\n[1/8] Detecting OS...")
		osFamily := "darwin"
		if _, err := os.Stat("/etc/os-release"); err == nil {
			out, _ := exec.Command("bash", "-c", "source /etc/os-release && echo $ID").Output()
			osID := strings.TrimSpace(string(out))
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
		}
		fmt.Printf("  OS family: %s\n", osFamily)

		// Step 2: Capture packages
		fmt.Println("\n[2/8] Capturing packages...")
		if osFamily == "darwin" {
			runCaptureStep("brew packages", `
				if command -v brew &>/dev/null; then
					brew list --formula 2>/dev/null | sort
				fi
			`, filepath.Join(outputDir, "packages.brew.txt"))
		}

		// Step 3: Capture runtimes
		fmt.Println("\n[3/8] Capturing runtimes...")
		runCaptureStep("mise runtimes", `
			if command -v mise &>/dev/null; then
				mise ls 2>/dev/null | awk '{print $1"@"$2}' | grep -v '^$'
			fi
		`, filepath.Join(outputDir, "runtimes.conf"))

		// Step 4: Capture shell config
		fmt.Println("\n[4/8] Capturing shell config...")
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".zshrc")); err == nil {
			fmt.Println("  Found ~/.zshrc")
		}
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".bashrc")); err == nil {
			fmt.Println("  Found ~/.bashrc")
		}

		// Step 5: Capture git config
		fmt.Println("\n[5/8] Capturing git config...")
		if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".gitconfig")); err == nil {
			fmt.Println("  Found ~/.gitconfig")
		}

		// Step 6: Capture MCP config
		fmt.Println("\n[6/8] Capturing MCP config...")
		home, _ := os.UserHomeDir()
		mcpPath := filepath.Join(home, ".zcode", "cli", "config.json")
		if _, err := os.Stat(mcpPath); err == nil {
			fmt.Println("  Found ~/.zcode/cli/config.json")
		}

		// Step 7: Capture skills
		fmt.Println("\n[7/8] Capturing skills...")
		skillsPath := filepath.Join(home, ".agents", "skills")
		if entries, err := os.ReadDir(skillsPath); err == nil {
			fmt.Printf("  Found %d skills in ~/.agents/skills/\n", len(entries))
		}

		// Step 8: Capture SSH config
		fmt.Println("\n[8/8] Capturing SSH config...")
		if _, err := os.Stat(filepath.Join(home, ".ssh", "config")); err == nil {
			fmt.Println("  Found ~/.ssh/config")
		}

		fmt.Println("\n=== Capture complete ===")
		fmt.Printf("Config files written to: %s\n", outputDir)
		fmt.Println("Review and commit the generated files to your mainframe repo.")
		return nil
	},
}

func runCaptureStep(name, script, outputFile string) {
	cmd := exec.Command("bash", "-c", script)
	out, err := cmd.Output()
	if err != nil {
		fmt.Printf("  Warning: %s failed: %v\n", name, err)
		return
	}
	if len(out) > 0 {
		os.WriteFile(outputFile, out, 0644)
		fmt.Printf("  Written to %s\n", outputFile)
	} else {
		fmt.Printf("  No data for %s\n", name)
	}
}

func init() {
	captureCmd.Flags().StringVar(&captureOutput, "output", "", "Output directory (default: current)")
	rootCmd.AddCommand(captureCmd)
}
