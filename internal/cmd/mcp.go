package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP (Model Context Protocol) servers",
}

func init() {
	mcpCmd.AddCommand(mcpListCmd())
	mcpCmd.AddCommand(mcpAddCmd())
	mcpCmd.AddCommand(mcpRemoveCmd())
	mcpCmd.AddCommand(mcpEnableCmd())
	mcpCmd.AddCommand(mcpDisableCmd())
	mcpCmd.AddCommand(mcpInstallCmd())
	mcpCmd.AddCommand(mcpTestCmd())
	mcpCmd.AddCommand(mcpApplyCmd())
	rootCmd.AddCommand(mcpCmd)
}

func mcpRegistryPath() string {
	return "mcp/registry.json"
}

func loadRegistry() (map[string]interface{}, error) {
	data, err := os.ReadFile(mcpRegistryPath())
	if err != nil {
		return nil, err
	}
	var reg map[string]interface{}
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, err
	}
	return reg, nil
}

func mcpListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all MCP servers and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})
			fmt.Printf("%-25s %-10s %-8s %s\n", "SERVER", "TYPE", "ENABLED", "DESCRIPTION")
			for name, info := range servers {
				infoMap := info.(map[string]interface{})
				sType := infoMap["type"].(string)
				enabled := infoMap["enabled"].(bool)
				desc := ""
				if d, ok := infoMap["description"].(string); ok {
					desc = d
				}
				status := "disabled"
				if enabled {
					status = "enabled"
				}
				fmt.Printf("%-25s %-10s %-8s %s\n", name, sType, status, desc)
			}
			return nil
		},
	}
}

func mcpInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install SERVER",
		Short: "Install an MCP server's dependencies",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})
			info, ok := servers[name].(map[string]interface{})
			if !ok {
				return fmt.Errorf("server %q not found in registry", name)
			}
			installPath, ok := info["install"].(string)
			if !ok || installPath == "" {
				fmt.Printf("No install script for %s (HTTP server, no binaries needed)\n", name)
				return nil
			}
			fmt.Printf("Installing %s...\n", name)
			c := exec.Command("bash", installPath)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}

func mcpTestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "test SERVER",
		Short: "Test an MCP server end-to-end",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})
			info, ok := servers[name].(map[string]interface{})
			if !ok {
				return fmt.Errorf("server %q not found in registry", name)
			}
			configPath := info["config"].(string)
			data, err := os.ReadFile(configPath)
			if err != nil {
				return err
			}
			var serverConfig map[string]interface{}
			if err := json.Unmarshal(data, &serverConfig); err != nil {
				return err
			}
			sType := serverConfig["type"].(string)
			fmt.Printf("Testing %s (%s)...\n", name, sType)

			if sType == "http" {
				url := serverConfig["url"].(string)
				fmt.Printf("  URL: %s\n", url)
				c := exec.Command("curl", "-s", "-o", "/dev/null", "-w", "%{http_code}", url)
				out, _ := c.Output()
				fmt.Printf("  HTTP status: %s\n", string(out))
				if string(out) == "200" || string(out) == "405" {
					fmt.Println("  OK Server reachable")
				} else {
					fmt.Println("  FAIL Server not reachable")
				}
			} else {
				cmd := serverConfig["command"].(string)
				args := []string{}
				if a, ok := serverConfig["args"].([]interface{}); ok {
					for _, arg := range a {
						args = append(args, arg.(string))
					}
				}
				input := `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}},"id":1}` + "\n"
				c := exec.Command(cmd, args...)
				c.Stdin = strings.NewReader(input)
				out, err := c.Output()
				if err != nil {
					fmt.Printf("  FAIL Failed to start: %v\n", err)
					return nil
				}
				if strings.Contains(string(out), "protocolVersion") {
					fmt.Println("  OK MCP handshake successful")
				} else {
					fmt.Printf("  FAIL Unexpected response: %s\n", string(out)[:200])
				}
			}
			return nil
		},
	}
}


func mcpAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add SERVER",
		Short: "Add a new MCP server to the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, err := loadRegistry()
			if err != nil {
				// Create new registry if it doesn't exist
				reg = map[string]interface{}{
					"version": 1,
					"servers": map[string]interface{}{},
				}
			}
			servers := reg["servers"].(map[string]interface{})
			if _, exists := servers[name]; exists {
				return fmt.Errorf("server %q already exists in registry", name)
			}

			// Create server directory
			serverDir := filepath.Join("mcp", "servers", name)
			os.MkdirAll(serverDir, 0755)

			// Create template config.json
			config := map[string]interface{}{
				"name":    name,
				"type":    "stdio",
				"command": name,
				"args":    []string{},
				"env":     map[string]string{},
				"secrets": map[string]string{},
			}
			configData, _ := json.MarshalIndent(config, "", "  ")
			os.WriteFile(filepath.Join(serverDir, "config.json"), configData, 0644)

			// Create template install.sh
			install := "#!/usr/bin/env bash\nset -e\necho \"No install steps defined for " + name + "\"\n"
			os.WriteFile(filepath.Join(serverDir, "install.sh"), []byte(install), 0755)

			// Create template README.md
			readme := "# " + name + "\n\nMCP server.\n\n## Dependencies\n\nTODO\n\n## Secrets\n\nTODO\n\n## Test\n\n```bash\nmf mcp test " + name + "\n```\n"
			os.WriteFile(filepath.Join(serverDir, "README.md"), []byte(readme), 0644)

			// Add to registry
			servers[name] = map[string]interface{}{
				"type":        "stdio",
				"enabled":     false,
				"description": "TODO: add description",
				"secrets":     []string{},
				"install":     "mcp/servers/" + name + "/install.sh",
				"config":      "mcp/servers/" + name + "/config.json",
			}
			data, _ := json.MarshalIndent(reg, "", "  ")
			os.WriteFile(mcpRegistryPath(), data, 0644)

			fmt.Printf("Added server %q to registry (disabled by default)\n", name)
			fmt.Printf("Edit mcp/servers/%s/config.json to configure\n", name)
			fmt.Printf("Enable with: mf mcp enable %s\n", name)
			return nil
		},
	}
}

func mcpRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove SERVER",
		Short: "Remove an MCP server from the registry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})
			if _, exists := servers[name]; !exists {
				return fmt.Errorf("server %q not found in registry", name)
			}
			delete(servers, name)
			data, _ := json.MarshalIndent(reg, "", "  ")
			os.WriteFile(mcpRegistryPath(), data, 0644)

			// Remove server directory
			serverDir := filepath.Join("mcp", "servers", name)
			os.RemoveAll(serverDir)

			fmt.Printf("Removed server %q from registry\n", name)
			return nil
		},
	}
}

func mcpEnableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "enable SERVER",
		Short: "Enable an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})
			info, ok := servers[name].(map[string]interface{})
			if !ok {
				return fmt.Errorf("server %q not found in registry", name)
			}
			info["enabled"] = true
			data, _ := json.MarshalIndent(reg, "", "  ")
			os.WriteFile(mcpRegistryPath(), data, 0644)
			fmt.Printf("Enabled %s\n", name)
			return nil
		},
	}
}

func mcpDisableCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "disable SERVER",
		Short: "Disable an MCP server",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})
			info, ok := servers[name].(map[string]interface{})
			if !ok {
				return fmt.Errorf("server %q not found in registry", name)
			}
			info["enabled"] = false
			data, _ := json.MarshalIndent(reg, "", "  ")
			os.WriteFile(mcpRegistryPath(), data, 0644)
			fmt.Printf("Disabled %s\n", name)
			return nil
		},
	}
}

func mcpApplyCmd() *cobra.Command {
	agent := ""
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply registry to agent-specific MCP configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := loadRegistry()
			if err != nil {
				return err
			}
			servers := reg["servers"].(map[string]interface{})

			type serverDef struct {
				Name    string            `json:"-"`
				Type    string            `json:"type"`
				Command string            `json:"command,omitempty"`
				Args    []string          `json:"args,omitempty"`
				Env     map[string]string `json:"env,omitempty"`
				URL     string            `json:"url,omitempty"`
				Enabled bool              `json:"-"`
			}
			var defs []serverDef
			for name, info := range servers {
				infoMap := info.(map[string]interface{})
				enabled, _ := infoMap["enabled"].(bool)
				if !enabled {
					continue
				}
				configPath := infoMap["config"].(string)
				data, err := os.ReadFile(configPath)
				if err != nil {
					continue
				}
				var def serverDef
				if err := json.Unmarshal(data, &def); err != nil {
					continue
				}
				def.Name = name
				def.Enabled = enabled
				defs = append(defs, def)
			}

			agents := []string{}
			if agent == "" || agent == "auto" {
				home, _ := os.UserHomeDir()
				if _, err := os.Stat(filepath.Join(home, ".zcode")); err == nil {
					agents = append(agents, "zcode")
				}
				if _, err := os.Stat(filepath.Join(home, ".claude.json")); err == nil {
					agents = append(agents, "claude")
				}
				if _, err := os.Stat(filepath.Join(home, ".codex")); err == nil {
					agents = append(agents, "codex")
				}
				if len(agents) == 0 {
					agents = []string{"zcode"}
				}
			} else {
				agents = strings.Split(agent, ",")
			}

			for _, a := range agents {
				a = strings.TrimSpace(a)
				switch a {
				case "zcode":
					zcodeServers := map[string]interface{}{}
					for _, d := range defs {
						if d.Type == "http" {
							zcodeServers[d.Name] = map[string]interface{}{"type": "http", "url": d.URL}
						} else {
							zcodeServers[d.Name] = map[string]interface{}{"type": "stdio", "command": d.Command, "args": d.Args, "env": d.Env}
						}
					}
					config := map[string]interface{}{"mcp": map[string]interface{}{"servers": zcodeServers}}
					home, _ := os.UserHomeDir()
					path := filepath.Join(home, ".zcode", "cli", "config.json")
					os.MkdirAll(filepath.Dir(path), 0755)
					data, _ := json.MarshalIndent(config, "", "  ")
					os.WriteFile(path, data, 0644)
					fmt.Printf("Generated ZCode config: %s (%d servers)\n", path, len(defs))

				case "claude":
					claudeServers := map[string]interface{}{}
					for _, d := range defs {
						if d.Type == "http" {
							claudeServers[d.Name] = map[string]interface{}{"type": "http", "url": d.URL}
						} else {
							claudeServers[d.Name] = map[string]interface{}{"type": "stdio", "command": d.Command, "args": d.Args, "env": d.Env}
						}
					}
					home, _ := os.UserHomeDir()
					path := filepath.Join(home, ".claude.json")
					existing := map[string]interface{}{}
					if data, err := os.ReadFile(path); err == nil {
						json.Unmarshal(data, &existing)
					}
					existing["mcpServers"] = claudeServers
					data, _ := json.MarshalIndent(existing, "", "  ")
					os.WriteFile(path, data, 0644)
					fmt.Printf("Generated Claude config: %s (%d servers)\n", path, len(defs))

				case "codex":
					var lines []string
					for _, d := range defs {
						lines = append(lines, fmt.Sprintf("[mcp_servers.%s]", d.Name))
						if d.Type == "http" {
							lines = append(lines, fmt.Sprintf("url = \"%s\"", d.URL))
						} else {
							lines = append(lines, fmt.Sprintf("command = \"%s\"", d.Command))
							if len(d.Args) > 0 {
								args := []string{}
								for _, a := range d.Args {
									args = append(args, fmt.Sprintf("\"%s\"", a))
								}
								lines = append(lines, fmt.Sprintf("args = [%s]", strings.Join(args, ", ")))
							}
							if len(d.Env) > 0 {
								lines = append(lines, "")
								lines = append(lines, "[mcp_servers." + d.Name + ".env]")
								for k, v := range d.Env {
									lines = append(lines, fmt.Sprintf("%s = \"%s\"", k, v))
								}
							}
						}
						lines = append(lines, "")
					}
					home, _ := os.UserHomeDir()
					dir := filepath.Join(home, ".codex")
					os.MkdirAll(dir, 0755)
					path := filepath.Join(dir, "config.toml")
					os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
					fmt.Printf("Generated Codex config: %s (%d servers)\n", path, len(defs))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&agent, "agent", "auto", "Target agents: zcode, claude, codex (comma-separated, or auto to detect installed)")
	return cmd
}
