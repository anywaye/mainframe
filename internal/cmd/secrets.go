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
	Short: "Manage secrets (Bitwarden, 1Password, etc.)",
}

var secretsUnlockPassword string
var secretsUnlockPasswordEnv string
var secretsUnlockPasswordFile string

var secretsUnlockCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock secrets vault and store session",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := os.Getenv("SECRETS_BACKEND")
		if backend == "" {
			backend = "bitwarden"
		}

		if backend != "bitwarden" {
			fmt.Printf("Backend %s not yet supported for unlock\n", backend)
			return nil
		}

		if _, err := exec.LookPath("bw"); err != nil {
			return fmt.Errorf("bitwarden CLI not found. Install with: npm install -g @bitwarden/cli")
		}

		// Check status
		statusOut, _ := exec.Command("bw", "status").Output()
		if !strings.Contains(string(statusOut), "unauthenticated") {
			// Already logged in
		} else {
			fmt.Println("Not logged in. Run 'bw login <email>' first.")
			return nil
		}

		// Build unlock command
		unlockArgs := []string{"unlock", "--raw"}
		if secretsUnlockPassword != "" {
			unlockArgs = append(unlockArgs, secretsUnlockPassword)
		} else if secretsUnlockPasswordEnv != "" {
			unlockArgs = append([]string{"--passwordenv", secretsUnlockPasswordEnv}, unlockArgs...)
		} else if secretsUnlockPasswordFile != "" {
			unlockArgs = append([]string{"--passwordfile", secretsUnlockPasswordFile}, unlockArgs...)
		}

		fmt.Println("Unlocking Bitwarden vault...")
		unlockCmd := exec.Command("bw", unlockArgs...)
		if secretsUnlockPassword == "" && secretsUnlockPasswordEnv == "" && secretsUnlockPasswordFile == "" {
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

		// Store session in file
		home, _ := os.UserHomeDir()
		sessionDir := filepath.Join(home, ".config", "bitwarden")
		os.MkdirAll(sessionDir, 0755)
		sessionPath := filepath.Join(sessionDir, "session")
		os.WriteFile(sessionPath, []byte(session+"\n"), 0600)

		// Also set env var for current session
		os.Setenv("BW_SESSION", session)

		fmt.Println("Vault unlocked. Session stored in ~/.config/bitwarden/session")
		fmt.Printf("BW_SESSION=%s\n", session)

		return nil
	},
}

var secretsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show secrets backend status",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := os.Getenv("SECRETS_BACKEND")
		if backend == "" {
			backend = "bitwarden"
		}

		fmt.Printf("Backend: %s\n", backend)

		if backend == "bitwarden" {
			if _, err := exec.LookPath("bw"); err != nil {
				fmt.Println("Bitwarden CLI: not installed")
				return nil
			}
			out2, _ := exec.Command("bw", "--version").Output(); fmt.Printf("Bitwarden CLI: %s\n", string(out2))
			out, _ := exec.Command("bw", "status").Output()
			fmt.Printf("Status: %s\n", string(out))

			// Check session file
			home, _ := os.UserHomeDir()
			sessionPath := filepath.Join(home, ".config", "bitwarden", "session")
			if data, err := os.ReadFile(sessionPath); err == nil && len(data) > 0 {
				fmt.Println("Session file: exists")
			} else {
				fmt.Println("Session file: not found")
			}
		} else {
			fmt.Printf("Backend %s: checking...\n", backend)
		}

		return nil
	},
}

var secretsLockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock secrets vault and remove session",
	RunE: func(cmd *cobra.Command, args []string) error {
		backend := os.Getenv("SECRETS_BACKEND")
		if backend == "" {
			backend = "bitwarden"
		}

		if backend == "bitwarden" {
			exec.Command("bw", "lock").Run()
			home, _ := os.UserHomeDir()
			sessionPath := filepath.Join(home, ".config", "bitwarden", "session")
			os.Remove(sessionPath)
			fmt.Println("Vault locked. Session file removed.")
		}

		return nil
	},
}

func init() {
	secretsUnlockCmd.Flags().StringVar(&secretsUnlockPassword, "password", "", "Master password (insecure, use --passwordenv or --passwordfile)")
	secretsUnlockCmd.Flags().StringVar(&secretsUnlockPasswordEnv, "passwordenv", "", "Environment variable storing password")
	secretsUnlockCmd.Flags().StringVar(&secretsUnlockPasswordFile, "passwordfile", "", "File containing password")
	secretsCmd.AddCommand(secretsUnlockCmd)
	secretsCmd.AddCommand(secretsStatusCmd)
	secretsCmd.AddCommand(secretsLockCmd)
	rootCmd.AddCommand(secretsCmd)
}
