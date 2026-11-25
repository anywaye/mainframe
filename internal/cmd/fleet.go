package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
)

// FleetMachine represents a single machine in a fleet.
type FleetMachine struct {
	Name    string `yaml:"name"`
	Host    string `yaml:"host"`    // "local" or "user@host"
	OS      string `yaml:"os"`      // macos, ubuntu-24.04, debian-12, etc.
	Mode    string `yaml:"mode"`    // dev or prod
	SSHKey  string `yaml:"ssh_key"` // Bitwarden item name for SSH key
}

// FleetConfig represents a fleet definition.
type FleetConfig struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Boax        bool           `yaml:"boax"` // register as boax pools
	Machines    []FleetMachine `yaml:"machines"`
}

var fleetBoax bool
var fleetMachineName string
var fleetMachineHost string
var fleetMachineOS string

var fleetCmd = &cobra.Command{
	Use:   "fleet",
	Short: "Manage machine fleets (groups of machines for config sync)",
	Long:  "Fleets are named groups of machines that share configuration. Apply, sync, and monitor all machines in a fleet at once.",
}

var fleetCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new fleet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

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

		fleetDir := filepath.Join(configDir, "fleet")
		if err := os.MkdirAll(fleetDir, 0755); err != nil {
			return fmt.Errorf("failed to create fleet directory: %w", err)
		}

		fleetFile := filepath.Join(fleetDir, name+".yml")
		if _, err := os.Stat(fleetFile); err == nil {
			return fmt.Errorf("fleet %s already exists at %s", name, fleetFile)
		}

		fleet := FleetConfig{
			Name:        name,
			Description: fmt.Sprintf("%s machines", name),
			Boax:        fleetBoax,
			Machines:    []FleetMachine{},
		}

		data, err := yaml.Marshal(&fleet)
		if err != nil {
			return fmt.Errorf("failed to marshal fleet config: %w", err)
		}

		header := fmt.Sprintf("# fleet/%s.yml - %s fleet machines\n# Add machines to this fleet, then run 'mf fleet apply %s'\n\n", name, name, name)
		if err := os.WriteFile(fleetFile, []byte(header+string(data)), 0644); err != nil {
			return fmt.Errorf("failed to write fleet file: %w", err)
		}

		fmt.Printf("Created fleet %q at %s\n", name, fleetFile)
		fmt.Printf("  boax integration: %v\n", fleetBoax)
		fmt.Println("\nAdd machines with:")
		fmt.Printf("  mf fleet add %s <name> --host <host> --os <os>\n", name)
		fmt.Println("\nOr edit the file directly:")
		fmt.Printf("  %s\n", fleetFile)

		return nil
	},
}

var fleetListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all fleets",
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fleetDir := filepath.Join(configDir, "fleet")
		entries, err := os.ReadDir(fleetDir)
		if err != nil {
			fmt.Println("No fleets found. Create one with: mf fleet create <name>")
			return nil
		}

		fmt.Println("Fleets:")
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".yml") {
				continue
			}
			fleetName := strings.TrimSuffix(e.Name(), ".yml")
			fleet, err := loadFleet(configDir, fleetName)
			if err != nil {
				fmt.Printf("  %-12s (error: %v)\n", fleetName, err)
				continue
			}
			boaxStr := ""
			if fleet.Boax {
				boaxStr = " [boax]"
			}
			fmt.Printf("  %-12s %d machines%s\n", fleetName, len(fleet.Machines), boaxStr)
		}
		return nil
	},
}

var fleetShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show fleet details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		configDir := resolveConfigDir()
		fleet, err := loadFleet(configDir, name)
		if err != nil {
			return err
		}

		fmt.Printf("Fleet: %s\n", fleet.Name)
		if fleet.Description != "" {
			fmt.Printf("Description: %s\n", fleet.Description)
		}
		fmt.Printf("Boax integration: %v\n", fleet.Boax)
		fmt.Printf("Machines: %d\n\n", len(fleet.Machines))

		if len(fleet.Machines) == 0 {
			fmt.Println("  No machines. Add with: mf fleet add", name, "<machine-name> --host <host> --os <os>")
			return nil
		}

		fmt.Printf("  %-16s %-28s %-16s %-6s %s\n", "NAME", "HOST", "OS", "MODE", "SSH KEY")
		for _, m := range fleet.Machines {
			sshKey := m.SSHKey
			if sshKey == "" {
				sshKey = "-"
			}
			fmt.Printf("  %-16s %-28s %-16s %-6s %s\n", m.Name, m.Host, m.OS, m.Mode, sshKey)
		}
		return nil
	},
}

var fleetAddCmd = &cobra.Command{
	Use:   "add <fleet> <machine-name>",
	Short: "Add a machine to a fleet",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleetName := args[0]
		machineName := args[1]
		configDir := resolveConfigDir()

		fleet, err := loadFleet(configDir, fleetName)
		if err != nil {
			return err
		}

		// Check if machine already exists
		for _, m := range fleet.Machines {
			if m.Name == machineName {
				return fmt.Errorf("machine %s already in fleet %s", machineName, fleetName)
			}
		}

		host := fleetMachineHost
		if host == "" {
			host = "local"
		}
		osType := fleetMachineOS
		if osType == "" {
			osType = "auto"
		}
		mode := "dev"
		if fleetName == "prod" {
			mode = "prod"
		}

		machine := FleetMachine{
			Name:   machineName,
			Host:   host,
			OS:     osType,
			Mode:   mode,
			SSHKey: "",
		}
		fleet.Machines = append(fleet.Machines, machine)

		if err := saveFleet(configDir, fleetName, fleet); err != nil {
			return err
		}

		fmt.Printf("Added machine %q to fleet %q\n", machineName, fleetName)
		fmt.Printf("  Host: %s, OS: %s, Mode: %s\n", host, osType, mode)
		if host != "local" {
			fmt.Println("\nNext: ensure SSH access works:")
			fmt.Printf("  ssh %s\n", host)
		}
		return nil
	},
}

var fleetRemoveCmd = &cobra.Command{
	Use:   "remove <fleet> <machine-name>",
	Short: "Remove a machine from a fleet",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleetName := args[0]
		machineName := args[1]
		configDir := resolveConfigDir()

		fleet, err := loadFleet(configDir, fleetName)
		if err != nil {
			return err
		}

		var remaining []FleetMachine
		found := false
		for _, m := range fleet.Machines {
			if m.Name == machineName {
				found = true
				continue
			}
			remaining = append(remaining, m)
		}
		if !found {
			return fmt.Errorf("machine %s not found in fleet %s", machineName, fleetName)
		}

		fleet.Machines = remaining
		if err := saveFleet(configDir, fleetName, fleet); err != nil {
			return err
		}

		fmt.Printf("Removed machine %q from fleet %q\n", machineName, fleetName)
		return nil
	},
}

var fleetApplyCmd = &cobra.Command{
	Use:   "apply <fleet>",
	Short: "Apply config to all machines in a fleet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleetName := args[0]
		configDir := resolveConfigDir()

		fleet, err := loadFleet(configDir, fleetName)
		if err != nil {
			return err
		}

		if len(fleet.Machines) == 0 {
			fmt.Println("No machines in fleet. Add with: mf fleet add", fleetName, "<name> --host <host> --os <os>")
			return nil
		}

		fmt.Printf("=== Applying config to fleet %q (%d machines) ===\n\n", fleetName, len(fleet.Machines))

		for i, m := range fleet.Machines {
			fmt.Printf("[%d/%d] %s (%s)\n", i+1, len(fleet.Machines), m.Name, m.Host)

			if m.Host == "local" || m.Host == "" {
				// Apply locally
				fmt.Println("  Applying locally...")
				applyCmd := exec.Command("mf", "apply")
				applyCmd.Stdout = os.Stdout
				applyCmd.Stderr = os.Stderr
				if err := applyCmd.Run(); err != nil {
					fmt.Printf("  ERROR: %v\n", err)
				} else {
					fmt.Println("  Done.")
				}
			} else {
				// Apply via SSH
				fmt.Printf("  Applying via SSH to %s...\n", m.Host)
				sshCmd := exec.Command("ssh", m.Host, "cd ~/.mainframe 2>/dev/null || cd ~/mainframe 2>/dev/null; mf apply")
				sshCmd.Stdout = os.Stdout
				sshCmd.Stderr = os.Stderr
				if err := sshCmd.Run(); err != nil {
					fmt.Printf("  ERROR: %v\n", err)
					fmt.Println("  (Machine may be offline or mf not installed)")
				} else {
					fmt.Println("  Done.")
				}
			}
			fmt.Println()
		}

		fmt.Println("=== Fleet apply complete ===")
		return nil
	},
}

var fleetStatusCmd = &cobra.Command{
	Use:   "status <fleet>",
	Short: "Show health status of all machines in a fleet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleetName := args[0]
		configDir := resolveConfigDir()

		fleet, err := loadFleet(configDir, fleetName)
		if err != nil {
			return err
		}

		fmt.Printf("=== Fleet %q status ===\n\n", fleetName)
		fmt.Printf("  %-16s %-28s %-16s %-10s %s\n", "NAME", "HOST", "OS", "STATUS", "DETAIL")
		for _, m := range fleet.Machines {
			status, detail := probeMachine(m)
			fmt.Printf("  %-16s %-28s %-16s %-10s %s\n", m.Name, m.Host, m.OS, status, detail)
		}
		return nil
	},
}

var fleetSyncCmd = &cobra.Command{
	Use:   "sync <fleet>",
	Short: "Sync config across all machines in a fleet",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleetName := args[0]
		configDir := resolveConfigDir()

		fleet, err := loadFleet(configDir, fleetName)
		if err != nil {
			return err
		}

		fmt.Printf("=== Syncing fleet %q ===\n\n", fleetName)

		// Pull latest config locally
		fmt.Println("[1] Pulling latest config locally...")
		gitCmd := exec.Command("git", "pull", "--rebase")
		gitCmd.Dir = configDir
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		if err := gitCmd.Run(); err != nil {
			fmt.Println("  Warning: git pull failed")
		}

		// Push config to each remote machine
		for i, m := range fleet.Machines {
			if m.Host == "local" || m.Host == "" {
				continue
			}
			fmt.Printf("\n[%d] Syncing to %s (%s)...\n", i+2, m.Name, m.Host)
			sshCmd := exec.Command("ssh", m.Host, "cd ~/.mainframe 2>/dev/null && git pull --rebase && mf apply")
			sshCmd.Stdout = os.Stdout
			sshCmd.Stderr = os.Stderr
			if err := sshCmd.Run(); err != nil {
				fmt.Printf("  ERROR: %v\n", err)
			}
		}

		fmt.Println("\n=== Fleet sync complete ===")
		return nil
	},
}

var fleetRegisterBoaxCmd = &cobra.Command{
	Use:   "register-boax <fleet>",
	Short: "Register fleet machines as boax pools",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fleetName := args[0]
		configDir := resolveConfigDir()

		fleet, err := loadFleet(configDir, fleetName)
		if err != nil {
			return err
		}

		if !fleet.Boax {
			fmt.Println("Warning: fleet was created without boax integration.")
			fmt.Println("Set 'boax: true' in the fleet config to enable.")
		}

		fmt.Printf("=== Registering fleet %q machines as boax pools ===\n\n", fleetName)

		for _, m := range fleet.Machines {
			fmt.Printf("  %s (%s): ", m.Name, m.Host)
			if m.Host == "local" || m.Host == "" {
				fmt.Println("local machine — register manually via boax CLI")
			} else {
				fmt.Printf("would register as boax pool %q on %s\n", m.Name, m.Host)
				fmt.Println("  (boax registration requires boax-server URL and credentials)")
			}
		}

		fmt.Println("\nTo complete boax registration:")
		fmt.Println("  1. Ensure boax-server is running")
		fmt.Println("  2. Set BOAX_URL and BOAX_API_KEY environment variables")
		fmt.Println("  3. Run: boax machine register --name <machine> --pool <pool>")
		fmt.Println("\nFleet machines can serve as boax execution backends.")
		fmt.Println("Mainframe configures them; boax places sandboxes on them.")

		return nil
	},
}

func init() {
	fleetCmd.AddCommand(fleetCreateCmd)
	fleetCmd.AddCommand(fleetListCmd)
	fleetCmd.AddCommand(fleetShowCmd)
	fleetCmd.AddCommand(fleetAddCmd)
	fleetCmd.AddCommand(fleetRemoveCmd)
	fleetCmd.AddCommand(fleetApplyCmd)
	fleetCmd.AddCommand(fleetStatusCmd)
	fleetCmd.AddCommand(fleetSyncCmd)
	fleetCmd.AddCommand(fleetRegisterBoaxCmd)
	rootCmd.AddCommand(fleetCmd)

	fleetCreateCmd.Flags().BoolVar(&fleetBoax, "boax", false, "Enable boax pool registration")
	fleetAddCmd.Flags().StringVar(&fleetMachineHost, "host", "", "Machine host (user@host or 'local')")
	fleetAddCmd.Flags().StringVar(&fleetMachineOS, "os", "", "Machine OS (macos, ubuntu-24.04, debian-12, etc.)")
}

// resolveConfigDir finds the mainframe config directory.
func resolveConfigDir() string {
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
	return configDir
}

// loadFleet reads a fleet config file.
func loadFleet(configDir, name string) (FleetConfig, error) {
	fleetFile := filepath.Join(configDir, "fleet", name+".yml")
	data, err := os.ReadFile(fleetFile)
	if err != nil {
		return FleetConfig{}, fmt.Errorf("fleet %s not found: %w", name, err)
	}

	var fleet FleetConfig
	if err := yaml.Unmarshal(data, &fleet); err != nil {
		return FleetConfig{}, fmt.Errorf("failed to parse fleet config: %w", err)
	}
	if fleet.Name == "" {
		fleet.Name = name
	}
	return fleet, nil
}

// saveFleet writes a fleet config file.
func saveFleet(configDir, name string, fleet FleetConfig) error {
	fleetDir := filepath.Join(configDir, "fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		return err
	}
	fleetFile := filepath.Join(fleetDir, name+".yml")
	data, err := yaml.Marshal(&fleet)
	if err != nil {
		return err
	}
	header := fmt.Sprintf("# fleet/%s.yml\n\n", name)
	return os.WriteFile(fleetFile, []byte(header+string(data)), 0644)
}

// probeMachine checks if a machine is reachable.
func probeMachine(m FleetMachine) (string, string) {
	if m.Host == "local" || m.Host == "" {
		return "online", "local machine"
	}
	// SSH probe with timeout
	cmd := exec.Command("ssh", "-o", "ConnectTimeout=5", "-o", "StrictHostKeyChecking=no", m.Host, "echo ok")
	output, err := cmd.Output()
	if err != nil {
		return "offline", fmt.Sprintf("SSH failed: %v", err)
	}
	if strings.TrimSpace(string(output)) == "ok" {
		return "online", "SSH OK"
	}
	return "unknown", "unexpected response"
}