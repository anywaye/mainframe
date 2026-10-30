package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/anywaye/mainframe/internal/assets"
	"github.com/spf13/cobra"
)

var (
	provisionMode     string
	provisionProject  string
	allowDBFrom       string
)

var provisionCmd = &cobra.Command{
	Use:   "provision",
	Short: "Provision a VPS as a dev or prod box",
	RunE: func(cmd *cobra.Command, args []string) error {
		if provisionMode != "dev" && provisionMode != "prod" {
			return fmt.Errorf("--mode must be 'dev' or 'prod'")
		}

		loadEnv()

		if os.Getenv("TAILSCALE_AUTHKEY") == "" {
			return fmt.Errorf("TAILSCALE_AUTHKEY is not set")
		}
		if os.Getenv("GH_TOKEN") == "" {
			return fmt.Errorf("GH_TOKEN is not set")
		}

		baseDir := getEnv("BASE_DIR", "/code")
		vpsName := getEnv("VPS_NAME", "mainframe")

		fmt.Printf("=== Provisioning %s (mode: %s) ===\n", vpsName, provisionMode)
		fmt.Printf("Base dir: %s\n\n", baseDir)

		fmt.Println("[1/8] Detecting OS...")
		osFamily, err := detectOS()
		if err != nil {
			return err
		}
		fmt.Printf("  OS family: %s\n", osFamily)

		fmt.Println("[2/8] Installing packages...")
		if err := runScript("lib/os/" + osFamily + ".sh"); err != nil {
			return fmt.Errorf("OS module failed: %w", err)
		}

		fmt.Println("[3/8] Setting up Tailscale...")
		if os.Getenv("TAILSCALE_ENABLED") != "false" {
			if err := runScript("lib/tailscale.sh"); err != nil {
				return fmt.Errorf("Tailscale setup failed: %w", err)
			}
		} else {
			fmt.Println("  Skipped (TAILSCALE_ENABLED=false)")
		}

		fmt.Println("[4/8] Applying hardening...")
		if os.Getenv("HARDEN_ENABLED") != "false" {
			if err := runScript("lib/harden.sh"); err != nil {
				return fmt.Errorf("hardening failed: %w", err)
			}
		} else {
			fmt.Println("  Skipped (HARDEN_ENABLED=false)")
		}

		fmt.Println("[5/8] Configuring firewall...")
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

		fmt.Println("[6/8] Setting up secrets...")
		if err := runScript("lib/secrets.sh"); err != nil {
			return fmt.Errorf("secrets setup failed: %w", err)
		}

		fmt.Println("[7/8] Creating directory structure...")
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return err
		}
		os.MkdirAll(baseDir+"/worktrees", 0755)
		if _, err := os.Lstat("/data/workspace"); os.IsNotExist(err) {
			os.MkdirAll("/data", 0755)
			os.Symlink(baseDir, "/data/workspace")
		}

		fmt.Printf("[8/8] Running %s mode setup...\n", provisionMode)
		if err := runScript("modes/" + provisionMode + ".sh"); err != nil {
			return fmt.Errorf("mode setup failed: %w", err)
		}

		if provisionProject != "" {
			fmt.Printf("Setting up project: %s...\n", provisionProject)
			if err := runScript("projects/" + provisionProject + "/setup.sh"); err != nil {
				return fmt.Errorf("project setup failed: %w", err)
			}
		}

		fmt.Println("\n=== Provisioning complete ===")
		fmt.Printf("VPS IP: %s\n", getIP())
		fmt.Println("Done!")
		return nil
	},
}

func init() {
	provisionCmd.Flags().StringVar(&provisionMode, "mode", "", "Provisioning mode: dev or prod")
	provisionCmd.Flags().StringVar(&provisionProject, "project", "", "Project name to set up after provisioning")
	provisionCmd.Flags().StringVar(&allowDBFrom, "allow-db-from", "", "Allow specific IP on DB ports (PostgreSQL, PgBouncer)")
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