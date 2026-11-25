package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

var initMachineName string
var initFleetName string

var initCmd = &cobra.Command{
	Use:   "init [repo-url]",
	Short: "Clone a mainframe config repo and set it up",
	Long:  "Clones a mainframe configuration repository and sets it as the active config. After cloning, runs 'mf apply' to set up the machine.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		repoURL := args[0]
		home, _ := os.UserHomeDir()
		configDir := filepath.Join(home, ".mainframe")

		if _, err := os.Stat(configDir); err == nil {
			fmt.Printf("Config directory %s already exists.\n", configDir)
			fmt.Println("Use 'mf apply' to apply, or remove the directory to re-init.")
			return nil
		}

		fmt.Printf("Cloning %s → %s\n", repoURL, configDir)
		gitCmd := exec.Command("git", "clone", repoURL, configDir)
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}

		// Write machine identity
		machineName := initMachineName
		if machineName == "" {
			machineName = "machine"
		}
		fleetName := initFleetName
		if fleetName == "" {
			fleetName = "dev"
		}

		machineFile := filepath.Join(configDir, "machine")
		machineData := map[string]string{
			"name":  machineName,
			"fleet": fleetName,
		}
		machineYAML, _ := yaml.Marshal(machineData)
		if err := os.WriteFile(machineFile, machineYAML, 0644); err != nil {
			fmt.Printf("Warning: could not write machine identity: %v\n", err)
		} else {
			fmt.Printf("Machine identity written: %s (fleet: %s)\n", machineName, fleetName)
		}

		// Set MAINFRAME_DIR
		fmt.Printf("\nConfig cloned to %s\n", configDir)
		fmt.Println("Set MAINFRAME_DIR in your shell profile:")
		fmt.Printf("  export MAINFRAME_DIR=%s\n", configDir)
		fmt.Println("\nRun 'mf apply' to set up this machine.")

		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initMachineName, "name", "", "Machine name (default: 'machine')")
	initCmd.Flags().StringVar(&initFleetName, "fleet", "dev", "Fleet name (default: 'dev')")
	rootCmd.AddCommand(initCmd)
}