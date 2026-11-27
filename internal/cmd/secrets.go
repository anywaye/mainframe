package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"path/filepath"

	"github.com/spf13/cobra"
)

var secretsCmd = &cobra.Command{
	Use:   "secrets",
	Short: "Manage secrets (machine: Infisical, personal: Bitwarden/1Password)",
	Long: `Manage secrets for machine and personal use.

Modes:
  --machine (default)  Machine secrets via Infisical (SSH keys, API tokens, env vars)
  --personal           Personal secrets via Bitwarden (web logins, credit cards, TOTP)

Machine secrets are used by 'mf apply' and stored in Infisical.
Personal secrets are for human use and stored in a password manager.

Examples:
  mf secrets status                    # Infisical status
  mf secrets --personal status        # Bitwarden status
  mf secrets set KEY=val               # Set secret in Infisical
  mf secrets --personal get "GitHub"   # Get item from Bitwarden
  mf secrets export > .env            # Export all machine secrets as .env`,
}

var secretsMode string
var secretsPersonal bool
var secretsMachine bool

func isPersonalMode() bool {
	if secretsPersonal {
		return true
	}
	if secretsMachine {
		return false
	}
	return secretsMode == "personal"
}

// ==================== MACHINE MODE (Infisical) ====================

var secretsSetupHost string
var secretsSetupSelfHost bool
var secretsSetupCloud bool

var secretsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up secrets backend",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			return personalSetup()
		}
		return machineSetup()
	},
}

func machineSetup() error {
	if _, err := exec.LookPath("infisical"); err != nil {
		fmt.Println("Infisical CLI not found. Installing...")
		if _, err := exec.LookPath("brew"); err == nil {
			exec.Command("brew", "install", "infisical/get-cli/infisical").Run()
		} else {
			fmt.Println("Install Infisical CLI manually: https://infisical.com/docs/cli/overview")
			return fmt.Errorf("infisical CLI not found")
		}
	}
	version, _ := exec.Command("infisical", "--version").Output()
	fmt.Printf("Infisical CLI: %s\n", strings.TrimSpace(string(version)))

	if secretsSetupSelfHost {
		fmt.Println("Setting up self-hosted Infisical via Docker Compose...")
		home, _ := os.UserHomeDir()
		infisicalDir := filepath.Join(home, "infisical")
		os.MkdirAll(infisicalDir, 0755)

		encKey := "change-me"
		authSecret := "change-me"
		if out, err := exec.Command("openssl", "rand", "-hex", "16").Output(); err == nil {
			encKey = strings.TrimSpace(string(out))
		}
		if out, err := exec.Command("openssl", "rand", "-hex", "32").Output(); err == nil {
			authSecret = strings.TrimSpace(string(out))
		}

		composeContent := `version: "3.8"
services:
  infisical:
    image: infisical/infisical:latest
    ports:
      - "8080:8080"
    environment:
      - ENCRYPTION_KEY=` + encKey + `
      - AUTH_SECRET=` + authSecret + `
      - DB_CONNECTION_URI=postgres://infisical:infisical@db:5432/infisical
      - REDIS_URL=redis://redis:6379
    depends_on:
      - db
      - redis
    restart: unless-stopped

  db:
    image: postgres:15-alpine
    environment:
      - POSTGRES_USER=infisical
      - POSTGRES_PASSWORD=infisical
      - POSTGRES_DB=infisical
    volumes:
      - infisical-db:/var/lib/postgresql/data
    restart: unless-stopped

  redis:
    image: redis:7-alpine
    restart: unless-stopped

volumes:
  infisical-db:
`
		composePath := filepath.Join(infisicalDir, "docker-compose.yml")
		os.WriteFile(composePath, []byte(composeContent), 0644)
		fmt.Printf("  Created %s\n", composePath)
		fmt.Println("  To start: cd ~/infisical && docker compose up -d")
		fmt.Println("  Infisical will be available at http://localhost:8080")
	} else if secretsSetupCloud {
		fmt.Println("Using Infisical Cloud (https://app.infisical.com)")
		fmt.Println("  Create an account at https://app.infisical.com")
		fmt.Println("  Then run: mf secrets login")
	} else if secretsSetupHost != "" {
		fmt.Printf("Using existing Infisical instance: %s\n", secretsSetupHost)
		fmt.Printf("  Run: mf secrets login --host=%s\n", secretsSetupHost)
	} else {
		fmt.Println("Infisical CLI is installed.")
		fmt.Println("  Self-host:  mf secrets setup --self-host")
		fmt.Println("  Cloud:      mf secrets setup --cloud")
		fmt.Println("  Existing:   mf secrets setup --host <url>")
	}
	return nil
}

var secretsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show secrets backend status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			return personalStatus()
		}
		return machineStatus()
	},
}

func machineStatus() error {
	if _, err := exec.LookPath("infisical"); err != nil {
		fmt.Println("Infisical CLI: not installed")
		fmt.Println("  Install: mf secrets setup")
		return nil
	}
	version, _ := exec.Command("infisical", "--version").Output()
	fmt.Printf("Infisical CLI: %s\n", strings.TrimSpace(string(version)))

	host := os.Getenv("INFISICAL_HOST")
	if host == "" {
		host = "http://localhost:8080"
	}
	projectID := os.Getenv("INFISICAL_PROJECT_ID")
	clientID := os.Getenv("INFISICAL_CLIENT_ID")

	fmt.Printf("Host: %s\n", host)
	fmt.Printf("Project ID: %s\n", projectID)
	if clientID != "" {
		fmt.Println("Machine identity: configured")
	} else {
		fmt.Println("Machine identity: not configured (run 'mf secrets login' for interactive)")
	}

	secretArgs := []string{"secrets", "--env=dev", "--domain=" + host, "--silent"}
	if projectID != "" {
		secretArgs = append(secretArgs, "--projectId="+projectID)
	}
	cmdErr := exec.Command("infisical", secretArgs...).Run()
	if cmdErr == nil {
		fmt.Println("Interactive login: authenticated")
	} else {
		fmt.Println("Interactive login: not authenticated")
	}
	return nil
}

var secretsLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to secrets backend",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			return personalUnlock("")
		}
		host := os.Getenv("INFISICAL_HOST")
		if host == "" {
			host = "http://localhost:8080"
		}
		fmt.Printf("Logging in to %s...\n", host)
		loginCmd := exec.Command("infisical", "login", "--domain="+host)
		loginCmd.Stdin = os.Stdin
		loginCmd.Stdout = os.Stdout
		loginCmd.Stderr = os.Stderr
		return loginCmd.Run()
	},
}

var secretsSetCmd = &cobra.Command{
	Use:   "set NAME=VALUE",
	Short: "Set a secret (machine mode) or get item (personal mode)",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			// Personal mode: search for item
			for _, arg := range args {
				personalGet(arg)
			}
			return nil
		}
		return machineSet(args)
	},
}

func machineSet(args []string) error {
	host := os.Getenv("INFISICAL_HOST")
	if host == "" {
		host = "http://localhost:8080"
	}
	projectID := os.Getenv("INFISICAL_PROJECT_ID")
	env := "dev"
	path := "/"
	if projectID == "" {
		return fmt.Errorf("INFISICAL_PROJECT_ID not set")
	}
	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) != 2 {
			fmt.Printf("Invalid format: %s (expected NAME=VALUE)\n", arg)
			continue
		}
		name, value := parts[0], parts[1]
		fmt.Printf("Setting %s...\n", name)
		setCmd := exec.Command("infisical", "secrets", "set", name+"="+value,
			"--path="+path, "--env="+env, "--projectId="+projectID, "--domain="+host)
		setCmd.Stdout = os.Stdout
		setCmd.Stderr = os.Stderr
		if err := setCmd.Run(); err != nil {
			fmt.Printf("  Failed: %v\n", err)
		}
	}
	return nil
}

var secretsGetCmd = &cobra.Command{
	Use:   "get NAME",
	Short: "Get a secret (machine mode) or item (personal mode)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			return personalGet(args[0])
		}
		return machineGet(args[0])
	},
}

func machineGet(name string) error {
	host := os.Getenv("INFISICAL_HOST")
	if host == "" {
		host = "http://localhost:8080"
	}
	projectID := os.Getenv("INFISICAL_PROJECT_ID")
	env := "dev"
	path := "/"
	if projectID == "" {
		return fmt.Errorf("INFISICAL_PROJECT_ID not set")
	}
	getCmd := exec.Command("infisical", "secrets", "get", name,
		"--path="+path, "--env="+env, "--projectId="+projectID, "--domain="+host, "--plain", "--silent")
	getCmd.Stdout = os.Stdout
	getCmd.Stderr = os.Stderr
	return getCmd.Run()
}

var secretsExportCmd = &cobra.Command{
	Use:   "export [env]",
	Short: "Export all machine secrets as .env format",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			fmt.Println("Export is only available in machine mode (Infisical).")
			return nil
		}
		host := os.Getenv("INFISICAL_HOST")
		if host == "" {
			host = "http://localhost:8080"
		}
		projectID := os.Getenv("INFISICAL_PROJECT_ID")
		env := "dev"
		if len(args) > 0 {
			env = args[0]
		}
		if projectID == "" {
			return fmt.Errorf("INFISICAL_PROJECT_ID not set")
		}
		exportCmd := exec.Command("infisical", "export",
			"--domain="+host, "--projectId="+projectID, "--env="+env, "--format=dotenv")
		exportCmd.Stdout = os.Stdout
		exportCmd.Stderr = os.Stderr
		return exportCmd.Run()
	},
}

var secretsIdentityCreateCmd = &cobra.Command{
	Use:   "identity-create",
	Short: "Create a machine identity for non-interactive access (machine mode)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			fmt.Println("Identity creation is only available in machine mode (Infisical).")
			return nil
		}
		fmt.Println("Machine identities are created in the Infisical web UI or via API.")
		fmt.Println("  Web UI: Infisical → Settings → Machine Identities → Create")
		fmt.Println("  Then set env vars on the target machine:")
		fmt.Println("    export INFISICAL_CLIENT_ID=<client-id>")
		fmt.Println("    export INFISICAL_CLIENT_SECRET=<client-secret>")
		return nil
	},
}

// ==================== PERSONAL MODE (Bitwarden) ====================

func personalSetup() error {
	if _, err := exec.LookPath("bw"); err != nil {
		fmt.Println("Bitwarden CLI not found. Installing...")
		if _, err := exec.LookPath("npm"); err == nil {
			exec.Command("npm", "install", "-g", "@bitwarden/cli").Run()
		} else {
			fmt.Println("Install Bitwarden CLI manually: npm install -g @bitwarden/cli")
			return fmt.Errorf("bw CLI not found")
		}
	}
	version, _ := exec.Command("bw", "--version").Output()
	fmt.Printf("Bitwarden CLI: %s\n", strings.TrimSpace(string(version)))
	fmt.Println("  Login: bw login <email>")
	fmt.Println("  Unlock: bw unlock")
	return nil
}

func personalStatus() error {
	if _, err := exec.LookPath("bw"); err != nil {
		fmt.Println("Bitwarden CLI: not installed")
		fmt.Println("  Install: mf secrets --personal setup")
		return nil
	}
	version, _ := exec.Command("bw", "--version").Output()
	fmt.Printf("Bitwarden CLI: %s\n", strings.TrimSpace(string(version)))
	out, _ := exec.Command("bw", "status").Output()
	fmt.Printf("Status: %s\n", strings.TrimSpace(string(out)))
	return nil
}

func personalUnlock(password string) error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bw CLI not found. Install: mf secrets --personal setup")
	}
	statusOut, _ := exec.Command("bw", "status").Output()
	if strings.Contains(string(statusOut), "unauthenticated") {
		fmt.Println("Not logged in. Run 'bw login <email>' first.")
		return nil
	}

	unlockArgs := []string{"unlock", "--raw"}
	if password != "" {
		unlockArgs = append(unlockArgs, password)
	}
	fmt.Println("Unlocking Bitwarden vault...")
	unlockCmd := exec.Command("bw", unlockArgs...)
	if password == "" {
		unlockCmd.Stdin = os.Stdin
	}
	unlockCmd.Stderr = os.Stderr
	out, err := unlockCmd.Output()
	if err != nil {
		return fmt.Errorf("unlock failed: %w", err)
	}
	session := strings.TrimSpace(string(out))
	if session == "" {
		return fmt.Errorf("no session token received")
	}

	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".config", "bitwarden")
	os.MkdirAll(sessionDir, 0755)
	sessionPath := filepath.Join(sessionDir, "session")
	os.WriteFile(sessionPath, []byte(session+"\n"), 0600)
	os.Setenv("BW_SESSION", session)

	fmt.Println("Vault unlocked. Session stored in ~/.config/bitwarden/session")
	return nil
}

func personalLock() error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bw CLI not found")
	}
	exec.Command("bw", "lock").Run()
	home, _ := os.UserHomeDir()
	sessionPath := filepath.Join(home, ".config", "bitwarden", "session")
	os.Remove(sessionPath)
	fmt.Println("Vault locked. Session file removed.")
	return nil
}

func personalGet(name string) error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bw CLI not found")
	}
	home, _ := os.UserHomeDir()
	sessionPath := filepath.Join(home, ".config", "bitwarden", "session")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return fmt.Errorf("vault not unlocked. Run: mf secrets --personal unlock")
	}
	session := strings.TrimSpace(string(data))
	os.Setenv("BW_SESSION", session)

	out, err := exec.Command("bw", "get", "item", name).Output()
	if err != nil {
		return fmt.Errorf("item not found: %s", name)
	}
	fmt.Println(string(out))
	return nil
}

func personalList() error {
	if _, err := exec.LookPath("bw"); err != nil {
		return fmt.Errorf("bw CLI not found")
	}
	home, _ := os.UserHomeDir()
	sessionPath := filepath.Join(home, ".config", "bitwarden", "session")
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return fmt.Errorf("vault not unlocked. Run: mf secrets --personal unlock")
	}
	session := strings.TrimSpace(string(data))
	os.Setenv("BW_SESSION", session)

	out, err := exec.Command("bw", "list", "items").Output()
	if err != nil {
		return fmt.Errorf("failed to list items: %w", err)
	}
	fmt.Println(string(out))
	return nil
}

// Personal mode commands
var secretsUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock personal vault (Bitwarden)",
	RunE: func(cmd *cobra.Command, args []string) error {
		password := ""
		if len(args) > 0 {
			password = args[0]
		}
		return personalUnlock(password)
	},
}

var secretsLockPersonalCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock personal vault (Bitwarden)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return personalLock()
	},
}

var secretsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List items (personal mode) or secrets (machine mode)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if isPersonalMode() {
			return personalList()
		}
		fmt.Println("List is only available in personal mode. Use 'mf secrets get <NAME>' for machine mode.")
		return nil
	},
}

func init() {
	secretsCmd.PersistentFlags().StringVar(&secretsMode, "mode", "machine", "Secrets mode: machine (Infisical) or personal (Bitwarden/1Password)")
	secretsCmd.PersistentFlags().BoolVar(&secretsPersonal, "personal", false, "Use personal secrets mode (Bitwarden)")
	secretsCmd.PersistentFlags().BoolVar(&secretsMachine, "machine", false, "Use machine secrets mode (Infisical)")

	secretsSetupCmd.Flags().StringVar(&secretsSetupHost, "host", "", "Use existing Infisical instance URL")
	secretsSetupCmd.Flags().BoolVar(&secretsSetupSelfHost, "self-host", false, "Self-host Infisical via Docker Compose")
	secretsSetupCmd.Flags().BoolVar(&secretsSetupCloud, "cloud", false, "Use Infisical Cloud")

	secretsCmd.AddCommand(secretsSetupCmd)
	secretsCmd.AddCommand(secretsStatusCmd)
	secretsCmd.AddCommand(secretsLoginCmd)
	secretsCmd.AddCommand(secretsSetCmd)
	secretsCmd.AddCommand(secretsGetCmd)
	secretsCmd.AddCommand(secretsExportCmd)
	secretsCmd.AddCommand(secretsIdentityCreateCmd)
	secretsCmd.AddCommand(secretsUnlockCmd)
	secretsCmd.AddCommand(secretsLockPersonalCmd)
	secretsCmd.AddCommand(secretsListCmd)

	rootCmd.AddCommand(secretsCmd)
}
