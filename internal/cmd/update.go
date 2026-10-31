package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update mainframe to the latest version",
	Long:  "Check for updates and install the latest version from GitHub releases.",
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonFlag, _ := cmd.Flags().GetBool("json")

		emit := func(msg string) {
			if jsonFlag {
				data, _ := json.Marshal(map[string]string{"type": "info", "msg": msg, "ts": time.Now().UTC().Format(time.RFC3339)})
				fmt.Println(string(data))
			} else {
				fmt.Println(msg)
			}
		}

		emit("Checking for updates...")

		// Get current version
		currentVersion := version
		emit(fmt.Sprintf("Current version: %s", currentVersion))

		// Get latest release tag
		out, err := exec.Command("curl", "-fsSL", "https://api.github.com/repos/anywaye/mainframe/releases/latest").Output()
		if err != nil {
			return fmt.Errorf("failed to check for updates: %w", err)
		}
		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.Unmarshal(out, &release); err != nil {
			return fmt.Errorf("failed to parse release info: %w", err)
		}
		latestVersion := strings.TrimPrefix(release.TagName, "v")
		emit(fmt.Sprintf("Latest version: %s", latestVersion))

		if latestVersion == currentVersion {
			emit("Already up to date.")
			return nil
		}

		emit(fmt.Sprintf("Updating %s -> %s...", currentVersion, latestVersion))

		// Detect platform
		osName := runtime.GOOS
		arch := runtime.GOARCH
		binaryName := fmt.Sprintf("mf-%s-%s", osName, arch)
		if osName == "linux" {
			binaryName = fmt.Sprintf("mf-linux-%s", arch)
		} else if osName == "darwin" {
			binaryName = fmt.Sprintf("mf-darwin-%s", arch)
		}

		// Download
		url := fmt.Sprintf("https://github.com/anywaye/mainframe/releases/download/%s/%s", release.TagName, binaryName)
		tmp := "/tmp/mf-update"
		emit(fmt.Sprintf("Downloading %s...", url))

		c := exec.Command("curl", "-fsSL", url, "-o", tmp)
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to download: %w", err)
		}

		// Verify
		out, err = exec.Command(tmp, "version").Output()
		if err != nil {
			os.Remove(tmp)
			return fmt.Errorf("downloaded binary is invalid: %w", err)
		}
		downloadedVersion := strings.TrimSpace(string(out))
		emit(fmt.Sprintf("Downloaded: %s", downloadedVersion))

		if !strings.Contains(downloadedVersion, latestVersion) {
			os.Remove(tmp)
			return fmt.Errorf("version mismatch: expected %s, got %s", latestVersion, downloadedVersion)
		}

		// Install
		exe, _ := os.Executable()
		if err := os.Rename(tmp, exe); err != nil {
			c2 := exec.Command("sudo", "mv", tmp, exe)
			c2.Stdout = os.Stdout
			c2.Stderr = os.Stderr
			if err := c2.Run(); err != nil {
				os.Remove(tmp)
				return fmt.Errorf("failed to install update: %w", err)
			}
		}

		emit("Updated successfully.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
	updateCmd.Flags().BoolVar(&jsonMode, "json", false, "Output as NDJSON (machine-parseable)")
}
