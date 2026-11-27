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
	Short: "Manage secrets via Infisical (self-hosted)",
}

var secretsSetupHost string
var secretsSetupSelfHost bool
var secretsSetupCloud bool

var secretsSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Set up Infisical secrets backend",
	Long: `Set up Infisical as the secrets backend for mainframe.

Options:
  --host <url>       Use an existing Infisical instance
  --self-host        Self-host Infisical via Docker Compose on this machine
  --cloud            Use Infisical Cloud (free tier)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("infisical"); err != nil {
			fmt.Println("Infisical CLI not found. Installing...")
			// Try to install
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

			// Create docker-compose file
			encKey := "change-me-generate-with-openssl-rand-hex-16"
	authSecret := "change-me-generate-with-openssl-rand-hex-32"
	// Try to generate keys
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
			fmt.Println("  IMPORTANT: Change ENCRYPTION_KEY and AUTH_SECRET in production!")
		} else if secretsSetupCloud {
			fmt.Println("Using Infisical Cloud (https://app.infisical.com)")
			fmt.Println("  Create an account at https://app.infisical.com")
			fmt.Println("  Then run: infisical login")
		} else if secretsSetupHost != "" {
			fmt.Printf("Using existing Infisical instance: %s\n", secretsSetupHost)
			fmt.Printf("  Run: infisical login --domain=%s\n", secretsSetupHost)
		} else {
			fmt.Println("Infisical CLI is installed.")
			fmt.Println("  Self-host:  mf secrets setup --self-host")
			fmt.Println("  Cloud:      mf secrets setup --cloud")
			fmt.Println("  Existing:   mf secrets setup --host <url>")
		}

		return nil
	},
}

var secretsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Infisical secrets backend status",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("infisical"); err != nil {
			fmt.Println("Infisical CLI: not installed")
			fmt.Println("  Install: mf secrets setup")
			return nil
		}
		version, _ := exec.Command("infisical", "--version").Output()
		fmt.Printf("Infisical CLI: %s\n", strings.TrimSpace(string(version)))

		// Check env vars
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
			fmt.Println("Machine identity: not configured (run 'infisical login' for interactive)")
		}

		// Check interactive login
		out, err := exec.Command("infisical", "user", "get", "--domain="+host).Output()
		if err == nil && strings.Contains(string(out), "email") {
			fmt.Println("Interactive login: authenticated")
		} else {
			fmt.Println("Interactive login: not authenticated")
		}

		return nil
	},
}

var secretsLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Login to Infisical (interactive)",
	RunE: func(cmd *cobra.Command, args []string) error {
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

var secretsIdentityCreateCmd = &cobra.Command{
	Use:   "identity-create",
	Short: "Create a machine identity for non-interactive access",
	Long: `Create a machine identity in Infisical for non-interactive secret access.
This is used for VPSes, CI/CD, and automated deployments.

After creating, set the env vars:
  export INFISICAL_CLIENT_ID=<client-id>
  export INFISICAL_CLIENT_SECRET=<client-secret>`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Machine identities are created in the Infisical web UI.")
		fmt.Println("  1. Open Infisical → Settings → Machine Identities")
		fmt.Println("  2. Create a new identity with Universal Auth")
		fmt.Println("  3. Assign it to your project with read access")
		fmt.Println("  4. Copy the Client ID and Client Secret")
		fmt.Println("  5. Set env vars on the target machine:")
		fmt.Println("     export INFISICAL_CLIENT_ID=<client-id>")
		fmt.Println("     export INFISICAL_CLIENT_SECRET=<client-secret>")
		fmt.Println()
		fmt.Println("  For persistent storage, add to ~/.zshenv or /etc/mainframe/infisical.env")
		return nil
	},
}

var secretsExportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export all secrets from Infisical",
	RunE: func(cmd *cobra.Command, args []string) error {
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
			return fmt.Errorf("INFISICAL_PROJECT_ID not set. Set it or run 'infisical init' in your config repo.")
		}

		fmt.Printf("Exporting secrets from %s (env: %s)...\n", host, env)
		exportCmd := exec.Command("infisical", "export",
			"--domain="+host,
			"--projectId="+projectID,
			"--env="+env,
			"--format=dotenv")
		exportCmd.Stdout = os.Stdout
		exportCmd.Stderr = os.Stderr
		return exportCmd.Run()
	},
}

func init() {
	secretsSetupCmd.Flags().StringVar(&secretsSetupHost, "host", "", "Use existing Infisical instance URL")
	secretsSetupCmd.Flags().BoolVar(&secretsSetupSelfHost, "self-host", false, "Self-host Infisical via Docker Compose")
	secretsSetupCmd.Flags().BoolVar(&secretsSetupCloud, "cloud", false, "Use Infisical Cloud")
	secretsCmd.AddCommand(secretsSetupCmd)
	secretsCmd.AddCommand(secretsStatusCmd)
	secretsCmd.AddCommand(secretsLoginCmd)
	secretsCmd.AddCommand(secretsIdentityCreateCmd)
	secretsCmd.AddCommand(secretsExportCmd)
	rootCmd.AddCommand(secretsCmd)
}
