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
	provisionForce    bool
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

		os.MkdirAll("/var/lib/mainframe", 0755)

		if provisionForce {
			os.Remove("/var/lib/mainframe/provision.state")
			logMsg("Force mode: re-running all steps")
		}

		if os.Getenv("TAILSCALE_AUTHKEY") == "" {
			return fmt.Errorf("TAILSCALE_AUTHKEY is not set. Set it in .env or environment.\n  Hint: generate one at https://console.tailscale.com/admin/settings/keys")
		}
		if os.Getenv("GH_TOKEN") == "" {
			return fmt.Errorf("GH_TOKEN is not set. Set it in .env or environment.\n  Hint: gh auth token")
		}

		baseDir := getEnv("BASE_DIR", "/code")
		vpsName := getEnv("VPS_NAME", "mainframe")

		initLogging()
		defer closeLogging()

		logMsg("=== Provisioning %s (mode: %s) ===", vpsName, provisionMode)
		logMsg("Base dir: %s", baseDir)
		logMsg("Dry run: %v", provisionDryRun)
		logMsg("")

		start := time.Now()
		step := 0
		total := 7

		stateFile := "/var/lib/mainframe/provision.state"
		saveState := func(s string) {
			f, _ := os.OpenFile(stateFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
			if f != nil {
				fmt.Fprintln(f, s)
				f.Close()
			}
		}
		hasState := func(s string) bool {
			data, _ := os.ReadFile(stateFile)
			return strings.Contains(string(data), s)
		}

		runStep := func(name string, fn func() error) bool {
			step++
			t0 := time.Now()
			progress := (step - 1) * 100 / total
			emitProgress(ProgressEvent{Type: "step_start", Step: step, Total: total, Name: name, Progress: progress})
			if provisionDryRun {
				emitProgress(ProgressEvent{Type: "step_skipped", Step: step, Total: total, Name: name, Message: "dry-run"})
				return true
			}
			if hasState(name) && !provisionForce {
				emitProgress(ProgressEvent{Type: "step_skipped", Step: step, Total: total, Name: name, Message: "already completed"})
				saveState(name)
				return true
			}
			if err := fn(); err != nil {
				elapsed := time.Since(t0).Milliseconds()
				emitProgress(ProgressEvent{Type: "step_failed", Step: step, Total: total, Name: name, Error: err.Error(), ElapsedMs: elapsed})
				return false
			}
			elapsed := time.Since(t0).Milliseconds()
			emitProgress(ProgressEvent{Type: "step_done", Step: step, Total: total, Name: name, Status: "ok", ElapsedMs: elapsed, Progress: step * 100 / total})
			saveState(name)
			return true
		}

		// Step 1: Detect OS (always run, fast and idempotent)
		var osFamily string
		f, err := detectOS()
		if err != nil {
			return fmt.Errorf("OS detection failed: %w", err)
		}
		osFamily = f
		logMsg("[1/8] Detecting OS...")
		logMsg("  OS family: %s", osFamily)
		logMsg("")

		// Step 2: Install packages
		if !runStep("Installing packages", func() error {
			if err := runScript("lib/os/" + osFamily + ".sh"); err != nil {
				return fmt.Errorf("OS module failed: %w", err)
			}
			pkgConf := findAsset("packages.conf")
			if pkgConf != "" {
				if err := runScriptWithArgs("lib/packages.sh", pkgConf); err != nil {
					logMsg("  Warning: package installation had issues: %v", err)
					logMsg("  Hint: some packages may need manual installation")
				}
			}
			return nil
		}) {
			return fmt.Errorf("package installation failed")
		}

		// Step 3: Tailscale
		if !runStep("Setting up Tailscale", func() error {
			if os.Getenv("TAILSCALE_ENABLED") == "false" {
				logMsg("  Skipped (TAILSCALE_ENABLED=false)")
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
				logMsg("  Skipped (HARDEN_ENABLED=false)")
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
				// Default to "public" (SSH on port 22) for safety.
				// tailscale-only mode blocks all SSH if Tailscale crashes,
				// which can lock you out. Use tailscale-only only if you
				// have a working Tailscale connection AND a console password.
				firewallMode = "public"
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
				logMsg("  Warning: %v", err)
				logMsg("  Hint: set BW_EMAIL and re-run for Bitwarden setup")
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
			logMsg("Setting up project: %s...", provisionProject)
			if err := runScript("projects/" + provisionProject + "/setup.sh"); err != nil {
				logMsg("  Warning: project setup failed: %v", err)
			}
		}

		elapsed := time.Since(start)
		emitProgress(ProgressEvent{Type: "complete", Status: "success", ElapsedMs: elapsed.Milliseconds(), Progress: 100, Message: "Provisioning complete"})
		return nil
	},
}

func init() {
	provisionCmd.Flags().StringVar(&provisionMode, "mode", "", "Provisioning mode: dev or prod")
	provisionCmd.Flags().StringVar(&provisionProject, "project", "", "Project name to set up after provisioning")
	provisionCmd.Flags().StringVar(&allowDBFrom, "allow-db-from", "", "Allow specific IP on DB ports (PostgreSQL, PgBouncer)")
	provisionCmd.Flags().BoolVar(&provisionDryRun, "dry-run", false, "Preview without executing")
	provisionCmd.Flags().BoolVar(&provisionForce, "force", false, "Re-run all steps (ignore saved state)")
	provisionCmd.Flags().BoolVar(&jsonMode, "json", false, "Output progress as NDJSON (machine-parseable)")
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
	// 10 minute timeout per script
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(10 * time.Minute):
		cmd.Process.Kill()
		return fmt.Errorf("script timed out after 10 minutes: %s", relPath)
	}
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
