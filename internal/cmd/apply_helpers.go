package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runShellStep executes a bash script and prints warnings on failure.
func runShellStep(name, script string) {
	cmd := exec.Command("bash", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  Warning: %s failed: %v\n", name, err)
	}
}

// findFrameworkLib locates the embedded or installed framework lib directory.
func findFrameworkLib() string {
	for _, dir := range []string{
		"/usr/local/lib/mainframe/lib",
		"/opt/mainframe/lib",
		filepath.Join(os.Getenv("HOME"), ".mainframe", "lib"),
		"lib",
	} {
		if _, err := os.Stat(filepath.Join(dir, "detect.sh")); err == nil {
			return dir
		}
	}
	return "lib"
}

// detectOSFamily detects the OS family from the framework lib.
func detectOSFamily(frameworkLib string) string {
	out, err := exec.Command("bash", "-c", fmt.Sprintf("source %s/detect.sh 2>/dev/null && detect_os >/dev/null 2>&1 && echo $OS_FAMILY", frameworkLib)).Output()
	if err != nil {
		out, _ = exec.Command("bash", "-c", "source lib/detect.sh 2>/dev/null && detect_os >/dev/null 2>&1 && echo $OS_FAMILY").Output()
	}
	family := strings.TrimSpace(string(out))
	if family == "" {
		if _, err := os.Stat("/etc/os-release"); err != nil {
			family = "darwin"
		} else {
			family = "debian"
		}
	}
	return family
}
