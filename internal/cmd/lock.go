package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Regenerate packages.lock from packages.conf",
	Long:  "Reads packages.conf, resolves versions, and writes packages.lock with pinned versions and checksums.",
	RunE: func(cmd *cobra.Command, args []string) error {
		confPath := "packages.conf"
		if _, err := os.Stat(confPath); err != nil {
			return fmt.Errorf("packages.conf not found: %w", err)
		}

		// Read packages.conf
		data, err := os.ReadFile(confPath)
		if err != nil {
			return err
		}

		type Package struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Type    string `json:"type"` // "os" or "toolchain"
		}

		var packages []Package
		for _, line := range splitLines(string(data)) {
			line = stripComment(line)
			line = trimSpace(line)
			if line == "" {
				continue
			}
			parts := splitColon(line)
			name := parts[0]
			version := ""
			if len(parts) > 1 {
				version = parts[1]
			}
			pkgType := "os"
			if version != "" {
				pkgType = "toolchain"
			}
			packages = append(packages, Package{Name: name, Version: version, Type: pkgType})
		}

		lock := map[string]interface{}{
			"generated_at": time.Now().UTC().Format(time.RFC3339),
			"os":            detectOSName(),
			"packages":      packages,
		}

		out, _ := json.MarshalIndent(lock, "", "  ")
		if err := os.WriteFile("packages.lock", out, 0644); err != nil {
			return err
		}

		fmt.Printf("Generated packages.lock (%d packages)\n", len(packages))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(lockCmd)
}

func splitLines(s string) []string {
	return splitBy(s, "\n")
}

func stripComment(s string) string {
	if i := indexOf(s, "#"); i >= 0 {
		return s[:i]
	}
	return s
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

func splitBy(s, sep string) []string {
	var result []string
	current := ""
	for _, c := range s {
		if string(c) == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(c)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}

func splitColon(s string) []string {
	return splitBy(s, ":")
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

func detectOSName() string {
	data, _ := os.ReadFile("/etc/os-release")
	for _, line := range splitBy(string(data), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), "\"")
		}
	}
	return "unknown"
}