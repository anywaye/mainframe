package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/anywaye/mainframe/internal/assets"
	"github.com/spf13/cobra"
)

var (
	provisionMode     string
	provisionProject  string
	allowDBFrom       string
	provisionDryRun   bool
)

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision a VPS as a dev or prod box",
	Long:  "Provision a fresh VPS. Detects OS, installs packages, sets up Tailscale, applies hardening, configures firewall, and runs mode-specific setup. Use --dry-run to preview without executing.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if provisionMode != "dev" && provisionMode != "prod" {
			return fmt.Errorf("--mode must be 'dev' or 'prod'")
		}

		loadEnv()

		if os.Getenv("TAILSCALE_AUTHKEY") == "" {
			return fmt.Errorf("TAILSCALE_AUTHKEY is not set. Set it in .env or environment.\n  Hint: generate one at https://console.tailscale.com/admin/settings/keys")
		}
		if os.Getenv("GH_TOKEN") == "" {
			return fmt.Errorf("GH_TOKEN is not set. Set it in .env or environment.\n  Hint: gh auth token")
		}

		baseDir := getEnv("BASE_DIR", "/code")
		vpsName := getEnv("VPS_NAME", "mainframe")

		fmt.Printf("=== Provisioning %s (mode: %s) ===\n", vpsName, provisionMode)
		fmt.Printf("Base dir: %s\n", baseDir)
		fmt.Printf("Dry run: %v\n\n", provisionDryRun)

		start := time.Now()
		step := 0
		total := 8

		runStep := func(name string, fn func() error) bool {
			step++
			t0 := time.Now()
			fmt.Printf("[%d/%d] %s...\n", step, total, name)
			if provisionDryRun {
				fmt.Printf("  (dry-run: skipped)\n\n")
				return true
			}
			if err := fn(); err != nil {
				elapsed := time.Since(t0).Round(time.Millisecond)
				fmt.Printf("  FAILED (%s): %v\n", elapsed, err)
				fmt.Printf("  Hint: check the output above for details. Re-run provision to retry.\n\n")
				return false
			}
			elapsed := time.Since(t0).Round(time.Millisecond)
			fmt.Printf("  OK (%s)\n\n", elapsed)
			return true
		}

		// Step 1: Detect OS
		var osFamily string
		if !runStep("Detecting OS", func() error {
			f, err := detectOS()
			if err != nil {
				return err
			}
			osFamily = f
			fmt.Printf("  OS family: %s\n", osFamily)
			return nil
		}) {
			return fmt.Errorf("OS detection failed")
		}

		// Step 2: Install packages
		if !runStep("Installing packages", func() error {
			if err := runScript("lib/os/" + osFamily + ".sh"); err != nil {
				return fmt.Errorf("OS module failed: %w", err)
			}
			pkgConf := findAsset("packages.conf")
			if pkgConf != "" {
				if err := runScriptWithArgs("lib/packages.sh", pkgConf); err != nil {
					fmt.Printf("  Warning: package installation had issues: %v\n", err)
					fmt.Printf("  Hint: some packages may need manual installation\n")
				}
			}
			return nil
		}) {
			return fmt.Errorf("package installation failed")
		}

		// Step 3: Tailscale
		if !runStep("Setting up Tailscale", func() error {
			if os.Getenv("TAILSCALE_ENABLED") == "false" {
				fmt.Println("  Skipped (TAILSCALE_ENABLED=false)")
				return nil
			}
			if err := runScript("lib/tailscale.sh"); err != nil {
				return fmt.Errorf("Tailscale setup failed: %w", err)
			}
			return nil
		}) {
			return fmt.Errorf("Tailscale setup failed")
		}

		// Step 4: Hardening
		if !runStep("Applying hardening", func() error {
			if os.Getenv("HARDEN_ENABLED") == "false" {
				fmt.Println("  Skipped (HARDEN_ENABLED=false)")
				return nil
			}
			if err := runScript("lib/harden.sh"); err != nil {
				return fmt.Errorf("hardening failed: %w", err)
			}
			return nil
		}) {
			return fmt.Errorf("hardening failed")
		}

		// Step 5: Firewall
		if !runStep("Configuring firewall", func() error {
			firewallMode := getEnv("FIREWALL_MODE", "")
			if firewallMode == "" {
				if provisionMode == "dev" {
					firewallMode = "tailscale-only"
				} else {
					firewallMode = "public"
				}
			}
			if err := runScriptWithArgs("lib/firewall.sh", provisionMode, firewallMode, os.Getenv("FIREWALL_EXTRA_PORTS"), allowDBFrom); err != nil {
				return fmt.Errorf("firewall failed: %w", err)
			}
			return nil
		}) {
			return fmt.Errorf("firewall configuration failed")
		}

		// Step 6: Secrets
		if !runStep("Setting up secrets", func() error {
			if err := runScript("lib/secrets.sh"); err != nil {
				// secrets.sh exits 0 when BW_EMAIL is not set (optional)
				fmt.Printf("  Warning: %v\n", err)
				fmt.Println("  Hint: set BW_EMAIL and re-run for Bitwarden setup")
			}
			return nil
		}) {
			// Non-fatal: secrets are optional
		}

		// Step 7: Directory structure
		if !runStep("Creating directory structure", func() error {
			if err := os.MkdirAll(baseDir, 0755); err != nil {
				return err
			}
			os.MkdirAll(baseDir+"/worktrees", 0755)
			if _, err := os.Lstat("/data/workspace"); os.IsNotExist(err) {
				os.MkdirAll("/data", 0755)
				os.Symlink(baseDir, "/data/workspace")
			}
			return nil
		}) {
			return fmt.Errorf("directory creation failed")
		}

		// Step 8: Mode setup
		if !runStep("Running "+provisionMode+" mode setup", func() error {
			if err := runScript("modes/" + provisionMode + ".sh"); err != nil {
				return fmt.Errorf("mode setup failed: %w", err)
			}
			return nil
		}) {
			return fmt.Errorf("mode setup failed")
		}

		if provisionProject != "" {
			fmt.Printf("Setting up project: %s...\n", provisionProject)
			if err := runScript("projects/" + provisionProject + "/setup.sh"); err != nil {
				fmt.Printf("  Warning: project setup failed: %v\n", err)
			}
		}

		elapsed := time.Since(start).Round(time.Second)
		fmt.Println("=== Provisioning complete ===")
		fmt.Printf("VPS IP: %s\n", getIP())
		fmt.Printf("Total time: %s\n", elapsed)
		fmt.Println("Done!")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Verify SSH from a new session: ssh " + vpsName)
		fmt.Println("  2. Run: mf audit (check security score)")
		fmt.Println("  3. Clone repos: git clone ... /code/anywaye/<repo>")
		return nil
	},
}

func init() {
	provisionCmd.Flags().StringVar(&provisionMode, "mode", "", "Provisioning mode: dev or prod")
	provisionCmd.Flags().StringVar(&provisionProject, "project", "", "Project name to set up after provisioning")
	provisionCmd.Flags().StringVar(&allowDBFrom, "allow-db-from", "", "Allow specific IP on DB ports (PostgreSQL, PgBouncer)")
	provisionCmd.Flags().BoolVar(&provisionDryRun, "dry-run", false, "Preview without executing")
	provisionCmd.MarkFlagRequired("mode")
}

func loadEnv() {
	if _, err := os.Stat(".env"); err == nil {
		data, _ := os.ReadFile(".env")
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				val := strings.TrimSpace(parts[1])
				val = strings.Trim(val, "\"'")
				if os.Getenv(key) == "" {
					os.Setenv(key, val)
				}
			}
		}
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func detectOS() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", fmt.Errorf("cannot read /etc/os-release: %w", err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			id := strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			switch id {
			case "ubuntu", "debian":
				return "debian", nil
			case "fedora", "rhel", "centos", "rocky", "almalinux", "ol":
				return "fedora", nil
			case "arch", "manjaro":
				return "arch", nil
			case "alpine":
				return "alpine", nil
			default:
				return "", fmt.Errorf("unsupported OS: %s", id)
			}
		}
	}
	return "", fmt.Errorf("could not detect OS")
}

func runScript(relPath string) error {
	return runScriptWithArgs(relPath)
}

func runScriptWithArgs(relPath string, args ...string) error {
	scriptPath := findAsset(relPath)
	if scriptPath == "" {
		return fmt.Errorf("script not found: %s", relPath)
	}
	cmd := exec.Command("bash", append([]string{scriptPath}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func ensureAssets() string {
	tmpDir := "/tmp/mainframe-assets"
	if _, err := os.Stat(tmpDir + "/.extracted"); err == nil {
		return tmpDir
	}
	os.MkdirAll(tmpDir, 0755)
	assets.Walk(".", func(path string, d fs.DirEntry, err error) error {
		if path == "." || err != nil {
			return nil
		}
		if d.IsDir() {
			os.MkdirAll(tmpDir+"/"+path, 0755)
			return nil
		}
		data, _ := assets.ReadFile(path)
		dir := path
		if i := len(dir) - 1; i >= 0 {
			for i >= 0 && dir[i] != '/' {
				i--
			}
			if i >= 0 {
				dir = dir[:i]
			} else {
				dir = ""
			}
		}
		if dir != "" {
			os.MkdirAll(tmpDir+"/"+dir, 0755)
		}
		os.WriteFile(tmpDir+"/"+path, data, 0644)
		return nil
	})
	os.WriteFile(tmpDir+"/.extracted", []byte("ok"), 0644)
	return tmpDir
}

func findAsset(relPath string) string {
	if _, err := os.Stat(relPath); err == nil {
		return relPath
	}
	exe, _ := os.Executable()
	dir := exe[:len(exe)-len("/mf")]
	if _, err := os.Stat(dir + "/" + relPath); err == nil {
		return dir + "/" + relPath
	}
	if assets.Exists(relPath) {
		tmpDir := ensureAssets()
		return tmpDir + "/" + relPath
	}
	return ""
}

func baseName(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func getIP() string {
	out, _ := exec.Command("hostname", "-I").Output()
	fields := strings.Fields(string(out))
	if len(fields) > 0 {
		return fields[0]
	}
	return "unknown"
}
